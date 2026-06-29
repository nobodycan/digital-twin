package agents

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/internal/llm"
	"github.com/nobodycan/digital-twin/internal/persona"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type PersonaAgent struct {
	BaseAgent
	client         llm.Client
	provider       string
	model          string
	fallbackPolicy string
	persona        persona.Persona
	renderer       persona.Renderer
}

type PersonaAgentConfig struct {
	Client         llm.Client
	Provider       string
	Model          string
	FallbackPolicy string
	Persona        persona.Persona
	Renderer       persona.Renderer
}

func NewPersonaAgent(skills *core.SkillRegistry, config ...PersonaAgentConfig) PersonaAgent {
	agent := PersonaAgent{BaseAgent: BaseAgent{NameValue: "persona-agent", Skills: skills}}
	if len(config) > 0 {
		agent.client = config[0].Client
		agent.provider = config[0].Provider
		agent.model = config[0].Model
		agent.fallbackPolicy = config[0].FallbackPolicy
		agent.persona = config[0].Persona
		agent.renderer = config[0].Renderer
	}
	return agent
}

func (a PersonaAgent) CanHandle(intent types.Intent) bool {
	return intent.Name == types.IntentPersonaChat
}

func (a PersonaAgent) Run(ctx context.Context, conversation types.Conversation, intent types.Intent) (types.AgentResult, error) {
	userContent, err := a.preflight(ctx, conversation, intent)
	if err != nil {
		return types.AgentResult{}, err
	}
	if asksModelIdentity(userContent) {
		return a.modelIdentityResult(intent), nil
	}
	if a.client != nil {
		messages, err := a.chatMessages(conversation)
		if err != nil {
			return types.AgentResult{}, err
		}
		response, err := a.client.Chat(ctx, llm.ChatRequest{Messages: messages})
		if err != nil {
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || a.fallbackPolicy == "fail_closed" {
				return types.AgentResult{}, err
			}
			return a.providerFallbackResult(intent, userContent, err), nil
		}
		if strings.TrimSpace(response.Message.Content) == "" {
			return a.fallbackResult(intent, a.emptyResponseFallbackCopy(userContent), "", "empty_response"), nil
		}
		if decision := a.guardDecision(response.Message.Content, float64(confidenceOrDefault(intent))); !decision.Allowed {
			return a.fallbackResult(intent, decision.SafeFallback, decision.Reason, "guard_rejected"), nil
		}
		generationMode := "llm"
		if a.provider == "" || a.provider == "local" || a.provider == "mock" {
			generationMode = "local"
		}
		return a.generatedResult(intent, response.Message.Content, generationMode, response.Usage), nil
	}
	return a.Result(
		"I'm here and keeping the same professional persona.",
		confidenceOrDefault(intent),
		types.Metadata{"intent": intent.Name, "generation_mode": "local"},
	), nil
}

