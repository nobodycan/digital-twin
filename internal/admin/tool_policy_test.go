package admin

import (
	"context"
	"testing"

	"github.com/nobodycan/digital-twin/internal/agents"
)

func TestToolPolicyServiceAllowsOnlyConfiguredTools(t *testing.T) {
	service := NewToolPolicyService(NewInMemoryToolPolicyStore())

	policy, err := service.Save("tenant-1", ToolPolicy{
		PersonaID:    "advisor",
		AllowedTools: []string{"calendar.read", "knowledge.search"},
		ApprovalMode: ApprovalManual,
	})
	if err != nil {
		t.Fatalf("Save returned error: %v", err)
	}
	if policy.ApprovalMode != ApprovalManual {
		t.Fatalf("approval mode = %q", policy.ApprovalMode)
	}

	if err := service.Authorize("tenant-1", "advisor", "knowledge.search"); err != nil {
		t.Fatalf("Authorize allowed tool returned error: %v", err)
	}
	if err := service.Authorize("tenant-1", "advisor", "http.call"); err == nil {
		t.Fatalf("expected unauthorized tool to be rejected")
	}
}

func TestFileToolPolicyStorePersistsPolicy(t *testing.T) {
	dir := t.TempDir()
	first := NewToolPolicyService(NewFileToolPolicyStore(dir))
	if _, err := first.Save("tenant-1", ToolPolicy{PersonaID: "advisor", AllowedTools: []string{"knowledge.search"}}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	second := NewToolPolicyService(NewFileToolPolicyStore(dir))
	if err := second.Authorize("tenant-1", "advisor", "knowledge.search"); err != nil {
		t.Fatalf("Authorize after reopen returned error: %v", err)
	}
}

func TestToolPolicyServiceAuthorizesAgentSkillCalls(t *testing.T) {
	service := NewToolPolicyService(NewInMemoryToolPolicyStore())
	if _, err := service.Save("tenant-1", ToolPolicy{
		PersonaID:    "advisor",
		AllowedTools: []string{"calendar"},
	}); err != nil {
		t.Fatalf("Save returned error: %v", err)
	}

	call := agents.SkillCall{
		TenantID:  "tenant-1",
		PersonaID: "advisor",
		AgentName: "tool-agent",
		SkillName: "calendar",
		Params:    map[string]any{"action": "list"},
	}
	if err := service.AuthorizeSkill(context.Background(), call); err != nil {
		t.Fatalf("AuthorizeSkill allowed tool returned error: %v", err)
	}

	call.SkillName = "http_call"
	if err := service.AuthorizeSkill(context.Background(), call); err == nil {
		t.Fatalf("expected AuthorizeSkill to reject denied tool")
	}
}

var _ agents.SkillAuthorizer = ToolPolicyService{}
