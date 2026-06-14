package core

import (
	"errors"
	"testing"
)

func TestWrapErrorPreservesSentinelMatching(t *testing.T) {
	err := WrapError(ErrAgentNotFound, "load agent")

	if !errors.Is(err, ErrAgentNotFound) {
		t.Fatalf("expected wrapped error to match ErrAgentNotFound")
	}
	if !IsAgentNotFound(err) {
		t.Fatalf("expected IsAgentNotFound to match wrapped error")
	}
	if IsLLMTimeout(err) {
		t.Fatalf("did not expect IsLLMTimeout to match agent error")
	}
}

func TestWrapErrorHandlesNilAndEmptyMessage(t *testing.T) {
	if got := WrapError(nil, "ignored"); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}

	err := WrapError(ErrInvalidConfig, "")
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("expected empty message to preserve original error")
	}
}

func TestDomainErrorPredicates(t *testing.T) {
	tests := []struct {
		name string
		err  error
		ok   func(error) bool
	}{
		{name: "agent not found", err: ErrAgentNotFound, ok: IsAgentNotFound},
		{name: "llm timeout", err: ErrLLMTimeout, ok: IsLLMTimeout},
		{name: "invalid config", err: ErrInvalidConfig, ok: IsInvalidConfig},
		{name: "invalid input", err: ErrInvalidInput, ok: IsInvalidInput},
		{name: "unauthorized", err: ErrUnauthorized, ok: IsUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := WrapError(tt.err, "context")
			if !tt.ok(err) {
				t.Fatalf("expected predicate to match %v", err)
			}
		})
	}
}

func TestResultEnvelope(t *testing.T) {
	ok := Ok("ready")
	if !ok.IsOK() {
		t.Fatalf("expected ok result")
	}

	value, err := ok.Unwrap()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if value != "ready" {
		t.Fatalf("expected value %q, got %q", "ready", value)
	}

	failed := Fail[string](ErrLLMTimeout)
	if failed.IsOK() {
		t.Fatalf("expected failed result")
	}

	_, err = failed.Unwrap()
	if !errors.Is(err, ErrLLMTimeout) {
		t.Fatalf("expected ErrLLMTimeout, got %v", err)
	}
}
