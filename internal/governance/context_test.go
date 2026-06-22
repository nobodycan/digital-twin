package governance

import (
	"errors"
	"testing"

	"github.com/nobodycan/digital-twin/internal/core"
)

func TestRuntimeContextRequiresGovernedVersions(t *testing.T) {
	ctx := RuntimeContext{
		TenantID:              "tenant-1",
		PersonaVersionID:      "persona-v1",
		ToolPolicyVersionID:   "tools-v1",
		KnowledgeVersionID:    "knowledge-v1",
		MemoryPolicyVersionID: "memory-v1",
		ModelPolicyVersionID:  "model-v1",
	}

	if err := ctx.Validate(); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	ctx.ToolPolicyVersionID = ""
	err := ctx.Validate()
	if !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("Validate error = %v, want invalid input", err)
	}
	if !containsAll(err.Error(), "tool_policy_version_id", "expected non-empty string") {
		t.Fatalf("Validate error %q is not actionable", err)
	}
}
