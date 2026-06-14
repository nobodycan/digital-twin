# digital-twin

面向专业数字人的 Go 多 Agent 系统规划与实现路线。

## 当前状态

本仓库处于 **Planning stage**。当前内容是项目蓝图与文档，不包含可运行代码、Go module、CI 或部署物料。下一步应从 Phase 0 开始建立工程基线。

## 项目定位

`digital-twin` 目标是构建一个可工程化落地的专业数字人系统，而不只是聊天机器人。它需要同时覆盖人格一致性、长期记忆、知识库问答、工具调用、运行时编排、语音/Avatar 表现层、管理后台、评测治理和安全合规。

第一版建议聚焦“文本 + 语音的专家顾问数字人”：

- 稳定 persona 与语气。
- 基于知识库回答并展示引用来源。
- 支持可查看、可删除的长期记忆。
- 提供 Web 聊天和基础语音播报。
- 支持后台配置 persona、知识库和工具权限。

## 核心能力

- **人格**：Persona 配置、System Prompt 渲染、人格一致性守卫。
- **记忆**：短期会话窗口、长期摘要记忆、语义召回、记忆治理。
- **知识**：RAG 检索、引用标注、知识库版本和来源管理。
- **工具**：Skill 参数校验、工具白名单、权限控制和失败降级。
- **编排**：Router、Agent、Skill、Orchestrator、状态机和并发调度。
- **表现层**：TTS、ASR、字幕时间轴、Avatar 状态、打断策略。
- **后台**：Persona 编辑、记忆管理、知识库管理、工具权限、运营看板。
- **治理**：AI 行为评测、隐私合规、安全策略、成本统计、发布回滚。

## 架构概览

```mermaid
flowchart TD
    User["用户 / 运营者"] --> Web["Web 用户端 / 管理后台"]
    Web --> API["CLI / HTTP / SSE / gRPC"]
    API --> Runtime["Orchestrator / 状态机 / 事件总线"]
    Runtime --> Router["Intent Router"]
    Router --> Agents["Persona / Memory / Knowledge / Task / Tool Agents"]
    Agents --> Skills["Memory / Knowledge / Task / Tool / Safety / Avatar Skills"]
    Skills --> Infra["LLM / Store / VectorStore / TTS / ASR"]
    Runtime --> Observability["日志 / 指标 / Trace / 审计"]
    Web --> Avatar["Avatar / 字幕 / 音频 / 表情状态"]
    Infra --> Governance["Eval / Safety / Privacy / Cost Control"]
    Governance --> Runtime
```

## Phase 路线图

| Phase | 名称 | 目标 |
| --- | --- | --- |
| Phase 0 | 项目定义与工程基线 | 明确 MVP，建立 Go 工程、配置、日志、错误、测试和 CI 基础。 |
| Phase 1 | 内核契约与基础设施 | 定义数据契约、接口、mock、Registry、LLM、存储、向量库和记忆。 |
| Phase 2 | 人格、路由、Skill 与 Agent | 落地 persona、路由、Skill 库和五类专家 Agent。 |
| Phase 3 | 编排运行时与 API 入口 | 串起 Orchestrator、状态机、容错、CLI、HTTP/SSE 和部署草案。 |
| Phase 4 | 数字人表现层与产品后台 | 建立 TTS/ASR/Avatar 事件流、Web 用户端和运营后台。 |
| Phase 5 | 治理、评测、安全与运营 | 建设 eval、安全、隐私、成本、发布回滚和反馈闭环。 |

完整拆解见 [plan.md](./plan.md)。

## 仓库结构

当前仓库只包含规划文档：

```text
digital-twin/
├── README.md
├── RELEASE_NOTES.md
└── plan.md
```

未来代码目录以 [plan.md](./plan.md) 中的目标目录结构为准。

## 推荐下一步

从 Phase 0 开始实现：

1. 初始化 Go module 和目录骨架。
2. 添加 `Makefile`、`.gitignore` 和基础测试命令。
3. 实现配置加载、结构化日志和统一错误处理。
4. 确保每一步都可以独立构建、测试和验证。

## 开发原则

- 小步提交：一个步骤一次实现，避免跨 Phase 混改。
- 接口先行：先定义数据契约和 interface，再写实现。
- Mock 优先：外部 LLM、DB、TTS、ASR 先用 fake server 或 mock。
- 可验证交付：每次变更都必须说明验证命令和预期结果。
- 文档同步：关键架构决策要进入 `docs/` 或 ADR。
