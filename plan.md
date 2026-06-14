在展开之前，我想说明一个判断：你说这是一个**大型项目**，那么真正的难点不在"写出 Agent 代码"，而在于**工程化体系**——可观测性、状态持久化、并发调度、容错重试、热插拔、版本演进、测试覆盖。所以下面的拆解我会把这些"大型项目才需要"的东西全部展开，而不只是把功能模块列细。

我把整个项目重新组织成 **11 个里程碑（Milestone）、约 70+ 个可独立交付的步骤**。每一步都设计成"可以单独喂给一次 `opencode run`、能编译、能验证"的粒度，这是大型项目对接 AI 编码工具的关键——粒度太大必然跑偏。

---

## 总览：里程碑地图

```
M0 工程基建      → 让项目"能跑能测能观测"
M1 内核契约      → 定义所有 interface 与数据契约（一切的地基）
M2 LLM/记忆/存储 → 三大基础设施层
M3 人格与路由    → 分身的"灵魂"与"神经中枢"
M4 专家 Agent    → 五大专家逐个落地
M5 Skill 库      → 原子能力逐个落地
M6 编排运行时    → Orchestrator + 并发调度 + 容错
M7 入口/联调/上线 → CLI/HTTP/gRPC + 端到端 + 部署
M8 数字人表现层  → 形象、语音、口型、表情、实时交互
M9 产品体验后台  → Web 端、管理台、人格/记忆/知识配置
M10 治理评测运营 → 安全合规、行为评测、成本性能、版本运营
```

---

## 产品定位：先定义"专业数字人"是什么

在写 Agent 之前，需要先把产品边界定清楚。否则系统会自然长成"一个很强的聊天后端"，而不是一个能交付给用户的专业数字人。

**目标用户与场景**
- 企业客服数字人：回答产品、订单、售后、工单问题。
- 专家顾问数字人：基于知识库做专业问答、方案建议、材料总结。
- 主播/讲解数字人：面向直播、课程、展厅、营销页面做语音与视频讲解。
- 个人分身数字人：保持某个人的表达风格、背景记忆和偏好，替本人处理沟通或内容生产。

**MVP 建议**
第一版不要同时做所有场景。建议先选择一种主场景，例如"文本 + 语音的专家顾问数字人"：
- 支持稳定人设与语气。
- 支持知识库问答与引用。
- 支持长期记忆但可查看、可删除。
- 支持 Web 聊天与基础语音播报。
- 支持后台配置 persona、知识库、工具权限。

**核心成功指标**
- 任务完成率：用户问题是否被解决。
- 人格一致性：多轮对话中是否像同一个人。
- 事实准确率：知识回答是否有来源、少幻觉。
- 首字延迟与总延迟：文本/语音体验是否顺滑。
- 单次会话成本：LLM、Embedding、TTS、ASR、存储成本是否可控。
- 安全拦截率：敏感请求、越权工具调用、prompt injection 是否被识别。

---

## Phase 交付路线图

Phase 是比 Milestone 更适合执行管理的交付单位。Milestone 描述系统模块，Phase 描述真实推进顺序：每个 Phase 都有明确输入、输出、验收标准和进入下一阶段的条件。

### Phase 0 — 项目定义与工程基线

**目标**
把项目从规划文档推进到可持续开发的工程起点，明确产品边界、目录骨架、配置、可观测性、错误处理和 CI 脚手架。

**包含 Milestone/步骤**
- 产品定位与 MVP 定义。
- 目标目录结构。
- M0.1 项目初始化与目录骨架。
- M0.2 配置系统。
- M0.3 可观测性底座。
- M0.4 错误处理与统一返回。
- M0.5 测试与 CI 脚手架。

**主要交付物**
- Go module、目录骨架、`Makefile`、`.gitignore`。
- `configs/app.yaml` 示例配置。
- `internal/config`、`internal/observability` 基础包。
- 统一错误类型与基础测试。
- CI workflow 草案。

**验收标准**
- `make build` 通过。
- `make test` 通过，至少覆盖配置加载与错误判定。
- 应用启动时能输出结构化日志。
- README 能说明当前 MVP 目标和工程进入方式。

**推荐执行顺序**
先做 M0.1，再做 M0.2 和 M0.3，最后补 M0.4 与 M0.5。CI 可以在本地命令稳定后再接入。

**可并行项**
配置系统和可观测性底座可以并行；README 初稿可以和目录骨架同步推进。

**阻塞依赖**
无。Phase 0 是整个项目的起点。

**建议 opencode prompt 粒度**
一条 prompt 只实现 M0 的一个步骤，例如“执行 M0.2 配置系统”，并要求输出文件清单、测试和验证命令。

### Phase 1 — 内核契约与基础设施

