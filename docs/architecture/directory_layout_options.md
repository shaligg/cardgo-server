# 后端目录划分方案对比

## 1. 文档定位

本文档用于评估当前卡牌休闲游戏后端的目录划分方案。

它不是最终权威架构文档。最终采用方案确定后，再同步到：

- `backend_technical_architecture.md`
- `architecture_v2_task_breakdown.md`

当前状态：历史评审材料。

- 本文档中的目录树、示例文件名和迁移步骤只用于解释当时的决策过程。
- 后续实现不得直接以本文档为准。
- 当前目录、接口、协议、数据表和迁移白名单的唯一权威来源是根目录 `backend_technical_architecture.md`。
- 如果本文档与权威技术文档冲突，以权威技术文档为准，不反向同步本文档中的旧细节。

当前目标：

1. 参考成熟游戏后端和 Go 项目实践。
2. 对比几种可选目录方案。
3. 选择适合当前项目的演进方向。
4. 避免为了“看起来专业”过度拆分。

## 2. 参考实践

成熟项目没有唯一目录标准，但有几条共同点：

1. 网络层和业务层通常分开。
2. 协议路由和玩法逻辑通常分开。
3. 复杂玩法通常有独立目录。
4. 启动装配代码通常集中在应用入口或进程目录。
5. 业务代码不应该直接绑定某一种传输协议，例如 WS、TCP、KCP。

参考资料：

- Go 官方模块布局说明：https://go.dev/doc/modules/layout
- Nakama Server Framework：https://heroiclabs.com/docs/nakama/server-framework/introduction/
- Pitaya Go game server framework：https://github.com/topfreegames/pitaya
- Leaf Go game server framework：https://github.com/name5566/leaf
- Zinx Go game server framework：https://github.com/aceld/zinx

这些资料对应的启发：

| 参考 | 主要启发 |
|---|---|
| Go 官方布局 | `cmd/` 放进程入口；`internal/` 放项目私有代码。 |
| Nakama | 运行时入口、RPC/Hook、存储和匹配等边界清晰；业务入口不等于网络实现。 |
| Pitaya | 路由、组件、服务端类型、集群边界是核心概念，适合理解游戏服模块化。 |
| Leaf | 传统游戏服里模块、网络、gate、login、game 等边界清晰。 |
| Zinx | 网络框架负责连接、拆包、路由、worker；业务 handler 不应该反向污染网络框架。 |

## 3. 当前项目现状

当前主链路：

```text
cmd/gameserver/main.go
  -> internal/app/gameserver.Bootstrap
  -> internal/framework/gateway/ws
  -> internal/handler.Dispatcher
  -> internal/handler.Router
  -> internal/handler.*Handler
  -> internal/game/*Service
  -> internal/repo
  -> internal/infra/db
```

当前目录大体是合理的：

```text
internal/framework     框架与网关
internal/platform      登录、鉴权、会话、在线状态
internal/game          玩法业务
internal/repo          数据访问
internal/infra         基础设施
internal/contract      协议契约
internal/app/gameserver 应用装配、生命周期、配置
internal/handler       游戏业务协议入口
```

当前已经完成两步收敛：

```text
业务协议 handler 已从 internal/app 迁移到 internal/handler。
gameserver 启动装配已从 internal/app 迁移到 internal/app/gameserver。
```

现在位于 `app/gameserver` 的文件：

```text
bootstrap.go
lifecycle.go
config.go
admin_http.go
metrics_hooks.go
state_restore.go
```

现在位于 `handler` 的文件：

```text
dispatcher.go
router.go
routes.go
player_handler.go
asset_handler.go
card_handler.go
level_handler.go
workshop_handler.go
helpers.go
```

HTTP 管理入口已从 `bootstrap.go` 拆出到 `admin_http.go`，避免启动装配函数继续变长。

## 4. 目录职责原则

不管采用哪种方案，都建议坚持这些原则：