func (a PersonaAgent) Stream(ctx context.Context, conversation types.Conversation, intent types.Intent, sink core.AssistantDeltaSink) (types.AgentResult, error) {
	userContent, err := a.preflight(ctx, conversation, intent)
	if err != nil {
		return types.AgentResult{}, err
	}
	if asksModelIdentity(userContent) {
		return a.modelIdentityResult(intent), nil
	}

	if a.client == nil {
		return a.Result(
			"I'm here and keeping the same professional persona.",
			confidenceOrDefault(intent),
			types.Metadata{"intent": intent.Name, "generation_mode": "local"},
		), nil
	}

	messages, err := a.chatMessages(conversation)
	if err != nil {
		return types.AgentResult{}, err
	}

	streamGuard := persona.NewStreamGuard(persona.Guard{Persona: a.persona}, float64(confidenceOrDefault(intent)))
	var accepted strings.Builder
	streamErr := a.client.Stream(ctx, llm.ChatRequest{Messages: messages}, func(chunk llm.ChatChunk) error {
		if chunk.Done {
			return nil
		}
		step := streamGuard.Add(chunk.Content)
		if !step.Decision.Allowed {
			return core.WrapError(core.ErrProviderFailure, "persona stream rejected")
		}
		for _, segment := range step.Segments {
			if err := emitAssistantDelta(ctx, sink, &accepted, segment); err != nil {
				return err
			}
		}
		return nil
	})
	if streamErr != nil {
		if errors.Is(streamErr, context.Canceled) || errors.Is(streamErr, context.DeadlineExceeded) || a.fallbackPolicy == "fail_closed" {
			return types.AgentResult{}, streamErr
		}
		if streamGuard.HasVisibleOutput() {
			return types.AgentResult{}, streamErr
		}
		return a.providerFallbackResult(intent, userContent, streamErr), nil
	}

	final := streamGuard.Finalize()
	if !final.Decision.Allowed {
		if streamGuard.HasVisibleOutput() {
			return types.AgentResult{}, core.WrapError(core.ErrProviderFailure, "persona final guard rejected streamed output")
		}
		return a.fallbackResult(intent, final.Decision.SafeFallback, final.Decision.Reason, "guard_rejected"), nil
	}
	for _, segment := range final.Segments {
		if err := emitAssistantDelta(ctx, sink, &accepted, segment); err != nil {
			return types.AgentResult{}, err
		}
	}

	content := accepted.String()
	if strings.TrimSpace(content) == "" {
		return a.fallbackResult(intent, a.emptyResponseFallbackCopy(userContent), "", "empty_response"), nil
	}

	generationMode := "llm"
	if a.provider == "" || a.provider == "local" || a.provider == "mock" {
		generationMode = "local"
	}
	return a.generatedResult(intent, content, generationMode, llm.Usage{}), nil
}

type MemoryAgent struct{ BaseAgent }

func NewMemoryAgent(skills *core.SkillRegistry) MemoryAgent {
	return MemoryAgent{BaseAgent: BaseAgent{NameValue: "memory-agent", Skills: skills}}
}

func (a MemoryAgent) CanHandle(intent types.Intent) bool {
	return intent.Name == types.IntentMemoryRecall
}

func (a MemoryAgent) Run(ctx context.Context, _ types.Conversation, intent types.Intent) (types.AgentResult, error) {
	result, err := a.RunSkill(ctx, "mem_recall", map[string]any{"query": intent.Query, "limit": 3})
	if err != nil {
		return types.AgentResult{}, err
	}
	return a.Result(fmt.Sprintf("I found relevant memory: %v", result.Output), confidenceOrDefault(intent), types.Metadata{"skill": result.SkillName}), nil
}

type KnowledgeAgent struct{ BaseAgent }

func NewKnowledgeAgent(skills *core.SkillRegistry) KnowledgeAgent {
	return KnowledgeAgent{BaseAgent: BaseAgent{NameValue: "knowledge-agent", Skills: skills}}
}

func (a KnowledgeAgent) CanHandle(intent types.Intent) bool {
	return intent.Name == types.IntentKnowledgeQuery
}

func (a KnowledgeAgent) Run(ctx context.Context, _ types.Conversation, intent types.Intent) (types.AgentResult, error) {
	result, err := a.RunSkill(ctx, "vector_search", map[string]any{"vector": map[string]any{"values": []float64{}}, "limit": 3})
	if err != nil {
		return types.AgentResult{}, err
	}
	return a.Result(fmt.Sprintf("I found knowledge results for %q: %v", intent.Query, result.Output), confidenceOrDefault(intent), types.Metadata{"skill": result.SkillName}), nil
}

type TaskAgent struct{ BaseAgent }

func NewTaskAgent(skills *core.SkillRegistry) TaskAgent {
	return TaskAgent{BaseAgent: BaseAgent{NameValue: "task-agent", Skills: skills}}
}

func (a TaskAgent) CanHandle(intent types.Intent) bool {
	return intent.Name == types.IntentTaskExecution
}

