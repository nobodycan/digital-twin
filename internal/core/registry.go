package core

import (
	"sort"
	"sync"

	"github.com/nobodycan/digital-twin/pkg/types"
)

// AgentRegistry stores agents by stable name.
type AgentRegistry struct {
	mu     sync.RWMutex
	agents map[string]Agent
}

// NewAgentRegistry creates an empty agent registry.
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{agents: make(map[string]Agent)}
}

// Register adds agent by name and rejects duplicates.
func (r *AgentRegistry) Register(agent Agent) error {
	name := ""
	if agent != nil {
		name = agent.Name()
	}
	if name == "" {
		return NewNamedError(ErrInvalidInput, "agent", name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.agents[name]; ok {
		return NewNamedError(ErrDuplicateName, "agent", name)
	}
	r.agents[name] = agent
	return nil
}

// Get returns an agent by name.
func (r *AgentRegistry) Get(name string) (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	agent, ok := r.agents[name]
	if !ok {
		return nil, NewNamedError(ErrAgentNotFound, "agent", name)
	}
	return agent, nil
}

// Find returns the first registered agent that can handle intent.
func (r *AgentRegistry) Find(intent types.Intent) (Agent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, name := range sortedKeys(r.agents) {
		agent := r.agents[name]
		if agent.CanHandle(intent) {
			return agent, nil
		}
	}
	return nil, NewNamedError(ErrAgentNotFound, "intent", string(intent.Name))
}

// Names returns registered names in deterministic order.
func (r *AgentRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return sortedKeys(r.agents)
}

// SkillRegistry stores skills by stable name.
type SkillRegistry struct {
	mu     sync.RWMutex
	skills map[string]Skill
}

// NewSkillRegistry creates an empty skill registry.
func NewSkillRegistry() *SkillRegistry {
	return &SkillRegistry{skills: make(map[string]Skill)}
}

// Register adds skill by name and rejects duplicates.
func (r *SkillRegistry) Register(skill Skill) error {
	name := ""
	if skill != nil {
		name = skill.Name()
	}
	if name == "" {
		return NewNamedError(ErrInvalidInput, "skill", name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.skills[name]; ok {
		return NewNamedError(ErrDuplicateName, "skill", name)
	}
	r.skills[name] = skill
	return nil
}

// Get returns a skill by name.
func (r *SkillRegistry) Get(name string) (Skill, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	skill, ok := r.skills[name]
	if !ok {
		return nil, NewNamedError(ErrSkillNotFound, "skill", name)
	}
	return skill, nil
}

// Names returns registered names in deterministic order.
func (r *SkillRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return sortedKeys(r.skills)
}

func sortedKeys[T any](items map[string]T) []string {
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
