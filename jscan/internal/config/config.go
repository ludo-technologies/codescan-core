package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/viper"
)

// Default complexity thresholds based on McCabe complexity standards
const (
	// DefaultLowComplexityThreshold defines the upper bound for low complexity functions
	// Functions with complexity <= 9 are considered low risk and easy to maintain
	DefaultLowComplexityThreshold = 9

	// DefaultMediumComplexityThreshold defines the upper bound for medium complexity functions
	// Functions with complexity 10-19 are considered medium risk and may need refactoring
	DefaultMediumComplexityThreshold = 19

	// DefaultMinComplexityFilter defines the minimum complexity to report
	// Functions with complexity >= 1 will be included in reports
	DefaultMinComplexityFilter = 1

	// DefaultMaxComplexityLimit defines no upper limit for complexity analysis
	// Setting to 0 means no maximum complexity enforcement
	DefaultMaxComplexityLimit = 0
)

// Default dead code detection settings
const (
	// DefaultDeadCodeMinSeverity defines the minimum severity level to report.
	// Reporting from info up is what the analysis has always done, and the
	// health score is calibrated against that full set of findings.
	DefaultDeadCodeMinSeverity = "info"

	// DefaultDeadCodeContextLines defines the number of context lines to show
	DefaultDeadCodeContextLines = 3

	// DefaultDeadCodeSortBy defines the default sorting criteria
	DefaultDeadCodeSortBy = "severity"
)

// Config represents the main configuration structure
type Config struct {
	// Complexity holds complexity analysis configuration
	Complexity ComplexityConfig `json:"complexity" mapstructure:"complexity" yaml:"complexity"`

	// DeadCode holds dead code detection configuration
	DeadCode DeadCodeConfig `json:"dead_code" mapstructure:"dead_code" yaml:"dead_code"`

	// Clones holds the unified clone detection configuration
	Clones *PyscnConfig `json:"clones,omitempty" mapstructure:"clones" yaml:"clones"`

	// SystemAnalysis holds system-level analysis configuration
	SystemAnalysis SystemAnalysisConfig `json:"system_analysis,omitempty" mapstructure:"system_analysis" yaml:"system_analysis"`

	// Dependencies holds dependency analysis configuration
	Dependencies DependencyAnalysisConfig `json:"dependencies,omitempty" mapstructure:"dependencies" yaml:"dependencies"`

	// Architecture holds architecture validation configuration
	Architecture ArchitectureConfig `json:"architecture,omitempty" mapstructure:"architecture" yaml:"architecture"`

	// ModuleAnalysis holds module analysis configuration
	ModuleAnalysis ModuleAnalysisConfig `json:"module_analysis,omitempty" mapstructure:"module_analysis" yaml:"module_analysis"`

	// Output holds output formatting configuration
	Output OutputConfig `json:"output" mapstructure:"output" yaml:"output"`

	// Analysis holds general analysis configuration
	Analysis AnalysisConfig `json:"analysis,omitempty" mapstructure:"analysis" yaml:"analysis"`
}

// ComplexityConfig holds configuration for cyclomatic complexity analysis
type ComplexityConfig struct {
	// LowThreshold is the upper bound for low complexity (inclusive)
	LowThreshold int `json:"low_threshold" mapstructure:"low_threshold" yaml:"low_threshold"`

	// MediumThreshold is the upper bound for medium complexity (inclusive)
	// Values above this are considered high complexity
	MediumThreshold int `json:"medium_threshold" mapstructure:"medium_threshold" yaml:"medium_threshold"`

	// Enabled controls whether complexity analysis is performed
	Enabled bool `json:"enabled" mapstructure:"enabled" yaml:"enabled"`

	// ReportUnchanged controls whether to report functions with complexity = 1
	ReportUnchanged bool `json:"report_unchanged" mapstructure:"report_unchanged" yaml:"report_unchanged"`

	// MaxComplexity is the maximum allowed complexity before failing analysis
	// 0 means no limit
	MaxComplexity int `json:"max_complexity" mapstructure:"max_complexity" yaml:"max_complexity"`
}