**目标**
先定义稳定契约，再落地 LLM、记忆、存储、向量检索等基础设施，使后续 Agent、Skill 和运行时可以基于 mock 并行开发。

**包含 Milestone/步骤**
- M1.1 基础数据契约。
- M1.2 核心接口定义。
- M1.3 基础设施接口定义。
- M1.4 Mock 实现集。
- M1.5 Registry。
- M2.1 LLMClient 抽象与重试封装。
- M2.2 OpenAI 兼容实现。
- M2.3 持久化存储层。
- M2.4 向量存储层。
- M2.5 短期记忆。
- M2.6 长期记忆。

**主要交付物**
- `pkg/types` 数据结构与 JSON 契约。
- Agent、Skill、Router、Orchestrator、LLM、Memory、Store、VectorStore、EventBus、TTS、ASR、Safety、Eval 接口。
- 可复用 mock 集合。
- Agent/Skill registry。
- SQLite 起步存储、内存向量库、短期/长期记忆基础实现。

**验收标准**
- 所有接口有文档注释并可 `go doc`。
- mock 可实例化并满足接口。
- 存储层可写入并读回一条会话。
- 向量库 top-k 检索测试通过。
- 短期记忆能按 token 预算裁剪并保留 system。
- 长期记忆能摘要写入并语义召回。

**推荐执行顺序**
严格先完成 M1.1-M1.5，再推进 M2。M2 中先做 LLM 抽象和 store，再做 vector store、短期记忆、长期记忆。

**可并行项**
M2.3 持久化存储、M2.4 向量存储和 M2.5 短期记忆可以由不同任务并行推进，但都必须依赖 M1 接口。

**阻塞依赖**
Phase 0 必须完成，尤其是配置、测试和错误处理基础。

**建议 opencode prompt 粒度**
M1 每个步骤单独 prompt；M2 每个基础设施实现单独 prompt。涉及外部服务时必须使用本地 fake server 或 mock，不调用真实 API。

### Phase 2 — 人格、路由、Skill 与 Agent

**目标**
让系统具备“像同一个专业数字人”一样理解请求、选择能力、调用原子 Skill、并由专家 Agent 产出稳定结果的核心智能。

**包含 Milestone/步骤**
- M3.1 Persona 数据模型。
- M3.2 System Prompt 渲染器。
- M3.3 人格一致性守卫。
- M3.4 规则路由。
- M3.5 LLM 分类路由。
- M3.6 融合路由与兜底。
- M5.1-M5.8 Skill 库。
- M4.1-M4.6 专家 Agent。

**主要交付物**
- `configs/persona.yaml` 示例与校验逻辑。
- Persona prompt 模板与 golden file。
- 规则 + LLM 混合路由。
- Skill 参数校验框架及记忆、知识、任务、工具、人格、安全、表现层 Skill。
- BaseAgent、PersonaAgent、MemoryAgent、KnowledgeAgent、TaskAgent、ToolAgent。

**验收标准**
- Persona 加载失败时有明确校验错误。
- System Prompt 快照测试稳定。
- 路由三条路径均有测试：规则命中、LLM 兜底、低置信度闲聊。
- 每个 Skill 覆盖合法参数、非法参数、错误路径。
- 每个 Agent 覆盖 `CanHandle`、`Handle`、Registry 注册。

**推荐执行顺序**
先做 Persona 与路由，再做 Skill 5.1 框架，随后并行实现 Skill；Agent 必须在 BaseAgent 和必要 Skill 可用后推进。

**可并行项**
M5.2-M5.8 Skill 可并行；M4.2-M4.6 Agent 可在 BaseAgent 完成后并行，但 KnowledgeAgent 依赖长期记忆与向量库。

**阻塞依赖**
Phase 1 的接口、mock、Registry、LLM 抽象、记忆与向量库必须可用。

**建议 opencode prompt 粒度**
每个 Skill 或 Agent 一个 prompt；不要把多个专家 Agent 合并到一次执行。每条 prompt 明确依赖哪些 mock 和已有接口。

### Phase 3 — 编排运行时与 API 入口

**目标**
把分散的路由、Agent、Skill、记忆和基础设施串成可对话、可观测、可降级、可通过 CLI/HTTP 调用的运行系统。

**包含 Milestone/步骤**
- M6.1 Orchestrator 主循环。
- M6.2 会话状态机。
- M6.3 并发调度。
- M6.4 容错与降级。
- M6.5 事件总线与可观测埋点。
- M6.6 多轮对话与上下文衔接。
- M7.1 CLI 入口。
- M7.2 HTTP API。
- M7.3 gRPC 可选。
- M7.4 鉴权与限流。
- M7.5 端到端测试。
- M7.6 部署物料。
- M7.7 文档与 ADR。

