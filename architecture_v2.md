# 项目后端架构总览（V2 / 卡牌休闲 MVP）

## 1. 文档定位

本文档是项目级后端架构总览。

它回答：

- 当前项目后端整体采用什么形态。
- MVP 要实现哪些后端模块。
- 哪些完整业务后置，哪些代码边界从 MVP 起建立。
- 模块之间的边界如何划分。
- 当前 LoginServer/GameServer 独立进程如何扩展节点或继续拆分公共模块。

更细的技术实现放在：

- [backend_technical_architecture.md](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md)

已有基础架构落地记录放在：

- [architecture_v2_task_breakdown.md](/Users/bigfish/Project/go_orm_1/architecture_v2_task_breakdown.md)

## 2. 设计输入

策划与范围文档：

- [总设计文档](/Users/bigfish/Project/go_orm_1/docs/design/card_casual_game_design.md)
- [MVP 范围与里程碑](/Users/bigfish/Project/go_orm_1/docs/design/mvp_scope.md)
- [卡牌系统设计](/Users/bigfish/Project/go_orm_1/docs/design/card_system_design.md)
- [订单与关卡系统设计](/Users/bigfish/Project/go_orm_1/docs/design/order_level_design.md)
- [背包与资产系统设计](/Users/bigfish/Project/go_orm_1/docs/design/inventory_asset_design.md)
- [工坊系统设计](/Users/bigfish/Project/go_orm_1/docs/design/workshop_system_design.md)
- [经济系统设计](/Users/bigfish/Project/go_orm_1/docs/design/economy_design.md)

技术细节文档：

- [后端技术架构设计](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md)
- [基础架构落地记录](/Users/bigfish/Project/go_orm_1/architecture_v2_task_breakdown.md)

## 3. 架构结论

当前 MVP 采用：

```text
独立 LoginServer 进程
  + 一个或多个 GameServer 进程
  + 多 goroutine
  + 模块化单体
  + 按未来多进程/服务拆分边界编码
```

LoginServer 与 GameServer 同仓库、独立构建和启动。单节点与多节点使用同一套登录分配链路，GameServer 业务仍采用模块化单体。

目标是：

- 先用 1 个游戏服进程承载单服 2000 在线。
- LoginServer 独立提供登录、Redis 节点分配和 ticket 签发。
- 新增 GameServer 只需唯一节点配置和新进程，多个 GameServer 共享一个 LoginServer。
- 业务模块按服务边界写，后续可平滑拆分。
- 战斗、房间等高频局内状态可以保存在本机内存。
- 玩家权威数据必须在 DB。
- Redis 当前用于 GameServer 节点注册、玩家节点归属和少量跨节点控制通知；ticket nonce 与连接 Session 暂存在各 GameServer 进程内存。

## 4. 运行形态

MVP 运行形态：

```text
LoginServer 进程
  - HTTP Login API
  - Redis 节点表与最近归属读取
  - NodeAllocator / TicketIssuer
  - 基础 health

GameServer 进程
  - WebSocket Gateway
  - Auth / Session
  - 管理 HTTP（健康、指标、drain、会话管理）
  - Dispatcher
  - Game Services
  - globalcore domain core
  - globalserver same-process jobs/service process boundary
  - State Manager
  - Repository / MySQL
  - Infra

Redis
DB
```

进程边界：

- LoginServer 处理账号注册、登录态、进入分配和自身存活检查；独立连接账号 MySQL，读取 Redis 节点与最近归属，不注册为游戏节点或创建游戏玩家。
- GameServer 负责验票、玩家初始化、会话和归属认领、玩法及持久化；管理 HTTP 不承载登录 API。
- 两个进程分别持有配置和资源，共享票据签验契约及 Redis 节点/归属数据，不通过 LoginServer 转发游戏消息。
- GameServer 增减节点通过 Redis 注册表被发现，无需重启 LoginServer。停止 LoginServer 会阻断新登录和重连换票，但不主动断开已有 GameServer 连接。

