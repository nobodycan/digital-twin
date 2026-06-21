package evals

import "testing"

func TestRunnerFailsSuiteWhenRequiredCheckFails(t *testing.T) {
	runner := Runner{Evaluators: []Evaluator{PersonaEvaluator{}}}
	result := runner.Run([]Case{{
		ID:       "persona-disclosure",
		Category: CategoryPersona,
		Expected: ExpectedBehavior{Persona: &PersonaExpectation{
			MustDiscloseAI: true,
		}},
	}}, map[string]EvaluationOutput{
		"persona-disclosure": {AssistantText: "I am your advisor."},
	})

	if result.Status != SuiteFailed {
		t.Fatalf("suite status = %q, want failed", result.Status)
	}
	if len(result.Checks) != 1 || result.Checks[0].Status != CheckFailed {
		t.Fatalf("checks = %#v", result.Checks)
	}
	if result.FailedCaseIDs[0] != "persona-disclosure" {
		t.Fatalf("failed case IDs = %#v", result.FailedCaseIDs)
	}
}

func TestRunnerReportsSkippedEvaluatorWithReason(t *testing.T) {
	runner := Runner{Evaluators: []Evaluator{PersonaEvaluator{}}}
	result := runner.Run([]Case{{
		ID:       "tool-denied-http",
		Category: CategoryTools,
	}}, map[string]EvaluationOutput{
		"tool-denied-http": {},
	})

	if result.Status != SuitePassed {
		t.Fatalf("suite status = %q, want passed because skipped checks are informational by default", result.Status)
	}
	if len(result.Checks) != 1 || result.Checks[0].Status != CheckSkipped {
		t.Fatalf("checks = %#v", result.Checks)
	}
}