**主要交付物**
- Orchestrator 主链路和会话状态机。
- 并发调度与 context 取消。
- trace id、事件总线和指标埋点。
- CLI REPL、HTTP `/chat`、`/health`、`/metrics`。
- SSE 流式输出基础能力。
- 鉴权、限流、e2e 脚本、Docker/compose/k8s 草案。

**验收标准**
- mock 全链路可跑通一次多轮对话。
- 非法状态转移被拒。
- race detector 下并发调度无竞态。
- 单个 Agent/Skill 失败时仍返回兜底回复。
- `curl /chat` 能完成一次问答，`/health` 和 `/metrics` 可访问。
- e2e 剧本断言通过。

**推荐执行顺序**
先完成 Orchestrator 与状态机，再接并发、容错和埋点；入口层先 CLI 后 HTTP，gRPC 保持可选。

**可并行项**
CLI、HTTP、部署物料、ADR 可以在 Orchestrator 稳定后并行推进。

**阻塞依赖**
Phase 2 的 Router、Agent、Skill 必须有可用 mock 或真实实现。

**建议 opencode prompt 粒度**
运行时每个能力一个 prompt；入口层每个协议一个 prompt。涉及并发和流式输出时，必须要求 race/e2e 验证。

### Phase 4 — 数字人表现层与产品后台

**目标**
把后端智能转化为用户可感知、可配置、可运营的数字人体验，覆盖语音、字幕、avatar 状态、Web 用户端和管理后台。

**包含 Milestone/步骤**
- M8.1 Avatar 形象资产规范。
- M8.2 TTS 抽象。
- M8.3 ASR 抽象。
- M8.4 口型同步与字幕时间轴。
- M8.5 表情与动作状态机。
- M8.6 实时交互协议。
- M8.7 打断与半双工/全双工策略。
- M9.1 用户端 Web 聊天界面。
- M9.2 语音/数字人交互界面。
- M9.3 Persona 编辑器。
- M9.4 记忆管理台。
- M9.5 知识库管理台。
- M9.6 工具权限配置。
- M9.7 会话审计与运营看板。

**主要交付物**
- Avatar manifest 与示例资产规范。
- TTS/ASR mock 与 provider 适配接口。
- `SpeechTimeline`、字幕、avatar 状态事件。
- Web 聊天、语音输入、TTS 播放、基础 avatar 状态渲染。
- Persona、记忆、知识库、工具权限和运营看板后台。

**验收标准**
- 一次对话能流式产生文本、字幕、音频占位和 avatar 状态事件。
- 用户端能完成文本对话和基础语音交互。
- 后台能发布 persona 版本，并让新会话生效。
- 删除记忆后后续召回不再出现。
- 上传知识文档后可完成引用测试。
- 未授权工具调用被拒。

**推荐执行顺序**
先定义表现层协议和 mock，再做 Web 用户端；后台按 persona、记忆、知识库、工具权限、看板顺序推进。

**可并行项**
TTS、ASR、avatar 状态和 Web UI 可以并行，但必须共享同一实时事件协议。

**阻塞依赖**
Phase 3 的 HTTP/SSE 或 WebSocket 事件流必须稳定；Phase 2/3 的 persona、记忆、知识库和工具权限必须有 API 支撑。

**建议 opencode prompt 粒度**
表现层接口、用户端交互、每个后台模块分别独立 prompt。真实 TTS/ASR provider 可以后置，第一版必须支持 mock。

### Phase 5 — 治理、评测、安全与运营

**目标**
让数字人具备可上线运营的安全边界、行为评测、隐私治理、成本控制、版本发布和人工反馈闭环。

**包含 Milestone/步骤**
- M10.1-M10.5 AI 行为评测体系。
- M10.6-M10.9 记忆、隐私与合规治理。
- M10.10-M10.14 安全防护与上线运营。

**主要交付物**
- `evals/conversations` golden conversation 剧本。
- Persona、RAG、工具调用、成本性能评测 runner。
- 记忆写入策略、可解释字段、删除/过期机制。
- 多租户隔离、审计日志、身份披露。
- Prompt injection 防护、高风险内容策略。
- 模型路由、版本发布/回滚、人工接管与反馈闭环。

**验收标准**
- eval runner 能跑完标准剧本并输出结果。
- 偏离 persona、错误引用、越权工具调用会被标记。
- 敏感信息不会写入长期记忆。
- 跨租户无法访问对方记忆或知识库。
- 知识库中的恶意提示不能覆盖 system prompt。
- persona/prompt/知识库版本发布前必须跑 eval，失败不可上线。

**推荐执行顺序**
先建设 eval runner 和黄金剧本，再接隐私记忆、多租户、安全策略，最后做模型路由、发布回滚和反馈闭环。

