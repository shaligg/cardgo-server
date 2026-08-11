# 架构落地任务拆分（V2 / 后端 MVP）

## 0. 文档状态说明

本文档前半部分保留 V2 基础架构落地历史进度。

当前项目已经新增卡牌休闲游戏策划文档，因此后续任务需要在已完成的 V2 基建之上继续推进“卡牌 MVP 主链路”。

主架构口径以 [architecture_v2.md](/Users/bigfish/Project/go_orm_1/architecture_v2.md) 为准。

后端技术细节以 [backend_technical_architecture.md](/Users/bigfish/Project/go_orm_1/backend_technical_architecture.md) 为准。

MVP 范围口径以 [docs/design/mvp_scope.md](/Users/bigfish/Project/go_orm_1/docs/design/mvp_scope.md) 为准。

## 1. 执行原则
1. 先文档后代码：先完成架构设计、接口契约、协议定稿，再进入编码。
2. 先主链路后扩展：优先打通登录模块、接入鉴权、会话、核心读写链路。
3. 先可观测后优化：先保证可监控可回滚，再做性能细调。
4. 单节点优先：先完成单进程承载 2000 在线目标，不提前拆微服务。
5. 接口先行：登录模块与实时模块通过接口边界交互，后续可独立拆分。
6. 一致性优先：先定义 A/B/C 数据分级与写入规则，再落代码。
7. 全局域优先：`globalcore(Friend/Chat/Guild/Mail/Rank/Notice)` 作为公共领域核心，同进程先落地；`globalserver` 从 MVP 起建立公共服/job 编排边界，但初版不独立启动、不做网络传输层。
8. 迁移白名单：只有技术文档“可迁移模块列表”中的模块才按可远程化边界实现，列表外默认简单本地实现。
9. 范围冻结：超出 MVP 的能力全部进入 backlog，不在当前迭代实现。

## 2. 阶段总览
| 阶段 | 名称 | 输出 | 状态 |
|---|---|---|---|
| P0 | 架构冻结 | 设计文档定稿 | DONE |
| P1 | 工程骨架 | 目录与模块边界落地 | DONE |
| P2 | 协议与接口 | Login API、ticket、WS协议、错误码、接口骨架 | DONE |
| P3 | 主链路实现 | 登录发票 + 接入鉴权 + 会话 + 缓存读写链路 | DONE |
| P4 | 稳定性能力 | 背压、限流、断线重连、刷盘 | DONE |
| P5 | 观测与压测 | 监控告警 + 2k 压测 + 验收报告 | IN_PROGRESS |
| P6 | 发布准备 | runbook、灰度、回滚预案 | IN_PROGRESS |

## 3. 阶段拆分

## 3.1 P0 架构冻结（文档阶段）
### 目标
- 完成 V2 架构文档评审并冻结关键决策。

### 任务
1. 审核并确认 [architecture_v2.md](/Users/bigfish/Project/go_orm_1/architecture_v2.md)。
2. 输出协议附录（字段约束、错误码、兼容策略）。
3. 输出配置附录（本地/测试/生产最小配置集合）。
4. 输出风险清单（容量、数据一致性、故障恢复）。
5. 冻结数据分级清单（A/B/C）与幂等策略（`req_id` + 唯一键）。
6. 冻结 `globalcore` 公共领域核心与 `globalserver` 公共服/job 编排边界。
7. 冻结事件可靠性方案（同进程重试队列 + DLQ，后续 Outbox + MQ 迁移）。
8. 冻结 MVP 边界（本期实现项与延期项）并评审通过。

### 交付物
1. `architecture_v2.md`（定稿）
2. `protocol.md`（可选，若拆分独立文档）
3. `risk_register.md`（可选）

### 验收
1. 关键问题无 open blocker。
2. 模块边界和职责无冲突。
3. 编码所需输入信息完整。

## 3.2 P1 工程骨架
### 目标
- 将架构映射为工程结构与模块依赖关系。

