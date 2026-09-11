# 独立 LoginServer 拆分任务

## 1. 任务状态

- 状态：`TODO`
- 类型：后端技术架构任务
- 整体优先级：`P0`，多 GameServer 部署前必须完成
- 实现范围：拆分进程、配置和启动边界，不改玩法逻辑
- 完成标志：LoginServer 与 GameServer 可独立启动，登录后仍可直连目标 GameServer 完成鉴权

本文件是该任务的唯一执行说明。新会话应按本文完成代码、验证和文档同步，不需要重新读取全部策划文档或历史讨论。

执行约束：

- 一次只完成本任务，不顺带修改其他架构或玩法问题。
- 以调用链清晰和代码易读为优先，非必要不封装、不增加接口和中间层。
- 新增模块和重要函数使用中文注释，简单赋值和显而易见的代码不写无效注释。
- 不为兼容当前未上线的旧目录、旧配置名或旧启动方式增加分支。
- 小步骤完成后不反复运行全量测试；实现阶段完成后统一验证。
- 发现非本任务问题时记录到任务结果，不直接修改。

## 2. 新会话读取范围

开始实现前只需要读取：

1. `docs/README.md`
2. 本文件 `docs/tasks/loginserver_split.md`
3. `backend_technical_architecture.md` 的以下章节：
   - `5.1 login`
   - `6.0 账号登录态与 GameServer 入场票`
   - `6.1 登录接入协议`
   - `6.2 enter_ticket 签名契约`
   - `7.1 登录接入流程`
   - `13. 高可用与扩展准备`
   - `16. 目录结构`
   - `18.1 Login`
   - `19.1 Login API`
   - `20.1 连接创建请求-返回`
4. 当前实现文件：
   - `cmd/gameserver/main.go`
   - `internal/app/gameserver/bootstrap.go`
   - `internal/app/gameserver/config.go`
   - `internal/app/gameserver/lifecycle.go`
   - `internal/app/gameserver/admin_http.go`
   - `internal/platform/login/`
   - `internal/infra/redis/node_registry.go`
   - `internal/infra/redis/player_owner_store.go`

本任务不要求读取具体玩法策划文档。

## 2.1 执行入口

新会话从第 10 节的 `LS-01` 开始，严格按依赖顺序执行。第 3 至第 9 节用于解释任务边界和目标，不代替执行清单。

优先级口径：

- `P0`：进程拆分主链路，未完成则任务不可用。
- `P1`：任务收尾，必须完成后才能将整个任务标记为 `DONE`，但不得阻塞前面的代码实现。
- 本任务没有 `P2` 可选项；非目标统一留到后续独立任务。

状态口径：

- `TODO`：尚未开始。
- `DOING`：当前正在执行，同一时间只允许一个步骤处于此状态。
- `DONE`：该步骤的代码、单步检查和交付物全部完成。
- `BLOCKED`：存在无法在当前任务内解决的外部阻塞，必须记录原因。

## 3. 任务目标

把当前 GameServer 进程中的登录 HTTP 服务拆成独立 LoginServer 进程，使多个 GameServer 可以共享一个登录入口和一套 Redis 节点注册数据。

目标运行拓扑：

```text
                         Redis
                 +-------------------+
                 | GameServer 节点表 |
                 | 玩家最近归属      |
                 +---------+---------+
                           ^
                           |
+--------+  HTTP   +-------+-------+
| Client | ------> |  LoginServer  |
+--------+          |  :8080        |
    |               +---------------+
    |                 返回 server_id
    |                 ws_addr/ticket
    v
+----------------+       +----------------+
| GameServer A   |       | GameServer B   |
| WS :8081       |       | WS :8091       |
| Admin :8082    |       | Admin :8092    |
+----------------+       +----------------+
        |                         |
        +-----------+-------------+
                    |
               MySQL / Redis
```

关键结果：