**可并行项**
评测用例、安全策略、成本统计和审计看板可以并行推进。

**阻塞依赖**
Phase 3 的全链路运行时和 Phase 4 的后台/看板能力越完整，Phase 5 的评测与运营闭环越有价值；但 eval 剧本应从 Phase 2 起逐步沉淀。

**建议 opencode prompt 粒度**
每类 eval 或治理策略单独 prompt。所有安全相关 prompt 必须包含负例测试，避免只验证 happy path。

## 目标目录结构（最终形态）

```
digital-twin/
├── cmd/
│   ├── server/        # HTTP/gRPC 入口
│   └── cli/           # 命令行入口
├── internal/
│   ├── core/          # 人格内核
│   ├── router/        # 意图路由
│   ├── agents/        # 专家 Agent
│   ├── skills/        # 原子 Skill
│   ├── runtime/       # 编排、调度、状态机
│   ├── llm/           # LLM 客户端抽象与实现
│   ├── memory/        # 记忆层
│   ├── store/         # 持久化（DB/向量库）
│   ├── observability/ # 日志/指标/追踪
│   └── config/        # 配置加载
├── pkg/
│   └── types/         # 对外可复用的数据契约
├── configs/           # persona.yaml / app.yaml
├── web/               # 用户端 Web、管理后台、数字人前端体验
├── assets/            # avatar、音色、动作、表情、品牌素材
├── evals/             # AI 行为评测、golden conversations、压测脚本
├── test/              # 集成测试 / e2e
├── docs/              # 架构文档 / ADR
└── deploy/            # Dockerfile / k8s / compose
```

---

# M0 — 工程基建（让项目"能跑能测能观测"）

> 大型项目最常见的失败是先写功能后补基建。这里先把骨架立稳。

**步骤 0.1 项目初始化与目录骨架**
`go mod init`，建立上面全部目录（含空的 `doc.go` 占位），配 `.gitignore`、`Makefile`（build/test/lint/run 四个 target）。验证：`make build` 通过。

**步骤 0.2 配置系统**
`internal/config`：支持 yaml + 环境变量覆盖（用 `viper` 或纯标准库）。定义 `AppConfig`（端口、LLM key、DB DSN、日志级别、TTS/ASR provider、对象存储、租户配置）。验证：加载 `configs/app.yaml` 打印生效配置。

**步骤 0.3 可观测性底座**
`internal/observability`：结构化日志（`slog`）、Prometheus 指标接口、可选 OpenTelemetry trace 钩子。定义 `Logger`/`Metrics` 接口，便于 mock。验证：启动时打印一条结构化日志。

**步骤 0.4 错误处理与统一返回**
定义领域错误类型（`ErrAgentNotFound`、`ErrLLMTimeout` 等）、错误包装规范（`errors.Join`/`fmt.Errorf %w`）、统一 `Result` 包络。验证：单测覆盖错误判定。

**步骤 0.5 测试与 CI 脚手架**
建立 `make test`（含 race detector）、`make lint`（golangci-lint 配置）、GitHub Actions/任意 CI 的 workflow 模板。验证：CI 配置文件可被解析、本地 test 通过。

---

# M1 — 内核契约（一切的地基）

> 大型项目必须"接口先行"。这一里程碑只定义契约，不写实现，但要配 mock。

**步骤 1.1 基础数据契约** `pkg/types`
`Message`（role/content/meta/timestamp）、`Conversation`、`Intent`、`AgentResult`、`SkillResult`、`UserProfile`、`Tenant`。全部带 JSON tag。验证：序列化/反序列化往返测试。

**步骤 1.2 核心接口定义**
`Agent`、`Skill`、`Router`、`Orchestrator` 四大接口（方法签名 + 文档注释）。验证：编译通过 + `go doc` 可读。

**步骤 1.3 基础设施接口定义**
`LLMClient`、`Memory`、`Store`、`VectorStore`、`EventBus`、`TTSClient`、`ASRClient`、`SafetyGuard`、`EvalRunner`。这些都是后续可替换实现的抽象。验证：编译通过。

**步骤 1.4 Mock 实现集**
为每个接口生成 mock（手写或 `mockery`）。这是后续所有单测的前提。验证：mock 可被实例化并满足接口。

**步骤 1.5 注册中心 (Registry)**
`AgentRegistry` 与 `SkillRegistry`：支持运行时注册/查询/列举，这是"热插拔"的基础。验证：注册 3 个假 Agent 后能正确按 intent 查出。

---

# M2 — 基础设施层（LLM / 记忆 / 存储）

**步骤 2.1 LLMClient 抽象与重试封装**
`internal/llm`：接口 + 通用装饰器（超时、重试、退避、限流、token 计数）。验证：用 mock 触发一次重试。