| 层 | 应该做 | 不应该做 |
|---|---|---|
| `cmd` | 进程入口 | 业务逻辑 |
| `app` | 应用装配、启动、关闭、配置 | 玩法规则、协议细节堆积 |
| `framework/gateway` | 连接、收包、发包、心跳、限流、编解码 | 调用具体玩法 service |
| `handler` | op_code 路由、payload 解析、参数校验、调用 service、错误转换 | 直接查库、写玩法核心规则 |
| `game` | 玩法规则、状态流转、奖励/消耗编排 | 解析 WS JSON、操作连接对象 |
| `repo` | 数据读写、事务内写入、幂等记录、资产流水 | 判断玩法是否可领奖、是否通关 |
| `infra` | DB/Redis/log/metrics/cache 驱动封装 | 具体业务规则 |
| `platform` | 登录、鉴权、session、在线状态、事件总线 | 卡牌/工坊/关卡等玩法规则 |
| `contract` | 协议号、请求/响应 DTO | 业务实现 |

一句话：

```text
外层负责接入，中层负责协议，内层负责玩法，底层负责数据。
```

## 4.1 快速对比

| 方案 | 核心做法 | 改动量 | 清晰度 | 扩展性 | 当前推荐 |
|---|---|---:|---:|---:|---|
| 方案 A：保持现状 | `app` 继续放启动装配，`handler` 已独立 | 低 | 中高 | 中 | 已不采用 |
| 方案 B：横向分层 | `app/gameserver` 管启动，`handler` 管协议，`game` 管玩法 | 中 | 高 | 高 | 已采用 |
| 方案 C：纵向玩法模块 | 每个玩法目录内放 handler/service/repo | 高 | 中 | 中高 | 只适合局部复杂玩法借鉴 |
| 方案 D：进程优先 | 先按 gameserver/loginserver/globalserver 拆 | 高 | 中高 | 高 | 当前过重 |

当前判断：

```text
已完成方案 B：`handler` 独立，`app/gameserver` 管启动装配。
复杂玩法内部吸收方案 C 的优点。
暂不采用方案 D 的完整多进程目录。
```

## 5. 方案 A：保持现状，小幅约束

### 5.1 目录示例

```text
internal/app/
  bootstrap.go
  lifecycle.go
  config.go
  biz_dispatcher.go
  biz_router.go
  biz_routes.go
  player_handler.go
  card_handler.go
  level_handler.go
  workshop_handler.go

internal/game/
  player/
  asset/
  card/
  battle/
  workshop/

internal/framework/gateway/ws/
internal/repo/
internal/infra/
internal/platform/
```

### 5.2 优点

1. 改动最小。
2. 当前 demo 不需要迁移文件。
3. 新人从 `Bootstrap -> bizRouter -> handler -> service` 也能看懂。
4. 不会因为目录调整引入额外风险。

### 5.3 缺点

1. `app` 职责偏大。
2. 玩法增多后，`app` 会堆很多 handler。
3. `app` 名字不能表达“协议入口层”。
4. 后续想区分启动装配和业务协议，会越晚越难改。

### 5.4 适用情况

适合只做短期 demo，不准备快速扩玩法。

### 5.5 对当前项目评价

可继续用，但不是最佳长期方案。

## 6. 方案 B：横向分层，handler 独立

### 6.1 目录示例

```text
internal/app/
  gameserver/
    bootstrap.go
    lifecycle.go
    config.go
    admin_http.go
    metrics_hooks.go
    state_restore.go

internal/handler/
  dispatcher.go
  router.go
  routes.go
  player_handler.go
  asset_handler.go
  card_handler.go
  level_handler.go
  workshop_handler.go
  helpers.go

internal/game/
  player/
  asset/
  inventory/
  card/
  battle/
  workshop/
  activity/
    dice/
      service.go
      roll.go
      reward.go
      settle.go
      errors.go

internal/framework/gateway/
  ws/
  tcp/       # 未来需要再加

internal/contract/protocol/
internal/repo/
internal/infra/
internal/platform/
```

### 6.2 请求链路

```text
Client
  -> framework/gateway/ws
  -> handler.Dispatcher
  -> handler.Router
  -> handler.CardHandler
  -> game/card.Service
  -> repo
  -> DB
```

未来如果换 TCP：

```text
Client
  -> framework/gateway/tcp
  -> handler.Dispatcher
  -> handler.Router
  -> handler.CardHandler
  -> game/card.Service
  -> repo
  -> DB
```

`handler`、`game`、`repo` 不需要因为 WS/TCP 变化而改目录。

### 6.3 优点