1. LoginServer 是独立可执行进程，负责 `/api/login`、节点选择和票据签发。
2. GameServer 不再创建 `login.Service`，也不再暴露 `/api/login`。
3. GameServer 继续负责 WS 接入、票据校验、玩家会话、玩法业务、MySQL 持久化和 Redis 节点上报。
4. 客户端流程保持不变：先请求登录地址，再直连返回的 GameServer 地址。
5. 启动第二个 GameServer 时，只需提供独立节点配置，不修改登录或玩法代码。

## 4. 当前基线

当前只有一个程序入口：

```text
cmd/gameserver/main.go
```

当前 `internal/app/gameserver.Bootstrap` 同时创建：

- GameServer WS 与业务组件。
- MySQL、Redis、节点注册和玩家归属组件。
- `login.RegistryNodeAllocator`。
- `login.LocalTicketIssuer`。
- `login.Service`。
- 同时包含 `/api/login`、管理接口、健康检查和指标的 HTTP Server。

当前已经具备、应直接复用的边界：

- `login.Service`
- `login.NodeAllocator`
- `login.RegistryNodeAllocator`
- `login.NodeRegistry`
- `login.LocalTicketIssuer`
- Redis `NodeRegistry`
- Redis `PlayerOwnerStore.GetLastServerID`
- GameServer `auth.Verifier`

当前票据签发端与验证端通过相同的 `GAME_TICKET_SECRET` 和 issuer 保持一致。不得更改票据字段、签名算法或 WebSocket `auth_req` 语义。

当前 Demo 登录把 `account` 直接作为 UID，尚未实现正式账号认证。该限制保留，不属于本任务。

## 5. 进程职责

### 5.1 LoginServer

只负责：

- 监听登录 HTTP 端口。
- 提供 `POST /api/login`。
- 从 Redis 节点注册表读取存活 GameServer。
- 读取 Redis 玩家最近归属，优先分配原 GameServer。
- 原节点不可用时执行现有负载感知“两选一”分配。
- 使用 HMAC-SHA256 签发短期 `enter_ticket`。
- 提供基础 `/healthz`。
- 收到停止信号后关闭 HTTP Server 和 Redis 客户端。

不得负责：

- WebSocket 长连接。
- 玩家玩法协议。
- GameServer 消息转发。
- 注册为 GameServer 节点。
- 认领或刷新玩家在线归属。
- 连接 MySQL 或执行数据库迁移。
- 正式账号、密码、平台 SDK、refresh token。

### 5.2 GameServer

继续负责：

- WebSocket 监听和首帧验票。
- 玩家会话、顶号、归属认领和归属刷新。
- 玩家业务 Handler、Service、Repository 和 MySQL。
- GameServer 节点注册、心跳、drain 和注销。
- 自身管理接口、健康检查和指标。

拆分后必须移除：

- `login.Service` 的构造。
- `login.LocalTicketIssuer` 的构造。
- `/api/login` 路由。
- GameServer HTTP 组件对 `login.Provider` 的依赖。

GameServer 仍可使用 `login.NodeInfo` 和 `login.NodeRegistrar` 作为共享节点契约。本任务不移动这些接口，避免无关目录重构。

## 6. 目标目录

新增：

```text
cmd/
  loginserver/
    main.go

internal/app/
  loginserver/
    config.go
    bootstrap.go
    lifecycle.go
```

如果 HTTP 路由放在 `bootstrap.go` 会明显降低可读性，可以增加：

```text
internal/app/loginserver/http.go
```

不要为只有一次调用的简单逻辑继续拆文件或增加抽象层。

调整：

```text
internal/app/gameserver/bootstrap.go
internal/app/gameserver/admin_http.go
internal/app/gameserver/config.go
configs/
README.md
architecture_v2.md
backend_technical_architecture.md
docs/ops/runbook.md
```

`internal/platform/login/` 继续保存可复用登录领域边界和实现，不复制到 `internal/app/loginserver/`。

## 7. 配置拆分

项目尚未上线，不保留旧配置兼容入口。将当前混合配置明确拆为：

```text
configs/
  gameserver.local.yaml
  gameserver.staging.yaml
  gameserver.prod.yaml
  loginserver.local.yaml
  loginserver.staging.yaml
  loginserver.prod.yaml
```