### 任务
1. 按 V2 文档创建分层目录骨架：`framework` 放网关/分发/传输，`platform` 放登录/鉴权/会话/在线状态，`game` 放玩法业务，`infra` 放基础设施；`internal/pkg` 只作为纯通用工具的预留规则，不预创建空包。
2. 增加 `platform/login` 模块骨架（handler/allocator/ticket issuer）。
3. 定义模块依赖规则（禁止反向依赖、禁止跨层直连，`internal/pkg` 只能被引用，不能引用业务/框架/平台/仓储/基础设施）。
4. 创建统一错误码、日志字段、trace_id 规范。
5. 建立配置加载与环境注入机制。
6. 完成 login/realtime 资源隔离骨架（独立 worker 池与限流配置）。

### 交付物
1. 工程目录骨架
2. 模块依赖约束说明
3. 基础配置加载能力

### 验收
1. 所有模块可编译（空实现允许）。
2. 依赖关系符合架构主线。

## 3.3 P2 协议与接口
### 目标
- 完成接入协议与核心接口契约实现骨架。

### 任务
1. 实现 ticket verifier 接口与 nonce 一次性消费接口。
2. 定义 `LoginProvider / NodeAllocator / TicketIssuer` 接口。
3. 定义并实现 Login API、WS 通用消息结构与编解码。
4. 定义 session manager、repo、cached repo 接口。
5. 完成错误码映射（协议层 -> 业务层 -> 客户端）。
6. 明确 `SERVER_FULL` 单节点行为（候选节点可为空）。

### 交付物
1. `platform/login` 与 `platform/auth` 接口及基础实现
2. `framework/transport/dto` 与 `framework/transport/errors`
3. `platform/session`、`repo`、`infra/cache` 接口骨架

### 验收
1. Login API + 首帧 auth 协议可在本地冒烟通过。
2. 错误码可稳定返回并可观测。

## 3.4 P3 主链路实现
### 目标
- 打通“接入 -> 会话 -> 读写 -> 回包”主链路。

### 任务
1. 登录链路：登录模块认证后签发 ticket。
2. 接入链路：auth(ticket) 成功后绑定 session。
3. 读链路：Service -> CachedRepository -> Repository -> DB。
4. 写链路：DB 事务写入 + 缓存失效/更新。
5. 连接上限控制：`max_connections=2000`，超限返回 `SERVER_FULL`。
6. A类数据写入必须事务 + 幂等；禁止走“仅内存后刷盘”。

### 交付物
1. 登录 + 接入主链路可跑通
2. 基础业务接口（示例：玩家信息查询/资产增减）

### 验收
1. 登录 + 接入 + 业务 e2e 用例通过。
2. 连接超限策略符合设计。

## 3.5 P4 稳定性能力
### 目标
- 补齐高并发下的生存能力。

### 任务
1. 每连接发送队列与优先级策略。
2. 背压、限流、慢客户端降级或踢线。
3. dispatcher 分片执行（按路由键：player/guild/channel）。
4. 断线重连恢复与异步刷盘队列。
5. 多路由键分片落地（player/guild/channel）。
6. Redis 故障时的鉴权熔断与渐进恢复策略。
7. globalcore 保留公共领域核心边界，承载接口、DTO、Local/Remote 适配和可复用规则；globalserver 建立结算/批处理编排边界，但不作为独立进程进入 MVP 主链路。

### 交付物
1. 稳定性中间件能力
2. 断线重连策略实现

### 验收
1. 慢客户端场景不拖垮整体服务。
2. 重连流程稳定，状态恢复正确。

## 3.6 P5 观测与压测
### 目标
- 以指标验证单节点 2000 在线目标。

### 任务
1. 打点：连接数、认证率、队列积压、P95/P99、DB/Redis RT。
2. 告警：认证失败率、延迟超阈、队列溢出、错误率。
3. 压测场景执行（S1~S5，见架构文档）。
4. 输出压测报告与瓶颈分析。

### 当前进度（2026-03-24）
1. DONE：`scripts/loadtest/k6_2k_online.js` 已实现 S1/S2/S3 参数化压测脚本（登录、WS 鉴权、心跳、业务请求、SERVER_FULL 判定、阈值）。
2. DONE：`docs/ops/runbook.md` 已补齐压测执行步骤与验收门槛。
3. DONE：`docs/ops/loadtest_report.md` 已提供统一压测报告结构（S1~S3）。
4. DONE：`scripts/loadtest/k6_report` 已实现 k6 summary 自动汇总与 Gate 判定报告输出。
5. DONE：已完成缩时压测与结果汇总，报告见 `reports/loadtest_report_20260324_short.md`。
6. DONE：修正压测脚本尾部采样偏差（`BIZ_STOP_BEFORE_CLOSE_MS` 默认 15000ms），S1 回归门槛通过。
7. TODO：在独立压测机按 `docs/ops/runbook.md` 默认 stages 执行 S1/S2/S3，并回填最终验收结论。

