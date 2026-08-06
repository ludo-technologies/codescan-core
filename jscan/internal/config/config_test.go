package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config == nil {
		t.Fatal("DefaultConfig should not return nil")
	}

	// Verify complexity defaults
	if config.Complexity.LowThreshold != DefaultLowComplexityThreshold {
		t.Errorf("Expected LowThreshold %d, got %d", DefaultLowComplexityThreshold, config.Complexity.LowThreshold)
	}
	if config.Complexity.MediumThreshold != DefaultMediumComplexityThreshold {
		t.Errorf("Expected MediumThreshold %d, got %d", DefaultMediumComplexityThreshold, config.Complexity.MediumThreshold)
	}
	if !config.Complexity.Enabled {
		t.Error("Complexity should be enabled by default")
	}
	if !config.Complexity.ReportUnchanged {
		t.Error("ReportUnchanged should be true by default")
	}

	// Verify dead code defaults
	if !config.DeadCode.Enabled {
		t.Error("DeadCode should be enabled by default")
	}
	if config.DeadCode.MinSeverity != DefaultDeadCodeMinSeverity {
		t.Errorf("Expected MinSeverity %s, got %s", DefaultDeadCodeMinSeverity, config.DeadCode.MinSeverity)
	}
	if config.DeadCode.ContextLines != DefaultDeadCodeContextLines {
		t.Errorf("Expected ContextLines %d, got %d", DefaultDeadCodeContextLines, config.DeadCode.ContextLines)
	}

	// Verify output defaults
	if config.Output.Format != "text" {
		t.Errorf("Expected Format 'text', got '%s'", config.Output.Format)
	}
	if config.Output.SortBy != "complexity" {
		t.Errorf("Expected SortBy 'complexity', got '%s'", config.Output.SortBy)
	}

	// Verify analysis defaults
	if !config.Analysis.Recursive {
		t.Error("Recursive should be true by default")
	}
	if len(config.Analysis.IncludePatterns) == 0 {
		t.Error("IncludePatterns should not be empty")
	}
	if len(config.Analysis.ExcludePatterns) == 0 {
		t.Error("ExcludePatterns should not be empty")
	}
}

func TestConfig_Validate_Valid(t *testing.T) {
	config := DefaultConfig()

	err := config.Validate()
	if err != nil {
		t.Errorf("Default config should be valid, got error: %v", err)
	}
}

func TestConfig_Validate_InvalidLowThreshold(t *testing.T) {
	config := DefaultConfig()
	config.Complexity.LowThreshold = 0

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for LowThreshold < 1")
	}
}

func TestConfig_Validate_InvalidMediumThreshold(t *testing.T) {
	config := DefaultConfig()
	config.Complexity.MediumThreshold = config.Complexity.LowThreshold

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for MediumThreshold <= LowThreshold")
	}
}

func TestConfig_Validate_InvalidMaxComplexity(t *testing.T) {
	config := DefaultConfig()
	config.Complexity.MaxComplexity = -1

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for MaxComplexity < 0")
	}
}

func TestConfig_Validate_MaxComplexityTooLow(t *testing.T) {
	config := DefaultConfig()
	config.Complexity.MaxComplexity = config.Complexity.MediumThreshold

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for MaxComplexity <= MediumThreshold")
	}
}

func TestConfig_Validate_InvalidOutputFormat(t *testing.T) {
	config := DefaultConfig()
	config.Output.Format = "xml"

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for invalid output format")
	}
}

func TestConfig_Validate_InvalidSortBy(t *testing.T) {
	config := DefaultConfig()
	config.Output.SortBy = "invalid"

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for invalid sort_by")
	}
}

func TestConfig_Validate_InvalidMinComplexity(t *testing.T) {
	config := DefaultConfig()
	config.Output.MinComplexity = 0

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for MinComplexity < 1")
	}
}

func TestConfig_Validate_EmptyIncludePatterns(t *testing.T) {
	config := DefaultConfig()
	config.Analysis.IncludePatterns = []string{}

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for empty include patterns")
	}
}

