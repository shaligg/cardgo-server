# 文档索引

本文档目录按用途拆分：

- `design/`：产品总纲、版本范围和单功能策划文档。
- `ops/`：运行手册、压测模板和测试执行文档。
- `architecture/`：架构方案调研和对比文档，用于评审决策；最终口径仍需同步到根目录技术架构文档。
- `tasks/`：可交给独立会话执行的技术任务文档；每个任务只保留一份范围、步骤和验收口径。
- `archive/`：历史旧文档，仅用于追溯，不作为当前设计和实现依据。

后端架构入口仍保留在项目根目录：

- `architecture_v2.md`：项目级后端架构总览。
- `backend_technical_architecture.md`：后端技术架构细节。
- `architecture_v2_task_breakdown.md`：基础架构与 Prototype 主链路落地记录。

独立技术任务：

- [独立 LoginServer 拆分](tasks/loginserver_split.md)：DONE，保留范围与验收记录；当前双进程启动、配置和运维命令见 [runbook](ops/runbook.md)。
- [正式账号域建设](tasks/account_domain.md)：TODO，首次上线前完成账号身份、登录态、刷新令牌与入场票边界。

维护规则：

- `docs/design/card_casual_game_design.md` 只维护产品定位、核心循环、系统地图和全局原则，不写具体玩法规则。
- `docs/design/mvp_scope.md` 只维护当前版本范围、里程碑和出口标准。
- 具体玩法、活动或系统规则写入独立功能策划文档；已有系统沿用 `docs/design/*_design.md`，单个活动使用 `docs/design/events/<event_name>.md`。
- 运维、压测、发布、回滚说明写入 `docs/ops/`。
- 架构调研、目录方案对比等评审材料写入 `docs/architecture/`。
- 跨模块技术改造在 `docs/tasks/` 建立独立任务文档，完成后在原文件记录状态和验证结果。
- 协议、接口、目录、数据表、迁移白名单等后端技术细节只维护在 `backend_technical_architecture.md`。
- `architecture_v2_task_breakdown.md` 保留基础架构落地记录，不再追加具体玩法任务。
- `docs/archive/` 中的文档不得被后续实现引用；如需恢复其中内容，必须先同步到当前权威文档。

新任务读取规则：

1. 如果存在对应的 `docs/tasks/<task_name>.md`，先读取本索引和该任务文档，并严格使用其中列出的补充阅读范围。
2. 没有独立任务文档时，先读取本索引和 `docs/design/card_casual_game_design.md`。
3. 再读取 `docs/design/mvp_scope.md` 与本任务对应的功能策划文档。
4. 后端任务只按需要读取 `architecture_v2.md` 和 `backend_technical_architecture.md` 的相关章节，不默认加载全部文档。
5. 如果对应功能策划文档不存在，先新建并完成规则评审，再拆代码任务。
6. 功能任务状态记录在对应功能策划文档；跨模块技术任务状态记录在对应任务文档，不回填到总设计或基础架构任务记录。
