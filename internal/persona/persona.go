// Package persona contains persona modeling, prompt rendering, and guard logic.
package persona

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nobodycan/digital-twin/pkg/types"
)

// ErrInvalidPersona marks an invalid persona configuration.
var ErrInvalidPersona = errors.New("invalid persona")

// Persona describes the stable identity and behavioral boundaries of a digital twin.
type Persona struct {
	ID              string         `json:"id"`
	Identity        string         `json:"identity"`
	Role            string         `json:"role"`
	Expertise       []string       `json:"expertise,omitempty"`
	Tone            []string       `json:"tone"`
	Boundaries      []string       `json:"boundaries,omitempty"`
	AllowedClaims   []string       `json:"allowed_claims,omitempty"`
	ForbiddenClaims []string       `json:"forbidden_claims,omitempty"`
	Locale          string         `json:"locale,omitempty"`
	Metadata        types.Metadata `json:"metadata,omitempty"`
}

// Validate checks that persona fields are usable and non-contradictory.
func (p Persona) Validate() error {
	if strings.TrimSpace(p.Identity) == "" {
		return invalidField("identity", "required")
	}
	if strings.TrimSpace(p.Role) == "" {
		return invalidField("role", "required")
	}
	if len(trimmedNonEmpty(p.Tone)) == 0 {
		return invalidField("tone", "at least one value is required")
	}

	if conflict := firstConflict(p.AllowedClaims, p.ForbiddenClaims); conflict != "" {
		return fmt.Errorf("%w: allowed_claims conflicts with forbidden_claims: %q", ErrInvalidPersona, conflict)
	}
	if unsafe := firstUnsafeFragment(p.Boundaries, p.AllowedClaims, p.ForbiddenClaims); unsafe != "" {
		return fmt.Errorf("%w: unsafe prompt fragment: %q", ErrInvalidPersona, unsafe)
	}

	return nil
}

func invalidField(field, reason string) error {
	return fmt.Errorf("%w: %s %s", ErrInvalidPersona, field, reason)
}

func trimmedNonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if v := strings.TrimSpace(value); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func firstConflict(left, right []string) string {
	seen := make(map[string]string, len(left))
	for _, value := range left {
		normalized := normalizeText(value)
		if normalized != "" {
			seen[normalized] = strings.TrimSpace(value)
		}
	}
	for _, value := range right {
		normalized := normalizeText(value)
		if normalized == "" {
			continue
		}
		if original, ok := seen[normalized]; ok {
			return original
		}
	}
	return ""
}

func firstUnsafeFragment(groups ...[]string) string {
	unsafeMarkers := []string{
		"ignore previous instructions",
		"reveal hidden system",
		"bypass safety",
		"developer message",
	}
	for _, group := range groups {
		for _, value := range group {
			normalized := normalizeText(value)
			for _, marker := range unsafeMarkers {
				if strings.Contains(normalized, marker) {
					return strings.TrimSpace(value)
				}
			}
		}
	}
	return ""
}

func normalizeText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}