### 交付物
1. 监控看板
2. 压测脚本
3. 压测报告

### 验收
1. 达到 SLO 阈值（认证成功率、延迟、稳定性）。
2. 无 OOM/崩溃。

## 3.7 P6 发布准备
### 目标
- 具备可灰度、可回滚、可运维能力。

### 任务
1. 编写 runbook（发布、回滚、扩容、故障处置）。
2. 实现 drain 模式并验证。
3. 预发灰度演练（5% -> 20% -> 50% -> 100%）。
4. 故障注入演练（Redis抖动、DB慢查询、网络丢包）。

### 当前进度（2026-03-24）
1. DONE：`docs/ops/runbook.md` 已补齐启动、冒烟、压测、drain、回滚步骤。
2. DONE：新增 `POST/GET /admin/drain` 运行时开关并完成本地端到端验证。
3. DONE：新增 `GET /admin/sessions` 在线会话数查询，支持 drain 期间观测存量连接。
4. DONE：`/healthz` 已返回 `drain_mode` 与 `ready` 联动状态。
5. TODO：灰度演练与故障注入演练记录（需预发环境执行）。

### 交付物
1. 发布与回滚手册
2. 演练记录

### 验收
1. 灰度可控，回滚路径清晰。
2. 关键故障场景有明确处置方案。

## 4. 任务依赖图（简化）
```text
P0 -> P1 -> P2 -> P3 -> P4 -> P5 -> P6
         \____________________________^
```

说明：
- P5 压测发现问题后，允许回流到 P3/P4 修复再压测。

## 5. 角色建议
| 角色 | 职责 |
|---|---|
| 架构负责人 | 设计定稿、评审主持、关键取舍 |
| 服务端开发 | P1~P4 主实现 |
| QA/测试 | 用例、回归、压测执行 |
| SRE/运维 | 监控告警、灰度、发布回滚 |

## 6. 风险与缓解
1. 风险：缓存与数据库一致性缺陷。
- 缓解：写路径统一“先库后缓存”，加幂等键与审计日志。

2. 风险：慢客户端拖垮广播链路。
- 缓解：发送队列上限、优先级、主动踢线。

3. 风险：重连风暴导致瞬时过载。
- 缓解：登录模块与接入层双侧限流 + 指数退避重试。

4. 风险：压测结果不达标。
- 缓解：按 P3/P4 回流迭代，先瓶颈定位再优化。

## 7. 当前后续任务（2026-08-11）
1. DONE：P0-P4 与卡牌 MVP B0-B7 的本地设计、实现、集成验收和旧代码清理已经完成。
2. TODO：在独立压测机按 `docs/ops/runbook.md` 默认 stages 执行 P5 正式压测，回填最终容量结论。
3. TODO：在预发环境执行 P6 灰度发布和故障注入演练，补充演练记录。
4. P5/P6 得出结论前，不继续增加架构层或预留模块；发现瓶颈后按 P3/P4 回流做针对性修复。

## 8. 卡牌 MVP 后续任务拆分

### 8.1 B0 文档重构与架构冻结
目标：

- 将通用游戏服架构对齐到卡牌休闲 MVP。
- 明确哪些系统进入主链路，哪些完整业务后置但先建立代码边界。

任务：

1. 重构 `architecture_v2.md`，补齐卡牌、订单、工坊、经济模块映射。
2. 对齐 `docs/design/mvp_scope.md` 中的 Prototype/MVP 范围。
3. 确认 `globalcore` 作为 Friend/Chat/Guild/Mail/Rank/Notice 公共领域核心，`globalserver` 建立 MVP 编排边界，用于结算/批处理/未来独立公共服逻辑。
4. 冻结 MVP `op_code` 范围。

验收：

