package governance

import (
	"fmt"
	"time"

	"github.com/nobodycan/digital-twin/internal/core"
	"github.com/nobodycan/digital-twin/internal/evals"
	"github.com/nobodycan/digital-twin/pkg/types"
)

type CandidateType string

const (
	CandidatePersona     CandidateType = "persona"
	CandidateKnowledge   CandidateType = "knowledge"
	CandidateToolPolicy  CandidateType = "tool_policy"
	CandidateMemory      CandidateType = "memory"
	CandidateModelPolicy CandidateType = "model_policy"
)

type ReleaseDecision string

const (
	ReleasePermitted ReleaseDecision = "permitted"
	ReleaseBlocked   ReleaseDecision = "blocked"
)

type ReleaseCandidate struct {
	ID            string        `json:"id"`
	TenantID      string        `json:"tenant_id"`
	Type          CandidateType `json:"type"`
	TargetVersion string        `json:"target_version"`
	CreatedBy     string        `json:"created_by"`
	CreatedAt     time.Time     `json:"created_at"`
}

type GateDecision struct {
	CandidateID   string          `json:"candidate_id"`
	TenantID      string          `json:"tenant_id"`
	EvalRunID     string          `json:"eval_run_id"`
	Decision      ReleaseDecision `json:"decision"`
	FailedCaseIDs []string        `json:"failed_case_ids,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
}

type ReleaseGate struct {
	Decisions DecisionStore
}

func (g ReleaseGate) Decide(candidate ReleaseCandidate, suite evals.SuiteResult) (GateDecision, error) {
	if err := validateReleaseCandidate(candidate); err != nil {
		return GateDecision{}, err
	}
	decision := GateDecision{
		CandidateID:   candidate.ID,
		TenantID:      candidate.TenantID,
		EvalRunID:     suite.ID,
		Decision:      ReleasePermitted,
		FailedCaseIDs: append([]string(nil), suite.FailedCaseIDs...),
		CreatedAt:     time.Now().UTC(),
	}
	if suite.Status == evals.SuiteFailed {
		decision.Decision = ReleaseBlocked
	}
	if g.Decisions != nil {
		err := g.Decisions.SaveDecision(DecisionRecord{
			ID:        "release-" + candidate.ID,
			TenantID:  candidate.TenantID,
			Type:      DecisionRelease,
			ActorID:   candidate.CreatedBy,
			CreatedAt: decision.CreatedAt,
			Evidence: types.Metadata{
				"candidate_id":    candidate.ID,
				"candidate_type":  candidate.Type,
				"target_version":  candidate.TargetVersion,
				"eval_run_id":     suite.ID,
				"decision":        decision.Decision,
				"failed_case_ids": decision.FailedCaseIDs,
			},
		})
		if err != nil {
			return GateDecision{}, err
		}
	}
	return decision, nil
}

func validateReleaseCandidate(candidate ReleaseCandidate) error {
	for field, value := range map[string]string{
		"candidate_id":   candidate.ID,
		"tenant_id":      candidate.TenantID,
		"target_version": candidate.TargetVersion,
		"created_by":     candidate.CreatedBy,
	} {
		if err := validateSafeID(field, value); err != nil {
			return err
		}
	}
	if !validCandidateType(candidate.Type) {
		return fmt.Errorf("candidate_type %q: expected known candidate type: %w", candidate.Type, core.ErrInvalidInput)
	}
	return nil
}

func validCandidateType(value CandidateType) bool {
	switch value {
	case CandidatePersona, CandidateKnowledge, CandidateToolPolicy, CandidateMemory, CandidateModelPolicy:
		return true
	default:
		return false
	}
}