删除被替代的：

```text
configs/config.local.yaml
configs/config.staging.yaml
configs/config.prod.yaml
```

两个进程均可继续使用 `GAME_CONFIG` 指定各自配置文件，因为它们在不同进程环境中运行。不得增加旧路径 fallback。

### 7.1 GameServer 配置

保留：

- `server.node_id`
- GameServer 管理 HTTP 监听地址，并将含义模糊的 `api_host/api_port` 改为 `admin_host/admin_port`
- WS 监听地址与对外公布地址
- 单节点最大连接数与 dispatcher 分片数
- ticket 验证配置
- 管理接口配置
- WS 配置
- MySQL 配置
- Redis 节点注册、玩家归属与心跳配置
- state、debug、web_search、gamedata

删除只用于签发票据的 LoginServer 配置。若签发和验证共用同一个 `auth` 结构仍最清晰，可以保留共同字段，但 GameServer 不得再次创建 issuer。

项目未上线，不保留 `api_host/api_port` 到 `admin_host/admin_port` 的兼容解析。

### 7.2 LoginServer 配置

只包含：

- HTTP 监听地址。
- ticket issuer、算法、TTL、密钥环境变量名。
- Redis 地址、密码环境变量名、DB。
- Redis GameServer 节点 key 前缀。
- Redis 玩家归属 key 前缀。

LoginServer 不应包含：

- GameServer `node_id`。
- WS、dispatcher、state、玩法配置。
- MySQL DSN。
- GameServer 节点心跳配置。
- GameServer 管理 Token。

### 7.3 本地端口

固定本地示例：

| 进程 | 功能 | 端口 |
|---|---|---:|
| LoginServer | Login HTTP | 8080 |
| GameServer A | WebSocket | 8081 |
| GameServer A | Admin/health/metrics HTTP | 8082 |
| GameServer B（验收时可选） | WebSocket | 8091 |
| GameServer B（验收时可选） | Admin/health/metrics HTTP | 8092 |

保留 LoginServer `8080` 和首个 GameServer WS `8081`，使现有 smoke 脚本尽量无需修改。

## 8. 组件装配

### 8.1 LoginServer Bootstrap

装配顺序：

```text
LoadConfigFromEnv
  -> 校验 ticket algorithm 与 secret
  -> 创建 Redis Client
  -> 创建 Redis NodeRegistry
  -> 创建 Redis PlayerOwnerStore
  -> 创建 RegistryNodeAllocator
  -> 创建 LocalTicketIssuer
  -> 创建 login.Service
  -> 创建 HTTP mux 和 http.Server
  -> 返回 loginserver.Application
```

要求：

1. `RegistryNodeAllocator.Registry` 使用 Redis `NodeRegistry`。
2. `RegistryNodeAllocator.LastServer` 使用 Redis `PlayerOwnerStore`。
3. 不给 `login.Service.LastServer` 设置写入器；玩家权威归属仍只由 GameServer 在验票并绑定成功后认领。
4. Bootstrap 任一步失败时关闭已经创建的 Redis 资源。
5. 密钥为空或算法不支持时启动失败，不能降级为无签名票据。

### 8.2 LoginServer 生命周期

`Start` 必须先执行 `net.Listen`，监听失败直接返回错误，不能只在 goroutine 中记日志后继续运行。

`Stop` 顺序：

```text
HTTP Server Shutdown
  -> Redis Client Close
```

允许使用与 GameServer 一致的 `signal.NotifyContext` 主入口结构。

### 8.3 GameServer Bootstrap

保持原有业务装配顺序，只删除 LoginServer 相关构造和路由依赖。`repo.Migrate(gdb)` 暂时继续保留在 GameServer；独立迁移命令是后续任务，不能在本任务顺手处理。

## 9. 请求流程

### 9.1 登录与连接

