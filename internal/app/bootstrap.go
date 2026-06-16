package app

import (
	"github.com/nobodycan/digital-twin/internal/agents"
	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/internal/persona"
	"github.com/nobodycan/digital-twin/internal/router"
	"github.com/nobodycan/digital-twin/internal/runtime"
	"github.com/nobodycan/digital-twin/internal/skills"
)

type LocalRuntimeConfig struct{}

type LocalRuntime struct {
	Orchestrator runtime.Orchestrator
	Recorder     *runtime.EventRecorder
}

func NewLocalRuntime(LocalRuntimeConfig) (LocalRuntime, error) {
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
	guard := persona.Guard{Persona: defaultPersona}
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
		agents.NewPersonaAgent(skillRegistry),
		agents.NewMemoryAgent(skillRegistry),
		agents.NewKnowledgeAgent(skillRegistry),
		agents.NewTaskAgent(skillRegistry),
		agents.NewToolAgent(skillRegistry),
		agents.NewSafetyAgent(skillRegistry),
	} {
		if err := agentRegistry.Register(agent); err != nil {
			return LocalRuntime{}, err
		}
	}

	recorder := runtime.NewEventRecorder()
	orchestrator := runtime.NewOrchestrator(runtime.OrchestratorConfig{
		Router:   router.NewHybridRouter(router.NewRuleRouter(), nil),
		Agents:   agentRegistry,
		Recorder: recorder,
	})
	return LocalRuntime{Orchestrator: orchestrator, Recorder: recorder}, nil
}
