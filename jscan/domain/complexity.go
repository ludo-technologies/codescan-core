package domain

import (
	"context"
	"encoding/json"
	"io"
)

// OutputFormat represents the supported output formats
type OutputFormat string

const (
	OutputFormatText OutputFormat = "text"
	OutputFormatJSON OutputFormat = "json"
	OutputFormatYAML OutputFormat = "yaml"
	OutputFormatCSV  OutputFormat = "csv"
	OutputFormatHTML OutputFormat = "html"
	OutputFormatDOT  OutputFormat = "dot"
)

// SortCriteria represents the criteria for sorting results
type SortCriteria string

const (
	SortByComplexity SortCriteria = "complexity"
	SortByName       SortCriteria = "name"
	SortByRisk       SortCriteria = "risk"
	SortBySimilarity SortCriteria = "similarity"
	SortBySize       SortCriteria = "size"
	SortByLocation   SortCriteria = "location"
	SortByCoupling   SortCriteria = "coupling" // For CBO metrics
)

// RiskLevel represents the complexity risk level
type RiskLevel string

const (
	RiskLevelLow    RiskLevel = "low"
	RiskLevelMedium RiskLevel = "medium"
	RiskLevelHigh   RiskLevel = "high"
)

// ModuleFunctionName is the user-facing label used for module-scope (top-level) code
// in places that key/display per-function results. The angle brackets signal that this
// is not a real function defined in the source.
const ModuleFunctionName = "<module>"

// ComplexityRequest represents a request for complexity analysis
type ComplexityRequest struct {
	// Input files or directories to analyze
	Paths []string

	// Output configuration
	OutputFormat OutputFormat
	OutputWriter io.Writer
	OutputPath   string // Path to save output file (for HTML format)
	NoOpen       bool   // Don't auto-open HTML in browser
	ShowDetails  bool

	// Filtering and sorting
	MinComplexity int
	MaxComplexity int // 0 means no limit
	SortBy        SortCriteria

	// Complexity thresholds
	LowThreshold    int
	MediumThreshold int

	// Configuration
	ConfigPath string

	// Analysis options
	Recursive       bool
	IncludePatterns []string
	ExcludePatterns []string
}

// ComplexityMetrics represents detailed complexity metrics for a function
type ComplexityMetrics struct {
	// McCabe cyclomatic complexity
	Complexity int `json:"complexity" yaml:"complexity"`

	// CFG metrics
	Nodes int `json:"nodes" yaml:"nodes"`
	Edges int `json:"edges" yaml:"edges"`

	// Nesting depth
	NestingDepth int `json:"nesting_depth" yaml:"nesting_depth"`

	// Statement counts
	IfStatements      int `json:"if_statements" yaml:"if_statements"`
	LoopStatements    int `json:"loop_statements" yaml:"loop_statements"`
	ExceptionHandlers int `json:"exception_handlers" yaml:"exception_handlers"`
	SwitchCases       int `json:"switch_cases" yaml:"switch_cases"`
}

// FunctionComplexity represents complexity analysis result for a single function
type FunctionComplexity struct {
	// Function identification
	Name        string `json:"name" yaml:"name"`
	FilePath    string `json:"file_path" yaml:"file_path"`
	StartLine   int    `json:"start_line" yaml:"start_line"`
	StartColumn int    `json:"start_column" yaml:"start_column"`
	EndLine     int    `json:"end_line" yaml:"end_line"`

	// Complexity metrics
	Metrics ComplexityMetrics `json:"metrics" yaml:"metrics"`

	// Risk assessment
	RiskLevel RiskLevel `json:"risk_level" yaml:"risk_level"`
}

// ComplexitySummary represents aggregate statistics
type ComplexitySummary struct {
	// TotalFunctions is the post-filter count (functions included in results after min_complexity filtering).
	TotalFunctions int `json:"total_functions" yaml:"total_functions"`
	// FunctionsParsed is the count of reportable functions: parsed functions that survived
	// report_unchanged, counted before the min/max complexity filters. Functions dropped by
	// report_unchanged are excluded from both counts, so the difference between
	// FunctionsParsed and TotalFunctions is attributable to complexity filtering alone.
	FunctionsParsed   int     `json:"functions_parsed" yaml:"functions_parsed"`
	AverageComplexity float64 `json:"average_complexity" yaml:"average_complexity"`
	MaxComplexity     int     `json:"max_complexity" yaml:"max_complexity"`
	MinComplexity     int     `json:"min_complexity" yaml:"min_complexity"`
	FilesAnalyzed     int     `json:"files_analyzed" yaml:"files_analyzed"`

	// Risk distribution
	LowRiskFunctions    int `json:"low_risk_functions" yaml:"low_risk_functions"`
	MediumRiskFunctions int `json:"medium_risk_functions" yaml:"medium_risk_functions"`
	HighRiskFunctions   int `json:"high_risk_functions" yaml:"high_risk_functions"`

	// Complexity distribution
	ComplexityDistribution map[string]int `json:"complexity_distribution,omitempty" yaml:"complexity_distribution,omitempty"`
}