- 架构文档能指导后续代码目录、接口和数据表设计。
- 没有把 P1/P2 社交和活动系统混入 MVP 主链路。

### 8.2 B1 玩家与资产主链路

当前进度：DONE。已完成金币 `player_field` 通过 `AssetService` 发放/扣除、幂等记录、资产流水和 WS smoke；`inventory_stack` 留到 B2 接入。
目标：

- 打通新玩家初始化、资产查询、统一发奖扣费。

任务：

1. DONE：整理 `player_profile`、`asset_log`、`idempotency_record`，高频基础货币继续存玩家基础表。
2. DONE：实现 `AssetService.Grant` 的金币最小链路。
3. DONE：实现 `AssetService.Consume` 的金币最小链路。
4. MOVED：通用可堆叠背包 `player_item / inventory_stack` 移入 B2。
5. DONE：保留当前 `add_gold/consume_gold` 为 debug 接口，并改为经由 `AssetService`。
6. DONE：增加资产 repo 事务测试，覆盖幂等重试、资产流水、余额不足无副作用。
7. DONE：启动游戏服后执行 WS 资产 smoke test，验证加金币、扣金币、余额不足、幂等重试和资产流水落库。

验收：

- 新账号登录后能自动初始化玩家资料和资产。
- 同一 `req_id` 重试不会重复发奖或扣费。
- 资产变化有流水可查。

### 8.3 B2 背包可堆叠道具与配置层起步
当前进度：DONE。已完成 `player_item / inventory_stack` 最小闭环、`ItemConfig` JSON 化、`gamedata` 配置加载与跨引用校验框架；`AssetService` 已支持事务内批量发奖/扣费与相同 `item_id` 聚合。Prototype 已有可运行数值，后续数值平衡属于策划调优，不阻塞工程闭环。

目标：

- 支持通用可堆叠道具，例如基础材料、碎片、消耗券。
- 让卡牌、订单、关卡、奖励、消耗逐步从配置驱动。

任务：

1. DONE：新增 `player_item / inventory_stack` repo 写链路。
2. DONE：`AssetService` 支持 `ItemBasicMaterial=10001` 的发放和扣除。
3. DONE：新增 WS debug op：`asset.grant_item(1101)`、`asset.get_inventory(1102)`、`asset.consume_item(1103)`。
4. DONE：新增 `ws_inventory_smoke` 并完成端到端验证。
5. DONE：新增 `internal/gamedata` 配置加载模块。
6. DONE：定义 `ItemConfig`、`CardConfig`、`OrderConfig`、`LevelConfig`、`RewardConfig`、`CostConfig`。
7. DONE：先落地 Prototype 内容；已补齐 20 张卡、10 个订单、10 个关卡 JSON，并接入启动校验。
8. DONE：配置启动校验，服务启动时加载 `ItemConfig/CardConfig/OrderConfig/LevelConfig` 并做重复 ID、缺字段、引用不存在校验；配置错误时启动失败。
9. DONE：`AssetService.ApplyRewardInTx/ApplyCostInTx` 支持同一事务内处理多种资产，并在资产层合并相同 `item_id`。
10. DONE：卡牌升级消耗和设施升级消耗已接入配置，离线收益已接入金币和基础材料发奖。

验收：

- 可堆叠道具同一 `req_id` 重试不会重复发放或扣除。
- 可堆叠道具变化有资产流水可查。
- 新增卡牌/订单/关卡不需要改核心逻辑代码。
- 配置 ID 冲突、缺字段、引用不存在能被校验出来。

### 8.4 B3 关卡主链路
当前进度：DONE。已完成内存版关卡运行时、`level.start(1301)`、`level.play_card(1302)`、`level.settle(1303)`、结算事务发奖和 WS smoke；玩家卡组读取已移入 B4，B3 使用关卡 `fixed_cards` 作为 MVP 关卡入口。

目标：

- 打通 `level.start -> level.play_card -> level.settle`。

任务：

1. MOVED：实现玩家卡组读取，移入 B4 卡牌库存与卡组编辑；B3 先使用关卡 `fixed_cards`。
2. DONE：实现关卡开始与局内状态创建。
3. DONE：实现最小卡牌效果执行。
4. DONE：实现订单完成判定。
5. DONE：实现关卡结算并在事务内调用 `AssetService.ApplyRewardInTx`。
6. DONE：增加关卡 smoke test。

