# Release Notes
## Unreleased - Phase 10 Knowledge Base and Memory Control

### Added

- Added Phase 10 local knowledge lifecycle: document metadata, stable chunk IDs and ordinals, content hashing, disable/enable/delete/reindex support, and file-store persistence updates.
- Added deterministic lexical retrieval in `internal/knowledge`, including disabled-document filtering, stable ranking, and CJK substring fallback.
- Added runtime grounding wiring so persona chat can use local knowledge sources and emit allowlisted metadata such as `knowledge_used`, `knowledge_result_count`, `knowledge_citations`, and `retrieval_mode`.
- Added `/admin/knowledge` lifecycle endpoints and upgraded `/admin` from a mock knowledge button into a real operator-facing knowledge table with inspect, toggle, reindex, delete, and citation-test actions.
- Added `/app` grounding presentation for assistant turns, including `Knowledge grounded`, `No source used`, citation chips, and memory/knowledge state rendering.

### Documented

- Added Phase 10 spec, design, and plan docs for knowledge base management, memory control, deterministic retrieval, and prompt-injection boundaries.
- README now reflects Phase 10 status, the local knowledge workflow, and the expanded admin knowledge endpoints.

### Notes

- Phase 10 still keeps persistence local-first and file-backed; it does not introduce SQLite, an external vector database, or mandatory embedding-provider integration.
- CI remains deterministic and local: no real DeepSeek calls or paid provider dependencies are required for knowledge grounding tests.

## Unreleased - Phase 9 Experience and Provider Diagnostics

### Added

- Added Phase 9 provider diagnostics: `/runtime/status`, sanitized provider/model/fallback metadata, and OpenAI-compatible provider failure taxonomy for status, network, malformed stream, truncated stream, and empty response cases.
- Added explicit fallback metadata plumbing from runtime to presentation so the web experience can distinguish live LLM output, local fallback, and hard error states.
- Added a redesigned `/app` workspace with provider strip, status chip, presence panel, and labeled fallback/not-saved transcript states.
- Added safer DeepSeek local scripts: tracked background startup, PID-aware stop flow, and a smoke script that checks `/runtime/status` before running streaming conversation verification.

### Documented

- README now explains Phase 9 status, `/runtime/status`, `fail_closed` versus `fallback_to_local`, DeepSeek startup, and the local smoke workflow.

### Notes

- Phase 9 still uses the OpenAI-compatible boundary for DeepSeek-style providers; it does not add a provider-specific SDK.
- CI remains local and deterministic: fake servers only, no real paid provider calls.

## Unreleased - Phase 8 Real Conversation Loop Update

### Added

- Added Phase 8 conversation streaming groundwork: `types.TurnRequest`, turn/attempt persistence, replay-safe server history, and runtime-owned assistant message identity.
- Added real incremental `/chat/stream` behavior with assistant delta events, completed/done terminal events, retry semantics, and server tests for restart, replay, cancellation, and concurrent request ID behavior.
- Added a runtime-to-presentation streaming adapter so `/experience/stream` can consume runtime deltas and emit incremental presentation events instead of waiting for a final-only response.
- Added `scripts/smoke-conversation.ps1` and updated `cmd/smoke` coverage so local verification exercises multi-turn streaming, replay, and persisted conversation history.

### Documented

- README now documents the Phase 8 `TurnRequest` request body for `/chat/stream`, the compatibility shape of `/experience/stream`, the new smoke script, and the current project status as Phase 8 in progress.
- Added Phase 8 spec, design, and plan links to the repository SDD index.

### Notes

- Phase 8 still keeps `/chat` as the non-streaming compatibility path.
- The implementation remains local-first: no streaming TTS provider, no stream resume across processes, and no RAG/tool streaming in this slice.

## Unreleased - Phase 7A Persona LLM Update

### Added

- Added Phase 7A LLM persona groundwork: expanded `llm` config fields, local/mock LLM factory, OpenAI-compatible persona client selection, and server wiring for configured persona generation.
- Added `PersonaAgent` LLM injection, rendered system-prompt path, local transparency answer for model-identity questions, provider fallback, and post-generation persona guard fallback.
- Added runtime and server tests covering configured persona LLM replies, fallback behavior, and local deterministic CLI/server behavior.

### Documented

- README now describes the new local/mock persona behavior and the `DIGITAL_TWIN_LLM_*` environment variables for OpenAI-compatible local testing.

### Notes

- Phase 7A remains local-first: CI still uses fake clients or fake HTTP servers only, and token streaming / router-wide LLM behavior are still deferred.

## Unreleased - Phase 6A Update

### Added

- Added Phase 6A provider and deployment readiness: production-shaped config profiles, provider validation, secret redaction, `/ready`, request ID headers, provider/readiness metrics, local Docker Compose packaging, and `cmd/smoke`.
- Added HTTP-shaped TTS provider adapter with fake-server tests only; CI and local tests do not call real paid providers.
- Added deployment artifacts: `deploy/Dockerfile`, `deploy/docker-compose.yml`, `deploy/.env.example`, `deploy/README.md`, and `scripts/verify_deploy.ps1`.
- Added Go smoke verification for `/health`, `/ready`, `/chat`, `/chat/stream`, `/app`, `/admin`, and `/metrics`.