// OutputConfig holds configuration for output formatting
type OutputConfig struct {
	// Format specifies the output format: json, yaml, text, csv
	Format string `json:"format" mapstructure:"format" yaml:"format"`

	// ShowDetails controls whether to show detailed breakdown
	ShowDetails bool `json:"show_details" mapstructure:"show_details" yaml:"show_details"`

	// SortBy specifies how to sort results: name, complexity, risk
	SortBy string `json:"sort_by" mapstructure:"sort_by" yaml:"sort_by"`

	// MinComplexity is the minimum complexity to report (filters low values)
	MinComplexity int `json:"min_complexity" mapstructure:"min_complexity" yaml:"min_complexity"`

	// Directory specifies the output directory for reports (empty = tool default, e.g., ".pyscn/reports" under current working directory)
	Directory string `json:"directory" mapstructure:"directory" yaml:"directory"`
}

// DeadCodeConfig holds configuration for dead code detection
type DeadCodeConfig struct {
	// Enabled controls whether dead code detection is performed
	Enabled bool `json:"enabled" mapstructure:"enabled" yaml:"enabled"`

	// MinSeverity is the minimum severity level to report
	MinSeverity string `json:"min_severity" mapstructure:"min_severity" yaml:"min_severity"`

	// ShowContext controls whether to show surrounding code context
	ShowContext bool `json:"show_context" mapstructure:"show_context" yaml:"show_context"`

	// ContextLines is the number of context lines to show around dead code
	ContextLines int `json:"context_lines" mapstructure:"context_lines" yaml:"context_lines"`

	// SortBy specifies how to sort results: severity, line, file, function
	SortBy string `json:"sort_by" mapstructure:"sort_by" yaml:"sort_by"`

	// Detection options
	DetectAfterReturn         bool `json:"detect_after_return" mapstructure:"detect_after_return" yaml:"detect_after_return"`
	DetectAfterBreak          bool `json:"detect_after_break" mapstructure:"detect_after_break" yaml:"detect_after_break"`
	DetectAfterContinue       bool `json:"detect_after_continue" mapstructure:"detect_after_continue" yaml:"detect_after_continue"`
	DetectAfterThrow          bool `json:"detect_after_throw" mapstructure:"detect_after_throw" yaml:"detect_after_throw"`
	DetectUnreachableBranches bool `json:"detect_unreachable_branches" mapstructure:"detect_unreachable_branches" yaml:"detect_unreachable_branches"`

	// IgnorePatterns specifies patterns for code to ignore (e.g., comments, debug code)
	IgnorePatterns []string `json:"ignore_patterns" mapstructure:"ignore_patterns" yaml:"ignore_patterns"`
}