验收：

- 新账号可以进入第 1 关。
- 可以打出卡牌并推进局内状态。
- 可以完成订单并结算金币。
- 重复结算不会重复发奖。

### 8.5 B4 卡牌成长与卡组编辑
前置整理：

- 在进入更多卡牌协议前，先完成 WS 业务协议分发结构整理。
- 目标是把当前集中在 `biz_handler.go` 的协议号、路由表、具体 handler 拆成统一协议号表、统一路由注册表和模块 Handler，避免后续协议数量增长后重构成本变高。

协议分发整理任务：

1. DONE：技术文档补充协议分发实现规则。
2. DONE：`gateway/ws` 只保留 `BizHandler` 抽象入口，不依赖具体业务模块。
3. DONE：新增 `internal/contract/protocol/opcode.go`，集中维护全量 `op_code`。
4. DONE：新增纯 `bizRouter`，只负责 `op_code -> handler`。
5. DONE：新增 `biz_routes.go`，集中维护协议号到模块 Handler 函数的绑定。
6. DONE：拆出 `player_handler.go`、`asset_handler.go`、`level_handler.go`。
7. DONE：删除旧的混合职责 `biz_handler.go`。
8. DONE：跑通全量 Go 测试与现有 WS smoke 编译。
9. DONE：将请求协议号从 `payload.op_code` 上移到 Envelope `op_code`，payload 只保留业务参数。
10. DONE：新增 `EnvelopeCodec` 抽象，当前使用 `JSONEnvelopeCodec`，为后续 protobuf/binary 替换预留边界。
11. DONE：将业务协议入口从 `internal/app` 迁移到 `internal/handler`，`app` 回归应用装配、生命周期和配置职责。
12. DONE：将 gameserver 启动装配从 `internal/app` 迁移到 `internal/app/gameserver`，让应用层显式表达进程边界。
13. DONE：拆出 `internal/app/gameserver/admin_http.go`，收敛 health、metrics、drain、sessions 和 login API 路由组装。
14. DONE：删除 `internal/game/chat`、`internal/game/guild`、`internal/game/rank` 空壳，避免公共领域能力和本地玩法目录边界混淆；后续聊天、公会、排行入口以 `globalcore/*` 为准。
15. DONE：新增 `internal/globalserver` 最小契约包，先落地排行榜结算、批量邮件和通用 Job 接口；MVP 同进程直调，未来独立公共服时在接口外层增加 transport adapter。
16. DONE：补齐 `internal/globalcore` 的 Friend/Mail/Notice 接口契约，与技术文档中的公共领域核心清单对齐；只定义 DTO 和接口，不实现完整业务。
17. DONE：删除旧 `WorldService` 公告口径，公告统一收敛到 `NoticeService`，避免公共领域核心出现两套命名。

目标：

- 支撑卡牌库存、卡组保存、卡牌升级到 Lv.5。

任务：

1. DONE：实现 `player_card`，当前同一种卡牌聚合成一条拥有记录。
2. DONE：实现 `player_deck`，MVP 使用 JSON 保存卡牌 ID 列表，后续需要复杂检索时再拆明细表。
3. DONE：实现 `card.get_cards(1201)`，新玩家会自动补齐初始 5 张卡。
4. DONE：实现 `card.save_deck(1202)`，校验数量、重复卡、配置存在和玩家拥有关系。
5. DONE：实现 `card.upgrade(1203)`，升级消耗已接入 `CardConfig.upgrade_costs`，并通过统一资产扣费和幂等落库；具体数值后续按策划表继续调优。
6. DONE：新增 `internal/game/card` 单元测试，覆盖查询、合法卡组、非法卡组、升级扣费。

验收：

- DONE：玩家可以查询卡牌库存。
- DONE：玩家可以保存合法卡组。
- DONE：非法卡组会被拒绝。
- DONE：卡牌升级走统一扣费和幂等。

### 8.6 B5 工坊 MVP
当前进度：DONE。已完成工坊总览、设施升级、设施升级配置、离线收益预览、离线收益领取和可控离线收益 smoke。

目标：

- 支撑工坊总览、设施升级、离线收益。

