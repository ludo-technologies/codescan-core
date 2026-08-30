package constants

// Tool name and related constants
const (
	// ToolName is the name of this tool
	ToolName = "jscan"

	// ConfigFileName is the default config file name
	ConfigFileName = ".jscan.toml"

	// EnvVarPrefix is the prefix for environment variables
	EnvVarPrefix = "JSCAN"
)

// Analysis type constants
const (
	AnalysisComplexity = "complexity"
	AnalysisDeadCode   = "deadcode"
	AnalysisClones     = "clones"
	AnalysisCBO        = "cbo"
	AnalysisSystem     = "system"
)

// Output format constants
const (
	OutputFormatText = "text"
	OutputFormatJSON = "json"
	OutputFormatHTML = "html"
	OutputFormatCSV  = "csv"
)

// Clone detection threshold constants
// Calibrated for max(size1, size2) APTED similarity normalization (aligned with pyscn).
const (
	DefaultType1CloneThreshold = 0.85
	DefaultType2CloneThreshold = 0.75
	DefaultType3CloneThreshold = 0.70
	DefaultType4CloneThreshold = 0.65

	// DefaultCloneSimilarityThreshold is the general similarity threshold for clone detection.
	// Aligned with Type-4 threshold to include all detected clones in reports.
	DefaultCloneSimilarityThreshold = 0.65

	// DefaultCloneMinLines is the minimum number of source lines for a code fragment.
	DefaultCloneMinLines = 10

	// DefaultCloneMinNodes is the minimum number of AST nodes for a code fragment.
	DefaultCloneMinNodes = 20

	// DefaultLSHAutoThreshold is the fragment count threshold for automatic LSH activation.
	DefaultLSHAutoThreshold = 500

	// DefaultLSHAutoPairThreshold is the estimated pair count threshold for
	// automatic LSH activation. This catches small repos with enough fragments
	// to make exact pairwise APTED comparisons too expensive.
	DefaultLSHAutoPairThreshold = 10000
)