Go 运行模型：

```text
LoginServer 进程
  - 登录 HTTP goroutine
  - Redis 客户端

每个 GameServer 进程
  - 管理 HTTP goroutine
  - WebSocket accept goroutine
  - 每连接读 goroutine
  - 每连接写 goroutine
  - 请求 goroutine 内执行 dispatcher 分片锁与业务逻辑
  - state maintainer goroutine
  - GameServer 节点心跳 goroutine
```

## 5. MVP 必做模块

本文档保留 MVP 后端模块概述，不维护目录、接口、表结构等实现细节。

MVP 主链路：

```text
登录接入
  -> 建号/玩家资料
  -> 资产/背包/卡牌/卡组
  -> 订单/关卡/局内逻辑
  -> 结算奖励
  -> 卡牌成长/工坊成长
```

细节归属：

- 具体后端模块、目录、迁移状态，以 [backend_technical_architecture.md](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md) 的“所有模块列表”为准。
- Prototype/MVP 功能范围、数量、验收，以 [docs/design/mvp_scope.md](/Users/bigfish/Project/go_orm_1/docs/design/mvp_scope.md) 为准。

MVP 概述模块：

| 模块 | MVP 职责概述 |
|---|---|
| 登录入口（LoginServer） | 自建账号、登录态、节点分配和票据签发 |
| 实时接入（GameServer） | 票据校验、玩家准备、会话建立和归属认领 |
| 玩家资料 | 建号、基础资料、等级、章节进度 |
| 资产 | 金币、钻石、体力、声望、材料、碎片、统一发奖扣费 |
| 背包 | 普通道具、材料、消耗券、宝箱等长期物品 |
| 卡牌 | 卡牌库存、卡牌升级、卡牌碎片或同名卡消耗 |
| 卡组 | 卡组编辑、保存、合法性校验 |
| 订单/关卡 | 订单配置、关卡目标、订单完成判定、关卡结算 |
| 局内逻辑 | 出牌、回合、局内资源、局内订单进度、结算前状态 |
| 工坊 | 设施、升级、离线收益、简化装饰槽位 |
| 经济配置 | 奖励、消耗、资源价值、产消配置辅助 |

## 6. MVP 占位模块

以下模块在总设计中必须有位置，但不进入 MVP 的独立进程部署：

公共能力分两种，并在目录上明确拆开：

| 类型 | 含义 | 当前部署 | 未来演进 |
|---|---|---|---|
| `globalcore` 公共领域核心 | 公共域接口、DTO、核心规则、Local 实现和 RemoteClient，可被 GameServer 与 GlobalServer 复用 | 同进程 | 保持模块化，必要时替换为远程 client 或被独立公共服复用 |
| `globalserver` 公共服编排模块 | 周期结算、批处理、跨服聚合、公共服进程入口和 Job 编排 | MVP 写代码但同进程直调，无独立启动/无网络层 | 按压力点拆成独立进程或独立 job |

MVP 主请求链路先做 `globalcore` 本地实现；`globalserver` 先建立最小代码边界，不独立启动。

```text
globalcore/*
  公共领域核心
  包含接口、DTO、核心规则、Local 实现、RemoteClient
  当前与 GameServer 同进程
  未来可被独立 GlobalServer 复用

globalserver/*
  公共服/全局 job 编排层
  调用 globalcore 完成领域计算
  自己负责扫描、幂等、落库、重试、批处理
  初版不独立启动，不做数据传输层
  由 GameServer 或管理入口同进程直接调用
```

占位模块概述：

