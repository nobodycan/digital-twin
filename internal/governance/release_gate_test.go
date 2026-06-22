package governance

import (
	"testing"
	"time"

	"github.com/nobodycan/digital-twin/internal/evals"
)

func TestReleaseGateBlocksFailingRequiredSuites(t *testing.T) {
	store := NewInMemoryDecisionStore()
	gate := ReleaseGate{Decisions: store}
	candidate := ReleaseCandidate{
		ID:            "release-1",
		TenantID:      "tenant-1",
		Type:          CandidatePersona,
		TargetVersion: "persona-v2",
		CreatedBy:     "operator-1",
		CreatedAt:     time.Now().UTC(),
	}

	decision, err := gate.Decide(candidate, evals.SuiteResult{
		ID:            "run-1",
		Status:        evals.SuiteFailed,
		FailedCaseIDs: []string{"persona-disclosure"},
	})
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if decision.Decision != ReleaseBlocked {
		t.Fatalf("decision = %q, want blocked", decision.Decision)
	}
	if decision.FailedCaseIDs[0] != "persona-disclosure" {
		t.Fatalf("failed case IDs = %#v", decision.FailedCaseIDs)
	}

	records, err := store.ListDecisions("tenant-1")
	if err != nil {
		t.Fatalf("ListDecisions: %v", err)
	}
	if len(records) != 1 || records[0].Type != DecisionRelease {
		t.Fatalf("release decision was not recorded: %#v", records)
	}
}

func TestReleaseGatePermitsPassingSuite(t *testing.T) {
	gate := ReleaseGate{Decisions: NewInMemoryDecisionStore()}

	decision, err := gate.Decide(ReleaseCandidate{
		ID:            "release-1",
		TenantID:      "tenant-1",
		Type:          CandidateToolPolicy,
		TargetVersion: "tools-v2",
		CreatedBy:     "operator-1",
		CreatedAt:     time.Now().UTC(),
	}, evals.SuiteResult{ID: "run-1", Status: evals.SuitePassed})
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if decision.Decision != ReleasePermitted {
		t.Fatalf("decision = %q, want permitted", decision.Decision)
	}
}

func TestReleaseGateBlocksSkippedRequiredChecks(t *testing.T) {
	gate := ReleaseGate{Decisions: NewInMemoryDecisionStore()}

	decision, err := gate.Decide(ReleaseCandidate{
		ID:            "release-1",
		TenantID:      "tenant-1",
		Type:          CandidatePersona,
		TargetVersion: "persona-v2",
		CreatedBy:     "operator-1",
	}, evals.SuiteResult{
		ID:     "run-1",
		Status: evals.SuitePassed,
		Checks: []evals.CheckResult{{
			CaseID:   "tenant-isolation",
			Check:    "tenant",
			Status:   evals.CheckSkipped,
			Required: true,
			Message:  "no tenant evaluator",
		}},
	})
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if decision.Decision != ReleaseBlocked {
		t.Fatalf("decision = %q, want blocked for skipped required check", decision.Decision)
	}
	if len(decision.FailedCaseIDs) != 1 || decision.FailedCaseIDs[0] != "tenant-isolation" {
		t.Fatalf("failed case IDs = %#v", decision.FailedCaseIDs)
	}
}
