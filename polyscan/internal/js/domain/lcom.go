package domain

// LCOMMetrics is the cohesion measurement of one class.
type LCOMMetrics struct {
	// LCOM4 is the number of connected components among the methods, where
	// two methods are connected when they share an instance variable or one
	// calls the other.
	LCOM4 int `json:"lcom4" yaml:"lcom4"`
	// TotalMethods counts every method of the class; ExcludedMethods the
	// ones kept out of the graph because they cannot reach instance state.
	TotalMethods    int `json:"total_methods" yaml:"total_methods"`
	ExcludedMethods int `json:"excluded_methods" yaml:"excluded_methods"`
	// InstanceVariables counts the distinct instance variables the methods
	// access.
	InstanceVariables int `json:"instance_variables" yaml:"instance_variables"`
	// MethodGroups lists the method names of each connected component.
	MethodGroups [][]string `json:"method_groups" yaml:"method_groups"`
}

// ClassCohesion is the LCOM result for a single class.
type ClassCohesion struct {
	Name      string `json:"name" yaml:"name"`
	FilePath  string `json:"file_path" yaml:"file_path"`
	Language  string `json:"language,omitempty" yaml:"language,omitempty"`
	StartLine int    `json:"start_line" yaml:"start_line"`
	EndLine   int    `json:"end_line" yaml:"end_line"`

	Metrics LCOMMetrics `json:"metrics" yaml:"metrics"`

	RiskLevel RiskLevel `json:"risk_level" yaml:"risk_level"`
}

// LCOMSummary aggregates LCOM statistics.
type LCOMSummary struct {
	TotalClasses int     `json:"total_classes" yaml:"total_classes"`
	AverageLCOM  float64 `json:"average_lcom" yaml:"average_lcom"`
	MaxLCOM      int     `json:"max_lcom" yaml:"max_lcom"`
	MinLCOM      int     `json:"min_lcom" yaml:"min_lcom"`

	LowRiskClasses    int `json:"low_risk_classes" yaml:"low_risk_classes"`
	MediumRiskClasses int `json:"medium_risk_classes" yaml:"medium_risk_classes"`
	HighRiskClasses   int `json:"high_risk_classes" yaml:"high_risk_classes"`
}

// LCOMResponse is the complete LCOM analysis result.
type LCOMResponse struct {
	// Classes is sorted by descending LCOM4, then by location.
	Classes []ClassCohesion `json:"classes" yaml:"classes"`
	Summary LCOMSummary     `json:"summary" yaml:"summary"`

	Warnings []string `json:"warnings" yaml:"warnings"`
	Errors   []string `json:"errors" yaml:"errors"`

	GeneratedAt string      `json:"generated_at" yaml:"generated_at"`
	Version     string      `json:"version" yaml:"version"`
	Config      interface{} `json:"config" yaml:"config"`
}