```text
Client
  -> LoginServer HTTPHandler
  -> login.Service.LoginAndIssueTicket
  -> RegistryNodeAllocator
  -> Redis NodeRegistry.ListNodes
  -> Redis PlayerOwnerStore.GetLastServerID
  -> LocalTicketIssuer.Issue
  <- server_id + ws_addr + enter_ticket + expire_at

Client
  -> 目标 GameServer /ws
  -> 首帧 auth_req(enter_ticket)
  -> auth.Verifier
  -> PreparePlayer
  -> SessionManager 绑定连接
  -> Redis PlayerOwnerStore.Claim
  <- auth_ack
```

LoginServer 只参与第一段。第二段以及后续全部玩法请求不经过 LoginServer。

### 9.2 重连

```text
Client 发现连接不可用
  -> 再次请求 LoginServer /api/login
  -> allocator 读取玩家最近归属
  -> 原节点健康、非 drain、未满载：仍返回原节点
  -> 原节点不可用：分配其他可用节点
  -> 签发新的 enter_ticket
  -> Client 直连返回节点
```

不得复用旧 `enter_ticket`，不得由 GameServer 自己选服。

### 9.3 无可用节点

Redis 中没有健康、非 drain、未满载节点时：

- `RegistryNodeAllocator` 返回 `login.ErrNoAvailableNode`。
- Login HTTP 返回受控失败响应。
- 不回退到静态节点配置。
- 不返回当前 LoginServer 自己的地址。

本任务可以保持当前 `loginAPIResponse` 的协议格式，不要求新增完整业务错误码体系。

## 10. 执行任务清单

### 10.1 总览

| 编号 | 任务 | 优先级 | 依赖 | 状态 |
|---|---|---|---|---|
| `LS-01` | 拆分进程配置 | P0 | 无 | TODO |
| `LS-02` | 实现独立 LoginServer | P0 | LS-01 | TODO |
| `LS-03` | 移除 GameServer 登录职责 | P0 | LS-02 | TODO |
| `LS-04` | 完成单节点集成验收 | P0 | LS-03 | TODO |
| `LS-05` | 完成双节点分配验收 | P0 | LS-04 | TODO |
| `LS-06` | 同步架构和运维文档 | P1 | LS-05 | TODO |
| `LS-07` | 完成全量验证与提交 | P1 | LS-06 | TODO |

执行规则：

1. 开始步骤时，将该步骤状态改为 `DOING`。
2. 只修改该步骤列出的文件；发现其他问题先记录，不顺手处理。
3. 单步完成条件满足后改为 `DONE`，再进入下一步。
4. 聚焦测试可以在对应步骤执行；`go test ./...` 和 `go vet ./...` 只在 `LS-07` 执行。
5. 每一步只迁移现有职责，不重写 allocator、ticket、auth、session 或业务路由。

### 10.2 LS-01 拆分进程配置

- 优先级：`P0`
- 依赖：无
- 状态：`TODO`

执行步骤：

1. 将三个现有 `configs/config.*.yaml` 重命名为 `configs/gameserver.*.yaml`。
2. 将 GameServer 默认配置路径改为 `configs/gameserver.local.yaml`。
3. 把 GameServer 的 `api_host/api_port` 配置和 Go 字段改名为 `admin_host/admin_port`。
4. 保留 GameServer 所需的 WS、MySQL、Redis 节点上报、玩家归属、玩法和管理配置。
5. 新增 `internal/app/loginserver/config.go`，只声明 HTTP、ticket issuer 和 Redis 读取字段。
6. 新增 `loginserver.local/staging/prod.yaml`，本地默认监听 `0.0.0.0:8080`。
7. 两个进程分别使用自己的 `LoadConfigFromEnv`，均允许进程级 `GAME_CONFIG` 覆盖默认路径。
8. 不增加旧配置路径和旧字段名兼容逻辑。
9. 为正常加载和关键配置缺失补充聚焦测试。

完成条件：

- [ ] 六份进程配置职责明确。
- [ ] LoginServer 配置不包含 MySQL、WS、玩法或 GameServer 节点身份。
- [ ] 旧 `configs/config.*.yaml` 已删除且没有 fallback。
- [ ] 配置聚焦测试通过。

### 10.3 LS-02 实现独立 LoginServer