**步骤 2.2 OpenAI 兼容实现**
对接 OpenAI 兼容 `/chat/completions`，支持流式与非流式。key 从 config 读。验证：用本地假 server 测试请求构造正确（不依赖真实 API）。

**步骤 2.3 持久化存储层**
`internal/store`：会话/消息落库（SQLite 起步，接口可换 Postgres）。建表迁移脚本。验证：写入并读回一条会话。

**步骤 2.4 向量存储层**
`VectorStore` 实现：起步用内存版（暴力余弦相似度），接口预留 Qdrant/pgvector。验证：插入向量后 top-k 检索正确。

**步骤 2.5 短期记忆（会话窗口）**
`internal/memory`：滑动窗口 + token 预算裁剪。验证：超长会话被正确截断且保留 system。

**步骤 2.6 长期记忆（摘要 + 检索）**
对话摘要写入向量库，按需召回。验证：存入若干"事实"，用语义 query 能召回相关项。

---

# M3 — 人格与路由（分身的灵魂与神经中枢）

**步骤 3.1 Persona 数据模型**
`internal/core`：身份、性格特质、语气、价值观、禁忌、口头禅、背景故事、专业领域、知识边界、拒答策略、情绪策略、默认音色、avatar 绑定。从 `configs/persona.yaml` 加载。验证：加载并校验必填字段。

**步骤 3.2 System Prompt 渲染器**
把 Persona 渲染成稳定的 system prompt（模板化，支持变量注入如当前时间）。验证：快照测试（golden file）确保输出稳定。

**步骤 3.3 人格一致性守卫**
回复后的可选校验：检测是否违反禁忌/偏离语气/越过专业边界/错误冒充真人（规则 + 可选 LLM 复核）。验证：构造违规回复被标记。

**步骤 3.4 路由：规则引擎**
`internal/router`：关键词/正则映射 intent。验证：典型输入命中正确 intent。

**步骤 3.5 路由：LLM 分类器**
用 LLM 做 few-shot 意图分类，输出结构化 intent + 置信度。验证：mock LLM 返回固定分类，路由正确。

**步骤 3.6 路由：融合策略与兜底**
规则优先、LLM 兜底、低置信度走 PersonaAgent 闲聊。验证：三条路径各一个用例。

---

# M4 — 专家 Agent（逐个落地，每个独立交付）

> 每个 Agent 都是一个独立步骤：实现 + 注册 + 单测，互不阻塞。

**步骤 4.1 BaseAgent 公共骨架**
抽出通用逻辑（LLM 调用、Skill 调度、日志埋点），其余 Agent 内嵌它。验证：内嵌后接口满足。

**步骤 4.2 PersonaAgent** — 闲聊、风格化回复、人设兜底。
**步骤 4.3 MemoryAgent** — 读写记忆、上下文压缩、"你记得我说过…吗"。
**步骤 4.4 KnowledgeAgent** — RAG 检索 + 引用标注，回答知识问题。
**步骤 4.5 TaskAgent** — 复杂请求拆解为子任务 + 规划 + 进度跟踪。
**步骤 4.6 ToolAgent** — 外部 API/工具调用（带参数校验与白名单）。

每个 Agent 的验证标准统一：`CanHandle` 命中正确、`Handle` 在 mock LLM 下产出预期结构、至少 3 个单测、正确注册进 Registry。

---

# M5 — Skill 库（原子能力，可被多个 Agent 复用）

> Skill 是无状态原子操作，独立测试最简单，适合密集并行交付。

**步骤 5.1 Skill 基础与参数校验框架** — 统一 params 校验（JSON schema 或 struct tag）。
**步骤 5.2 记忆类 Skill** — `mem_store` / `mem_recall` / `summarize`。
**步骤 5.3 知识类 Skill** — `embed` / `vector_search` / `cite`。
**步骤 5.4 任务类 Skill** — `task_decompose` / `plan` / `track`。
**步骤 5.5 工具类 Skill** — `http_call`（含 SSRF 防护/白名单）/ `search_web` / `calendar`。
**步骤 5.6 人格类 Skill** — `tone_adjust` / `persona_check`。
**步骤 5.7 安全治理类 Skill** — `pii_detect` / `prompt_injection_check` / `risk_classify` / `policy_decide`。
**步骤 5.8 表现层 Skill** — `tts_speak` / `asr_transcribe` / `avatar_state` / `subtitle_timeline`。

每个 Skill 验证：合法 params 正确执行、非法 params 被拒、错误路径有测试。

---

# M6 — 编排运行时（系统真正"活起来"）

**步骤 6.1 Orchestrator 主循环**
输入 → 路由 → Agent → Skill → 聚合 → 人格守卫 → 输出。验证：mock 全链路跑通一次对话。