// DirectoryComplexityMetrics aggregates reported ComplexityResponse.Functions
// entries for one project-root-relative directory.
type DirectoryComplexityMetrics struct {
	DirectoryPath         string  `json:"directory_path" yaml:"directory_path"`
	FunctionCount         int     `json:"function_count" yaml:"function_count"`
	AverageComplexity     float64 `json:"average_complexity" yaml:"average_complexity"`
	MaxComplexity         int     `json:"max_complexity" yaml:"max_complexity"`
	HighRiskFunctionCount int     `json:"high_risk_function_count" yaml:"high_risk_function_count"`
	AverageNestingDepth   float64 `json:"average_nesting_depth" yaml:"average_nesting_depth"`
	MaxNestingDepth       int     `json:"max_nesting_depth" yaml:"max_nesting_depth"`
}

// DirectoryComplexityMetricsList is the stable serialized collection contract.
// A zero value is encoded as an empty array so callers never need to distinguish
// an uninitialized collection from a completed analysis with no reported rows.
type DirectoryComplexityMetricsList []DirectoryComplexityMetrics

// MarshalJSON encodes an uninitialized collection as an empty JSON array.
func (metrics DirectoryComplexityMetricsList) MarshalJSON() ([]byte, error) {
	if metrics == nil {
		return []byte("[]"), nil
	}
	type plainDirectoryComplexityMetricsList DirectoryComplexityMetricsList
	return json.Marshal(plainDirectoryComplexityMetricsList(metrics))
}

// MarshalYAML encodes an uninitialized collection as an empty YAML array.
func (metrics DirectoryComplexityMetricsList) MarshalYAML() (interface{}, error) {
	if metrics == nil {
		return []DirectoryComplexityMetrics{}, nil
	}
	return []DirectoryComplexityMetrics(metrics), nil
}

// ComplexityResponse represents the complete analysis result
type ComplexityResponse struct {
	// Analysis results
	Functions   []FunctionComplexity           `json:"functions" yaml:"functions"`
	ByDirectory DirectoryComplexityMetricsList `json:"by_directory" yaml:"by_directory"`
	Summary     ComplexitySummary              `json:"summary" yaml:"summary"`

	// ModuleRollups are derived before the report filters are applied, so they
	// describe the whole analyzed population rather than what min/max complexity
	// left visible. They are consumed by the unified analyze command and are not
	// part of standalone complexity output.
	ModuleRollups map[string]ModuleComplexityMetrics `json:"-" yaml:"-"`

	// Warnings and issues
	Warnings []string `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Errors   []string `json:"errors,omitempty" yaml:"errors,omitempty"`

	// Metadata
	GeneratedAt string      `json:"generated_at" yaml:"generated_at"`
	Version     string      `json:"version" yaml:"version"`
	Config      interface{} `json:"config,omitempty" yaml:"config,omitempty"` // Configuration used for analysis
}

// ComplexityService defines the core business logic for complexity analysis
type ComplexityService interface {
	// Analyze performs complexity analysis on the given request
	Analyze(ctx context.Context, req ComplexityRequest) (*ComplexityResponse, error)

	// AnalyzeFile analyzes a single JavaScript/TypeScript file
	AnalyzeFile(ctx context.Context, filePath string, req ComplexityRequest) (*ComplexityResponse, error)
}

// FileReader defines the legacy interface for reading and collecting files.
// NOTE: Historical Python-style method names are kept for API compatibility.
type FileReader interface {
	// CollectPythonFiles recursively finds all JavaScript/TypeScript files in the given paths.
	CollectPythonFiles(paths []string, recursive bool, includePatterns, excludePatterns []string) ([]string, error)

	// ReadFile reads the content of a file
	ReadFile(path string) ([]byte, error)

	// IsValidPythonFile checks if a file is a valid JavaScript/TypeScript file.
	IsValidPythonFile(path string) bool

	// FileExists checks if a file exists and returns an error if not
	FileExists(path string) (bool, error)
}

// JSFileReader defines JavaScript/TypeScript-specific file operations.
type JSFileReader interface {
	CollectJSFiles(paths []string, recursive bool, includePatterns, excludePatterns []string) ([]string, error)
	ReadFile(path string) ([]byte, error)
	IsValidJSFile(path string) bool
	FileExists(path string) (bool, error)
}

// OutputFormatter defines the interface for formatting analysis results
type OutputFormatter interface {
	// Format formats the analysis response according to the specified format
	Format(response *ComplexityResponse, format OutputFormat) (string, error)

	// Write writes the formatted output to the writer
	Write(response *ComplexityResponse, format OutputFormat, writer io.Writer) error
}