- 优先级：`P0`
- 依赖：`LS-01`
- 状态：`TODO`

执行步骤：

1. 新增 `loginserver.Application`，只持有配置、HTTP Server 和 Redis 客户端。
2. 在 `Bootstrap` 中读取配置并校验算法、issuer、ticket TTL 和密钥环境变量。
3. 创建 Redis Client、`NodeRegistry` 和 `PlayerOwnerStore`。
4. 使用上述对象组装现有 `RegistryNodeAllocator`、`LocalTicketIssuer` 和 `login.Service`。
5. 不设置 `login.Service.LastServer` 写入器，玩家权威归属仍由 GameServer 登录成功后认领。
6. 注册公开的 `POST /api/login` 和基础 `/healthz`。
7. `Start` 先调用 `net.Listen`，端口绑定失败直接返回错误并释放资源。
8. `Stop` 先关闭 HTTP Server，再关闭 Redis Client。
9. 新增 `cmd/loginserver/main.go`，处理 `SIGINT/SIGTERM` 并调用 Bootstrap、Start、Stop。
10. 为 HTTP 路由、错误配置和端口占用补充聚焦测试。

涉及文件：

```text
cmd/loginserver/main.go
internal/app/loginserver/bootstrap.go
internal/app/loginserver/lifecycle.go
internal/app/loginserver/config.go
```

只有 HTTP 路由影响 `bootstrap.go` 可读性时才增加 `internal/app/loginserver/http.go`。

完成条件：

- [ ] `cmd/loginserver` 可以独立编译、启动和停止。
- [ ] LoginServer 不导入 GameServer、玩法、Repository 或 MySQL 包。
- [ ] `/api/login` 和 `/healthz` 可访问。
- [ ] Bootstrap 或监听失败时已创建资源会被释放。
- [ ] 聚焦测试通过。

### 10.4 LS-03 移除 GameServer 登录职责

- 优先级：`P0`
- 依赖：`LS-02`
- 状态：`TODO`

执行步骤：

1. 从 `gameserver.Bootstrap` 删除 `RegistryNodeAllocator`、`LocalTicketIssuer` 和 `login.Service` 的构造。
2. 删除 `buildAPIMux` 的 `login.Provider` 参数。
3. 删除 GameServer HTTP mux 中的 `/api/login` 路由。
4. 保留 `/healthz`、`/metricsz` 和 `/admin/*`。
5. 使用 `admin_host/admin_port` 构造 GameServer 管理 HTTP 地址。
6. 保留 `auth.Verifier`、Redis 节点注册、玩家归属、顶号和 WS 逻辑。
7. 保留 `repo.Migrate(gdb)`，不在本步骤拆迁移命令。
8. 删除不再使用的 import、字段和参数，不保留兼容空壳。
9. 调整聚焦测试，确认 GameServer `/api/login` 返回 404。

涉及文件：

```text
internal/app/gameserver/bootstrap.go
internal/app/gameserver/admin_http.go
internal/app/gameserver/config.go
相关 GameServer 测试
```

完成条件：

- [ ] GameServer 不再创建 `login.Service` 或 TicketIssuer。
- [ ] GameServer 管理端口的 `/api/login` 返回 404。
- [ ] GameServer 仍能注册节点、监听 WS 并验证 LoginServer 签发的 ticket。
- [ ] 聚焦测试通过。

### 10.5 LS-04 完成单节点集成验收

- 优先级：`P0`
- 依赖：`LS-03`
- 状态：`TODO`

执行步骤：

1. 确认本地 MySQL、Redis、`GAME_DB_DSN` 和 `GAME_TICKET_SECRET` 可用。
2. 使用 `configs/gameserver.local.yaml` 启动 GameServer。
3. 确认 GameServer 已在 Redis 注册 `node-a`。
4. 使用 `configs/loginserver.local.yaml` 启动 LoginServer。
5. 请求 LoginServer `/api/login`，核对 `server_id`、`ws_addr`、`enter_ticket` 和 `expire_at`。
6. 使用返回地址建立 WebSocket，并用返回 ticket 完成首帧鉴权。
7. 确认收到 `auth_ack`，再执行现有主链路 smoke。
8. 检查 GameServer 管理端口的健康、指标和管理路由。
9. 检查 GameServer 管理端口不再提供 `/api/login`。

