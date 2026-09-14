# 正式账号域建设任务

## 1. 任务状态

- 状态：`TODO`
- 类型：后端技术架构任务
- 整体优先级：`P0（首次上线前）`
- 前置任务：独立 LoginServer 拆分（已完成）
- 实现范围：账号身份、登录态、刷新令牌及进入 GameServer 的接口边界
- 非目标：玩法登录奖励、角色创建引导、支付账号、实名认证和多个第三方平台同时接入

本文件只维护执行步骤、优先级、依赖和验收结果。最终协议、接口、配置和数据表字段统一写入 `backend_technical_architecture.md`，不得在本文件复制一套容易漂移的定义。

执行约束：

1. 一次只推进一个步骤，同一时间只允许一个步骤标记为 `DOING`。
2. 不顺带修改玩法、GameServer 业务协议、数据库分片或 Redis 拓扑。
3. 新增模块和重要函数使用中文注释，非必要不封装。
4. 不兼容当前未上线的 `account=uid` 正式调用方式。
5. 实现阶段完成后统一运行全量验证，不在每个小改动后反复执行 smoke。

## 2. 背景

当前 Demo 的 `POST /api/login` 直接把客户端提交的 `account` 当作 UID，并立即完成节点分配和 `enter_ticket` 签发。它只能用于本地联调，不能作为正式账号认证：

1. 客户端可以提交其他玩家的 UID。
2. LoginServer 没有验证密码或平台凭证。
3. 没有可续期、可撤销的账号登录态。
4. 外部身份和游戏内部稳定 UID 没有隔离。
5. 账号登录、断线恢复和进入 GameServer 混在一个接口中。

正式架构需要区分：

```text
账号登录态
  -> account_token / refresh_token
  -> 证明玩家是谁

GameServer 入场票
  -> enter_ticket
  -> 证明当前 UID 可以进入指定 GameServer
```

## 3. 新会话读取范围

开始实现前只读取：

1. `docs/README.md`
2. 本文件 `docs/tasks/account_domain.md`
3. `backend_technical_architecture.md` 的以下章节：
   - `5.1 login`
   - `6.0 账号登录态与 GameServer 入场票`
   - `6.1 登录接入协议`
   - `6.2 enter_ticket 签名契约`
   - `7.1 登录接入流程`
   - `9. 运行态与数据分层`
   - `17. 配置契约`
   - `18.1 Login`
   - `19.1 Login API`
   - `20.1 连接创建请求-返回`
   - `20.4 断线重连`
4. 当前实现：
   - `cmd/loginserver/main.go`
   - `internal/app/loginserver/`
   - `internal/platform/login/`
   - `internal/infra/db/`
   - `internal/repo/model/`

本任务不要求读取具体玩法策划文档。

## 4. 冻结边界

1. LoginServer 与 GameServer 继续独立运行。
2. 客户端从 LoginServer 获得目标地址和 `enter_ticket` 后直连 GameServer。
3. GameServer 始终只验证 `enter_ticket`，不处理密码、平台 Token 或 Refresh Token。
4. 内部 UID 由服务端生成，客户端和外部平台都不能指定。
5. `players.uid` 继续作为游戏数据稳定标识，不使用账号名、邮箱或平台用户标识代替。
6. LoginServer 可以增加 MySQL 依赖；GameServer 不访问账号凭证表。
7. 账号核心维护“外部身份到内部 UID”的映射，但首期只实现一个已确定的正式身份来源。
8. 外部账号平台已经完整提供稳定 UID、身份绑定和 Token 撤销时，只实现适配器，不重复建设本地密码体系。

## 5. 目标模块边界

本节只约束职责，最终目录和接口签名在 `AD-01` 写入技术架构文档。

- `platform/account`：账号查找、创建、状态、身份绑定和登录态生命周期。
- `platform/login`：使用已验证 UID 选择 GameServer 并签发 `enter_ticket`。
- `repo`：账号、身份映射和刷新会话持久化，不判断认证业务规则。
- `app/loginserver`：配置、MySQL/Redis组装、HTTP路由和生命周期。
- `GameServer`：保持现有票据验证、会话和玩法边界，不依赖账号域。

## 6. 执行总表

| ID | 任务 | 优先级 | 依赖 | 状态 |
|---|---|---:|---|---|
| `AD-01` | 冻结正式账号协议、Token规则、错误码、表结构和代码边界 | P0 | 无 | TODO |
| `AD-02` | 确定首个正式身份来源并实现身份验证器 | P0 | AD-01 | TODO |
| `AD-03` | 实现账号域 Model、Repository 和显式测试建表 | P0 | AD-01 | TODO |
| `AD-04` | 实现账号、身份映射、状态判断和内部 UID 生成 | P0 | AD-02、AD-03 | TODO |
| `AD-05` | 实现 Access Token、Refresh Token、轮换和撤销 | P0 | AD-03、AD-04 | TODO |
| `AD-06` | 拆分账号登录、登录态刷新和进入 GameServer 的 HTTP 链路 | P0 | AD-04、AD-05 | TODO |
| `AD-07` | LoginServer 接入 MySQL 配置和资源生命周期 | P0 | AD-03 | TODO |
| `AD-08` | 删除正式路径中的 `account -> uid` 直通信任 | P0 | AD-06、AD-07 | TODO |
| `AD-09` | 完成单元、MySQL集成和双进程端到端验收 | P1 | AD-01~AD-08 | TODO |
| `AD-10` | 同步架构、运维和客户端接入文档 | P1 | AD-09 | TODO |