### Documented

- README documents Phase 6A usage, verification commands, and explicit exclusions: no SQLite, no real provider requirement in CI, no compliance certification, and no production RBAC/OAuth/Kubernetes scope.
- Deployment runbook documents Docker Compose startup, local data volume backup/restore, provider env shape, and Docker-unavailable fallback commands.

### Notes

- Phase 6A is production-shaped local readiness, not an enterprise production claim. It keeps local file storage and still excludes SQLite/Postgres, compliance certification, cloud operations, OAuth/RBAC, and real provider calls in automated tests.

## Unreleased

### Added

- 新增 Phase 5 SDD spec、design 和 plan 文档，明确 Launch Gate MVP、runtime governance wiring、确定性 eval、release gate、rollback、feedback 和本地存储边界。
- 新增 `internal/evals` eval fixture schema、parser、seed cases、确定性 evaluator、tenant isolation evaluator、suite runner、JSON report 和 Markdown report。
- 新增 `cmd/cli eval` 本地评测命令，可从 `evals/conversations` 读取 fixture evidence，并向 `evals/reports` 写入报告。
- 新增 `cmd/cli decisions` 本地 governance decision records 查询命令，按 tenant 读取本地文件存储且不暴露其他 tenant 记录。
- 新增 `internal/app` governed runtime adapter，可从 admin active persona/tool policy 解析运行时治理配置和版本元数据。
- 新增 `internal/governance` runtime governance metadata、tenant-scoped decision store、memory write policy、release gate、rollback records 和 feedback triage 基础。
- 新增 `internal/admin` decision audit exporter，可将 governance decision 投影为现有 audit 记录。
- 新增工具执行前治理 hook：`BaseAgent.RunSkill` 可接入 `SkillAuthorizer`，`ToolPolicyService` 可复用 admin 工具策略作为运行时 skill authorizer。
- 新增 Phase 4 数字人表现层：`PresentationEvent`、字幕时间轴、mock TTS/ASR、Avatar manifest、Avatar 状态机、打断控制和 `/experience/stream`。
- 新增 Web 用户端 `/app`，支持文本输入、SSE presentation stream 渲染、字幕、Avatar 状态、mock audio 状态和 mock voice flow。
- 新增本地优先产品后台 `/admin`，支持 Persona draft/publish/rollback、记忆禁用、知识上传/切块/引用测试、工具策略保存/授权检查和会话审计。
- 新增本地 admin services 与 store：persona、memory、knowledge、tool policy 和 audit。
- 新增 Phase 4 SDD spec、design 和 plan 文档。
- CLI `ask` 新增 `--json` 输出，便于脚本和测试消费完整 `AgentResult`。
- `/chat/stream` 新增已记录 runtime events 的 SSE 输出，包括 request、routing、agent 和 completion 相关事件。
- 新增真实本地 HTTP `/chat` 全链路测试，覆盖 `cmd/server` handler 到 local runtime 的路径。

### Documented

- README 更新为 Phase 5 本地治理闭环基础已完成，并补充 Phase 5 SDD 文档入口、本地 eval/report 命令、decision records 命令和本地数据目录说明。
- README 更新为 Phase 4 已完成，并补充 `/app`、`/admin` 和 `/experience/stream` 的本地使用说明。
- README 补充 CLI JSON 和 SSE curl 示例。

### Notes

- Phase 5 仍为 mock/local-first；不包含外部 eval 平台、云端内容审核、生产认证、SQLite、真实 provider、合规认证或完整治理 dashboard。

## v0.4.0 Phase 3 Runtime API Release - 2026-06-16

### Added

- 新增生产 Orchestrator，串联 conversation validation、Router、AgentRegistry、Agent execution、runtime events 和 safe fallback。
- 新增 runtime event、EventRecorder、状态机、并发 request ID、context cancellation 和 panic recovery 行为。
- 新增本地 deterministic runtime bootstrap，用于无外部 provider 的 CLI/HTTP 本地运行。
- 新增 `cmd/cli ask` one-shot 命令。
- 新增 HTTP `/health`、`/metrics`、`/chat` 和 SSE `/chat/stream`。
- 新增可选 API key auth 和 in-memory rate limiting。
- 新增 Phase 3 spec、design、plan 和 ADR。

### Documented

- README 更新为 Phase 3 已完成，并补充 CLI/HTTP 本地运行示例。
- ADR 记录 HTTP/SSE、local-first、无 SQLite 的 Phase 3 取舍。

### Notes

- Phase 3 仍不包含 Web UI、产品后台、真实 TTS/ASR、真实 Avatar、SQLite、gRPC 或真实外部 provider 集成。

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