func (a TaskAgent) Run(ctx context.Context, _ types.Conversation, intent types.Intent) (types.AgentResult, error) {
	result, err := a.RunSkill(ctx, "task_decompose", map[string]any{"request": intent.Query})
	if err != nil {
		return types.AgentResult{}, err
	}
	return a.Result(fmt.Sprintf("I broke the task into steps: %v", result.Output), confidenceOrDefault(intent), types.Metadata{"skill": result.SkillName}), nil
}

type ToolAgent struct{ BaseAgent }

func NewToolAgent(skills *core.SkillRegistry) ToolAgent {
	return ToolAgent{BaseAgent: BaseAgent{NameValue: "tool-agent", Skills: skills}}
}

func (a ToolAgent) CanHandle(intent types.Intent) bool { return intent.Name == types.IntentToolCall }

func (a ToolAgent) Run(ctx context.Context, _ types.Conversation, intent types.Intent) (types.AgentResult, error) {
	result, err := a.RunSkill(ctx, "http_call", map[string]any{"url": fmt.Sprint(intent.Entities["url"])})
	if err != nil {
		return types.AgentResult{}, err
	}
	return a.Result(fmt.Sprintf("Tool call result: %v", result.Output), confidenceOrDefault(intent), types.Metadata{"skill": result.SkillName}), nil
}

type SafetyAgent struct{ BaseAgent }

func NewSafetyAgent(skills *core.SkillRegistry) SafetyAgent {
	return SafetyAgent{BaseAgent: BaseAgent{NameValue: "safety-agent", Skills: skills}}
}

func (a SafetyAgent) CanHandle(intent types.Intent) bool {
	return intent.Name == types.IntentSafetyCheck
}

func (a SafetyAgent) Run(ctx context.Context, _ types.Conversation, intent types.Intent) (types.AgentResult, error) {
	result, err := a.RunSkill(ctx, "risk_classify", map[string]any{"content": intent.Query})
	if err != nil {
		return types.AgentResult{}, err
	}
	return a.Result(fmt.Sprintf("Safety classification: %v", result.Output), confidenceOrDefault(intent), types.Metadata{"skill": result.SkillName}), nil
}

func confidenceOrDefault(intent types.Intent) types.Confidence {
	if intent.Confidence.Valid() && intent.Confidence > 0 {
		return intent.Confidence
	}
	return types.Confidence(0.5)
}

func (a PersonaAgent) preflight(ctx context.Context, conversation types.Conversation, intent types.Intent) (string, error) {
	userContent := lastUserContent(conversation)
	if _, err := a.RunSkill(ctx, "persona_check", map[string]any{"content": userContent, "confidence": float64(intent.Confidence)}); err != nil {
		return "", err
	}
	return userContent, nil
}

func lastUserContent(conversation types.Conversation) string {
	for i := len(conversation.Messages) - 1; i >= 0; i-- {
		if conversation.Messages[i].Role == types.RoleUser {
			return conversation.Messages[i].Content
		}
	}
	return ""
}

func (a PersonaAgent) chatMessages(conversation types.Conversation) ([]types.Message, error) {
	messages := make([]types.Message, 0, len(conversation.Messages)+1)
	for _, message := range conversation.Messages {
		if message.Role == types.RoleSystem {
			continue
		}
		messages = append(messages, message)
	}
	if a.persona.Identity == "" {
		return messages, nil
	}
	prompt, err := a.renderer.SystemPrompt(a.persona, persona.RenderContext{})
	if err != nil {
		return nil, err
	}
	return append([]types.Message{{Role: types.RoleSystem, Content: prompt}}, messages...), nil
}

func asksModelIdentity(content string) bool {
	normalized := strings.ToLower(strings.TrimSpace(content))
	return strings.Contains(normalized, "what model") ||
		strings.Contains(normalized, "which model") ||
		strings.Contains(content, "什么模型") ||
		strings.Contains(content, "背后是什么模型")
}

