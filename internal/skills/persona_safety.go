package skills

import (
	"context"
	"regexp"
	"strings"

	"github.com/nobodycan/digital-twin/internal/persona"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type ToneAdjustSkill struct{}

func NewToneAdjustSkill() ToneAdjustSkill { return ToneAdjustSkill{} }
func (s ToneAdjustSkill) Name() string    { return "tone_adjust" }

func (s ToneAdjustSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{
		{Name: "content", Type: String, Required: true},
		{Name: "tone", Type: String, Required: true},
	}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	return types.SkillResult{SkillName: s.Name(), Output: "[" + valid["tone"].(string) + "] " + valid["content"].(string)}, nil
}

type PersonaCheckSkill struct{ guard persona.Guard }

func NewPersonaCheckSkill(guard persona.Guard) PersonaCheckSkill {
	return PersonaCheckSkill{guard: guard}
}
func (s PersonaCheckSkill) Name() string { return "persona_check" }

func (s PersonaCheckSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{
		{Name: "content", Type: String, Required: true},
		{Name: "confidence", Type: Number, Default: 1.0},
	}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	decision := s.guard.Check(valid["content"].(string), valid["confidence"].(float64))
	return types.SkillResult{
		SkillName: s.Name(),
		Output:    decision,
		Metadata:  types.Metadata{"allowed": decision.Allowed, "reason": decision.Reason},
	}, nil
}

type PIIDetectSkill struct{}

func NewPIIDetectSkill() PIIDetectSkill { return PIIDetectSkill{} }
func (s PIIDetectSkill) Name() string   { return "pii_detect" }

func (s PIIDetectSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{{Name: "content", Type: String, Required: true}}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	hasPII := regexp.MustCompile(`(?i)[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}`).MatchString(valid["content"].(string))
	return types.SkillResult{SkillName: s.Name(), Output: hasPII, Metadata: types.Metadata{"contains_pii": hasPII}}, nil
}

type PromptInjectionCheckSkill struct{}

func NewPromptInjectionCheckSkill() PromptInjectionCheckSkill { return PromptInjectionCheckSkill{} }
func (s PromptInjectionCheckSkill) Name() string              { return "prompt_injection_check" }

func (s PromptInjectionCheckSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{{Name: "content", Type: String, Required: true}}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	content := strings.ToLower(valid["content"].(string))
	detected := strings.Contains(content, "ignore previous instructions") || strings.Contains(content, "reveal system prompt")
	return types.SkillResult{SkillName: s.Name(), Output: detected, Metadata: types.Metadata{"prompt_injection": detected}}, nil
}

type RiskClassifySkill struct{}

func NewRiskClassifySkill() RiskClassifySkill { return RiskClassifySkill{} }
func (s RiskClassifySkill) Name() string      { return "risk_classify" }

func (s RiskClassifySkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{{Name: "content", Type: String, Required: true}}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	content := strings.ToLower(valid["content"].(string))
	risk := "low"
	if strings.Contains(content, "delete") || strings.Contains(content, "private") || strings.Contains(content, "password") {
		risk = "high"
	}
	return types.SkillResult{SkillName: s.Name(), Output: risk}, nil
}

type PolicyDecideSkill struct{}

func NewPolicyDecideSkill() PolicyDecideSkill { return PolicyDecideSkill{} }
func (s PolicyDecideSkill) Name() string      { return "policy_decide" }

func (s PolicyDecideSkill) Run(_ context.Context, params map[string]any) (types.SkillResult, error) {
	valid, err := (Spec{Params: []Param{{Name: "risk", Type: String, Required: true}}}).Validate(params)
	if err != nil {
		return types.SkillResult{}, err
	}
	decision := "allow"
	if strings.ToLower(valid["risk"].(string)) == "high" {
		decision = "deny"
	}
	return types.SkillResult{SkillName: s.Name(), Output: decision}, nil
}
