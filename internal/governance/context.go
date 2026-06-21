package governance

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/nobodycan/digital-twin/internal/core"
)

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

type RuntimeContext struct {
	TenantID              string `json:"tenant_id"`
	PersonaVersionID      string `json:"persona_version_id"`
	ToolPolicyVersionID   string `json:"tool_policy_version_id"`
	KnowledgeVersionID    string `json:"knowledge_version_id"`
	MemoryPolicyVersionID string `json:"memory_policy_version_id"`
	ModelPolicyVersionID  string `json:"model_policy_version_id"`
}

func (c RuntimeContext) Validate() error {
	required := map[string]string{
		"tenant_id":                c.TenantID,
		"persona_version_id":       c.PersonaVersionID,
		"tool_policy_version_id":   c.ToolPolicyVersionID,
		"knowledge_version_id":     c.KnowledgeVersionID,
		"memory_policy_version_id": c.MemoryPolicyVersionID,
		"model_policy_version_id":  c.ModelPolicyVersionID,
	}
	for field, value := range required {
		if value == "" {
			return fmt.Errorf("%s: expected non-empty string: %w", field, core.ErrInvalidInput)
		}
		if err := validateSafeID(field, value); err != nil {
			return err
		}
	}
	return nil
}

func validateSafeID(field, value string) error {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, `/\`) || !safeIDPattern.MatchString(value) {
		return fmt.Errorf("%s %q: expected safe identifier: %w", field, value, core.ErrInvalidInput)
	}
	return nil
}
