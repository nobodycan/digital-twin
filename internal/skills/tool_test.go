package skills

import (
	"context"
	"errors"
	"testing"
)

func TestHTTPCallSkillAllowsOnlyAllowlistedPublicTargets(t *testing.T) {
	skill := NewHTTPCallSkill([]string{"api.example.com"})

	result, err := skill.Run(context.Background(), map[string]any{"url": "https://api.example.com/data"})
	if err != nil {
		t.Fatalf("http_call Run() error = %v", err)
	}
	if result.SkillName != "http_call" || result.Metadata["allowed"] != true {
		t.Fatalf("http_call result = %#v, want allowed metadata", result)
	}
}

func TestHTTPCallSkillRejectsNonAllowlistedTargets(t *testing.T) {
	skill := NewHTTPCallSkill([]string{"api.example.com"})

	_, err := skill.Run(context.Background(), map[string]any{"url": "https://evil.example.com/data"})
	if !errors.Is(err, ErrDenied) {
		t.Fatalf("http_call error = %v, want ErrDenied", err)
	}
}

func TestHTTPCallSkillRejectsLocalTargets(t *testing.T) {
	skill := NewHTTPCallSkill([]string{"127.0.0.1"})

	_, err := skill.Run(context.Background(), map[string]any{"url": "http://127.0.0.1/admin"})
	if !errors.Is(err, ErrDenied) {
		t.Fatalf("http_call error = %v, want ErrDenied", err)
	}
}

func TestHTTPCallSkillRejectsNonHTTPSchemes(t *testing.T) {
	skill := NewHTTPCallSkill([]string{"api.example.com"})

	_, err := skill.Run(context.Background(), map[string]any{"url": "file://api.example.com/etc/passwd"})
	if !errors.Is(err, ErrDenied) {
		t.Fatalf("http_call error = %v, want ErrDenied", err)
	}
}

func TestSearchWebAndCalendarAreDeterministicPlaceholders(t *testing.T) {
	search, err := NewSearchWebSkill().Run(context.Background(), map[string]any{"query": "digital twin"})
	if err != nil {
		t.Fatalf("search_web Run() error = %v", err)
	}
	if search.SkillName != "search_web" {
		t.Fatalf("search_web skill = %q", search.SkillName)
	}

	calendar, err := NewCalendarSkill().Run(context.Background(), map[string]any{"action": "list"})
	if err != nil {
		t.Fatalf("calendar Run() error = %v", err)
	}
	if calendar.Metadata["placeholder"] != true {
		t.Fatalf("calendar metadata = %#v, want placeholder", calendar.Metadata)
	}
}