## 7. 分步要求

### AD-01 冻结设计

只修改文档，不写业务代码：

1. 在 `backend_technical_architecture.md` 确定正式登录、刷新、进入和重连使用哪些 API，删除重复语义。
2. 明确 Access Token、Refresh Token 与 `enter_ticket` 的用途、有效期、轮换和撤销规则。
3. 明确内部 UID 生成规则以及外部身份唯一键。
4. 明确账号、身份映射、刷新会话的最终表结构、索引和数据归属。
5. 明确公开错误码、日志脱敏、受信任代理和限流要求。
6. 明确模块接口与目录，不为单一实现增加无意义适配层。

完成条件：技术架构文档中只有一套可直接指导实现的正式账号协议和数据结构。

### AD-02 身份验证

1. 根据正式发行渠道选择自建账号或一个外部身份平台。
2. 自建账号使用成熟密码哈希库，禁止保存明文或可逆密码。
3. 外部平台必须服务端校验凭证，不能信任客户端提交的 UID。
4. 登录失败不得泄露账号是否存在。

完成条件：有效凭证得到可信身份，伪造、过期和错误凭证均被拒绝。

### AD-03 数据访问

1. 按 `AD-01` 的最终结构增加账号域 Model 和 Repository。
2. 唯一键冲突转换为账号域可判断的错误。
3. 业务进程启动不得执行 DDL。
4. 测试数据库由测试入口显式准备表。

完成条件：账号、身份和刷新会话的持久化测试通过。

### AD-04 账号核心

1. 外部身份只能映射到一个内部账号。
2. 同一身份重复登录始终得到相同 UID。
3. 新 UID 只能由服务端生成。
4. 封禁或删除状态阻止继续登录和进入游戏。

完成条件：并发首次登录不会创建两个账号或两个 UID。

### AD-05 登录态

1. Access Token 为短期身份凭证。
2. Refresh Token 可撤销、只存哈希并在刷新时轮换。
3. 旧 Refresh Token、过期会话和已撤销会话不能再次使用。
4. 密码、平台凭证和完整 Token 不进入日志。

完成条件：签发、过期、刷新、轮换、撤销和封禁测试通过。

### AD-06 HTTP 主链路

1. HTTP Handler 只解析 DTO、调用 Service 和映射错误。
2. 账号认证不再隐式完成 GameServer 分配。
3. 进入或重连时先验证账号登录态，再调用现有 `NodeAllocator` 和 `TicketIssuer`。
4. `enter_ticket` 响应和 GameServer 首帧鉴权语义保持不变。

完成条件：客户端无需重新输入密码即可刷新登录态并重新取得入场票。

### AD-07 LoginServer 组装

1. LoginServer 独立创建账号数据库连接，凭证通过环境变量提供。
2. Bootstrap 失败释放已创建资源。
3. 正常停止关闭 HTTP、MySQL和Redis。
4. GameServer 配置和生命周期不增加账号数据库依赖。

完成条件：资源生命周期测试通过，任一关键依赖初始化失败时进程拒绝启动。

### AD-08 删除Demo信任路径

1. 正式 API 不再使用客户端 `account` 直接生成 UID。
2. 不保留未上线旧协议的兼容分支。
3. 本地 smoke 改为先取得正式账号登录态，再进入 GameServer。

完成条件：代码搜索不存在正式请求中 `uid := req.Account` 或同类直通逻辑。

### AD-09 验证

阶段完成后统一执行：

1. `go test ./...`
2. MySQL账号域集成测试。
3. LoginServer + GameServer 双进程端到端测试。
4. 登录失败、Token过期、刷新轮换、封禁、无可用节点和GameServer满载失败路径。

### AD-10 文档收口

1. 更新 `README.md` 和 `docs/ops/runbook.md` 的启动配置与请求示例。
2. 更新技术架构中的实现状态，不在总览或任务记录复制协议字段。
3. 将本任务总状态标记为 `DONE`，记录最终验证结果和提交号。

## 8. 最终验收标准

1. 客户端不能通过提交其他 UID 登录对应玩家。
2. 有效身份首次登录后获得服务端生成的稳定 UID。
3. 同一外部身份重复登录始终映射同一 UID。
4. 账号登录态与 GameServer 入场票已经分离。
5. 有效登录态可以取得入场票，过期、撤销或封禁状态不能取得。
6. Refresh Token 可以安全轮换，旧令牌不能重复使用。
7. GameServer 不增加账号数据库、密码或平台凭证依赖。
8. LoginServer 正确管理 MySQL、Redis及HTTP资源，且不自动执行 DDL。
9. 全量测试、MySQL集成测试和登录到WS的端到端测试全部通过。

## 9. 明确不做

- 不同时接入多个第三方平台。
- 不把 Refresh Token 存入 Redis 作为唯一权威状态。
- 不让 GameServer 直接验证账号密码或平台 Token。
- 不在本任务实现支付、实名认证、账号合并或角色转服。
- 不在本任务实施 MySQL 分库分表或 Redis Cluster。
