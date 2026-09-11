# 文档索引

本文档目录按用途拆分：

- `design/`：产品总纲、版本范围和单功能策划文档。
- `ops/`：运行手册、压测模板和测试执行文档。
- `architecture/`：架构方案调研和对比文档，用于评审决策；最终口径仍需同步到根目录技术架构文档。
- `archive/`：历史旧文档，仅用于追溯，不作为当前设计和实现依据。

后端架构入口仍保留在项目根目录：

- `architecture_v2.md`：项目级后端架构总览。
- `backend_technical_architecture.md`：后端技术架构细节。
- `architecture_v2_task_breakdown.md`：基础架构与 Prototype 主链路落地记录。

维护规则：

- `docs/design/card_casual_game_design.md` 只维护产品定位、核心循环、系统地图和全局原则，不写具体玩法规则。
- `docs/design/mvp_scope.md` 只维护当前版本范围、里程碑和出口标准。
- 具体玩法、活动或系统规则写入独立功能策划文档；已有系统沿用 `docs/design/*_design.md`，单个活动使用 `docs/design/events/<event_name>.md`。
- 运维、压测、发布、回滚说明写入 `docs/ops/`。
- 架构调研、目录方案对比等评审材料写入 `docs/architecture/`。
- 协议、接口、目录、数据表、迁移白名单等后端技术细节只维护在 `backend_technical_architecture.md`。
- `architecture_v2_task_breakdown.md` 保留基础架构落地记录，不再追加具体玩法任务。
- `docs/archive/` 中的文档不得被后续实现引用；如需恢复其中内容，必须先同步到当前权威文档。

新任务读取规则：

1. 先读取本索引和 `docs/design/card_casual_game_design.md`。
2. 再读取 `docs/design/mvp_scope.md` 与本任务对应的功能策划文档。
3. 后端任务只按需要读取 `architecture_v2.md` 和 `backend_technical_architecture.md` 的相关章节，不默认加载全部文档。
4. 如果对应功能策划文档不存在，先新建并完成规则评审，再拆代码任务。
5. 功能任务状态记录在对应功能策划文档，不回填到总设计或基础架构任务记录。