任务：

1. DONE：实现 `player_workshop` 数据模型并接入迁移。
2. DONE：实现 `player_facility` 数据模型并接入迁移。
3. DONE：实现 `workshop.get_overview(1401)` 基础链路，返回默认工坊、已有设施列表和离线收益预览；设施配置已接入，升级红点后续按客户端展示需求补充。
4. DONE：实现 `workshop.upgrade_facility(1402)`，设施升级消耗已接入 `FacilityConfig.levels.upgrade_costs`，并通过统一资产扣费和幂等落库；具体数值后续按策划表继续调优。
5. DONE：实现 `workshop.claim_offline_reward(1403)`，MVP 按离线时长批量发放金币和基础材料，并推进 `last_offline_reward_at`；具体收益数值后续按策划表继续调优。

验收：

- 新玩家有默认工坊数据。
- 设施升级会扣资源并提升等级。
- 离线收益可预览和领取。
- 重复领取不会重复发奖。

### 8.7 B6 Prototype 集成验收
目标：

- 完成从登录到第一关结算、卡牌升级、工坊升级的闭环。

任务：

1. DONE：通过 `ws_prototype_smoke` 串联登录、玩家初始化、关卡、结算、资产、卡牌、工坊和重登恢复。
2. DONE：新增 `scripts/loadtest/ws_prototype_smoke`，并完成本地端到端验证。
3. DONE：修复主链路错误码和日志；已完成 GORM `record not found` 正常查询日志降噪，并补充业务错误码分组映射。
4. DONE：输出 Prototype 验收记录 `docs/ops/prototype_acceptance_report.md`。
5. DONE：新增 `scripts/loadtest/ws_offline_reward_smoke`，验证可控 2 小时离线收益会发放金币和基础材料。
6. DONE：新增 `debug.enable_ws_debug_ops` 配置开关，本地默认开启，staging/prod 默认关闭 debug WS 协议。

验收：

```text
新账号
  -> 登录
  -> 获取玩家资料
  -> 进入第 1 关
  -> 出牌
  -> 完成订单
  -> 结算金币
  -> 升级卡牌
  -> 升级工坊
  -> 重新登录后数据存在
```

### 8.8 B7 历史原型目录清理
当前进度：DONE（2026-08-11）。早期原型已迁移到同级历史项目 `../go_orm_1_history`，当前项目不再保留旧运行链路和重复启动入口。

目标：

- 移除会干扰阅读和 IDE 跳转的历史原型代码。
- 保证新增功能只沿 `cmd/gameserver` 与 `internal/*` 当前架构入口实现。

迁移结果：

| 原目录 | 当前判断 | 处理结果 |
|---|---|---|
| 根目录 `main.go` | 与 `cmd/gameserver/main.go` 完全相同的重复入口 | 已删除，历史项目保留同内容参考源码 |
| `ws/` | 早期 WS 原型，当前入口已迁移到 `internal/framework/gateway/ws` | 已迁移到历史项目 |
| `session/` | 早期 session 原型，当前入口已迁移到 `internal/platform/session` | 已迁移到历史项目 |
| `db/` | 早期 DB 初始化原型，当前入口已迁移到 `internal/infra/db` | 已迁移到历史项目 |
| `redis/` | 早期 Redis 原型，当前入口已迁移到 `internal/infra/redis` | 已迁移到历史项目 |
| `config/` | 早期配置原型，当前配置由 `internal/app/gameserver` 和 `configs/` 承载 | 已迁移到历史项目 |
| `example/` | 重复 GameServer 启动入口，不属于正式入口 | 已作为只读源码参考迁移 |
| `test/` | 早期手写实验入口，不属于当前 Go 测试结构 | 已迁移到历史项目 |
| `gm_backend/` | 使用模拟数据的独立 GM 示例，没有接入当前业务和数据库 | 已迁移到历史项目并保留独立 module |

验收：

- DONE：`go list ./...` 不再出现根目录旧原型包。
- DONE：`go test ./...` 只覆盖当前架构代码和 loadtest 工具。
- DONE：历史项目根 module 与其 `gm_backend` 独立 module 均可通过 `go test ./...`。
- DONE：技术文档不再把旧目录作为当前可用入口。
