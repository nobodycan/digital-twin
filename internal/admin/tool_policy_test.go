package admin

import "testing"

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
