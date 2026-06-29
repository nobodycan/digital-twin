package app

import (
	"context"
	"os"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/internal/agents"
	"github.com/nobodycan/digital-twin/internal/conversation"
	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/internal/knowledge"
	"github.com/nobodycan/digital-twin/internal/llm"
	"github.com/nobodycan/digital-twin/internal/memory"
	"github.com/nobodycan/digital-twin/internal/persona"
	"github.com/nobodycan/digital-twin/internal/router"
	"github.com/nobodycan/digital-twin/internal/runtime"
	"github.com/nobodycan/digital-twin/internal/skills"
	"github.com/nobodycan/digital-twin/internal/store"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type LocalRuntimeConfig struct {
	TenantID                 string
	PersonaID                string
	SkillAuthorizer          agents.SkillAuthorizer
	PersonaLLM               llm.Client
	PersonaLLMProvider       string
	PersonaLLMModel          string
	PersonaLLMFallbackPolicy string
	DataDir                  string
	MemoryBudget             int
	KnowledgeStore           admin.KnowledgeStore
}

type LocalRuntime struct {
	Orchestrator runtime.Orchestrator
	Recorder     *runtime.EventRecorder
}

func NewLocalRuntime(config LocalRuntimeConfig) (LocalRuntime, error) {
	skillRegistry := core.NewSkillRegistry()
	defaultPersona := persona.Persona{
		ID:            "default",
		Identity:      "Digital Twin",
		Role:          "professional digital advisor",
		Tone:          []string{"calm", "precise"},
		Boundaries:    []string{"state uncertainty when confidence is low"},
		AllowedClaims: []string{"can help with planning and knowledge work"},
		Locale:        "en-US",
	}
	renderer := persona.Renderer{}
	guard := persona.Guard{Persona: defaultPersona}
	var knowledgeGrounder agents.KnowledgeGrounder
	if config.KnowledgeStore != nil {
		service := knowledge.NewService(config.KnowledgeStore)
		knowledgeGrounder = knowledgeGrounderAdapter{service: service}
	}
	for _, skill := range []core.Skill{
		skills.NewMemStoreSkill(nil),
		skills.NewMemRecallSkill(nil),
		skills.NewSummarizeSkill(),
		skills.NewEmbedSkill(nil),
		skills.NewVectorSearchSkill(nil),
		skills.NewCiteSkill(),
		skills.NewTaskDecomposeSkill(),
		skills.NewPlanSkill(),
		skills.NewTrackSkill(),
		skills.NewHTTPCallSkill([]string{"example.com"}),
		skills.NewSearchWebSkill(),
		skills.NewCalendarSkill(),
		skills.NewToneAdjustSkill(),
		skills.NewPersonaCheckSkill(guard),
		skills.NewPIIDetectSkill(),
		skills.NewPromptInjectionCheckSkill(),
		skills.NewRiskClassifySkill(),
		skills.NewPolicyDecideSkill(),
		skills.NewTTSSpeakSkill(),
		skills.NewASRTranscribeSkill(),
		skills.NewAvatarStateSkill(),
		skills.NewSubtitleTimelineSkill(),
	} {
		if err := skillRegistry.Register(skill); err != nil {
			return LocalRuntime{}, err
		}
	}

	agentRegistry := core.NewAgentRegistry()
	for _, agent := range []core.Agent{
		governPersonaAgent(agents.NewPersonaAgent(skillRegistry, agents.PersonaAgentConfig{
			Client:         config.PersonaLLM,
			Provider:       config.PersonaLLMProvider,
			Model:          config.PersonaLLMModel,
			FallbackPolicy: config.PersonaLLMFallbackPolicy,
			Persona:        defaultPersona,
			Renderer:       renderer,
			Knowledge:      knowledgeGrounder,
		}), config),
		governMemoryAgent(agents.NewMemoryAgent(skillRegistry), config),
		governKnowledgeAgent(agents.NewKnowledgeAgent(skillRegistry), config),
		governTaskAgent(agents.NewTaskAgent(skillRegistry), config),
		governToolAgent(agents.NewToolAgent(skillRegistry), config),
		governSafetyAgent(agents.NewSafetyAgent(skillRegistry), config),
	} {
		if err := agentRegistry.Register(agent); err != nil {
			return LocalRuntime{}, err
		}
	}

	recorder := runtime.NewEventRecorder()
	conversationStore, err := newConversationStore(config)
	if err != nil {
		return LocalRuntime{}, err
	}
	budget := config.MemoryBudget
	if budget <= 0 {
		budget = 64
	}
	coordinator := conversation.NewCoordinator(conversation.CoordinatorConfig{
		Store:  conversationStore,
		Memory: memory.NewShortTermMemory(budget),
	})
	orchestrator := runtime.NewOrchestrator(runtime.OrchestratorConfig{
		Router:      router.NewHybridRouter(router.NewRuleRouter(), nil),
		Agents:      agentRegistry,
		Recorder:    recorder,
		Coordinator: coordinator,
	})
	return LocalRuntime{Orchestrator: orchestrator, Recorder: recorder}, nil
}

type knowledgeGrounderAdapter struct {
	service knowledge.Service
}

func (a knowledgeGrounderAdapter) Ground(ctx context.Context, conversation types.Conversation, query string, limit int) (agents.Grounding, error) {
	grounding, err := a.service.Ground(ctx, conversation, query, limit)
	if err != nil {
		return agents.Grounding{}, err
	}
	result := agents.Grounding{RetrievalMode: grounding.RetrievalMode}
	for _, citation := range grounding.Citations {
		result.Citations = append(result.Citations, agents.GroundingCitation{
			DocumentID:   citation.DocumentID,
			DocumentName: citation.DocumentName,
			ChunkID:      citation.ChunkID,
			Rank:         citation.Rank,
			Score:        citation.Score,
			Text:         citation.Text,
		})
	}
	return result, nil
}

func newConversationStore(config LocalRuntimeConfig) (store.Store, error) {
	if config.DataDir == "" {
		return store.NewInMemoryStore(), nil
	}
	if err := os.MkdirAll(config.DataDir, 0o755); err != nil {
		return nil, err
	}
	return store.NewLocalStore(config.DataDir), nil
}

func applyGovernance(base *agents.BaseAgent, config LocalRuntimeConfig) {
	base.TenantID = config.TenantID
	base.PersonaID = config.PersonaID
	base.SkillAuthorizer = config.SkillAuthorizer
}

func governPersonaAgent(agent agents.PersonaAgent, config LocalRuntimeConfig) agents.PersonaAgent {
	applyGovernance(&agent.BaseAgent, config)
	return agent
}

func governMemoryAgent(agent agents.MemoryAgent, config LocalRuntimeConfig) agents.MemoryAgent {
	applyGovernance(&agent.BaseAgent, config)
	return agent
}

func governKnowledgeAgent(agent agents.KnowledgeAgent, config LocalRuntimeConfig) agents.KnowledgeAgent {
	applyGovernance(&agent.BaseAgent, config)
	return agent
}

func governTaskAgent(agent agents.TaskAgent, config LocalRuntimeConfig) agents.TaskAgent {
	applyGovernance(&agent.BaseAgent, config)
	return agent
}

func governToolAgent(agent agents.ToolAgent, config LocalRuntimeConfig) agents.ToolAgent {
	applyGovernance(&agent.BaseAgent, config)
	return agent
}

func governSafetyAgent(agent agents.SafetyAgent, config LocalRuntimeConfig) agents.SafetyAgent {
	applyGovernance(&agent.BaseAgent, config)
	return agent
}