1. 层次清晰，符合大多数服务端项目的阅读习惯。
2. `app` 回归启动装配职责。
3. `handler` 不绑定 WS，未来换 TCP/KCP/Protobuf 更自然。
4. 所有协议入口集中，能统一看 op_code、请求 DTO、错误转换。
5. 比按玩法全量纵向拆分更容易控制边界。

### 6.4 缺点

1. 新增玩法时通常要改多个目录：`contract`、`handler`、`game`、`repo`、`gamedata`。
2. 看单个玩法时，需要在不同目录间跳转。
3. 需要明确约束：handler 只做协议适配，不能写业务规则。

### 6.5 适用情况

适合当前项目：

1. 已经有 WS 网关和协议路由。
2. 后续可能从 WS 演进到 TCP/二进制协议。
3. 希望代码调用链清晰。
4. 玩法会逐步增加，但还没有大到需要每个玩法完全自成模块。

### 6.6 对当前项目评价

这是推荐方案。

它解决当前最明显的问题：

```text
app 混入 handler。
```

同时不会过度拆分 `game/*` 和 `repo/*`。

## 7. 方案 C：纵向玩法模块

### 7.1 目录示例

```text
internal/module/
  card/
    handler.go
    service.go
    repo.go
    model.go
    config.go
    errors.go
  workshop/
    handler.go
    service.go
    repo.go
    model.go
    config.go
    errors.go
  activity/
    dice/
      handler.go
      service.go
      repo.go
      model.go
      config.go
      roll.go
      reward.go
      settle.go
      errors.go

internal/framework/gateway/ws/
internal/platform/
internal/infra/
internal/contract/
```

或者：

```text
internal/game/card/
  handler.go
  service.go
  repo.go
```

### 7.2 优点

1. 一个玩法相关代码集中在一个目录。
2. 新增、删除、迁移玩法很直观。
3. 适合活动多、玩法团队按系统分工的项目。
4. 复杂玩法内部文件组织很舒服。

### 7.3 缺点

1. handler、service、repo 边界容易混。
2. 每个玩法都可能重复一套错误转换、协议解析、repo 约定。
3. 统一 op_code 注册和统一协议出口仍然要额外维护。
4. 早期容易写快，后期容易变成“每个模块都有自己的小框架”。
5. 当前项目已经有 `game/*`、`repo/*`、`handler` 拟拆分方向，改成全纵向迁移成本偏大。

### 7.4 适用情况

适合以下项目：

1. 玩法很多，每个玩法由独立小组维护。
2. 已经有严格代码规范和 Review，能防止 handler/service/repo 混用。
3. 某些玩法未来真的要独立成服务或插件。

### 7.5 对当前项目评价

不建议作为全局目录方案。

但复杂玩法内部可以采用纵向拆分思想：

```text
internal/game/activity/dice/
  service.go
  roll.go
  reward.go
  settle.go
  errors.go
```

也就是说：

```text
handler 不放进玩法目录；玩法内部规则可以按文件拆。
```

## 8. 方案 D：进程/服务优先划分

### 8.1 目录示例

```text
internal/app/
  gameserver/
    bootstrap.go
    lifecycle.go
  loginserver/
    bootstrap.go
    lifecycle.go
  globalserver/
    bootstrap.go
    lifecycle.go

internal/service/
  game/
    handler/
    player/
    card/
    battle/
  login/
  global/

internal/framework/
internal/infra/
internal/repo/
internal/contract/
```

### 8.2 优点

1. 非常贴近未来多进程部署。
2. 登录服、游戏服、公共服边界清楚。
3. 后续拆服务时入口清晰。

### 8.3 缺点

1. 对当前 MVP 过重。
2. 容易提前制造很多空目录和空接口。
3. 当前登录、globalserver 还没有独立进程需求，过早按进程拆会增加理解成本。
4. 可能让 demo 失去“快速可用”的优势。

### 8.4 适用情况

适合已经明确要多进程部署的中后期项目。

### 8.5 对当前项目评价

暂时不建议。

可以保留：

```text
internal/app/gameserver
internal/globalserver
```

但不要现在新增完整 `loginserver/globalserver` 进程骨架。

## 9. 推荐方案

推荐采用方案 B：横向分层，handler 独立。

### 9.1 推荐目录

