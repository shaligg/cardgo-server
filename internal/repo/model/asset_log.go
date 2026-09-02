package model

import "time"

// AssetLog 是资产流水表。
//
// 所有通过 asset 模块产生的金币和背包道具变更都应写入这里，方便查账和问题追踪。
// gameserver 只写不读；只有运营后台会按各种维度 SELECT。
//
// # 索引设计（MySQL）
//
// 原则：复合索引遵循"等值列在前、范围列(created_at)在后"，让 B-Tree range scan
// 精准命中；单列索引只留给"无等值前置条件"的场景。删掉所有被复合 leftmost prefix
// 等价覆盖的单列索引，把写放大压到最低。
//
// 三个复合索引对齐后台四类查询：
//   - idx_uid_created(uid, created_at)：
//     Q1  WHERE uid=? AND created_at BETWEEN ? AND ?
//     Q2  WHERE uid=? AND item_id=? AND created_at BETWEEN ? AND ?
//     （Q2 先走本索引砍到该玩家该时间段几十~几百行，item_id 在结果集里筛零成本）
//     同时覆盖 uid-only 查询（leftmost prefix = uid）
//   - idx_reason_created(reason, created_at)：
//     Q3  WHERE reason=? AND created_at BETWEEN ? AND ?
//     （leftmost prefix reason 等价替代单列 reason 索引）
//   - idx_itemid_created(item_id, created_at)：
//     Q4  WHERE item_id=? AND created_at BETWEEN ? AND ?
//     （leftmost prefix item_id 等价替代单列 item_id 索引）
//
// 两个单列索引：
//   - req_id：跨玩家全局反查某个 req_id（事故排查，无等值前置复合可覆盖）。
//   - created_at：不带任何等值条件的全服时间范围扫（例如某时段异常分析）。
//
// 删掉的冗余索引：
//   - idx_uid_reqid(uid, req_id)：真实运营查询必带 created_at 时间范围兜底，
//     idx_uid_created 先砍掉 99% 数据后在结果集里筛 req_id 零成本，死树。
//   - idx_uid_itemid_created(uid, item_id, created_at)：同理，idx_uid_created
//     先砍到几十~几百行，再筛 item_id 零成本，不值得多养一棵 3 列 B-Tree。
//   - reason / item_id 单列：被 idx_reason_created / idx_itemid_created 的
//     leftmost prefix 等价替代。
//
// 注意：故意不用 UNIQUE——
//  1. 一次奖励同一个 uid+req_id 合法产生多条流水，加 UNIQUE 会误伤；
//  2. 重试时 created_at 会变，DB UNIQUE 判不了真正的幂等；
//  3. 线上 MySQL 走 PARTITION BY created_at，UNIQUE 必须含分区列，
//     (uid,req_id,created_at) 判不重，等于摆设。
//     幂等靠协议层 CommandCache(uid + req_id)。严格 DB 级 (uid, req_id) 唯一
//     只能放弃分区表改走 pt-archiver，详见 docs/ops/dba_optimizations.md §1.8。
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