| 模块 | 定位 | MVP 状态 |
|---|---|---|
| 好友 | 轻社交关系链、申请、互助入口 | 基础关系闭环已实现，互助后置 |
| 聊天 | 世界/系统/公会频道能力 | 世界/公会发送与历史已实现，系统频道和实时推送后置 |
| 公会 | 成员、职位、签到、捐献、协作玩法基础 | 基础成员闭环已实现，成长玩法后置 |
| 邮件 | 系统邮件、奖励补发、离线通知 | 可保留批量/补发边界 |
| 排行榜 | 无尽订单、活动榜、赛季奖励 | 可保留结算边界 |
| 公告 | 系统公告、运营通知、活动提示 | 占位，完整业务后置 |

技术细节以 [backend_technical_architecture.md](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md) 的“可迁移模块列表”和“所有模块列表”为准。

原则：

```text
domain/* 和 gameplay/* 不拥有好友、聊天、公会、邮件、排行榜的核心状态和核心规则。
gameplay/* 可以通过接口调用 domain/* 和 globalcore/*；domain/* 不反向依赖 gameplay/*。
globalcore/* 是公共领域核心，不等于独立公共服务进程，也不只是 client。
globalserver/* 是公共服编排层，MVP 就可以有代码，但不独立启动、不做 RPC/HTTP 等数据传输层。
```

判断规则：

- 如果是玩家请求链路内的公共数据读写，先通过 `globalcore/*` 接口调用。
- 如果是公共领域核心规则，例如排行奖励分段、奖励生成、聊天消息校验、公会权限规则，放 `globalcore/*`。
- 如果是全局周期结算、批量发奖、跨服聚合、赛季清算，放 `globalserver/*` 编排。
- 如果 `globalcore/*` 的请求期逻辑未来需要跨多个 GameServer 实时统一状态、独立扩容、故障隔离或独立 SLA，再将 Local 实现替换为 RemoteClient。
- gameplay 可以调用 globalcore 接口，但不能直接操作 globalcore 的内部表、map、Redis key 或 ZSET。
- 初版 `globalserver/*` 由 GameServer 同进程直调；未来拆分时再补 `cmd/globalserver` 和传输层。
- `globalserver/*` 可以复用 `globalcore/*` 规则和 `domain/asset` 发奖接口，但不能依赖连接、session 或 GameServer 私有运行态。
- 只有需要或未来可能迁移的模块才按可远程化方式实现，不把所有本地业务强行套成 service/client/adapter。
- 强依赖连接、在线内存、局内状态、单玩家高频轻逻辑的业务，优先保持 GameServer 本地内聚。
- 可迁移模块以技术文档中的“可迁移模块列表”为准；列表外默认简单本地实现。

模块清单说明：

- 本文档允许保留模块概述，帮助阅读。
- 详细目录、迁移白名单、接口、协议和数据表只维护在 [backend_technical_architecture.md](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md)。
- 后续如果概述和技术文档冲突，以技术文档为准，并同步修正本文档。

例子：

```text
活动 A 玩家完成一局并提交排行榜分数：
  gameplay/activity_a
    -> globalcore/rank.UpdateScore(board_id, uid, score, req_id)
    -> globalcore/rank 内部可以本地执行 Redis ZADD / DB 记录

活动 A 排行榜赛季结算与奖励发放：
  globalserver/rank 或 globalserver/activity_a
    -> 扫描榜单
    -> 调 globalcore/rank 生成排名奖励
    -> 调 AssetService/MailService 发奖或生成待领取记录
```

## 7. 核心分层

以下为 GameServer 业务链路；LoginServer 的账号链路访问专用 Repository 和 MySQL，分配/签票不进入玩法层。

```text
Gateway / Transport
  -> Handler / BizRouter
  -> Gameplay Service（具体玩法编排，可选）
  -> Domain Service（玩家、资产、背包等通用业务能力）
  -> Repository
  -> Model
  -> DB
```

规则：