func TestConfig_Validate_InvalidDeadCodeSeverity(t *testing.T) {
	config := DefaultConfig()
	config.DeadCode.MinSeverity = "invalid"

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for invalid dead code severity")
	}
}

func TestConfig_Validate_InvalidContextLines(t *testing.T) {
	config := DefaultConfig()
	config.DeadCode.ContextLines = -1

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for negative context lines")
	}

	config.DeadCode.ContextLines = 25
	err = config.Validate()
	if err == nil {
		t.Error("Expected error for context lines > 20")
	}
}

func TestConfig_Validate_InvalidDeadCodeSortBy(t *testing.T) {
	config := DefaultConfig()
	config.DeadCode.SortBy = "invalid"

	err := config.Validate()
	if err == nil {
		t.Error("Expected error for invalid dead code sort_by")
	}
}

func TestComplexityConfig_AssessRiskLevel(t *testing.T) {
	config := &ComplexityConfig{
		LowThreshold:    5,
		MediumThreshold: 10,
	}

	tests := []struct {
		complexity int
		expected   string
	}{
		{1, "low"},
		{5, "low"},
		{6, "medium"},
		{10, "medium"},
		{11, "high"},
		{100, "high"},
	}

	for _, tc := range tests {
		result := config.AssessRiskLevel(tc.complexity)
		if result != tc.expected {
			t.Errorf("AssessRiskLevel(%d) = %s, expected %s", tc.complexity, result, tc.expected)
		}
	}
}

func TestLoad_Default(t *testing.T) {
	// Load with empty paths should return default
	result, err := Load("", "")
	if err != nil {
		t.Fatalf("Load with empty path failed: %v", err)
	}
	if result.Config == nil {
		t.Fatal("Config should not be nil")
	}

	// Verify it matches default
	defaultCfg := DefaultConfig()
	if result.Config.Complexity.LowThreshold != defaultCfg.Complexity.LowThreshold {
		t.Error("Loaded config should match default")
	}
}

func TestLoad_NonExistent(t *testing.T) {
	_, err := Load("/nonexistent/path/config.yaml", "")
	if err == nil {
		t.Error("Expected error for non-existent config file")
	}
}

func TestLoad_IgnoredKeys(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "jscan.config.json")
	contents := `{
  "complexity": {"low_threshold": 5, "medium_threshold": 10, "enabled": false},
  "output": {"format": "json", "sort_by": "name"},
  "analysis": {"exclude_patterns": ["dist"], "follow_symlinks": true},
  "typo_group": {"whatever": 1}
}`
	if err := os.WriteFile(configPath, []byte(contents), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	result, err := Load(configPath, "")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if result.Path != configPath {
		t.Errorf("Path = %q, expected %q", result.Path, configPath)
	}

	// Keys that reach behavior are absent, keys that do not are listed, and an
	// unknown key is reported like any other key that changes nothing.
	expected := []string{
		"analysis.follow_symlinks",
		"complexity.enabled",
		"output.format",
		"typo_group.whatever",
	}
	if len(result.IgnoredKeys) != len(expected) {
		t.Fatalf("IgnoredKeys = %v, expected %v", result.IgnoredKeys, expected)
	}
	for i, key := range expected {
		if result.IgnoredKeys[i] != key {
			t.Errorf("IgnoredKeys[%d] = %q, expected %q", i, result.IgnoredKeys[i], key)
		}
	}
}

func TestLoad_NoIgnoredKeysWithoutFile(t *testing.T) {
	result, err := Load("", "")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if result.Path != "" {
		t.Errorf("Path = %q, expected empty when no file was found", result.Path)
	}
	if len(result.IgnoredKeys) != 0 {
		t.Errorf("IgnoredKeys = %v, expected none", result.IgnoredKeys)
	}
}

