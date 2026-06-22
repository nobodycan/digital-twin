package app

import (
	"testing"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/internal/governance"
	"github.com/nobodycan/digital-twin/internal/persona"
)

func TestGovernedRuntimeAdapterResolvesActiveAdminVersions(t *testing.T) {
	personas := admin.NewPersonaService(admin.NewInMemoryPersonaStore())
	tools := admin.NewToolPolicyService(admin.NewInMemoryToolPolicyStore())
	draft, err := personas.SaveDraft("tenant-1", persona.Persona{
		ID:            "advisor",
		Identity:      "Digital Twin",
		Role:          "professional advisor",
		Tone:          []string{"calm"},
		Boundaries:    []string{"disclose AI identity"},
		AllowedClaims: []string{"can help with planning"},
		Locale:        "en-US",
	})
	if err != nil {
		t.Fatalf("SaveDraft: %v", err)
	}
	published, err := personas.Publish("tenant-1", draft.ID)
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if _, err := tools.Save("tenant-1", admin.ToolPolicy{
		PersonaID:    "advisor",
		AllowedTools: []string{"knowledge_search"},
	}); err != nil {
		t.Fatalf("Save tool policy: %v", err)
	}

	adapter := GovernedRuntimeAdapter{
		PersonaAdmin: personas,
		ToolPolicy:   tools,
		Defaults: governance.RuntimeContext{
			KnowledgeVersionID:    "knowledge-v1",
			MemoryPolicyVersionID: "memory-v1",
			ModelPolicyVersionID:  "model-v1",
		},
	}

	resolved, err := adapter.Resolve("tenant-1")
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if resolved.Context.TenantID != "tenant-1" || resolved.Context.PersonaVersionID != published.ID {
		t.Fatalf("context = %#v, want active persona version", resolved.Context)
	}
	if resolved.Config.PersonaID != "advisor" || resolved.Config.SkillAuthorizer == nil {
		t.Fatalf("config = %#v, want active persona and tool authorizer", resolved.Config)
	}
	if err := resolved.Context.Validate(); err != nil {
		t.Fatalf("resolved context should validate: %v", err)
	}
}
