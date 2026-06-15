package core

import (
	"errors"
	"reflect"
	"testing"

	"github.com/nobodycan/digital-twin/pkg/types"
)

func TestAgentRegistryRegistersListsAndFindsByIntent(t *testing.T) {
	registry := NewAgentRegistry()
	memory := namedAgent{name: "memory", handles: types.IntentMemoryRecall}
	knowledge := namedAgent{name: "knowledge", handles: types.IntentKnowledgeQuery}

	if err := registry.Register(knowledge); err != nil {
		t.Fatalf("register knowledge: %v", err)
	}
	if err := registry.Register(memory); err != nil {
		t.Fatalf("register memory: %v", err)
	}

	if got := registry.Names(); !reflect.DeepEqual(got, []string{"knowledge", "memory"}) {
		t.Fatalf("expected sorted names, got %#v", got)
	}

	got, err := registry.Get("memory")
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if got.Name() != "memory" {
		t.Fatalf("expected memory agent, got %q", got.Name())
	}

	match, err := registry.Find(types.Intent{Name: types.IntentKnowledgeQuery})
	if err != nil {
		t.Fatalf("find knowledge: %v", err)
	}
	if match.Name() != "knowledge" {
		t.Fatalf("expected knowledge agent, got %q", match.Name())
	}
}

func TestAgentRegistryRejectsDuplicatesAndMissingEntries(t *testing.T) {
	registry := NewAgentRegistry()
	if err := registry.Register(namedAgent{name: "memory"}); err != nil {
		t.Fatalf("register memory: %v", err)
	}
	if err := registry.Register(namedAgent{name: "memory"}); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	if _, err := registry.Get("missing"); !errors.Is(err, ErrAgentNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
	if _, err := registry.Find(types.Intent{Name: types.IntentTaskExecution}); !errors.Is(err, ErrAgentNotFound) {
		t.Fatalf("expected no matching agent error, got %v", err)
	}
}

func TestSkillRegistryRegistersAndListsSkills(t *testing.T) {
	registry := NewSkillRegistry()
	if err := registry.Register(namedSkill{name: "memory.write"}); err != nil {
		t.Fatalf("register memory.write: %v", err)
	}
	if err := registry.Register(namedSkill{name: "knowledge.search"}); err != nil {
		t.Fatalf("register knowledge.search: %v", err)
	}

	if got := registry.Names(); !reflect.DeepEqual(got, []string{"knowledge.search", "memory.write"}) {
		t.Fatalf("expected sorted names, got %#v", got)
	}
	if _, err := registry.Get("missing"); !errors.Is(err, ErrSkillNotFound) {
		t.Fatalf("expected skill not found error, got %v", err)
	}
	if err := registry.Register(namedSkill{name: "memory.write"}); !errors.Is(err, ErrDuplicateName) {
		t.Fatalf("expected duplicate error, got %v", err)
	}
}

type namedAgent struct {
	agentFunc
	name    string
	handles types.IntentName
}

func (a namedAgent) Name() string { return a.name }

func (a namedAgent) CanHandle(intent types.Intent) bool {
	return a.handles != "" && intent.Name == a.handles
}

type namedSkill struct {
	skillFunc
	name string
}

func (s namedSkill) Name() string { return s.name }