// AnalysisConfig holds general analysis configuration
type AnalysisConfig struct {
	// IncludePatterns specifies file patterns to include
	IncludePatterns []string `json:"include_patterns" mapstructure:"include_patterns" yaml:"include_patterns"`

	// ExcludePatterns specifies file patterns to exclude
	ExcludePatterns []string `json:"exclude_patterns" mapstructure:"exclude_patterns" yaml:"exclude_patterns"`

	// Recursive controls whether to analyze directories recursively
	Recursive bool `json:"recursive" mapstructure:"recursive" yaml:"recursive"`

	// FollowSymlinks controls whether to follow symbolic links
	FollowSymlinks bool `json:"follow_symlinks" mapstructure:"follow_symlinks" yaml:"follow_symlinks"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	config := &Config{
		Complexity: ComplexityConfig{
			LowThreshold:    DefaultLowComplexityThreshold,
			MediumThreshold: DefaultMediumComplexityThreshold,
			Enabled:         true,
			ReportUnchanged: true,
			MaxComplexity:   DefaultMaxComplexityLimit,
		},
		DeadCode: DeadCodeConfig{
			Enabled:                   true,
			MinSeverity:               DefaultDeadCodeMinSeverity,
			ShowContext:               false,
			ContextLines:              DefaultDeadCodeContextLines,
			SortBy:                    DefaultDeadCodeSortBy,
			DetectAfterReturn:         true,
			DetectAfterBreak:          true,
			DetectAfterContinue:       true,
			DetectAfterThrow:          true,
			DetectUnreachableBranches: true,
			IgnorePatterns:            []string{},
		},
		// Use unified pyscn configuration
		Clones: DefaultPyscnConfig(),

		// System analysis configuration
		SystemAnalysis: SystemAnalysisConfig{
			Enabled:               false, // Disabled by default - opt-in feature
			EnableDependencies:    true,
			EnableArchitecture:    true,
			UseComplexityData:     true,
			UseClonesData:         true,
			UseDeadCodeData:       true,
			GenerateUnifiedReport: true,
		},

		// Dependency analysis configuration
		Dependencies: DependencyAnalysisConfig{
			Enabled:           false, // Disabled by default - opt-in feature
			IncludeStdLib:     false,
			IncludeThirdParty: true,
			FollowRelative:    true,
			DetectCycles:      true,
			CalculateMetrics:  true,
			FindLongChains:    true,
			MinCoupling:       0,
			MaxCoupling:       0, // No limit
			MinInstability:    0.0,
			MaxDistance:       1.0,
			SortBy:            "name",
			ShowMatrix:        false,
			ShowMetrics:       false,
			ShowChains:        false,
			GenerateDotGraph:  false,
			CycleReporting:    "summary", // all, critical, summary
			MaxCyclesToShow:   10,
			ShowCyclePaths:    false,
		},

		// Architecture validation configuration
		Architecture: ArchitectureConfig{
			Enabled:                         false, // Disabled by default - opt-in feature
			ValidateLayers:                  true,
			ValidateCohesion:                true,
			ValidateResponsibility:          true,
			Layers:                          []LayerDefinition{}, // Empty by default
			Rules:                           []LayerRule{},       // Empty by default
			MinCohesion:                     0.5,
			MaxCoupling:                     10,
			MaxResponsibilities:             3,
			LayerViolationSeverity:          "error",
			CohesionViolationSeverity:       "warning",
			ResponsibilityViolationSeverity: "warning",
			ShowAllViolations:               false,
			GroupByType:                     true,
			IncludeSuggestions:              true,
			MaxViolationsToShow:             20,
			CustomPatterns:                  []string{},
			AllowedPatterns:                 []string{},
			ForbiddenPatterns:               []string{},
			StrictMode:                      false,
			FailOnViolations:                false,
		},

		// Module analysis configuration
		ModuleAnalysis: ModuleAnalysisConfig{
			Enabled:            false, // Disabled by default - opt-in feature
			IncludeBuiltins:    true,
			ResolveRelative:    false,
			IncludeTypeImports: true,
			AliasPatterns:      []string{"@/", "~/"},
		},

		Output: OutputConfig{
			Format:        "text",
			ShowDetails:   false,
			SortBy:        "complexity",
			MinComplexity: DefaultMinComplexityFilter,
		},
		Analysis: AnalysisConfig{
			IncludePatterns: []string{
				"**/*.js", "**/*.ts", "**/*.jsx", "**/*.tsx",
				"**/*.mjs", "**/*.cjs", "**/*.mts", "**/*.cts",
			},
			ExcludePatterns: []string{
				// Package managers and dependencies
				"node_modules",
				"bower_components",
				"jspm_packages",
				// Vendored / third-party code
				"vendor",
				"assets",
				"overrides",
				"third_party",
				"third-party",
				"extern",
				"external",
				// Build outputs
				"dist",
				"build",
				"out",
				".output",
				// Framework-specific
				".next",
				".nuxt",
				".vercel",
				// Cache directories
				".cache",
				".turbo",
				"coverage",
				// Version control
				".git",
				// Minified and bundled files
				"*.min.js",
				"*.min.mjs",
				"*.min.cjs",
				"*.bundle.js",
				// Source maps
				"*.map",
			},
			Recursive:      true,
			FollowSymlinks: false,
		},
	}

	return config
}

// LoadResult is the outcome of a configuration load.
type LoadResult struct {
	// Config is always populated: the defaults when no file was found, the
	// defaults with the file's values applied on top otherwise.
	Config *Config

	// Path is the file the configuration was read from, empty when no file was
	// found and the defaults were used.
	Path string

	// IgnoredKeys are the keys the file sets that reach no behavior, sorted.
	// They are parsed and validated like any other key, so a wrong value in one
	// still fails the load; they simply change nothing when correct.
	IgnoredKeys []string
}

// appliedKeys lists the configuration keys that reach behavior. Every other key
// in the schema is parsed, validated, and then ignored, which is what
// LoadResult.IgnoredKeys reports so that a caller can say so out loud.
//
// Wiring a key up means adding it here in the same change.
var appliedKeys = map[string]bool{
	"complexity.low_threshold":    true,
	"complexity.medium_threshold": true,
	"complexity.max_complexity":   true,
	"complexity.report_unchanged": true,
	"dead_code.min_severity":      true,
	"dead_code.sort_by":           true,
	"output.min_complexity":       true,
	"output.sort_by":              true,
	"analysis.include_patterns":   true,
	"analysis.exclude_patterns":   true,
	"analysis.recursive":          true,
}

// discoverConfigFile finds the appropriate config file path
// Single responsibility: configuration file discovery only
func discoverConfigFile(targetPath string) string {
	return findDefaultConfig(targetPath)
}

// loadConfigFromFile reads and parses a configuration file
// Single responsibility: file loading and parsing only
func loadConfigFromFile(configPath string) (*LoadResult, error) {
	if configPath == "" {
		return &LoadResult{Config: DefaultConfig()}, nil
	}

	// Create a new viper instance to avoid race conditions
	v := viper.New()
	config := DefaultConfig()
	v.SetConfigFile(configPath)

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	// Unmarshal into config struct
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// No defaults are registered with viper, so AllKeys holds exactly the keys
	// this file sets, including any that the schema does not define.
	var ignored []string
	for _, key := range v.AllKeys() {
		if !appliedKeys[key] {
			ignored = append(ignored, key)
		}
	}
	sort.Strings(ignored)

	return &LoadResult{Config: config, Path: configPath, IgnoredKeys: ignored}, nil
}

// Load loads the configuration for a target path, discovering the file when
// configPath is empty. Orchestrates discovery and loading but delegates
// specific concerns.
func Load(configPath string, targetPath string) (*LoadResult, error) {
	// If no config path specified, discover one
	if configPath == "" {
		configPath = discoverConfigFile(targetPath)
	}

	// Load the configuration from the determined path
	return loadConfigFromFile(configPath)
}

// searchConfigInDirectory searches for configuration files in a specific directory
func searchConfigInDirectory(dir string, candidates []string) string {
	for _, candidate := range candidates {
		path := filepath.Join(dir, candidate)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// findDefaultConfig looks for default configuration files in common locations.
// targetPath is the path being analyzed.
func findDefaultConfig(targetPath string) string {
	candidates := []string{
		"jscan.config.json",
		".jscanrc.json",
		"jscan.yaml",
		"jscan.yml",
		".jscan.toml",
		".jscan.yml",
		"jscan.json",
		".jscan.json",
		// Legacy pyscn names (kept for backward compatibility)
		"pyscn.yaml",
		"pyscn.yml",
		".pyscn.toml",
		".pyscn.yml",
		"pyscn.json",
		".pyscn.json",
	}

	// If targetPath is provided, search from there upward
	if targetPath != "" {
		// Convert to absolute path
		absPath, err := filepath.Abs(targetPath)
		if err == nil {
			// If it's a file, start from its directory
			info, err := os.Stat(absPath)
			if err == nil && !info.IsDir() {
				absPath = filepath.Dir(absPath)
			}

			// Search from target directory up to root with robust termination
			// Handle Windows edge cases: volume roots (C:\), UNC paths (\\server\share), long paths
			volume := filepath.VolumeName(absPath)
			for dir := absPath; ; dir = filepath.Dir(dir) {
				if config := searchConfigInDirectory(dir, candidates); config != "" {
					return config
				}

				// Robust termination conditions for cross-platform compatibility
				parent := filepath.Dir(dir)
				if parent == dir || // Unix-style root reached (/), Windows UNC root (\\server)
					dir == volume || // Windows volume root reached (C:\)
					(volume != "" && dir == volume+string(filepath.Separator)) { // Alternative volume root format
					break
				}
			}
		}
	}

	// Fallback to current directory
	if config := searchConfigInDirectory(".", candidates); config != "" {
		return config
	}

	// Check XDG config directory (Linux/Mac standard)
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		if config := searchConfigInDirectory(filepath.Join(xdgConfig, "jscan"), candidates); config != "" {
			return config
		}
		// Legacy directory
		if config := searchConfigInDirectory(filepath.Join(xdgConfig, "pyscn"), candidates); config != "" {
			return config
		}
	}

	// Check ~/.config/jscan/ (XDG default)
	if home, err := os.UserHomeDir(); err == nil {
		configDir := filepath.Join(home, ".config", "jscan")
		if config := searchConfigInDirectory(configDir, candidates); config != "" {
			return config
		}

		// Legacy directory
		legacyConfigDir := filepath.Join(home, ".config", "pyscn")
		if config := searchConfigInDirectory(legacyConfigDir, candidates); config != "" {
			return config
		}

		// Check home directory (backward compatibility)
		if config := searchConfigInDirectory(home, candidates); config != "" {
			return config
		}
	}

	// Check JSCAN_CONFIG environment variable as fallback.
	// Keep PYSCN_CONFIG for backward compatibility.
	if envConfig := os.Getenv("JSCAN_CONFIG"); envConfig != "" {
		if _, err := os.Stat(envConfig); err == nil {
			return envConfig
		}
	}

	if envConfig := os.Getenv("PYSCN_CONFIG"); envConfig != "" {
		if _, err := os.Stat(envConfig); err == nil {
			return envConfig
		}
	}

	return ""
}

// Validate validates the configuration values
func (c *Config) Validate() error {
	// Validate complexity thresholds
	if c.Complexity.LowThreshold < 1 {
		return fmt.Errorf("complexity.low_threshold must be >= 1, got %d", c.Complexity.LowThreshold)
	}

	if c.Complexity.MediumThreshold <= c.Complexity.LowThreshold {
		return fmt.Errorf("complexity.medium_threshold (%d) must be > low_threshold (%d)",
			c.Complexity.MediumThreshold, c.Complexity.LowThreshold)
	}

	if c.Complexity.MaxComplexity < 0 {
		return fmt.Errorf("complexity.max_complexity must be >= 0, got %d", c.Complexity.MaxComplexity)
	}

	if c.Complexity.MaxComplexity > 0 && c.Complexity.MaxComplexity <= c.Complexity.MediumThreshold {
		return fmt.Errorf("complexity.max_complexity (%d) must be > medium_threshold (%d) or 0 for no limit",
			c.Complexity.MaxComplexity, c.Complexity.MediumThreshold)
	}

	// Validate output format
	validFormats := map[string]bool{
		"text": true,
		"json": true,
		"yaml": true,
		"csv":  true,
		"html": true,
	}

	if !validFormats[c.Output.Format] {
		return fmt.Errorf("invalid output.format '%s', must be one of: text, json, yaml, csv, html", c.Output.Format)
	}

	// Validate sort options
	validSortBy := map[string]bool{
		"name":       true,
		"complexity": true,
		"risk":       true,
	}

	if !validSortBy[c.Output.SortBy] {
		return fmt.Errorf("invalid output.sort_by '%s', must be one of: name, complexity, risk", c.Output.SortBy)
	}

	if c.Output.MinComplexity < 1 {
		return fmt.Errorf("output.min_complexity must be >= 1, got %d", c.Output.MinComplexity)
	}

	// Validate include patterns (at least one must be specified)
	if len(c.Analysis.IncludePatterns) == 0 {
		return fmt.Errorf("analysis.include_patterns cannot be empty")
	}

	// Validate dead code configuration
	if err := c.validateDeadCodeConfig(); err != nil {
		return err
	}

	// Validate clone detection configuration
	if c.Clones != nil {
		if err := c.Clones.Validate(); err != nil {
			return err
		}
	}

	return nil
}

// AssessRiskLevel determines risk level based on complexity and thresholds
func (c *ComplexityConfig) AssessRiskLevel(complexity int) string {
	if complexity <= c.LowThreshold {
		return "low"
	} else if complexity <= c.MediumThreshold {
		return "medium"
	}
	return "high"
}

// validateDeadCodeConfig validates the dead code configuration
func (c *Config) validateDeadCodeConfig() error {
	// Validate severity level
	validSeverities := map[string]bool{
		"critical": true,
		"warning":  true,
		"info":     true,
	}

	if !validSeverities[c.DeadCode.MinSeverity] {
		return fmt.Errorf("invalid dead_code.min_severity '%s', must be one of: critical, warning, info", c.DeadCode.MinSeverity)
	}

	// Validate context lines
	if c.DeadCode.ContextLines < 0 {
		return fmt.Errorf("dead_code.context_lines must be >= 0, got %d", c.DeadCode.ContextLines)
	}

	if c.DeadCode.ContextLines > 20 {
		return fmt.Errorf("dead_code.context_lines cannot exceed 20, got %d", c.DeadCode.ContextLines)
	}

	// Validate sort criteria
	validSortBy := map[string]bool{
		"severity": true,
		"line":     true,
		"file":     true,
		"function": true,
	}

	if !validSortBy[c.DeadCode.SortBy] {
		return fmt.Errorf("invalid dead_code.sort_by '%s', must be one of: severity, line, file, function", c.DeadCode.SortBy)
	}

	return nil
}

// SystemAnalysisConfig holds configuration for system-level analysis
type SystemAnalysisConfig struct {
	// Enabled controls whether system analysis is performed
	Enabled bool `json:"enabled" mapstructure:"enabled" yaml:"enabled"`

	// Analysis components to enable
	EnableDependencies bool `json:"enable_dependencies" mapstructure:"enable_dependencies" yaml:"enable_dependencies"`
	EnableArchitecture bool `json:"enable_architecture" mapstructure:"enable_architecture" yaml:"enable_architecture"`

	// Integration with other analyses
	UseComplexityData bool `json:"use_complexity_data" mapstructure:"use_complexity_data" yaml:"use_complexity_data"`
	UseClonesData     bool `json:"use_clones_data" mapstructure:"use_clones_data" yaml:"use_clones_data"`
	UseDeadCodeData   bool `json:"use_dead_code_data" mapstructure:"use_dead_code_data" yaml:"use_dead_code_data"`

	// Output options
	GenerateUnifiedReport bool `json:"generate_unified_report" mapstructure:"generate_unified_report" yaml:"generate_unified_report"`
}

// DependencyAnalysisConfig holds configuration for dependency analysis
type DependencyAnalysisConfig struct {
	// Enabled controls whether dependency analysis is performed
	Enabled bool `json:"enabled" mapstructure:"enabled" yaml:"enabled"`

	// Scope options
	IncludeStdLib     bool `json:"include_stdlib" mapstructure:"include_stdlib" yaml:"include_stdlib"`
	IncludeThirdParty bool `json:"include_third_party" mapstructure:"include_third_party" yaml:"include_third_party"`
	FollowRelative    bool `json:"follow_relative" mapstructure:"follow_relative" yaml:"follow_relative"`

	// Analysis options
	DetectCycles     bool `json:"detect_cycles" mapstructure:"detect_cycles" yaml:"detect_cycles"`
	CalculateMetrics bool `json:"calculate_metrics" mapstructure:"calculate_metrics" yaml:"calculate_metrics"`
	FindLongChains   bool `json:"find_long_chains" mapstructure:"find_long_chains" yaml:"find_long_chains"`

	// Filtering thresholds
	MinCoupling    int     `json:"min_coupling" mapstructure:"min_coupling" yaml:"min_coupling"`
	MaxCoupling    int     `json:"max_coupling" mapstructure:"max_coupling" yaml:"max_coupling"`
	MinInstability float64 `json:"min_instability" mapstructure:"min_instability" yaml:"min_instability"`
	MaxDistance    float64 `json:"max_distance" mapstructure:"max_distance" yaml:"max_distance"`

	// Reporting options
	SortBy           string `json:"sort_by" mapstructure:"sort_by" yaml:"sort_by"` // name, coupling, instability, distance, risk
	ShowMatrix       bool   `json:"show_matrix" mapstructure:"show_matrix" yaml:"show_matrix"`
	ShowMetrics      bool   `json:"show_metrics" mapstructure:"show_metrics" yaml:"show_metrics"`
	ShowChains       bool   `json:"show_chains" mapstructure:"show_chains" yaml:"show_chains"`
	GenerateDotGraph bool   `json:"generate_dot_graph" mapstructure:"generate_dot_graph" yaml:"generate_dot_graph"`

	// Cycle analysis
	CycleReporting  string `json:"cycle_reporting" mapstructure:"cycle_reporting" yaml:"cycle_reporting"` // all, critical, summary
	MaxCyclesToShow int    `json:"max_cycles_to_show" mapstructure:"max_cycles_to_show" yaml:"max_cycles_to_show"`
	ShowCyclePaths  bool   `json:"show_cycle_paths" mapstructure:"show_cycle_paths" yaml:"show_cycle_paths"`
}

// ArchitectureConfig holds configuration for architecture validation
type ArchitectureConfig struct {
	// Enabled controls whether architecture validation is performed
	Enabled bool `json:"enabled" mapstructure:"enabled" yaml:"enabled"`

	// Validation modes
	ValidateLayers         bool `json:"validate_layers" mapstructure:"validate_layers" yaml:"validate_layers"`
	ValidateCohesion       bool `json:"validate_cohesion" mapstructure:"validate_cohesion" yaml:"validate_cohesion"`
	ValidateResponsibility bool `json:"validate_responsibility" mapstructure:"validate_responsibility" yaml:"validate_responsibility"`

	// Layer definitions
	Layers []LayerDefinition `json:"layers" mapstructure:"layers" yaml:"layers"`
	Rules  []LayerRule       `json:"rules" mapstructure:"rules" yaml:"rules"`

	// Thresholds
	MinCohesion         float64 `json:"min_cohesion" mapstructure:"min_cohesion" yaml:"min_cohesion"`
	MaxCoupling         int     `json:"max_coupling" mapstructure:"max_coupling" yaml:"max_coupling"`
	MaxResponsibilities int     `json:"max_responsibilities" mapstructure:"max_responsibilities" yaml:"max_responsibilities"`

	// Violation severity levels
	LayerViolationSeverity          string `json:"layer_violation_severity" mapstructure:"layer_violation_severity" yaml:"layer_violation_severity"`
	CohesionViolationSeverity       string `json:"cohesion_violation_severity" mapstructure:"cohesion_violation_severity" yaml:"cohesion_violation_severity"`
	ResponsibilityViolationSeverity string `json:"responsibility_violation_severity" mapstructure:"responsibility_violation_severity" yaml:"responsibility_violation_severity"`

	// Reporting options
	ShowAllViolations   bool `json:"show_all_violations" mapstructure:"show_all_violations" yaml:"show_all_violations"`
	GroupByType         bool `json:"group_by_type" mapstructure:"group_by_type" yaml:"group_by_type"`
	IncludeSuggestions  bool `json:"include_suggestions" mapstructure:"include_suggestions" yaml:"include_suggestions"`
	MaxViolationsToShow int  `json:"max_violations_to_show" mapstructure:"max_violations_to_show" yaml:"max_violations_to_show"`

	// Custom rules
	CustomPatterns    []string `json:"custom_patterns" mapstructure:"custom_patterns" yaml:"custom_patterns"`
	AllowedPatterns   []string `json:"allowed_patterns" mapstructure:"allowed_patterns" yaml:"allowed_patterns"`
	ForbiddenPatterns []string `json:"forbidden_patterns" mapstructure:"forbidden_patterns" yaml:"forbidden_patterns"`

	// Strict mode enforcement
	StrictMode       bool `json:"strict_mode" mapstructure:"strict_mode" yaml:"strict_mode"`
	FailOnViolations bool `json:"fail_on_violations" mapstructure:"fail_on_violations" yaml:"fail_on_violations"`
}

// ModuleAnalysisConfig holds configuration for module import/export analysis
type ModuleAnalysisConfig struct {
	// Enabled controls whether module analysis is performed
	Enabled bool `json:"enabled" mapstructure:"enabled" yaml:"enabled"`

	// IncludeBuiltins includes Node.js builtin modules in analysis
	IncludeBuiltins bool `json:"include_builtins" mapstructure:"include_builtins" yaml:"include_builtins"`

	// ResolveRelative enables resolution of relative import paths
	ResolveRelative bool `json:"resolve_relative" mapstructure:"resolve_relative" yaml:"resolve_relative"`

	// IncludeTypeImports includes TypeScript type imports
	IncludeTypeImports bool `json:"include_type_imports" mapstructure:"include_type_imports" yaml:"include_type_imports"`

	// AliasPatterns are path alias patterns to recognize (@/, ~/, etc.)
	AliasPatterns []string `json:"alias_patterns" mapstructure:"alias_patterns" yaml:"alias_patterns"`
}

// LayerDefinition defines an architectural layer
type LayerDefinition struct {
	Name        string   `json:"name" mapstructure:"name" yaml:"name"`
	Packages    []string `json:"packages" mapstructure:"packages" yaml:"packages"`
	Description string   `json:"description" mapstructure:"description" yaml:"description"`
	IsAbstract  bool     `json:"is_abstract" mapstructure:"is_abstract" yaml:"is_abstract"`
}

// LayerRule defines dependency rules between layers
type LayerRule struct {
	From        string   `json:"from" mapstructure:"from" yaml:"from"`
	Allow       []string `json:"allow" mapstructure:"allow" yaml:"allow"`
	Deny        []string `json:"deny" mapstructure:"deny" yaml:"deny"`
	Description string   `json:"description" mapstructure:"description" yaml:"description"`
}