完成条件：

- [ ] 第 11.3 节全部验收项通过。
- [ ] 登录响应和 WS 鉴权协议未改变。
- [ ] 现有玩法 smoke 通过。

### 10.6 LS-05 完成双节点分配验收

- 优先级：`P0`
- 依赖：`LS-04`
- 状态：`TODO`

执行步骤：

1. 创建只用于验收的 `node-b` GameServer 配置。
2. 为 `node-b` 设置唯一 `node_id`、WS `8091`、管理 HTTP `8092` 和对外 WS 地址。
3. 同时启动 `node-a`、`node-b` 和一个 LoginServer。
4. 确认 Redis 节点注册表存在两个有效节点。
5. 使用多个不同 UID 请求登录，确认分配不会固定集中到单节点。
6. 让玩家进入某节点后再次登录，确认原节点可用时优先返回原节点。
7. 将原节点设为 drain 或停止，确认新登录不再分配到该节点。
8. 对异常退出场景等待节点 TTL，确认过期节点被排除。
9. 确认 LoginServer 无需重启即可感知节点增减。
10. 验收后删除不属于正式配置集的临时文件。

完成条件：

- [ ] 第 11.4 和第 11.5 节相关验收项通过。
- [ ] 新增 GameServer 只需新配置和新进程，不需要修改代码。
- [ ] 单节点 2000 连接硬上限仍由各 GameServer 独立执行。

### 10.7 LS-06 同步架构和运维文档

- 优先级：`P1`
- 依赖：`LS-05`
- 状态：`TODO`

执行步骤：

1. 更新根目录 `README.md` 的双进程启动顺序和命令。
2. 更新 `architecture_v2.md`，将登录模块改为独立进程现状。
3. 更新 `backend_technical_architecture.md` 的进程图、目录、配置、登录时序和职责描述。
4. 更新 `docs/ops/runbook.md` 的端口、环境变量、启动、停止和 smoke 操作。
5. 删除失效的单进程描述，不保留两套冲突口径。
6. 在本文件记录实际验证结果并更新任务状态。

完成条件：

- [ ] 权威文档不再描述 LoginServer 与 GameServer 同进程。
- [ ] 新会话可以仅按 README 和 runbook 启动完整服务。
- [ ] 产品总设计和玩法文档没有被加入技术实现细节。

### 10.8 LS-07 完成全量验证与提交

- 优先级：`P1`
- 依赖：`LS-06`
- 状态：`TODO`

执行步骤：

1. 对所有修改过的 Go 文件运行 `gofmt`。
2. 运行 `go test ./...`。
3. 运行 `go vet ./...`。
4. 运行 `git diff --check`。
5. 检查最终差异，确认没有玩法改动、兼容分支和无关重构。
6. 确认第 12 节完成清单全部满足。
7. 将本文件整体状态和步骤状态更新为 `DONE`，记录验证结果。
8. 创建一次本地提交，不执行 push。

完成条件：

- [ ] 全量测试、vet 和 diff 检查通过。
- [ ] 本任务只有一套最终实现口径。
- [ ] 已本地提交且未 push。

## 11. 测试要求

### 11.1 聚焦测试

至少覆盖：

1. LoginServer 配置正常加载。
2. 配置文件缺失、密钥为空、算法不支持时 Bootstrap 失败。
3. LoginServer HTTP mux 提供 `/api/login`。
4. GameServer HTTP mux 不再提供 `/api/login`。
5. allocator 仍优先可用原节点。
6. 原节点不可用时仍能分配其他节点。
7. 没有可用节点时返回受控错误。
8. LoginServer HTTP 监听端口占用时 `Start` 返回错误。

不要为简单构造函数逐个生成低价值测试。

### 11.2 全量静态验证

阶段完成后执行：

```bash
go test ./...
go vet ./...
```

### 11.3 单节点集成验收