**步骤 6.2 会话状态机**
定义会话状态（idle/thinking/awaiting_tool/error/done）与合法转移。验证：非法转移被拒。

**步骤 6.3 并发调度（goroutine + channel）**
多 Agent 可并行（如 Knowledge + Memory 同时检索），用 `errgroup` 聚合，`context` 控制取消/超时。验证：race detector 下无竞态。

**步骤 6.4 容错与降级**
单个 Agent/Skill 失败不拖垮全局：超时降级、部分结果可用、重试。验证：注入失败仍返回兜底回复。

**步骤 6.5 事件总线与可观测埋点**
全链路 trace id、每步耗时指标、关键事件发布（便于审计/回放）。验证：一次对话产出完整 trace 日志。

**步骤 6.6 多轮对话与上下文衔接**
跨轮记忆注入、指代消解所需上下文拼装。验证：第二轮能引用第一轮信息。

---

# M7 — 入口、联调与上线

**步骤 7.1 CLI 入口** `cmd/cli` — 交互式 REPL，支持加载指定 persona。验证：本地对话跑通。
**步骤 7.2 HTTP API** `cmd/server` — `/chat`（支持流式 SSE）、`/health`、`/metrics`。验证：curl 跑通一次问答。
**步骤 7.3 gRPC（可选）** — 为高性能/内部调用提供 proto 接口。验证：grpcurl 调通。
**步骤 7.4 鉴权与限流** — API key/JWT + 每用户限流。验证：未授权被拒、超限被限。
**步骤 7.5 端到端测试** `test/e2e` — 真实/录制 LLM 下跑完整剧本对话。验证：剧本断言全过。
**步骤 7.6 部署物料** `deploy/` — Dockerfile（多阶段构建）、docker-compose（含 DB/向量库/Web 前端/对象存储可选）、k8s manifest。验证：`docker compose up` 起得来。
**步骤 7.7 文档与 ADR** `docs/` — 架构图、扩展指南（如何新增 Agent/Skill）、关键决策记录。验证：按文档能成功加一个新 Skill。

---

# M8 — 数字人表现层（让系统真正"像一个数字人"）

> 专业数字人不只是文本 Agent。用户感知到的是形象、声音、表情、响应节奏和情绪一致性。M8 负责把后端智能转化为可被感知的"人"。

**步骤 8.1 Avatar 形象资产规范**
定义数字人形象类型：2D 立绘、Live2D、3D 模型、真人拟真视频流、纯语音形象。建立 `assets/avatar/` 规范，记录版权、授权、版本、适用场景。验证：加载一个示例 avatar manifest。

**步骤 8.2 语音 TTS 抽象**
定义 `TTSClient` 接口：输入文本、音色、语速、情绪、输出音频流或文件。支持 mock TTS 与至少一个真实 provider 的适配层。验证：mock 下返回可播放音频占位，接口可流式输出。

**步骤 8.3 语音识别 ASR 抽象**
定义 `ASRClient` 接口：支持上传音频与实时流式识别，输出分段文本、置信度、时间戳。验证：mock 输入音频片段后输出稳定 transcript。

**步骤 8.4 口型同步与字幕时间轴**
定义 `SpeechTimeline`：文本分句、音频时间戳、字幕、口型 phoneme/viseme 数据。先用简化规则实现，后续可替换专业 lip-sync provider。验证：一段文本能生成字幕时间轴。

**步骤 8.5 表情与动作状态机**
定义数字人的非语言状态：idle、listening、thinking、speaking、happy、apologetic、serious、confused。根据对话意图、情绪和回复类型触发表情/动作。验证：不同 Agent 结果映射到正确表现状态。

**步骤 8.6 实时交互协议**
在 HTTP/SSE 或 WebSocket 中输出结构化事件：`text_delta`、`audio_chunk`、`avatar_state`、`subtitle`、`tool_status`、`done`。验证：一次对话可按事件流驱动前端渲染。

**步骤 8.7 打断与半双工/全双工策略**
支持用户在数字人说话时打断：停止 TTS、取消当前 LLM context、保留必要上下文并进入新一轮。验证：mock 流式对话中断后能正确取消旧任务。

---

# M9 — 产品体验与管理后台（让用户能真正使用和运营）

> 后端 API 不是产品。专业数字人需要用户端体验，也需要运营者能配置、观察和修正它。

**步骤 9.1 用户端 Web 聊天界面**
`web/app`：支持文本输入、流式回复、会话列表、引用来源展示、错误提示、重新生成。验证：本地 Web 端可完成一次流式对话。

**步骤 9.2 语音/数字人交互界面**
支持麦克风输入、ASR 中间态、TTS 播放、字幕、avatar 状态切换。第一版可用 2D avatar 或占位动画，不强依赖 3D。验证：用户说一句话后能看到字幕并听到回复。

