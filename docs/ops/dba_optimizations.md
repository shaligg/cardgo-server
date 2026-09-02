# DBA & Database Optimization Handbook

> MySQL 数据库运维、Schema 变更、性能优化的**总入口**。
> 所有 DDL、大表治理、慢查询治理、归档策略，都应先在本目录补齐方案再执行。
>
> 适用范围：`cardgo-server` 本地、测试和线上 MySQL。

---

## 0. DBA 通用规约

### 0.1 DDL 执行规范

- **线上任何 DDL**（加列、改索引、加约束、改主键、改表引擎）：禁止直接 `ALTER TABLE`，必须走在线改表工具：
  - 首选 [gh-ost](https://github.com/github/gh-ost)（GitHub 出品，无触发器、支持暂停/回退、主从延迟自动节流）
  - 次选 [pt-online-schema-change](https://docs.percona.com/percona-toolkit/pt-online-schema-change.html)（Percona Toolkit 套件，基于触发器）
- 选择低峰期（凌晨 02:00~05:00）执行，并提前确认主从延迟 < 200ms、磁盘剩余 > 2× 目标表体积。
- **分区表的 `ADD/DROP/EXCHANGE PARTITION` 除外**：纯元数据操作，毫秒级完成，可在业务期执行。

### 0.2 DML 批量操作规范

- **禁止不带 LIMIT 的 DELETE / UPDATE**。
- 批量变更模板：
  ```sql
  WHILE ROW_COUNT() > 0 DO
      DELETE FROM tbl WHERE cond ORDER BY id LIMIT 1000;
      DO SLEEP(0.05);
      -- 或者 COMMIT; 在事务外循环
  END WHILE;
  ```
- 或者直接用 `pt-archiver` / 应用层分块任务，带上 `--max-lag` / `--sleep-coef` 节流参数。

### 0.3 变更前自检清单

1. `EXPLAIN` / `EXPLAIN ANALYZE` 目标 SQL，确认走正确索引；
2. 在 **shadow / staging 实例**完整跑一遍 DDL/DML，记录耗时、锁等待、行数；
3. 留好回退 SQL（反向 ALTER、备份快照、binlog 回滚点）；
4. 登记在本文件对应章节（如「2. 索引治理」「3. 大表治理」）。

---

## 1. `asset_log` 资产流水：3 个月自动归档（分区表 + EXCHANGE PARTITION）

> 最后更新：2026-08-27
> 适用阶段：MySQL 生产上线前 / 迁移到 MySQL 时同步实施
> 目标：`asset_log` 热表只保留 **3 个月**数据，更早数据**在线搬迁**到同实例 `asset_log_archive` 归档表（仍可 SQL 查询），1 年+ 数据可进一步冷存到对象存储。

### 1.1 背景

- `asset_log` 是 append-only 资产审计流水，2000 玩家节点估算约 **6 万行 / 天**，1 年约 2200 万行（~8GB 含索引）。
- 长期不清理导致：热表臃肿、备份耗时、全表扫描变慢、`OPTIMIZE` 重建不可承受。
- 业务侧 99% 查询只看近 3 个月（客服、即时对账）；3 个月以上仅偶发审计、客诉排查。

### 1.2 表结构改造要点

#### 1.2.0 GORM model 当前已内置的部分

`internal/repo/model/asset_log.go` 已通过 GORM tag 建好：

```go
type AssetLog struct {
    ID        uint      `gorm:"primaryKey;autoIncrement"`
    UID       string    `gorm:"size:64;not null;index:idx_uid_created,priority:1"`
    ItemID    int64     `gorm:"not null;index:idx_itemid_created,priority:1"`
    Delta     int64     `gorm:"not null"`
    Balance   int64     `gorm:"not null"`
    Reason    string    `gorm:"size:64;not null;index:idx_reason_created,priority:1"`
    ReqID     string    `gorm:"size:128;not null;index"`
    CreatedAt time.Time `gorm:"index;index:idx_uid_created,priority:2;index:idx_reason_created,priority:2;index:idx_itemid_created,priority:2"`
}
```

索引策略（写放大优先，gameserver 只写不读，对齐后台四类查询）：

三个复合索引（等值列在前、范围列 created_at 在后）：
- `idx_uid_created(uid, created_at)`：**Q1** `WHERE uid=? AND created_at BETWEEN ?`；同时覆盖 **Q2** `WHERE uid=? AND item_id=? AND created_at BETWEEN ?`（先砍到该玩家该时间段几十~几百行，item_id 在结果集里筛零成本）；leftmost prefix 同时覆盖 uid-only 查询。
- `idx_reason_created(reason, created_at)`：**Q3** `WHERE reason=? AND created_at BETWEEN ?`；leftmost prefix 等价替代单列 reason 索引。
- `idx_itemid_created(item_id, created_at)`：**Q4** `WHERE item_id=? AND created_at BETWEEN ?`；leftmost prefix 等价替代单列 item_id 索引。

两个单列索引：
- `req_id`：跨玩家全局反查某个 req_id（事故排查，无等值前置复合可覆盖）。
- `created_at`：不带任何等值条件的全服时间范围扫（例如某时段异常分析）。

删掉的冗余索引：
- `idx_uid_reqid(uid, req_id)`：真实运营查询必带 created_at 时间范围兜底，`idx_uid_created` 先砍 99% 数据后在结果集里筛 req_id 零成本，这棵索引是死树。
- `idx_uid_itemid_created(uid, item_id, created_at)`：同理，`idx_uid_created` 先砍到几十~几百行，再筛 item_id 零成本，不值得多养一棵 3 列 B-Tree。
- 单列 `reason` / `item_id`：被对应复合索引的 leftmost prefix 等价替代。

#### 1.2.1 MySQL 专属改造：主键 + 分区（不进 GORM tag，独立 migration SQL）

MySQL 分区表硬性要求**分区列必须出现在主键里**。分区属于生产数据库运维结构，不写进 GORM model tag，必须在第一次部署时作为 migration 单独执行。

**Step 1**：正常跑 `AutoMigrate`（GORM 按单列 id PK 建表 + 建上面的所有索引/唯一约束）。

**Step 2**：立刻执行 DBA migration SQL（改复合主键 + 建分区）：

```sql
-- 必须在空表时执行（0 行），否则主键重建会锁表 + 很慢
ALTER TABLE asset_log
  DROP PRIMARY KEY,
  ADD PRIMARY KEY (id, created_at);

-- 初始分区：至少覆盖当前月 + 前后 2 个月
ALTER TABLE asset_log
PARTITION BY RANGE (TO_DAYS(created_at)) (
  PARTITION p202606 VALUES LESS THAN (TO_DAYS('2026-07-01')),
  PARTITION p202607 VALUES LESS THAN (TO_DAYS('2026-08-01')),
  PARTITION p202608 VALUES LESS THAN (TO_DAYS('2026-09-01')),
  PARTITION p202609 VALUES LESS THAN (TO_DAYS('2026-10-01')),
  PARTITION p202610 VALUES LESS THAN (TO_DAYS('2026-11-01'))
);

-- 可选：确认分区裁剪生效
-- EXPLAIN PARTITIONS SELECT COUNT(*) FROM asset_log WHERE created_at = '2026-08-15';
-- 结果里 partitions 列应只显示 p202608 一个分区
```

> 如果表已经有历史数据（线上从非分区表迁到分区表），必须用 **gh-ost** 做在线重建，不能直接 ALTER：
> `gh-ost --alter="DROP PRIMARY KEY, ADD PRIMARY KEY(id, created_at), REMOVE PARTITIONING, PARTITION BY RANGE(...)"`
> （gh-ost 的 PARTITION BY 支持在新版本可用，不支持时就先 gh-ost 改复合主键，再手动 `ALTER TABLE PARTITION BY RANGE(...)` —— 此时 ALTER 会拷贝数据，但分区 DDL 执行期间不阻塞 DML，除了末尾元数据切换的短暂锁。）

#### 1.2.2 (uid, req_id) 幂等判重的现状与妥协

- **分区表上无法实现真正的 DB 级 `UNIQUE(uid, req_id)`**：MySQL 强制要求"分区表的所有 UNIQUE KEY 必须包含分区列"，而只要把 `created_at` 加进来：
  - 重试时 `created_at` 必然不同 → **判重失效**；
  - 一次多道具奖励（金币+N 件道具）共享 uid+req_id，在 DATETIME 秒级精度下同秒 → **误伤正常事务，回滚正常发奖**。
- 所以 model 里**没有任何 UNIQUE 索引**，只建普通复合索引对齐后台查询（详见 §1.2.0 的 4 个复合 + 2 个单列索引清单）。幂等判重不靠 DB 约束，靠下面三层链。
- 真正的幂等保证链（仍完整有效）：
  1. **前端策略**：切节点 / 换连接后不再复用历史 req_id；
  2. **内存 CommandCache**：uid + req_id 最近 120s 内命中即返回缓存结果，参数 hash 不同直接抛 REQUEST_ID_CONFLICT；
  3. **写入层事务原子性**：资产变更 + asset_log INSERT 同事务，成功就都写，失败就全回滚（即使重放，也得穿透前两层才会到这里，实际已不可能）。
- **如果将来要求严格 DB 级 `(uid, req_id)` 唯一**（不接受内存兜底）：只能放弃分区表方案，改用 §1.8「不分区 + `pt-archiver` 分块搬迁」—— 普通表没有分区列限制，可以直接建 `UNIQUE(uid, req_id)` 数据库强判重。

### 1.3 归档表 `asset_log_archive` 初始化

归档表仍设计为**分区表**（和热表一样按月分区），好处是：
- 查询时冷热两边都能按分区裁剪；
- 1 年以上冷存时可以按分区粒度 `DISCARD/IMPORT TABLESPACE` 物理卸/装载。

但注意：**MySQL `ALTER TABLE ... EXCHANGE PARTITION ... WITH TABLE tbl` 要求 WITH 后的目标表必须是「非分区普通表」**，不能直接用分区↔分区或分区↔分区交换。因此归档流程采用：
**`热分区 ⇄ 临时普通表（EXCHANGE，毫秒级） → 再 INSERT 进归档表对应分区`**。

```sql
-- Step 1: 先和热表一样建归档分区表（空结构就行，数据是后来 INSERT 进来的）
CREATE TABLE asset_log_archive LIKE asset_log;
-- LIKE 不会复制分区定义；去掉热表刚通过 migration 加的分区再重建（避免 REMOVE PARTITIONING 报错）
ALTER TABLE asset_log_archive REMOVE PARTITIONING;
ALTER TABLE asset_log_archive
PARTITION BY RANGE (TO_DAYS(created_at)) (
  PARTITION p202606 VALUES LESS THAN (TO_DAYS('2026-07-01')),
  PARTITION p202607 VALUES LESS THAN (TO_DAYS('2026-08-01')),
  PARTITION p202608 VALUES LESS THAN (TO_DAYS('2026-09-01')),
  PARTITION p202609 VALUES LESS THAN (TO_DAYS('2026-10-01')),
  PARTITION p202610 VALUES LESS THAN (TO_DAYS('2026-11-01'))
);
```

### 1.4 VIEW：冷热表透明查询

```sql
CREATE OR REPLACE VIEW asset_log_all AS
SELECT * FROM asset_log
UNION ALL
SELECT * FROM asset_log_archive;
```

- 客服、运营、对账脚本一律改查 `asset_log_all`，完全不感知归档；
- 已明确查 3 个月内的业务代码，继续查 `asset_log` 热表享受裁剪优势。

### 1.5 月度自动化维护（每月 1 号 03:00 cron）

建议用 MySQL Event Scheduler（开 `event_scheduler=ON`）或独立运维 cron 执行以下脚本。核心归档步骤分两段：

1. **EXCHANGE（毫秒级）**：把热表过期分区 ↔ 一张临时**非分区**普通表交换。此时：热表对应分区瞬间变空，对线上零影响；所有行搬到临时普通表。
2. **INSERT SELECT（后台批量）**：把临时普通表的行 insert 进归档表对应分区，然后 DROP 临时表。

```sql
-- ============================================================
-- monthly_asset_log_maintenance.sql
-- 执行频率：每月 1 号 03:00
-- 作用：1) 提前建未来 2 个月的分区（防止越界报错）
--       2) 热分区 EXCHANGE → 临时非分区表 → INSERT 进归档对应分区
--       3) 可选：1 年前的归档分区 .ibd 导出对象存储冷存
-- ============================================================

DELIMITER $$

CREATE EVENT IF NOT EXISTS evt_asset_log_monthly_maintenance
ON SCHEDULE EVERY 1 MONTH
    STARTS DATE_ADD(DATE_ADD(CURDATE(), INTERVAL 1 DAY), INTERVAL 3 HOUR)  -- 下月 1 号 03:00
COMMENT 'Monthly asset_log partition rotate + 3-month archive via EXCHANGE-then-INSERT'
DO
BEGIN
    -- ---------- Step 0: 生成当前月标识 ----------
    SET @expire_month := DATE_FORMAT(DATE_ADD(CURDATE(), INTERVAL -3 MONTH), '%Y%m');
    SET @expire_lower := DATE_FORMAT(DATE_ADD(CURDATE(), INTERVAL -3 MONTH), '%Y-%m-01');

    -- ---------- Step 1: 预创建未来 2 个月分区（热表 + 归档表） ----------
    -- 本月 +1
    SET @next1 := DATE_FORMAT(DATE_ADD(CURDATE(), INTERVAL 1 MONTH), '%Y%m');
    SET @next1_bound := DATE_FORMAT(DATE_ADD(DATE_ADD(CURDATE(), INTERVAL 1 MONTH), INTERVAL 1 MONTH), '%Y-%m-01');
    SET @sql := CONCAT('ALTER TABLE asset_log         ADD PARTITION (PARTITION p', @next1, ' VALUES LESS THAN (TO_DAYS(''', @next1_bound, ''')))');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
    SET @sql := CONCAT('ALTER TABLE asset_log_archive ADD PARTITION (PARTITION p', @next1, ' VALUES LESS THAN (TO_DAYS(''', @next1_bound, ''')))');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

    -- 本月 +2
    SET @next2 := DATE_FORMAT(DATE_ADD(CURDATE(), INTERVAL 2 MONTH), '%Y%m');
    SET @next2_bound := DATE_FORMAT(DATE_ADD(DATE_ADD(CURDATE(), INTERVAL 2 MONTH), INTERVAL 1 MONTH), '%Y-%m-01');
    SET @sql := CONCAT('ALTER TABLE asset_log         ADD PARTITION (PARTITION p', @next2, ' VALUES LESS THAN (TO_DAYS(''', @next2_bound, ''')))');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;
    SET @sql := CONCAT('ALTER TABLE asset_log_archive ADD PARTITION (PARTITION p', @next2, ' VALUES LESS THAN (TO_DAYS(''', @next2_bound, ''')))');
    PREPARE stmt FROM @sql; EXECUTE stmt; DEALLOCATE PREPARE stmt;

    -- ---------- Step 2: 关键：EXCHANGE PARTITION + 临时非分区表 ----------
    -- 2.1 校验归档目标分区为空（避免 INSERT SELECT 重复）
    SET @check_arch := CONCAT('SELECT COUNT(*) INTO @arch_cnt FROM asset_log_archive PARTITION(p', @expire_month, ')');
    PREPARE stmt FROM @check_arch; EXECUTE stmt; DEALLOCATE PREPARE stmt;
    IF @arch_cnt <> 0 THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Archive target partition not empty — possible re-run / leftover data. Manual check required.';
    END IF;

    -- 2.2 创建临时「非分区普通表」，结构 + 索引 + 主键必须和热分区完全一致
    --     这里故意用 CREATE TABLE ... LIKE asset_log 然后 REMOVE PARTITIONING：
    --     得到的是和 asset_log 列定义/索引/主键完全相同但不带分区的普通空表。
    SET @tmp_name := CONCAT('game_db.asset_log_exch_tmp_p', @expire_month);
    SET @drop_tmp := CONCAT('DROP TEMPORARY TABLE IF EXISTS ', @tmp_name);   -- defense
    PREPARE stmt FROM @drop_tmp; EXECUTE stmt; DEALLOCATE PREPARE stmt;
    -- 注意：这里必须用 PERMANENT TABLE，TEMPORARY 不能做 EXCHANGE PARTITION
    SET @create_tmp := CONCAT(
        'CREATE TABLE ', @tmp_name, ' LIKE asset_log; ',
        'ALTER TABLE ',  @tmp_name, ' REMOVE PARTITIONING;'
    );
    PREPARE stmt FROM @create_tmp; EXECUTE stmt; DEALLOCATE PREPARE stmt;

    -- 2.3 防御式：临时表必须为空（刚创建就应该是空，防止误操作同名表）
    SET @check_tmp := CONCAT('SELECT COUNT(*) INTO @tmp_cnt FROM ', @tmp_name);
    PREPARE stmt FROM @check_tmp; EXECUTE stmt; DEALLOCATE PREPARE stmt;
    IF @tmp_cnt <> 0 THEN
        SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT = 'Exchange temp table is not empty. Abort to avoid data loss.';
    END IF;

    -- 2.4 ✅ EXCHANGE — 纯元数据操作，毫秒级。
    --     WITH TABLE 后的是普通非分区表，符合 MySQL 官方语法要求。
    --     VALIDATION=1（默认）会校验行是否满足目标分区边界条件并在失败时报错，适合首次上线确认。
    SET @exchange := CONCAT(
        'ALTER TABLE asset_log EXCHANGE PARTITION p', @expire_month,
        ' WITH TABLE ', @tmp_name, ' WITH VALIDATION'
    );
    PREPARE stmt FROM @exchange; EXECUTE stmt; DEALLOCATE PREPARE stmt;

    -- ---------- Step 3: 把临时非分区表的数据写入归档对应分区 ----------
    -- 这里是真正拷贝数据（≈ 百万级 / 几十秒），但此时数据已离线表，对线上零影响。
    -- 归档表对应分区之前在 Step 0 / Step 1 已经提前建好，且 Step 2.1 校验为空。
    SET @insert := CONCAT(
        'INSERT INTO asset_log_archive PARTITION(p', @expire_month, ') ',
        'SELECT * FROM ', @tmp_name
    );
    PREPARE stmt FROM @insert; EXECUTE stmt; DEALLOCATE PREPARE stmt;

    -- ---------- Step 4: 行数一致性校验（EXCHANGE 前热表行数 == 临时表 == 归档新分区） ----------
    -- （实际工程上通常在 Step 2 前记录热分区行数，这里和归档对比；简单版本先 SELECT COUNT 归档分区即可）
    SET @verify := CONCAT('SELECT COUNT(*) INTO @final_cnt FROM asset_log_archive PARTITION(p', @expire_month, ')');
    PREPARE stmt FROM @verify; EXECUTE stmt; DEALLOCATE PREPARE stmt;

    -- ---------- Step 5: 清理临时表 ----------
    SET @cleanup := CONCAT('DROP TABLE ', @tmp_name);
    PREPARE stmt FROM @cleanup; EXECUTE stmt; DEALLOCATE PREPARE stmt;

    -- ---------- Step 6 (可选): 1 年前的归档分区冷存到对象存储 ----------
    -- 见 §1.6 冷存流程（DISCARD TABLESPACE → 上传 S3/OSS → 删除本地 .ibd）

END$$

DELIMITER ;

-- 确认事件调度器开启（my.cnf 里也要配上 event_scheduler=ON 持久化）
SET GLOBAL event_scheduler = ON;
```

**月度维护脚本的关键保证**：

- `EXCHANGE PARTITION` 是纯元数据操作，**毫秒级完成**，不持有行锁、不复制行、不写 redo/undo，对线上零影响；
- EXCHANGE 的 WITH TABLE 目标是「非分区普通表」，完全符合 MySQL 官方语法要求；
- 两次空表校验（归档分区 & 临时表）+ 不一致时 `SIGNAL` 告警，避免重复执行或误操作导致数据覆盖；
- INSERT SELECT 发生在**已经离开热表**之后，查询压力全部指向归档表/临时表，对线上资产写入无任何影响（唯一共享的是 buffer pool，低峰期执行几乎无感）。

### 1.6（可选）1 年以上分区的冷存（节省 MySQL 磁盘）

`asset_log_archive` 里满 1 年仍占 MySQL 磁盘且极少被查时，把分区 `.ibd` 文件导出到对象存储：

```sql
-- Step A: 目标分区 .ibd 解绑（保留 .frm/.cfg 元数据）
ALTER TABLE asset_log_archive PARTITION p202506 DISCARD TABLESPACE;

-- Step B: 操作系 shell 把 datadir/game_db/asset_log_archive#P#p202506.ibd 上传 S3/OSS
--   例：aws s3 cp ... s3://game-cardgo-dba-cold/asset_log/p202506.ibd
--   上传完成后本地删除 .ibd 文件

-- --------------------------------------------------------------
-- 将来要查回这批数据时：
--   Step R1: 从 S3 下载 .ibd 到 datadir/game_db/ 下（目录权限 mysql:mysql）
--   Step R2: ALTER TABLE asset_log_archive PARTITION p202506 IMPORT TABLESPACE;
--   Step R3: 正常 SELECT
--   Step R4 (用完还回去): 再次 DISCARD + 删除本地 .ibd
```

### 1.7 回退方案

如果分区表现场出问题（BUG、业务不满足、运维不接受），无损回退到"非分区普通表 + pt-archiver"：

```sql
-- gh-ost 在线去分区：gh-ost 内部会读全量数据重建一张非分区表，再切表名
gh-ost \
  --host=prod-mysql --database=game_db --table=asset_log \
  --alter="REMOVE PARTITIONING" \
  --execute
-- asset_log_archive 同理
```

耗时 ~ 表大小复制时间（千万级 ~30 分钟），对线上无锁影响。

### 1.8 备选方案：不分区 + `pt-archiver`

如果不接受「(uid, req_id) 跨月不唯一」或不便改主键结构：

```bash
# 每月 1 号跑一次，分块把 3 个月前数据搬到归档表
pt-archiver \
  --source h=prod-mysql,D=game_db,t=asset_log,u=game \
  --dest   h=prod-mysql,D=game_db,t=asset_log_archive,u=game \
  --where "created_at < NOW() - INTERVAL 3 MONTH" \
  --limit 1000 --commit-each --sleep-coef 0.05 \
  --max-lag 1 --statistics \
  --progress 10000
```

- 优点：不改表结构；(uid, req_id) 唯一约束可以原样建在非分区表上；
- 缺点：真·行级复制（千万级几小时），删完热表有 InnoDB 碎片，需事后 `gh-ost --alter="engine=InnoDB"` 重建回收空间。

### 1.9 验收清单（上线 / 变更后必查）

1. **分区裁剪**：`EXPLAIN PARTITIONS SELECT * FROM asset_log WHERE created_at = '2026-08-15';` 只扫 `p202608` 一个分区。
2. **月度事件完整走一轮（建议手动触发验证）**：
   ```sql
   -- 手工：选一个测试分区走一次 EXCHANGE-then-INSERT 全流程
   CALL monthly_exchange_archive_month('202606');   -- 或把 event body 在一个测试会话里顺序执行一遍
   ```
   检查项：
   - 热表对应分区 `TABLE_ROWS = 0`；
   - 临时非分区表已不存在（事件末尾应 DROP）；
   - 归档表对应分区 `TABLE_ROWS` = 迁移前热表行数；
   - `SELECT COUNT(*) FROM asset_log_all WHERE created_at < '2026-07-01'` 数字与迁移前一致。
3. **结构校验**：`asset_log_exch_tmp_p*` 相关临时表不会残留（异常时应 SIGNAL 让 DBA 手动处理并保留现场）。
4. **VIEW 权限**：运营/客服账号授予 `asset_log_all` 的 SELECT 权限。
5. **监控**：分区总量、归档表大小、事件执行日志（`information_schema.EVENTS.STATUS/LAST_EXECUTED` + error log）。

---

## 2. [待补] 慢查询 Top 10 治理

- 保留章节，后续把线上慢查询日志、`sys.statement_analysis` TOP SQL 优化记录沉淀于此。
- 模板：
  - SQL 指纹
  - EXPLAIN 前后对比
  - 索引变更 / 查询改写
  - QPS / Rows Examined / Latency 前后对比
  - 影响范围评估

---

## 3. [待补] 索引治理

- 保留章节。后续包含：
  - 未使用索引审计（`sys.schema_unused_indexes`），定期清理；
  - 重复 / 冗余索引清理；
  - 新增索引必须附慢查询依据 + gh-ost 工单；
  - 复合索引最左前缀原则核查。

---

## 4. MySQL 生产部署清单

当前代码、配置和测试数据库口径统一为 MySQL，不再保留其他数据库驱动。数据库必须由部署系统预先创建，GameServer 只负责连接和迁移当前 MVP 表。

### 4.1 首次部署的 5 步必做

```bash
# Step 0: 通过密钥系统设置环境变量，不把账号密码写入 YAML
export GAME_DB_DSN='game:password@tcp(mysql-host:3306)/game_db?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s&readTimeout=3s&writeTimeout=3s'

# Step 1: 启动 gameserver，AutoMigrate 建表（此时 asset_log 是单列 PK，已建好 3 个复合索引 idx_uid_created/idx_reason_created/idx_itemid_created + 2 个单列索引 req_id/created_at）
#    必须保证新实例无玩家请求接入（LB 先不挂），否则 ALTER TABLE 改主键时锁表。

# Step 2: 对空表执行 MySQL 专属 migration（见 §1.2.1）：
#   2a) ALTER TABLE asset_log DROP PRIMARY KEY, ADD PRIMARY KEY(id, created_at);
#   2b) ALTER TABLE asset_log PARTITION BY RANGE(TO_DAYS(created_at)) (...初始分区...);
#   2c) 建 asset_log_archive（§1.3）+ asset_log_all VIEW（§1.4）;
#   2d) 注册 evt_asset_log_monthly_maintenance（§1.5，event_scheduler=ON）。

# Step 3: 核对 configs/config.prod.yaml 的连接池配置
#   max_open_conns: 100
#   max_idle_conns: 30
#   conn_max_lifetime_sec: 3600
#   conn_max_idle_time_sec: 600
#   多 GameServer 节点共享数据库时，总连接数必须小于 MySQL max_connections 的安全预算。

# Step 4: 验证
#   SELECT @@event_scheduler;                        —— ON
#   SHOW CREATE TABLE asset_log\G                     —— Partition by range 存在, PK(id, created_at)
#   EXPLAIN PARTITIONS SELECT ... WHERE created_at = —— 只扫目标分区
#   SHOW INDEX FROM asset_log                       —— 索引与文档一致

# Step 5: LB 接入流量，走 smoke test（完整登录→开卡→升级→结算→流水行数核对）。
```

### 4.2 时区与时间类型注意事项

- **DSN 必须加 `parseTime=True&loc=Local`**：让 GORM 把 MySQL DATETIME 解析成 Go `time.Time`，并使用服务器时区。
- **线上 MySQL 全局 `time_zone = '+08:00'`**（或 UTC，和应用服务器一致即可；项目里 CreatedAt 展示用本地时间，对账时要统一基准）。
- **避免 `TIMESTAMP` 类型**（2038 上限、时区自动转换歧义）。全部用 `DATETIME(6)` 存带毫秒的墙钟时间；当前 GORM `time.Time` 默认映射 DATETIME，无需额外指定。
- **NOW(6) / CURRENT_TIMESTAMP(6)** 存储过程和 DDL 里统一毫秒精度，不要混用 `NOW()` 与 `SYSDATE()`。

### 4.3 Redis 玩家归属与 MySQL 相互独立

`player_owner_store`（Redis 存 uid → server_id 归属）不进入 MySQL。`db.Players.GetByUID` 仍然在当前节点本地事务执行，`node_registry` / `player_owner_store` / `allocator` 不参与业务表读写。

### 4.4 压测验证项

- **资产写入吞吐**：单节点 2k 并发玩家、战斗结算/升级/离线三种混合流量下记录 asset_log INSERT TPS 和 P95/P99。
- **主键+分区 0 行 ALTER 耗时**：Step 2 的 DDL 必须 < 1s（空表理应毫秒级）。
- **跨表事务原子性**：在扣资产和发奖之间注入错误，验证事务完整回滚，不出现部分提交。
- **慢查询**：上线 7 天内，把 `sys.statement_analysis` 的 Top10 同步写入 §2。

---

## 5. [待补] 备份与恢复

- 保留章节。
- XtraBackup 全量 + 增量计划、RPO / RTO 目标、binlog 保留期、跨地域灾备、定期恢复演练记录。
