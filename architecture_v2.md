# 项目后端架构总览（V2 / 卡牌休闲 MVP）

## 1. 文档定位

本文档是项目级后端架构总览。

它回答：

- 当前项目后端整体采用什么形态。
- MVP 要实现哪些后端模块。
- 哪些完整业务后置，哪些代码边界从 MVP 起建立。
- 模块之间的边界如何划分。
- 当前单进程架构未来如何演进到多进程或服务拆分。

更细的技术实现放在：

- [backend_technical_architecture.md](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md)

落地任务拆分放在：

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
- [架构落地任务拆分](/Users/bigfish/Project/go_orm_1/architecture_v2_task_breakdown.md)

## 3. 架构结论

当前 MVP 采用：

```text
单 GameServer 进程
  + 多 goroutine
  + 模块化单体
  + 按未来多进程/服务拆分边界编码
```

当前不是微服务，也不是多进程集群优先。

目标是：

- 先用 1 个游戏服进程承载单服 2000 在线。
- 登录模块当前同进程实现，但接口按独立登录服务设计。
- 业务模块按服务边界写，后续可平滑拆分。
- 高频在线热状态可以在本机内存。
- 玩家权威数据必须在 DB。
- Redis 用于 ticket nonce、session_index、短 TTL 跨进程状态。

## 4. 运行形态

MVP 运行形态：

```text
GameServer 进程
  - HTTP Login API
  - WebSocket Gateway
  - Auth / Session
  - Dispatcher
  - Game Services
  - globalcore domain core
  - globalserver same-process jobs/service process boundary
  - State Manager
  - Repository / Cache
  - Infra

Redis
DB
```

Go 运行模型：