**步骤 9.3 Persona 编辑器**
后台编辑身份、语气、价值观、禁忌、口头禅、知识边界、拒答策略。支持版本号、草稿、发布、回滚。验证：发布新版 persona 后新会话生效，旧会话可选择保留旧版本。

**步骤 9.4 记忆管理台**
用户或管理员可以查看长期记忆、来源、时间、置信度；支持删除、禁用、修正。验证：删除某条记忆后后续召回不再出现。

**步骤 9.5 知识库管理台**
支持上传文档、解析状态、切片预览、embedding 状态、引用测试、版本发布。验证：上传一个文档后可在前端问答中引用。

**步骤 9.6 工具权限配置**
后台配置 Tool/Skill 白名单、参数约束、审批策略、用户/租户权限。验证：未授权工具调用被拒，有权限工具可执行。

**步骤 9.7 会话审计与运营看板**
展示会话量、满意度、失败率、延迟、成本、常见问题、拦截原因。支持按 persona、知识库版本、模型版本过滤。验证：一次对话产生可查询审计记录。

---

# M10 — 治理、评测、安全与运营（让数字人可持续上线）

> 专业数字人的长期难点不是第一次回答正确，而是上线后持续稳定、可信、可控、可降本。

## M10.1 AI 行为评测体系

**步骤 10.1 Golden Conversation 剧本库**
在 `evals/conversations/` 建立标准多轮对话剧本，覆盖闲聊、专业问答、拒答、工具调用、记忆召回、打断等场景。验证：本地 eval runner 能跑完一组剧本。

**步骤 10.2 人格一致性评测**
评估回复是否符合 persona 的语气、禁忌、身份边界和价值观。可先用规则 + LLM judge，后续沉淀人工标注集。验证：构造偏离人设的回答会被扣分。

**步骤 10.3 事实准确与引用评测**
对 RAG 回答检查：是否引用真实片段、是否答非所问、是否编造来源。验证：错误引用和无来源断言会被标记。

**步骤 10.4 工具调用评测**
评估工具选择、参数生成、权限检查、失败降级是否正确。验证：危险参数、越权工具、错误 schema 均被拦截。

**步骤 10.5 性能与成本评测**
记录首 token 延迟、总耗时、token 数、TTS 时长、ASR 时长、embedding 次数、单轮成本。验证：压测脚本输出 p50/p95 和成本估算。

## M10.2 记忆、隐私与合规治理

**步骤 10.6 记忆写入策略**
定义哪些内容可以进入长期记忆：用户偏好、稳定事实、任务背景；哪些不能写入：敏感证件、密码、短期情绪噪声、未经确认的推测。验证：敏感样例不会被写入长期记忆。

**步骤 10.7 记忆可解释与可删除**
每条记忆必须带来源会话、生成时间、置信度、版本；支持用户删除、管理员禁用、系统过期。验证：删除/过期后召回结果不包含该记忆。

**步骤 10.8 多租户与数据隔离**
所有会话、记忆、知识库、工具权限均带 tenant/user 维度，默认跨租户不可见。验证：A 租户无法召回 B 租户知识与记忆。

**步骤 10.9 审计日志与身份披露**
记录关键决策、工具调用、记忆写入、权限拒绝。前端明确披露"这是 AI 数字人"，避免误导用户以为是真人。验证：一次敏感拒答有完整审计链路。

## M10.3 安全防护与上线运营

**步骤 10.10 Prompt Injection 防护**
对用户输入、RAG 文档、网页搜索结果做指令隔离，区分"内容"与"系统指令"。验证：知识库中的恶意提示不能覆盖 system prompt。

**步骤 10.11 内容安全与高风险场景限制**
建立敏感内容分类：医疗、法律、金融、未成年人、自伤、违法、隐私。根据场景选择拒答、转人工、给出安全建议。验证：高风险问题走正确策略。

**步骤 10.12 模型路由与降本策略**
按任务复杂度选择模型：轻量闲聊、小模型分类、强模型规划、embedding 批处理、TTS 缓存。验证：简单问题不会调用最高成本链路。

**步骤 10.13 版本发布与回滚**
persona、prompt、工具、知识库、模型配置都要有版本。发布前跑 eval，失败不可上线；上线后可按会话回滚。验证：一次 persona 发布失败会阻止部署。

**步骤 10.14 人工接管与反馈闭环**
支持用户点踩、标记错误、请求人工；后台可把失败案例加入 eval 或知识库修正流程。验证：低满意度会话进入待处理列表。

---

## Phase 执行总表

