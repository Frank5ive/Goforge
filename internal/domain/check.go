package domain

// Steps of `goforge check`, in the order they run.
const (
	CheckFormat = "format"
	CheckVet    = "vet"
	CheckTest   = "test"
	CheckLint   = "lint"
)

// Statuses of a check step.
const (
	StepPassed  = "passed"
	StepFailed  = "failed"
	StepSkipped = "skipped"
)

// CheckStep is the outcome of one step of `goforge check`.
type CheckStep struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Problems   int    `json:"problems"`
	DurationMS int64  `json:"duration_ms"`
	// Note explains a skipped step.
	Note string `json:"note,omitempty"`
}

// CheckProblem is one thing `goforge check` found wrong, located as precisely
// as the tool that found it allows.
type CheckProblem struct {
	Step string `json:"step"`
	// File is relative to the project, with forward slashes.
	File   string `json:"file,omitempty"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	// Package and Test identify a failing test.
	Package string `json:"package,omitempty"`
	Test    string `json:"test,omitempty"`
	// Linter is the golangci-lint linter that reported a lint problem.
	Linter  string `json:"linter,omitempty"`
	Message string `json:"message"`
}

// CheckReport is the result of `goforge check`.
type CheckReport struct {
	Steps    []CheckStep    `json:"steps"`
	Problems []CheckProblem `json:"problems"`
}

// Passed reports whether every step that ran passed.
func (r CheckReport) Passed() bool {
	for _, step := range r.Steps {
		if step.Status == StepFailed {
			return false
		}
	}
	return true
}

// CheckOptions configures `goforge check`.
type CheckOptions struct {
	// Race runs the tests with the race detector.
	Race bool
}

// ProjectChecker formats, vets, tests, and lints Go projects.
type ProjectChecker interface {
	Check(dir string, opts CheckOptions) (CheckReport, error)
}
