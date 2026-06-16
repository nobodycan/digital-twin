package skills

import (
	"errors"
	"strings"
	"testing"
)

func TestSpecValidateRequiresParamsAndAppliesDefaults(t *testing.T) {
	spec := Spec{
		Params: []Param{
			{Name: "query", Type: String, Required: true},
			{Name: "limit", Type: Number, Default: 3.0},
			{Name: "dry_run", Type: Bool, Default: true},
		},
	}

	got, err := spec.Validate(map[string]any{"query": "hello"})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got["query"] != "hello" || got["limit"] != 3.0 || got["dry_run"] != true {
		t.Fatalf("Validate() = %#v, want normalized values with defaults", got)
	}
}

func TestSpecValidateRejectsMissingRequiredParam(t *testing.T) {
	spec := Spec{Params: []Param{{Name: "query", Type: String, Required: true}}}

	_, err := spec.Validate(map[string]any{})
	if err == nil {
		t.Fatal("Validate() error = nil, want validation error")
	}
	if !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("Validate() error = %v, want ErrInvalidParams", err)
	}
	if !strings.Contains(err.Error(), "query") {
		t.Fatalf("Validate() error = %q, want field name", err.Error())
	}
}

func TestSpecValidateRejectsTypeMismatch(t *testing.T) {
	spec := Spec{Params: []Param{{Name: "tags", Type: StringSlice, Required: true}}}

	_, err := spec.Validate(map[string]any{"tags": []any{"ok", 42}})
	if err == nil {
		t.Fatal("Validate() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "tags") || !strings.Contains(err.Error(), "string_slice") {
		t.Fatalf("Validate() error = %q, want type context", err.Error())
	}
}

func TestSpecValidatePreservesUnknownParams(t *testing.T) {
	spec := Spec{Params: []Param{{Name: "query", Type: String, Required: true}}}

	got, err := spec.Validate(map[string]any{"query": "hello", "extra": "kept"})
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if got["extra"] != "kept" {
		t.Fatalf("Validate() extra = %v, want preserved", got["extra"])
	}
}