func TestSearchConfigInDirectory(t *testing.T) {
	// Create temp directory with config file
	tempDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a config file
	configPath := filepath.Join(tempDir, "jscan.yaml")
	err = os.WriteFile(configPath, []byte("complexity:\n  low_threshold: 5"), 0644)
	if err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Search for config
	candidates := []string{"jscan.yaml", "jscan.yml"}
	result := searchConfigInDirectory(tempDir, candidates)

	if result != configPath {
		t.Errorf("Expected %s, got %s", configPath, result)
	}

	// Search in empty directory
	emptyDir, _ := os.MkdirTemp("", "empty_test")
	defer os.RemoveAll(emptyDir)

	result = searchConfigInDirectory(emptyDir, candidates)
	if result != "" {
		t.Error("Expected empty string for directory without config")
	}
}

func TestDefaultConstants(t *testing.T) {
	// Verify constants have expected values
	if DefaultLowComplexityThreshold != 9 {
		t.Errorf("DefaultLowComplexityThreshold should be 9, got %d", DefaultLowComplexityThreshold)
	}
	if DefaultMediumComplexityThreshold != 19 {
		t.Errorf("DefaultMediumComplexityThreshold should be 19, got %d", DefaultMediumComplexityThreshold)
	}
	if DefaultMinComplexityFilter != 1 {
		t.Errorf("DefaultMinComplexityFilter should be 1, got %d", DefaultMinComplexityFilter)
	}
	if DefaultMaxComplexityLimit != 0 {
		t.Errorf("DefaultMaxComplexityLimit should be 0, got %d", DefaultMaxComplexityLimit)
	}
	if DefaultDeadCodeMinSeverity != "info" {
		t.Errorf("DefaultDeadCodeMinSeverity should be 'info', got '%s'", DefaultDeadCodeMinSeverity)
	}
	if DefaultDeadCodeContextLines != 3 {
		t.Errorf("DefaultDeadCodeContextLines should be 3, got %d", DefaultDeadCodeContextLines)
	}
	if DefaultDeadCodeSortBy != "severity" {
		t.Errorf("DefaultDeadCodeSortBy should be 'severity', got '%s'", DefaultDeadCodeSortBy)
	}
}

func TestConfig_ValidOutputFormats(t *testing.T) {
	config := DefaultConfig()
	validFormats := []string{"text", "json", "yaml", "csv", "html"}

	for _, format := range validFormats {
		config.Output.Format = format
		err := config.Validate()
		if err != nil {
			t.Errorf("Format '%s' should be valid, got error: %v", format, err)
		}
	}
}

func TestConfig_ValidSortOptions(t *testing.T) {
	config := DefaultConfig()
	validSortOptions := []string{"name", "complexity", "risk"}

	for _, sortBy := range validSortOptions {
		config.Output.SortBy = sortBy
		err := config.Validate()
		if err != nil {
			t.Errorf("SortBy '%s' should be valid, got error: %v", sortBy, err)
		}
	}
}

func TestConfig_ValidDeadCodeSeverities(t *testing.T) {
	config := DefaultConfig()
	validSeverities := []string{"critical", "warning", "info"}

	for _, severity := range validSeverities {
		config.DeadCode.MinSeverity = severity
		err := config.Validate()
		if err != nil {
			t.Errorf("Severity '%s' should be valid, got error: %v", severity, err)
		}
	}
}

func TestConfig_ValidDeadCodeSortBy(t *testing.T) {
	config := DefaultConfig()
	validSortOptions := []string{"severity", "line", "file", "function"}

	for _, sortBy := range validSortOptions {
		config.DeadCode.SortBy = sortBy
		err := config.Validate()
		if err != nil {
			t.Errorf("DeadCode SortBy '%s' should be valid, got error: %v", sortBy, err)
		}
	}
}

func TestAnalysisConfig_Defaults(t *testing.T) {
	config := DefaultConfig()

	// Check include patterns
	hasJsPattern := false
	for _, pattern := range config.Analysis.IncludePatterns {
		if pattern == "**/*.js" {
			hasJsPattern = true
			break
		}
	}
	if !hasJsPattern {
		t.Error("Include patterns should contain **/*.js")
	}

	// Check exclude patterns
	hasNodeModules := false
	for _, pattern := range config.Analysis.ExcludePatterns {
		if pattern == "node_modules" {
			hasNodeModules = true
			break
		}
	}
	if !hasNodeModules {
		t.Error("Exclude patterns should contain node_modules")
	}
}
