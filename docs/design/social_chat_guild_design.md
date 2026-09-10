# 轻社交系统设计

## 1. 定位

轻社交用于提供关系、归属和低压力沟通，不作为主线成长的强制门槛。

当前版本完成三个基础闭环：

- 好友：申请、同意、删除、关系列表。
- 公会：创建、搜索、申请、申请列表、审批、退出、详情与成员列表。
- 聊天：世界频道、公会频道、历史消息分页。

好友互助奖励、公会成长玩法和实时消息推送属于后续扩展，不混入本次基础闭环。

## 2. 好友系统

### 2.1 关系状态

| 服务端状态 | 申请者看到 | 接收者看到 |
|---|---|---|
| `pending` | `outgoing` | `incoming` |
| `accepted` | `accepted` | `accepted` |

同一对玩家只保存一条关系记录，不因为查看方向重复保存两行。

### 2.2 规则

1. 不能申请自己。
2. 目标玩家必须存在。
3. 已有待处理申请时不能重复申请；已有好友关系时不能重复添加。
4. 只有申请接收者可以同意申请。
5. 删除操作既可以解除好友，也可以撤销尚未处理的申请。
6. 请求中的操作玩家始终取自登录票据绑定的 UID，客户端只能提交目标 UID。

### 2.3 列表

好友列表同时返回已接受关系和待处理申请，使用不透明 `cursor` 正向分页。客户端不得解析或修改 cursor，只需在下一页请求中原样回传。

当前项目尚未实现独立昵称系统，列表暂时用 UID 作为展示名；接入玩家资料昵称后只替换资料读取，不修改好友关系结构。

## 3. 公会系统

### 3.1 基础流程

```text
创建者创建公会
  -> 自动成为 leader
  -> 其他玩家搜索公会
  -> 提交申请
  -> leader 查看待审批申请
  -> leader 审批
  -> 玩家成为 member
```

### 3.2 成员与权限

当前职位只有：

- `leader`：会长，可以审批入会申请。
- `member`：普通成员，可以查看详情和使用公会聊天。

一名玩家只能属于一个公会。玩家被批准加入或自行创建公会后，其他公会的待处理申请全部删除。

### 3.3 退出与解散

- 普通成员退出时直接删除成员关系。
- 会长退出且仍有成员时，职位转让给最早加入的成员。
- 会长退出且没有其他成员时，解散公会，同时删除待处理申请和该公会聊天历史。

### 3.4 查询

- 公会搜索支持名称模糊匹配和 cursor 分页。
- 搜索结果返回成员数量，以及当前玩家对该公会的 `none`、`applied` 或 `member` 状态。
- 公会详情返回会长、当前玩家职位和成员列表。
- 只有会长可以分页查看待审批申请；列表返回申请玩家 UID、等级、展示名和申请时间。
- 查询时不依赖某个 GameServer 的进程内状态，多个节点共享 MySQL 权威数据。

## 4. 聊天系统

### 4.1 频道

当前客户端只能传逻辑频道：

- `world`：世界频道，对所有已认证玩家开放。
- `guild`：当前玩家所在公会频道。

服务端根据当前公会成员关系把 `guild` 转换为真实 `guild:<guild_id>`，客户端不能指定其他公会 ID，从入口阻止越权读写。

系统频道由后续公告模块写入，客户端不能发送；私聊和跨服实时推送本期不做。

### 4.2 消息规则

1. 消息去除首尾空白后必须为 1 到 200 个 Unicode 字符。
2. 写请求必须携带 `req_id`，普通网络重试由 Dispatcher 近期结果缓存处理。
3. 消息写入 MySQL 后才返回成功。
4. 历史消息从最新页向更早页翻页，单页内按发送时间正序返回。
5. 公会频道的发送和拉取都必须实时校验成员关系。

当前已有 WS 入口请求限频；聊天专用敏感词、禁言、举报、消息保留周期和独立频控在正式社交运营前补充。

## 5. 后端边界

```text
WS Gateway
  -> Dispatcher（鉴权 UID、分片串行、近期结果缓存）
  -> Router
  -> Social Handler（解析协议 DTO）
  -> globalcore Friend/Guild/Chat 接口
  -> LocalService（规则与权限）
  -> Repository
  -> MySQL
```

约束：

- `globalcore` 不引用 WebSocket、连接、Session 或 GameServer 私有运行态。
- 当前 GameServer 在启动时注入 LocalService。
- 将来独立部署公共服务时，GameServer 只把接口实现替换为 RemoteClient；Handler、协议 DTO 和核心语义不变。
- 好友、公会和聊天是共享公共数据，不放入单个 GameServer 的进程内缓存作为权威状态。

正式协议号、请求 DTO、错误码和数据表以 [backend_technical_architecture.md](../../backend_technical_architecture.md) 为准。

### 5.1 当前协议

| op_code | 协议 | 请求 payload | 主要返回 |
|---:|---|---|---|
| 1601 | `friend.apply` | `target_uid, req_id` | `target_uid, status=outgoing` |
| 1602 | `friend.approve` | `target_uid, req_id` | `target_uid, status=accepted` |
| 1603 | `friend.remove` | `target_uid, req_id` | `target_uid, status=removed` |
| 1604 | `friend.list` | `cursor?, limit?` | `friends, next_cursor` |
| 1701 | `guild.create` | `name, req_id` | `guild` |
| 1702 | `guild.search` | `keyword?, cursor?, limit?` | `guilds, next_cursor` |
| 1703 | `guild.apply_join` | `guild_id, req_id` | `guild_id, status=applied` |
| 1704 | `guild.approve_join` | `guild_id, target_uid, req_id` | `guild_id, target_uid, status=joined` |
| 1705 | `guild.leave` | `req_id` | `status=left` |
| 1706 | `guild.get` | `guild_id?` | `guild` |
| 1707 | `guild.list_applications` | `guild_id, cursor?, limit?` | `applications, next_cursor` |
| 1801 | `chat.send` | `channel, content, req_id` | `message` |
| 1802 | `chat.history` | `channel, cursor?, limit?` | `messages, next_cursor` |

`cursor` 是服务端生成的不透明字符串；`limit` 默认为 20，范围为 1 到 50。写请求的 `req_id` 由客户端生成，用于短期重试结果复用和审计。

## 6. 暂缓内容

- 好友互送、点赞、借用猫咪、订单互助和友情币。
- 好友工坊参观与好友排行榜。
- 公会签到、捐献、任务、商店和公会战。
- 副会长、管理员、踢人和转让会长的手动操作。
- 私聊、跨服实时推送、离线未读数、敏感词、禁言和举报。
- 聊天 Redis 热历史、独立 ChatService 和消息队列。

这些能力出现真实需求后在现有领域内扩展，不提前创建空模块。

## 7. 验收标准

1. 双方能看到方向正确的好友申请，同意后双方均为好友，删除后双方均不可见。
2. 同一玩家无法加入或创建第二个公会。
3. 非会长无法审批申请，会长审批后成员和申请状态一致。
4. 会长退出后正确转让或解散，不留下无会长公会。
5. 非成员不能发送或读取公会消息，成员只能访问自己的公会频道。
6. 世界与公会消息写入成功后可分页读取，翻页无重复。
7. payload 中伪造操作 UID 不会改变实际执行玩家。
8. LocalService 不依赖 GameServer 私有运行时对象，可在未来公共服务进程中复用。