func (a PersonaAgent) modelIdentityResult(intent types.Intent) types.AgentResult {
	if a.client == nil || a.provider == "" || a.provider == "local" || a.provider == "mock" {
		return a.Result(
			"I'm running in local deterministic mode right now, so there isn't a configured external model behind this session.",
			confidenceOrDefault(intent),
			types.Metadata{"intent": intent.Name, "llm_provider": "local", "generation_mode": "local"},
		)
	}
	return a.Result(
		fmt.Sprintf("This session is configured to use provider %s with model %s.", a.provider, a.model),
		confidenceOrDefault(intent),
		types.Metadata{
			"intent":          intent.Name,
			"llm_provider":    a.provider,
			"llm_model":       a.model,
			"generation_mode": "transparency",
		},
	)
}

func (a PersonaAgent) generatedResult(intent types.Intent, content string, generationMode string, usage llm.Usage) types.AgentResult {
	metadata := types.Metadata{
		"intent":          intent.Name,
		"llm_provider":    a.provider,
		"llm_model":       a.model,
		"generation_mode": generationMode,
	}
	if usage.PromptTokens > 0 {
		metadata["prompt_tokens"] = usage.PromptTokens
	}
	if usage.CompletionTokens > 0 {
		metadata["completion_tokens"] = usage.CompletionTokens
	}
	if usage.TotalTokens > 0 {
		metadata["total_tokens"] = usage.TotalTokens
	}
	return a.Result(content, confidenceOrDefault(intent), metadata)
}

func (a PersonaAgent) fallbackResult(intent types.Intent, content, reason, category string) types.AgentResult {
	metadata := types.Metadata{
		"intent":          intent.Name,
		"llm_provider":    a.provider,
		"llm_model":       a.model,
		"generation_mode": "fallback",
	}
	if reason != "" {
		metadata["guard_reason"] = reason
	}
	if category != "" {
		metadata["fallback_category"] = category
	}
	return a.Result(content, confidenceOrDefault(intent), metadata)
}

func (a PersonaAgent) providerFallbackResult(intent types.Intent, userContent string, err error) types.AgentResult {
	category := llm.ProviderFailureCategory(err)
	if category == "" {
		category = "provider_error"
	}
	return a.fallbackResult(intent, a.providerFallbackCopy(userContent), "", category)
}

func (a PersonaAgent) providerFallbackCopy(userContent string) string {
	provider := a.providerLabel()
	if prefersChinese(userContent) {
		return fmt.Sprintf("%s 当前没有返回可用结果，我先用本地安全回复继续这次对话。请稍后检查 provider 配置或重试。", provider)
	}
	return fmt.Sprintf("The configured provider %s did not return a usable answer, so I am continuing with a local fallback reply for now. Please recheck the provider setup or retry.", provider)
}

func (a PersonaAgent) emptyResponseFallbackCopy(userContent string) string {
	provider := a.providerLabel()
	if prefersChinese(userContent) {
		return fmt.Sprintf("%s 已连接，但这次没有返回可用内容，我先切到本地安全回复继续。", provider)
	}
	return fmt.Sprintf("The configured provider %s returned no usable text for this turn, so I am switching to a local fallback reply.", provider)
}

func (a PersonaAgent) providerLabel() string {
	switch strings.ToLower(strings.TrimSpace(a.provider)) {
	case "", "local", "mock":
		return "local mode"
	case "deepseek":
		return "DeepSeek"
	default:
		return a.provider
	}
}

func prefersChinese(content string) bool {
	for _, r := range content {
		if r >= 0x4e00 && r <= 0x9fff {
			return true
		}
	}
	return false
}

func (a PersonaAgent) guardDecision(content string, confidence float64) persona.GuardDecision {
	if a.persona.Identity == "" {
		return persona.GuardDecision{Allowed: true, Reason: "ok"}
	}
	return persona.Guard{Persona: a.persona}.Check(content, confidence)
}

func emitAssistantDelta(ctx context.Context, sink core.AssistantDeltaSink, accepted *strings.Builder, segment string) error {
	if strings.TrimSpace(segment) == "" {
		return nil
	}
	if sink != nil {
		if err := sink.EmitAssistantDelta(ctx, segment); err != nil {
			return err
		}
	}
	_, _ = accepted.WriteString(segment)
	return nil
}
