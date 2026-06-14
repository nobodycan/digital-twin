# Release Notes

本文件记录 `digital-twin` 仓库的文档、工程基线和后续产品版本变化。

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