- `Gateway` 处理连接、协议、心跳、限流、背压。
- `Handler` 只做协议参数解析和调用 Service。
- `Gameplay Service` 承载具体玩法规则与跨领域编排，可以调用一个或多个 `Domain Service`。
- `Domain Service` 承载玩家、资产、背包等可被多个玩法复用的基础业务能力，不能反向依赖具体玩法。
- 简单的资料、资产或背包协议允许 `Handler -> Domain Service`，不强制增加空的 Gameplay Service。
- `Repository` 只做数据库访问，按业务聚合或事务边界组织，不按数据库表机械拆分。
- `Store` 专指进程内存或 Redis 运行状态；需要持久化数据时由 Store 调用 Repository，Repository 不感知 Store。
- `Model` 是持久化模型，不直接暴露给客户端。

## 8. 数据分层

```text
GameServer 本机内存
  - 当前连接与会话状态
  - 局内临时状态
  - 近期请求结果

Redis 共享状态
  - GameServer 节点注册
  - 玩家节点归属
  - 未来排行榜 ZSET 等公共数据

DB 持久化数据
  - 玩家权威数据
  - 资产
  - 卡牌
  - 背包
  - 工坊
  - 关卡进度
  - 流水
```

权威规则：

- 玩家资产、卡牌、背包、工坊、关卡进度以 DB 为准。
- GameServer 内存保存连接会话、局内临时状态和近期请求结果。
- 跨服重连不恢复旧服内存态，只从 DB 重建长期状态。

## 9. 登录与重连原则

登录分配由独立 LoginServer 内的 `Login / NodeAllocator` 决定。

账号域支持自建账号及设备游客：统一 register 建号，login 认证已有账号，再凭有效会话申请入场票。游客绑定用户名密码保留原 UID 和 session；设备认证通过游客恢复字段定位账号并检查身份与状态，已有正式身份就拒绝。正式身份验证失败不能降级为游客。内部 UID、退出、闲置续期和账号状态由 LoginServer 统一维护，外部平台认证后续接入。

GameServer 仅验证短期入场票；断线使用已有登录态重新进入，会话闲置 30 天过期后重新验证身份。最终协议、数据与配置见 `backend_technical_architecture.md`，执行和验收记录见 `docs/tasks/account_domain.md`。

MVP 接入方案：

```text
Client -> LoginServer（LoginService/NodeAllocator）
       <- server_id + GameServer ws_addr + enter_ticket
Client -> GameServer gateway/ws
```

说明：

- 登录服只在登录和重连时参与分配，不转发后续游戏消息。
- 客户端拿到真实 GameServer `ws_addr` 后直连目标 GameServer。
- `NodeAllocator` 是登录服内部的节点分配模块，不是独立进程。
- `AccessGateway` 不进入 MVP 主链路，仅作为未来统一入口、隐藏源站和安全防护的演进方案。

GameServer 接入顺序：

```text
校验票据签名、签发方、有效期、目标节点及 nonce
  -> 显式初始化或读取玩家基础资料
  -> 绑定本地 session
  -> 认领 Redis 玩家归属并处理旧连接/运行态
  -> 返回鉴权成功和同步数据
```

重连规则：

- 原 GameServer 存活、非 drain 且未满载时优先回原服，否则选择其他可用节点；无可用节点或 Redis 读取失败时进入失败，不回退到静态节点。
- Login 从 Redis 玩家归属读取最近节点，但不在签发 ticket 时改写归属。
- GameServer 验票并绑定会话成功后，才原子更新 Redis `uid -> server_id + conn_id`。
- 玩家长期数据始终从 DB 加载；如果 Redis 前一归属仍是本节点，可继续使用本机尚未清理的 `BattleSession` 和近期请求结果。
- 如果前一归属是其他节点，新 GameServer 通过 Redis Pub/Sub 通知原节点立即关闭指定旧连接；通知必须携带旧 `conn_id`，避免延迟消息误踢新会话。
- 原 GameServer 仍通过状态维护循环每 `5` 秒批量核对归属并清理已迁移玩家的 `BattleSession` 和近期请求结果，作为 Pub/Sub 丢消息的兜底。
- Redis 离线归属保留 `120` 秒，作为原节点重连窗口与最终异常兜底；Redis 不转发玩家业务请求。