```text
1 个进程
  - HTTP goroutine
  - WebSocket accept goroutine
  - 每连接读 goroutine
  - 每连接写 goroutine
  - dispatcher shard goroutine
  - flush worker goroutine
  - metrics / health goroutine
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
| 登录接入 | 账号进入、节点分配、发票、验票、会话建立 |
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
| 好友 | 轻社交关系链、申请、互助入口 | 占位，完整业务后置 |
| 聊天 | 世界/系统/公会频道能力 | 占位，完整业务后置 |
| 公会 | 成员、职位、签到、捐献、协作玩法基础 | 占位，完整业务后置 |
| 邮件 | 系统邮件、奖励补发、离线通知 | 可保留批量/补发边界 |
| 排行榜 | 无尽订单、活动榜、赛季奖励 | 可保留结算边界 |
| 公告 | 系统公告、运营通知、活动提示 | 占位，完整业务后置 |

技术细节以 [backend_technical_architecture.md](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md) 的“可迁移模块列表”和“所有模块列表”为准。

原则：

```text
game/* 不拥有好友、聊天、公会、邮件、排行榜的核心状态和核心规则。
game/* 可以通过接口调用 globalcore/*。
globalcore/* 是公共领域核心，不等于独立公共服务进程，也不只是 client。
globalserver/* 是公共服编排层，MVP 就可以有代码，但不独立启动、不做 RPC/HTTP 等数据传输层。
```

判断规则：

- 如果是玩家请求链路内的公共数据读写，先通过 `globalcore/*` 接口调用。
- 如果是公共领域核心规则，例如排行奖励分段、奖励生成、聊天消息校验、公会权限规则，放 `globalcore/*`。
- 如果是全局周期结算、批量发奖、跨服聚合、赛季清算，放 `globalserver/*` 编排。
- 如果 `globalcore/*` 的请求期逻辑未来需要跨多个 GameServer 实时统一状态、独立扩容、故障隔离或独立 SLA，再将 Local 实现替换为 RemoteClient。
- game 可以调用 globalcore 接口，但不能直接操作 globalcore 的内部表、map、Redis key 或 ZSET。
- 初版 `globalserver/*` 由 GameServer 同进程直调；未来拆分时再补 `cmd/globalserver` 和传输层。
- `globalserver/*` 可以复用 `globalcore/*` 规则和 `game/asset` 发奖接口，但不能依赖连接、session、在线热状态。
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
  game/activity_a
    -> globalcore/rank.SubmitScore(board_id, uid, score, req_id)
    -> globalcore/rank 内部可以本地执行 Redis ZADD / DB 记录

活动 A 排行榜赛季结算与奖励发放：
  globalserver/rank 或 globalserver/activity_a
    -> 扫描榜单
    -> 调 globalcore/rank 生成排名奖励
    -> 调 AssetService/MailService 发奖或生成待领取记录
```

## 7. 核心分层

```text
Gateway / Transport
  -> Handler / BizRouter
  -> Service
  -> CachedRepository
  -> Repository
  -> Model
  -> DB
```

规则：

- `Gateway` 处理连接、协议、心跳、限流、背压。
- `Handler` 只做协议参数解析和调用 Service。
- `Service` 承载业务规则。
- `Repository` 只做数据库访问。
- `CachedRepository` 只做缓存策略，不写业务 SQL。
- `Model` 是持久化模型，不直接暴露给客户端。

## 8. 数据分层

```text
L1 GameServer 内存
  - 在线热状态
  - 局内临时状态
  - 短时间断线恢复状态

L2 Redis
  - ticket nonce
  - session_index
  - 短 TTL 跨进程状态

L3 DB
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
- GameServer 内存只做在线热状态和局内临时状态。
- 跨服重连不恢复旧服内存态，只从 DB 重建长期状态。

## 9. 登录与重连原则

登录分配由 `Login / NodeAllocator` 决定。

凭证分层：

- `account_token / refresh_token` 证明“玩家是谁”，由登录/账号系统处理。
- `enter_ticket` 证明“玩家本次可以进入哪台 GameServer”，由登录服选服后短期签发。
- GameServer 只验证 `enter_ticket`，不直接处理账号密码、平台 SDK token 或 refresh token。
- 断线重连不是重新输入密码，而是用已有账号登录态重新换取新的 `enter_ticket`。

MVP 接入方案：

```text
Client -> LoginService/NodeAllocator
       <- server_id + GameServer ws_addr + enter_ticket
Client -> GameServer gateway/ws
```

说明：

- 登录服只在登录和重连时参与分配，不转发后续游戏消息。
- 客户端拿到真实 GameServer `ws_addr` 后直连目标 GameServer。
- `NodeAllocator` 是登录服内部的节点分配模块，不是独立进程。
- `AccessGateway` 不进入 MVP 主链路，仅作为未来统一入口、隐藏源站和安全防护的演进方案。

GameServer 只做：

```text
验证 ticket.server_id 是否等于自己
验票成功后建立 session
根据本机是否有热状态决定恢复方式
```

重连规则：

- 优先分配回原 GameServer。
- 如果原服健康、未满、非 drain，可恢复本机内存热状态。
- 如果分配到新 GameServer，不恢复旧服内存，只从 DB 重建玩家长期状态。
- 原 GameServer 的旧内存态 MVP 使用 TTL 清理。
- 后续可加迁移通知，让旧 GameServer 立即清理状态。

详细流程见：

- [backend_technical_architecture.md - 断线流程](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md)
- [backend_technical_architecture.md - 接入方案选择](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md)

## 10. 主链路

MVP 第一条主链路：

```text
登录
  -> 创建/读取玩家
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

## 11. 未来演进

阶段 1：MVP

```text
单进程 + 多 goroutine + 模块化单体
```

阶段 2：多 GameServer

```text
Login/Allocator 分配玩家到不同 GameServer
Redis 共享 session/nonce
DB 共享权威数据
```

阶段 3：公共模块远程化或独立进程

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

阶段 4：按压力点拆分

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

- 多节点候选服复杂分配。
- 好友完整业务。
- 世界聊天。
- 公会完整业务。
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
- 开发顺序、任务拆分、验收项变化，改 [architecture_v2_task_breakdown.md](/Users/bigfish/Project/go_orm_1/architecture_v2_task_breakdown.md)。
- 玩法规则变化，改 `docs/` 下对应策划文档。
- 不在总览文档或策划文档复制协议号、接口签名、目录清单、数据表清单等技术细节，只引用权威文档。
