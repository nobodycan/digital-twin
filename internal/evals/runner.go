package evals

type Evaluator interface {
	Evaluate(Case, EvaluationOutput) CheckResult
}

type SuiteStatus string

const (
	SuitePassed SuiteStatus = "passed"
	SuiteFailed SuiteStatus = "failed"
)

type SuiteResult struct {
	ID            string        `json:"id,omitempty"`
	Status        SuiteStatus   `json:"status"`
	Checks        []CheckResult `json:"checks"`
	FailedCaseIDs []string      `json:"failed_case_ids,omitempty"`
}

type Runner struct {
	Evaluators []Evaluator
}

func (r Runner) Run(cases []Case, outputs map[string]EvaluationOutput) SuiteResult {
	result := SuiteResult{Status: SuitePassed}
	seenFailed := make(map[string]bool)
	for _, evalCase := range cases {
		output := outputs[evalCase.ID]
		for _, evaluator := range r.Evaluators {
			check := evaluator.Evaluate(evalCase, output)
			result.Checks = append(result.Checks, check)
			if check.Status == CheckFailed {
				result.Status = SuiteFailed
				if !seenFailed[evalCase.ID] {
					result.FailedCaseIDs = append(result.FailedCaseIDs, evalCase.ID)
					seenFailed[evalCase.ID] = true
				}
			}
		}
	}
	return result
}