```text
cmd/
  gameserver/
    main.go

internal/
  app/
    gameserver/
      bootstrap.go
      lifecycle.go
      config.go
      admin_http.go
      metrics_hooks.go
      state_restore.go

  framework/
    gateway/
      ws/
        server.go
        client.go
        codec.go
        heartbeat.go
        limiter.go
    dispatcher/
      key_router.go
      shard_executor.go
    transport/
      dto/
        message.go
      errors/
        errors.go

  handler/
    dispatcher.go
    router.go
    routes.go
    player_handler.go
    asset_handler.go
    card_handler.go
    level_handler.go
    workshop_handler.go
    helpers.go

  contract/
    protocol/
      opcode.go
      request.go
      response.go      # 需要时再加

  game/
    player/
      service.go
    asset/
      service.go
    inventory/
      service.go
    card/
      service.go
    battle/
      service.go
    workshop/
      service.go
    activity/
      dice/
        service.go
        roll.go
        reward.go
        settle.go
        errors.go

  gamedata/
    game_data.go
    game_data_validate.go
    item_config.go
    workshop_config.go
    dice_config.go     # 有新玩法配置时再加

  globalcore/
    core.go
    rank/              # 公共领域核心：接口、DTO、Local/Remote 适配、排行规则
      service.go
      local.go
      remote_client.go # 未来需要时再加
      reward.go        # 排行奖励规则，供本地和远端复用
    chat/              # 聊天公共领域核心
    guild/             # 公会公共领域核心
    mail/              # 邮件公共领域核心
    friend/            # 好友公共领域核心
    notice/            # 公告公共领域核心

  globalserver/
    rank/
      settlement_job.go # 公共服/job 编排：扫描、幂等、落库、重试
    mail/
      batch_job.go      # 批量邮件/补偿编排，未来需要时再加
    activity/
      settlement_job.go # 活动结算编排，未来需要时再加

  platform/
    login/
    auth/
    session/
    state/
    eventbus/

  repo/
    repository.go
    player_repo.go
    workshop_repo.go
    snapshot_repo.go
    cached_player_repo.go
    cache_keys.go
    model/

  infra/
    db/
    cache/
    redis/
    log/
    metrics/
    health/

  pkg/                 # 不预创建，确有两个以上模块复用纯工具时再加
```

### 9.2 为什么不是 `handler/ws`

不建议：

```text
internal/handler/ws
```

原因：

```text
handler 是业务协议入口，不应该绑定 WebSocket。
```

更合理：

```text
framework/gateway/ws 负责 WS。
handler 负责游戏业务协议。
```

未来替换外层：

```text
framework/gateway/ws  -> handler
framework/gateway/tcp -> handler
framework/gateway/kcp -> handler
```

handler 不需要变。

### 9.3 为什么不把 handler 放进 game/service

有些成熟项目会这么做：

```text
game/card/handler.go
game/card/service.go
game/card/repo.go
```

但当前项目不推荐。

原因：

1. 你希望调用链清晰，协议层不要污染业务层。
2. 后续可能替换 WS 为 TCP 或二进制协议。
3. 当前已经有统一 op_code 注册表，集中 handler 更利于维护。
4. `game/*` 应该专注玩法规则，不解析 JSON，不依赖协议错误码。

允许复杂玩法内部拆多文件：

```text
internal/game/activity/dice/
  service.go
  roll.go
  reward.go
  settle.go
```

但协议入口仍然放：

```text
internal/handler/dice_handler.go
```

## 10. 新增复杂玩法时的落点

以“摇骰子活动”为例。

### 10.1 必要文件

```text
internal/handler/dice_handler.go
internal/game/activity/dice/service.go
internal/game/activity/dice/roll.go
internal/game/activity/dice/reward.go
internal/game/activity/dice/settle.go
internal/game/activity/dice/errors.go
internal/repo/dice_repo.go                 # 只有需要持久化时加
internal/repo/model/dice.go                # 只有需要新表时加
internal/gamedata/dice_config.go           # 只有需要策划配置时加
configs/gamedata/dice.json                 # 只有需要策划配置时加
```

### 10.2 固定修改点

```text
internal/contract/protocol/opcode.go
internal/contract/protocol/request.go
internal/handler/routes.go
internal/app/gameserver/bootstrap.go       # 注入 service 依赖
```

### 10.3 调用链

