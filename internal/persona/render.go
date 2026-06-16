package persona

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Renderer renders a validated persona into model-facing instructions.
type Renderer struct {
	Now func() time.Time
}

// RenderContext contains runtime values injected into a system prompt.
type RenderContext struct {
	TenantName string
	UserName   string
}

// SystemPrompt renders persona and context into a stable system prompt.
func (r Renderer) SystemPrompt(p Persona, ctx RenderContext) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}

	now := time.Now().UTC()
	if r.Now != nil {
		now = r.Now().UTC()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Identity: %s\n", strings.TrimSpace(p.Identity))
	fmt.Fprintf(&b, "Role: %s\n", strings.TrimSpace(p.Role))
	fmt.Fprintf(&b, "Locale: %s\n", strings.TrimSpace(p.Locale))
	fmt.Fprintf(&b, "Current Time: %s\n", now.Format(time.RFC3339))
	fmt.Fprintf(&b, "Tenant: %s\n", strings.TrimSpace(ctx.TenantName))
	fmt.Fprintf(&b, "User: %s\n", strings.TrimSpace(ctx.UserName))

	writeList(&b, "Expertise", p.Expertise, true)
	writeList(&b, "Tone", p.Tone, true)
	writeList(&b, "Boundaries", p.Boundaries, false)
	writeList(&b, "Allowed Claims", p.AllowedClaims, false)
	writeList(&b, "Forbidden Claims", p.ForbiddenClaims, false)
	writeMetadata(&b, p.Metadata)

	return b.String(), nil
}

func writeList(b *strings.Builder, label string, values []string, sortValues bool) {
	items := trimmedNonEmpty(values)
	if sortValues {
		sort.Strings(items)
	}
	fmt.Fprintf(b, "\n%s:\n", label)
	for _, item := range items {
		fmt.Fprintf(b, "- %s\n", item)
	}
}

func writeMetadata(b *strings.Builder, values map[string]any) {
	fmt.Fprint(b, "\nMetadata:\n")
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(b, "- %s=%v\n", key, values[key])
	}
}
