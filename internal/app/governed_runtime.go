package app

import (
	"strings"

	"github.com/nobodycan/digital-twin/internal/admin"
	"github.com/nobodycan/digital-twin/internal/governance"
)

type GovernedRuntimeAdapter struct {
	PersonaAdmin admin.PersonaService
	ToolPolicy   admin.ToolPolicyService
	Defaults     governance.RuntimeContext
}

type GovernedRuntimeResolution struct {
	Context governance.RuntimeContext
	Config  LocalRuntimeConfig
}

func (a GovernedRuntimeAdapter) Resolve(tenantID string) (GovernedRuntimeResolution, error) {
	active, err := a.PersonaAdmin.Active(tenantID)
	if err != nil {
		return GovernedRuntimeResolution{}, err
	}
	context := a.Defaults
	context.TenantID = tenantID
	context.PersonaVersionID = active.ID
	context.ToolPolicyVersionID = "tool-policy-" + safeVersionPart(active.Persona.ID)
	if err := context.Validate(); err != nil {
		return GovernedRuntimeResolution{}, err
	}
	return GovernedRuntimeResolution{
		Context: context,
		Config: LocalRuntimeConfig{
			TenantID:        tenantID,
			PersonaID:       active.Persona.ID,
			SkillAuthorizer: a.ToolPolicy,
		},
	}, nil
}

func safeVersionPart(value string) string {
	value = strings.ReplaceAll(value, " ", "-")
	if value == "" {
		return "default"
	}
	return value
}