```text
Client
  -> gateway/ws DecodeEnvelope
  -> handler.Dispatcher.Handle(uid, op_code, payload)
  -> handler.Router.Handle(op_code)
  -> DiceHandler.Roll
  -> game/activity/dice.Service.Roll
  -> asset.Service.Consume / asset.Service.Grant
  -> repo.DiceRepository.SaveState
  -> response
```

### 10.4 约束

1. Handler 只拆包、校验参数、调用 service。
2. Service 决定玩法规则、奖励内容、状态变化。
3. AssetService 统一扣费和发奖。
4. Repo 只负责数据读写和事务内写入能力。
5. 配置读取和校验放在 `gamedata`。

## 11. 迁移步骤

为了降低风险，建议分三步迁移，不一次性大重构。

### 11.1 第一步：只迁移 handler（已完成）

目标：把 `internal/app` 中的业务协议入口移到 `internal/handler`。

移动：

```text
internal/app/biz_dispatcher.go  -> internal/handler/dispatcher.go
internal/app/biz_router.go      -> internal/handler/router.go
internal/app/biz_routes.go      -> internal/handler/routes.go
internal/app/player_handler.go  -> internal/handler/player_handler.go
internal/app/asset_handler.go   -> internal/handler/asset_handler.go
internal/app/card_handler.go    -> internal/handler/card_handler.go
internal/app/level_handler.go   -> internal/handler/level_handler.go
internal/app/workshop_handler.go -> internal/handler/workshop_handler.go
internal/app/biz_helpers.go     -> internal/handler/helpers.go
```

保留：

```text
internal/app/bootstrap.go
internal/app/lifecycle.go
internal/app/config.go
internal/app/metrics_hooks.go
internal/app/state_restore.go
```

已调整：

```text
internal/app/bootstrap.go import internal/handler
```

验收：

```text
go test ./...
```

如果需要完整验证，再跑：

```text
go run ./scripts/loadtest/ws_prototype_smoke
```

### 11.2 第二步：迁移 app 到 app/gameserver（已完成）

目标：让 `app` 表达进程入口。

移动：

```text
internal/app/bootstrap.go      -> internal/app/gameserver/bootstrap.go
internal/app/lifecycle.go      -> internal/app/gameserver/lifecycle.go
internal/app/config.go         -> internal/app/gameserver/config.go
internal/app/metrics_hooks.go  -> internal/app/gameserver/metrics_hooks.go
internal/app/state_restore.go  -> internal/app/gameserver/state_restore.go
```

调整：

```text
cmd/gameserver/main.go import internal/app/gameserver
```

注意：

这一步会改 import 路径较多，但逻辑不应该变化。

### 11.3 第三步：拆出 admin_http（已完成）

HTTP 管理接口已经从 `bootstrap.go` 拆出：

```text
internal/app/gameserver/admin_http.go
```

`bootstrap.go` 只保留 `buildAPIMux(...)` 调用，不直接展开 health、metrics、drain 和 session 路由细节。

## 12. 迁移风险

| 风险 | 说明 | 控制方式 |
|---|---|---|
| import 路径改错 | 移动包后最常见 | 每步只移动一类文件，立即 `go test ./...` |
| 循环依赖 | `handler` 调 `game`，`app` 调 `handler`，不能反向 | 明确禁止 `game` import `handler/app` |
| 过度抽象 | 为了目录漂亮创造无意义接口 | 只移动目录，不新增业务抽象 |
| 新人迷路 | 目录变多后查找成本上升 | 保留 `handler/routes.go` 作为协议总入口 |
| 文档过期 | 代码目录变了，文档没改 | 迁移完成后同步技术架构文档 |

## 13. 最终建议

当前项目最适合的路线：

```text
已采用方案 B：handler 从 app 拆出，app 收敛为 app/gameserver。
长期：复杂玩法在 game/activity/<name> 内部纵向拆文件，但 handler 仍然集中。
```

不建议现在做：

1. 不建议把所有玩法改成 `module/<玩法>/handler/service/repo`。
2. 不建议提前创建大量空的 `loginserver/globalserver` 进程目录。
3. 不建议把 handler 命名为 `handler/ws`。
4. 不建议因为未来可能 protobuf，就现在重写协议层。

已完成落地顺序：

```text
1. 完成方案对比文档。
2. 迁移 internal/handler。
3. 迁移 internal/app/gameserver。
4. 每一步后跑 go test ./...。
```
