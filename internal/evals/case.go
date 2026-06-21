package evals

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type Category string

const (
	CategoryPersona  Category = "persona"
	CategoryRAG      Category = "rag"
	CategoryTools    Category = "tools"
	CategoryMemory   Category = "memory"
	CategorySafety   Category = "safety"
	CategoryTenant   Category = "tenant"
	CategoryCostPerf Category = "cost_performance"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type Case struct {
	ID           string           `json:"id"`
	Title        string           `json:"title"`
	TenantID     string           `json:"tenant_id"`
	Category     Category         `json:"category"`
	RiskLevel    RiskLevel        `json:"risk_level"`
	Conversation []types.Message  `json:"conversation"`
	Expected     ExpectedBehavior `json:"expected"`
}

type ExpectedBehavior struct {
	Persona *PersonaExpectation `json:"persona,omitempty"`
	Tools   []ToolExpectation   `json:"tools,omitempty"`
	Memory  *MemoryExpectation  `json:"memory,omitempty"`
	Safety  *SafetyExpectation  `json:"safety,omitempty"`
	RAG     *RAGExpectation     `json:"rag,omitempty"`
}

type PersonaExpectation struct {
	MustDiscloseAI  bool     `json:"must_disclose_ai,omitempty"`
	ForbiddenClaims []string `json:"forbidden_claims,omitempty"`
}

type ToolExpectation struct {
	Name    string `json:"name"`
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

type MemoryExpectation struct {
	ShouldWrite bool   `json:"should_write"`
	Reason      string `json:"reason,omitempty"`
}

type SafetyExpectation struct {
	Action string `json:"action"`
	Reason string `json:"reason,omitempty"`
}

type RAGExpectation struct {
	RequiredCitations []string `json:"required_citations,omitempty"`
	UnsupportedClaims bool     `json:"unsupported_claims,omitempty"`
}

func LoadCases(dir string) ([]Case, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, core.WrapError(core.ErrInvalidInput, "read eval cases")
	}

	var cases []Case
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, core.WrapError(core.ErrInvalidInput, "read eval case "+entry.Name())
		}
		var evalCase Case
		if err := json.Unmarshal(data, &evalCase); err != nil {
			return nil, core.WrapError(core.ErrInvalidInput, "decode eval case "+entry.Name())
		}
		if err := validateCase(entry.Name(), evalCase); err != nil {
			return nil, err
		}
		cases = append(cases, evalCase)
	}

	sort.Slice(cases, func(i, j int) bool {
		return cases[i].ID < cases[j].ID
	})
	return cases, nil
}

func validateCase(file string, evalCase Case) error {
	if evalCase.ID == "" {
		return caseFieldError(file, "id", "expected non-empty string")
	}
	if evalCase.Title == "" {
		return caseFieldError(file, "title", "expected non-empty string")
	}
	if evalCase.TenantID == "" {
		return caseFieldError(file, "tenant_id", "expected non-empty string")
	}
	if !validCategory(evalCase.Category) {
		return caseFieldError(file, "category", "expected one of persona, rag, tools, memory, safety, tenant, cost_performance")
	}
	if !validRisk(evalCase.RiskLevel) {
		return caseFieldError(file, "risk_level", "expected one of low, medium, high, critical")
	}
	if len(evalCase.Conversation) == 0 {
		return caseFieldError(file, "conversation", "expected at least one message")
	}
	for i, message := range evalCase.Conversation {
		if message.ID == "" {
			return caseFieldError(file, fmt.Sprintf("conversation[%d].id", i), "expected non-empty string")
		}
		if !message.Role.Valid() {
			return caseFieldError(file, fmt.Sprintf("conversation[%d].role", i), "expected one of system, user, assistant, tool")
		}
		if message.Content == "" {
			return caseFieldError(file, fmt.Sprintf("conversation[%d].content", i), "expected non-empty string")
		}
	}
	return nil
}

func validCategory(category Category) bool {
	switch category {
	case CategoryPersona, CategoryRAG, CategoryTools, CategoryMemory, CategorySafety, CategoryTenant, CategoryCostPerf:
		return true
	default:
		return false
	}
}

func validRisk(risk RiskLevel) bool {
	switch risk {
	case RiskLow, RiskMedium, RiskHigh, RiskCritical:
		return true
	default:
		return false
	}
}

func caseFieldError(file, field, fix string) error {
	return fmt.Errorf("%s field %s: %s: %w", file, field, fix, core.ErrInvalidInput)
}
