package domain

// CheckResult represents the result of a quality check
type CheckResult struct {
	Passed      bool             `json:"passed"`
	ExitCode    int              `json:"exit_code"`
	Violations  []CheckViolation `json:"violations"`
	Summary     CheckSummary     `json:"summary"`
	Duration    int64            `json:"duration_ms"`
	GeneratedAt string           `json:"generated_at"`
	Version     string           `json:"version"`
}

// CheckViolation represents a single threshold violation
type CheckViolation struct {
	Category  string `json:"category"`            // complexity, deadcode, deps
	Rule      string `json:"rule"`                // max-complexity, no-dead-code, etc.
	Severity  string `json:"severity"`            // error, warning
	Message   string `json:"message"`             // Human-readable description
	Location  string `json:"location,omitempty"`  // File:line if applicable
	Actual    string `json:"actual"`              // Actual value
	Threshold string `json:"threshold,omitempty"` // Configured threshold
}

// CheckSummary provides aggregate statistics
type CheckSummary struct {
	FilesAnalyzed           int  `json:"files_analyzed"`
	TotalViolations         int  `json:"total_violations"`
	ComplexityChecked       bool `json:"complexity_checked"`
	DeadCodeChecked         bool `json:"deadcode_checked"`
	DepsChecked             bool `json:"deps_checked"`
	HighComplexityFunctions int  `json:"high_complexity_functions"`
	DeadCodeFindings        int  `json:"dead_code_findings"`
	CircularDependencies    int  `json:"circular_dependencies"`
}