| Phase | 输入 | 输出 | 关键验收命令/检查 | 进入下一 Phase 的条件 |
| --- | --- | --- | --- | --- |
| Phase 0 项目定义与工程基线 | `plan.md`、MVP 定义、空仓库 | Go module、目录骨架、配置、日志、错误、测试和 CI 脚手架 | `make build`、`make test`、启动日志检查 | 工程能构建、能测试、能加载配置、能输出结构化日志 |
| Phase 1 内核契约与基础设施 | Phase 0 工程基线 | 数据契约、接口、mock、Registry、LLM/Store/Vector/Memory 基础实现 | `go test ./pkg/... ./internal/... -race`、存储读写测试、向量 top-k 测试 | 接口稳定，mock 可用，基础设施可被后续 Agent/Skill 调用 |
| Phase 2 人格、路由、Skill 与 Agent | Phase 1 契约和基础设施 | Persona、Prompt、Router、Skill 库、专家 Agent | golden prompt 测试、路由测试、Skill 参数测试、Agent 注册测试 | 专家 Agent 可在 mock LLM 和 mock Skill 下稳定产出结构化结果 |
| Phase 3 编排运行时与 API 入口 | Phase 2 Agent/Skill 能力 | Orchestrator、状态机、并发调度、CLI、HTTP/SSE、e2e、部署草案 | `go test ./... -race`、`curl /health`、`curl /chat`、e2e 剧本 | 一次多轮对话可通过 CLI/HTTP 跑通，并具备容错和观测日志 |
| Phase 4 数字人表现层与产品后台 | Phase 3 API/事件流 | Avatar/TTS/ASR/字幕/状态事件、Web 用户端、管理后台 | 浏览器手测、事件流检查、persona 发布测试、记忆删除测试 | 用户能完成文本/语音交互，运营者能配置 persona、记忆、知识库和工具权限 |
| Phase 5 治理、评测、安全与运营 | Phase 3/4 可运行产品链路 | eval runner、安全策略、隐私记忆、多租户、成本统计、发布回滚、人工反馈 | eval 剧本、prompt injection 负例、租户隔离测试、成本统计检查 | 发布前评测与安全门禁可执行，失败案例能进入反馈闭环 |

执行原则：每个 Phase 可以拆成多个 `opencode run`，但每次只交付一个明确步骤；每次完成后必须给出文件清单、验证命令和下一步建议。Phase 5 的评测资产不要等到最后才写，应该从 Phase 2 开始随功能逐步沉淀。

---

## 如何对接 opencode 执行

由于这是大型项目，关键原则是**一个步骤一次 `opencode run`，编译+测试通过再进下一步**。建议给每一步用统一的喂入模板：

````markdown
# 任务：执行步骤 {{编号}} {{标题}}
## 项目上下文
这是 digital-twin（Go 大型多 Agent 数字分身系统），当前已完成步骤 {{已完成列表}}。
请阅读现有代码，保持接口契约一致，不要重写已有文件。

## 本步目标
{{该步骤一句话目标}}

## 交付物
- 新增/修改文件清单
- 完整代码（可编译）
- 配套单元测试
- 验证命令及预期输出（如 go test ./internal/xxx -race）

## 约束
- 不一次性实现其他步骤
- 涉及外部依赖（LLM/DB）必须用 M1 的 mock，不依赖真实服务
- 完成后输出"下一步建议"
````

---

## 给你的几点工程提醒

依赖顺序上有强约束：**M0 → M1 必须最先完成**，因为接口和 mock 是后面一切并行开发的前提；M4（Agent）和 M5（Skill）之间，建议先做 M5 的 Skill 框架（5.1）再做 Agent，因为 Agent 要挂载 Skill。M2 里的向量库和长期记忆是 KnowledgeAgent 的硬依赖，不要跳过。M8 的 TTS/ASR/avatar 可以先用 mock 与占位资产，不必等真实供应商接入；M9 的后台依赖 M3/M5/M10 的配置与治理模型；M10 的 eval 应该从早期就开始建设，不要等上线前补。另外，**M4/M5 内部各步骤之间几乎无依赖**，如果你想加速，这两块可以并行多开 `opencode run`。

关于 opencode 本身的命令行为，我对它的最新参数细节并不完全确定（它的更新可能晚于我的知识），所以上面的喂入模板是按"它能接收一段完整任务描述"来设计的。等你实际跑一两步，如果它的会话/上下文保持方式和我假设的不一样（比如它是否能自动读取已有代码），把实际情况告诉我，我再帮你把模板和拆解粒度调成最适配的形态。

需要我现在把 **M0 的步骤 0.1 ~ 0.5** 直接写成可执行的 opencode 完整 Prompt（含具体文件内容和验证命令）吗？或者你想先确定 persona.yaml 的人设字段设计，我可以先把那一块定下来。
