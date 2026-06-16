package agents

import (
	"context"
	"fmt"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type PersonaAgent struct{ BaseAgent }

func NewPersonaAgent(skills *core.SkillRegistry) PersonaAgent {
	return PersonaAgent{BaseAgent: BaseAgent{NameValue: "persona-agent", Skills: skills}}
}

func (a PersonaAgent) CanHandle(intent types.Intent) bool {
	return intent.Name == types.IntentPersonaChat
}

func (a PersonaAgent) Run(ctx context.Context, conversation types.Conversation, intent types.Intent) (types.AgentResult, error) {
	if _, err := a.RunSkill(ctx, "persona_check", map[string]any{"content": lastUserContent(conversation), "confidence": float64(intent.Confidence)}); err != nil {
		return types.AgentResult{}, err
	}
	return a.Result("I'm here and keeping the same professional persona.", confidenceOrDefault(intent), types.Metadata{"intent": intent.Name}), nil
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

func lastUserContent(conversation types.Conversation) string {
	for i := len(conversation.Messages) - 1; i >= 0; i-- {
		if conversation.Messages[i].Role == types.RoleUser {
			return conversation.Messages[i].Content
		}
	}
	return ""
}
