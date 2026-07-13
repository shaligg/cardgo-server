# Docs Index

本文档目录按用途拆分：

- `design/`：策划、产品范围、系统设计与数值设计文档。
- `ops/`：运行手册、压测模板和测试执行文档。
- `architecture/`：架构方案调研和对比文档，用于评审决策；最终口径仍需同步到根目录技术架构文档。
- `archive/`：历史旧文档，仅用于追溯，不作为当前设计和实现依据。

后端架构入口仍保留在项目根目录：

- `architecture_v2.md`：项目级后端架构总览。
- `backend_technical_architecture.md`：后端技术架构细节。
- `architecture_v2_task_breakdown.md`：架构落地与开发任务拆分。

维护规则：

- 策划玩法和产品范围写入 `docs/design/`。
- 运维、压测、发布、回滚说明写入 `docs/ops/`。
- 架构调研、目录方案对比等评审材料写入 `docs/architecture/`。
- 协议、接口、目录、数据表、迁移白名单等后端技术细节只维护在 `backend_technical_architecture.md`。
- `docs/archive/` 中的文档不得被后续实现引用；如需恢复其中内容，必须先同步到当前权威文档。
