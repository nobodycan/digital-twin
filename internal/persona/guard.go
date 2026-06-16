package persona

import "strings"

// Guard checks assistant output against persona boundaries.
type Guard struct {
	Persona Persona
}

// GuardDecision describes whether output is safe for the persona.
type GuardDecision struct {
	Allowed      bool           `json:"allowed"`
	Reason       string         `json:"reason"`
	SafeFallback string         `json:"safe_fallback,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

// Check evaluates content against forbidden claims and low-confidence requirements.
func (g Guard) Check(content string, confidence float64) GuardDecision {
	normalized := normalizeText(content)
	for _, claim := range g.Persona.ForbiddenClaims {
		if claim == "" {
			continue
		}
		if strings.Contains(normalized, normalizeText(claim)) {
			return GuardDecision{
				Allowed:      false,
				Reason:       "forbidden_claim",
				SafeFallback: "I can't claim that. I can explain the tradeoffs and uncertainty instead.",
				Metadata: map[string]any{
					"claim": strings.TrimSpace(claim),
				},
			}
		}
	}

	if confidence < 0.5 && !containsUncertainty(normalized) {
		return GuardDecision{
			Allowed:      false,
			Reason:       "missing_uncertainty",
			SafeFallback: "I'm not certain enough to state that directly. I can share a cautious assessment instead.",
			Metadata: map[string]any{
				"confidence": confidence,
			},
		}
	}

	return GuardDecision{Allowed: true, Reason: "ok"}
}

func containsUncertainty(content string) bool {
	markers := []string{
		"not certain",
		"uncertain",
		"likely",
		"might",
		"may",
		"could",
		"i think",
	}
	for _, marker := range markers {
		if strings.Contains(content, marker) {
			return true
		}
	}
	return false
}
