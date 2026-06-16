# Release Notes

本文件记录 `digital-twin` 仓库的文档、工程基线和后续产品版本变化。

## Unreleased

### Added

- 新增 Phase 2 人格、路由、Skill 与 Agent 的 SDD 规格文档，明确 persona、prompt renderer、persona guard、router、Skill framework 和 expert agents 的范围。
- 新增 Phase 2 office-hours 设计文档，记录隐藏假设、替代方案、推荐架构、失败模式、测试矩阵和小步执行顺序。
- 新增 Phase 2 autoplan 执行计划文档，记录 CEO/Eng/DX review 摘要、任务拆解、并行策略、测试图、失败模式和决策审计。
- 新增 `internal/persona` Persona 模型、校验、确定性 system prompt renderer、golden fixture 和 persona guard。
- 新增 `internal/router` rule router、LLM JSON classifier router 和 hybrid router。
- 新增 `internal/skills` 参数校验框架，以及 memory、knowledge、task、tool、persona、safety、presentation deterministic skills。
- 新增 `internal/agents` BaseAgent、PersonaAgent、MemoryAgent、KnowledgeAgent、TaskAgent、ToolAgent 和 SafetyAgent。
- 新增 Phase 2 intent names：`persona.chat` 与 `safety.check`。

### Documented

- README 新增 Phase 2 SDD 文档入口，包括 spec、design 和 plan，并将当前状态更新为 Phase 2 已完成。

### Notes

- Phase 2 实现仍然不包含 Phase 3 runtime/API、Web UI、真实 TTS/ASR、真实 Avatar、SQLite 或外部 provider 接入。

## v0.3.0 Phase 1 Core Contracts and Infrastructure — 2026-06-15

### Added

- 新增 Phase 1 内核契约与基础设施 SDD 规格文档，明确数据契约、核心接口、mock/fake、Registry、LLM 抽象、存储、向量库和记忆基础设施的范围。
- 新增 Phase 1 设计文档，记录无外部依赖优先、接口归属、fake 策略、Registry 行为、LLM retry、本地文件 store、向量检索和记忆流设计。
- 新增 Phase 1 执行计划文档，按 SDD Stage 2 拆分小步任务、测试矩阵、失败模式和 approval gate。
- 新增 `pkg/types` 数据契约，包括 Message、Conversation、Intent、AgentResult、SkillResult、UserProfile、Tenant、Role、Confidence 和 Metadata。
- 新增 `internal/core` Agent、Skill、Router、Orchestrator 接口、Registry 和扩展领域错误。
- 新增 `internal/testutil` 手写 fake，覆盖 core、LLM、Store、VectorStore、Memory 和 EventBus。
- 新增 `internal/llm` provider-neutral client、retry decorator 和 OpenAI-compatible chat client，本地 fake server 测试覆盖请求与响应解析。
- 新增 `internal/store` 本地文件 Store、in-memory fake Store 和 in-memory VectorStore。
- 新增 `internal/memory` 短期窗口和长期记忆组合能力。
- 新增 `internal/runtime` 本地 EventBus。

### Documented

- README 新增 Phase 1 SDD 文档入口，并将项目状态更新为 Phase 1 已完成。
- 明确现阶段不使用 SQLite，Phase 1 采用本地文件存储，内存实现仅作为测试 fake 或轻量实现。

### Verified

- `go test ./...` 通过。
- `go vet ./...` 通过。
- `go build ./cmd/server` 通过。
- `.\scripts\dev.ps1 test` 通过。
- `.\scripts\dev.ps1 build` 通过。
- `.\scripts\dev.ps1 lint` 通过；本机未安装 `golangci-lint`，脚本按设计回退到 `go vet`。
- `go test -race ./...` 未运行成功，因为当前本机 Go 环境为 `windows/386`，不支持 race detector。

### Notes

- Phase 1 仍不包含 persona prompt、真实 intent classifier、专家 Agent、Skill 库、HTTP API、Web UI、TTS/ASR 或 Avatar 表现层。

## v0.2.0 Phase 0 Engineering Baseline — 2026-06-14

### Added

- 新增 Go module、基础目录骨架、CLI/server 占位入口、Makefile、PowerShell 开发脚本和 CI workflow。
- 新增 Phase 0 SDD 规格文档和设计文档，明确工程基线的验收标准、边界和设计决策。
- 新增 `configs/app.yaml` 默认配置模板，以及 `internal/config` 配置加载包，支持固定结构 YAML 子集和环境变量覆盖。
- 新增 `internal/observability` 可观测性底座，包括 `slog` logger、内存 metrics、Prometheus 文本导出接口和 no-op trace hook。
- 新增 `internal/core` 领域错误、错误包装 helper 和泛型 `Result[T]` 包络。

### Verified

- `go test ./...` 通过。
- `go build ./cmd/server` 通过。
- `go vet ./...` 通过。
- `go run ./cmd/server --config configs/app.yaml` 能输出结构化 JSON 启动日志。

### Notes

- 当前本机 Go 环境为 `windows/386`，不支持 race detector；`make test-race` 需要在支持 race 的平台执行。
- 本机未安装 `make` 和 `golangci-lint`，Windows 可使用 `scripts/dev.ps1` 执行 build/test/lint/run/clean。

## v0.1.0 Planning Release — 2026-06-14

### Added

- 新增专业数字人总体规划，覆盖产品定位、MVP 建议、核心成功指标和目标目录结构。
- 新增 Phase 0-5 交付路线图，将 M0-M10 Milestone 转换为可执行的阶段计划。
- 新增仓库首页 `README.md`，说明项目定位、核心能力、架构概览、Phase 路线和推荐下一步。
- 新增 `RELEASE_NOTES.md`，用于记录规划文档和后续版本变化。

### Documented

- 记录 M0-M10 的系统级规划：工程基建、内核契约、基础设施、人格路由、专家 Agent、Skill 库、编排运行时、入口上线、数字人表现层、产品后台和治理运营。
- 记录每个 Phase 的目标、包含步骤、主要交付物、验收标准、推荐执行顺序、可并行项、阻塞依赖和 opencode prompt 粒度。
- 记录数字人表现层设计，包括 Avatar、TTS、ASR、口型同步、字幕时间轴、表情状态机、实时交互协议和打断策略。
- 记录产品后台设计，包括 Persona 编辑器、记忆管理、知识库管理、工具权限配置和运营看板。
- 记录治理评测设计，包括 golden conversation、人格一致性、事实准确、工具调用、安全合规、隐私记忆、模型路由、发布回滚和人工反馈闭环。

### Notes

- 这是规划版本，不包含 Go module、应用代码、CI、部署脚本或可运行服务。
- 后续实现应从 Phase 0 开始，先建立工程基线，再推进接口、基础设施、Agent、Skill、运行时和产品体验。
- Release notes 仅记录已经进入仓库的真实文档变化，不描述尚未实现的产品能力为已发布能力。