前置环境：MySQL、Redis 可用，并设置 `GAME_DB_DSN`、`GAME_TICKET_SECRET`。

分别启动：

```bash
GAME_CONFIG=configs/gameserver.local.yaml go run ./cmd/gameserver
GAME_CONFIG=configs/loginserver.local.yaml go run ./cmd/loginserver
```

验收：

1. `POST http://127.0.0.1:8080/api/login` 返回 `server_id=node-a`、`ws_addr` 和有效 `enter_ticket`。
2. 客户端使用返回地址建立 WS 并收到 `auth_ack`。
3. GameServer `http://127.0.0.1:8082/healthz` 可访问。
4. GameServer `http://127.0.0.1:8082/api/login` 返回 404。
5. 现有主链路 smoke 可以完成登录、WS 鉴权和玩法请求。

### 11.4 多节点验收

使用第二份临时或测试配置启动 GameServer B，必须使用不同的：

- `node_id`
- WS 监听端口
- Admin HTTP 监听端口
- `advertised_ws_addr`

验收：

1. LoginServer 能从 Redis 看到两个 GameServer，不需要重启。
2. 不同新 UID 可以按现有分配策略落到不同节点。
3. 已有玩家重连时，原节点仍可用则优先返回原节点。
4. 原节点进入 drain、满载或 TTL 过期后，登录分配到其他节点。
5. 单节点 2000 连接硬上限仍由各 GameServer 独立执行。

### 11.5 故障验收

1. Redis 不可用时 LoginServer 不使用静态节点旁路。
2. 没有可用 GameServer 时登录请求返回明确失败，不签发不可用节点票据。
3. GameServer 停止并注销节点后不再被分配。
4. GameServer 异常退出后，等待节点 TTL 到期，不再被分配。
5. LoginServer 停止不应主动断开已经连入 GameServer 的在线玩家。

## 12. 完成标准

以下条件全部满足才可将任务标记为 `DONE`：

- [ ] `cmd/loginserver` 可独立启动和停止。
- [ ] GameServer 不再创建 LoginService。
- [ ] GameServer 不再暴露 `/api/login`。
- [ ] LoginServer 不依赖 MySQL、玩法 Service 或 WebSocket Server。
- [ ] LoginServer 使用 Redis 实时节点表，不使用静态节点列表。
- [ ] `enter_ticket` 的字段、签名和验签行为未改变。
- [ ] 重连原服优先规则未改变。
- [ ] 单节点集成验收通过。
- [ ] 多节点分配验收通过。
- [ ] `go test ./...` 通过。
- [ ] `go vet ./...` 通过。
- [ ] 启动与运维文档已同步。

## 13. 非目标

本任务禁止顺带实现：

- 正式账号表、密码校验、第三方平台登录。
- `account_token`、`refresh_token`、`/api/enter`、`/api/reconnect`。
- AccessGateway、长连接代理或多路复用。
- GlobalServer 独立部署。
- Redis nonce store 或 Redis session manager。
- OnlinePlayerStore、异步刷盘或消息队列。
- Protobuf、TCP 或协议重构。
- 独立数据库 migration 命令。
- 玩法、资产、战斗、工坊、好友、公会、聊天逻辑调整。
- 生产监控、完整 readiness、配置中心或服务编排平台。

发现上述问题时只记录为后续任务，不在本次代码中处理。

## 14. 文档同步范围

实现完成后只同步与本任务直接相关的说明：

- `README.md`：独立进程启动命令。
- `architecture_v2.md`：运行形态从“Demo 登录同进程”更新为独立 LoginServer。
- `backend_technical_architecture.md`：登录进程边界、目录、配置和时序图。
- `docs/ops/runbook.md`：启动顺序、端口和 smoke 命令。
- 本文件：任务状态、完成清单和验证结果。

不要把实现记录写入产品总设计或具体玩法策划文档。

## 15. 提交要求

建议本任务完整实现、测试和文档同步后提交一次：

```text
refactor: split loginserver from gameserver
```

按项目约定，任务完成后可以本地提交；未经用户明确要求不得 push。