详细流程见：

- [backend_technical_architecture.md - 断线流程](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md)
- [backend_technical_architecture.md - 接入方案选择](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md)

## 10. 主链路

MVP 第一条主链路：

```text
LoginServer 分配节点并签发票据
  -> 客户端直连目标 GameServer 并提交票据
  -> GameServer 验票、创建/读取玩家、建立会话与认领归属
  -> 获取玩家资料
  -> 开始关卡
  -> 出牌
  -> 完成订单
  -> 结算奖励
  -> 写入资产
  -> 升级卡牌
  -> 升级工坊
```

实现优先级：

1. 玩家与资产主链路。
2. 配置层与卡牌/订单配置。
3. 关卡主链路。
4. 卡牌成长与卡组编辑。
5. 工坊 MVP。
6. Prototype 集成验收。

## 11. 当前运行形态与未来演进

当前默认：独立登录入口 + 单 GameServer

```text
独立 LoginServer + 单 GameServer
GameServer 内多 goroutine + 模块化单体
```

当前已支持：同一登录入口 + 多 GameServer

```text
独立单实例 LoginServer/Allocator
  -> 分配玩家到多个纯 GameServer 节点
Redis 共享节点注册、玩家归属和控制通知
各 GameServer 内存保存本节点 nonce 与连接 Session
DB 共享权威数据
```

单、双 GameServer 的进程集成验收已完成；多节点接入能力不代表正式容量压测与生产发布验收完成。验收记录见 [独立 LoginServer 拆分](docs/tasks/loginserver_split.md)，启动与扩容操作见 [runbook](docs/ops/runbook.md)。

后续演进：公共模块远程化或独立进程

```text
GameServer
  -> 调用本地 globalcore interface
  -> 将部分实现替换为 Remote Client

GlobalServer
  -> 复用 globalcore 领域规则
  -> 执行结算、批处理、补偿和 job 编排

可选独立进程：
  -> chat-service
  -> guild-service
  -> rank-service
  -> mail-service
```

说明：

- 不要求 `globalcore` 整体一次性拆成一个大公共服。
- 也不要求所有公共模块都独立进程。
- 可以先只把压力最大的模块远程化，例如 Chat 或 Rank。
- Friend/Guild/Rank 在数据量不大时，可以长期保持本地模块 + DB/Redis 权威存储。
- 非迁移候选模块不为了形式统一而过度拆分，避免产生大量只有一个实现、一个调用方的无用接口。
- LocalService 和 RemoteClient 只代表调用方式差异，不允许各自复制一套公共业务规则。

后续按压力点逐项拆分

```text
ChatService
GuildService
RankService
MailService
```

拆分原则：

- 不按“每出一个玩法就拆一个服务”。
- 按领域边界、一致性边界、扩容需求、故障隔离和 SLA 拆。

## 12. 当前不做

MVP 不做：

- 超出当前负载分配、原服优先及不可用节点避让的复杂选服策略。
- LoginServer 多实例扩容与高可用。
- 好友互助、参观和好友排行榜。
- 私聊、聊天实时推送与聊天治理。
- 公会签到、捐献、任务、商店和公会战。
- 邮件奖励补发。
- 排行榜。
- 活动。
- 通行证、月卡、广告、真支付。
- 玩家自由交易、拍卖行。
- 实时 PVP、公会战、跨服排行榜。
- 跨服恢复局内 BattleSession。

## 13. 文档维护规则

- 项目级边界变化，改本文档。
- 技术流程、协议、时序、数据层、压测变化，改 [backend_technical_architecture.md](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md)。
- 新功能的开发顺序、任务拆分和验收项写入对应功能策划文档；`architecture_v2_task_breakdown.md` 不再追加玩法任务。
- 玩法规则变化，改 `docs/` 下对应策划文档。
- 不在总览文档或策划文档复制协议号、接口签名、目录清单、数据表清单等技术细节，只引用权威文档。
