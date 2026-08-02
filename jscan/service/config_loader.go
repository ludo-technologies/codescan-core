package service

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/internal/config"
)

// ConfigurationLoaderImpl implements the ConfigurationLoader interface
type ConfigurationLoaderImpl struct{}

// NewConfigurationLoader creates a new configuration loader service
func NewConfigurationLoader() *ConfigurationLoaderImpl {
	return &ConfigurationLoaderImpl{}
}

// LoadConfig loads configuration from the specified path
func (c *ConfigurationLoaderImpl) LoadConfig(path string) (*domain.ComplexityRequest, error) {
	cfg, err := config.LoadConfig(path)
	if err != nil {
		return nil, domain.NewConfigError("failed to load configuration file", err)
	}

	return c.convertToComplexityRequest(cfg), nil
}

// LoadDefaultConfig loads the default configuration, first checking for jscan.config.json
func (c *ConfigurationLoaderImpl) LoadDefaultConfig() *domain.ComplexityRequest {
	cfg, err := config.LoadConfigWithTarget("", "")
	if err == nil {
		return c.convertToComplexityRequest(cfg)
	}

	// Fall back to hardcoded default configuration
	cfg = config.DefaultConfig()
	return c.convertToComplexityRequest(cfg)
}

// FindDefaultConfigFile searches for a default configuration file
func (c *ConfigurationLoaderImpl) FindDefaultConfigFile() string {
	// List of possible config file names in order of preference
	configFiles := []string{
		"jscan.config.json",
		".jscanrc.json",
		"jscan.yaml",
		"jscan.yml",
		".jscan.toml",
		".jscan.yml",
		"jscan.json",
		".jscan.json",
	}

	// Check current directory
	for _, file := range configFiles {
		if _, err := os.Stat(file); err == nil {
			return file
		}
	}

	// Check parent directories up to root
	currentDir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		for _, file := range configFiles {
			configPath := filepath.Join(currentDir, file)
			if _, err := os.Stat(configPath); err == nil {
				return configPath
			}
		}

		// Move to parent directory
		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			// Reached root directory
			break
		}
		currentDir = parentDir
	}

	return ""
}

// MergeConfig merges CLI flags with configuration file
func (c *ConfigurationLoaderImpl) MergeConfig(base *domain.ComplexityRequest, override *domain.ComplexityRequest) *domain.ComplexityRequest {
	// Start with base configuration
	merged := *base

	// Override with non-zero values from override
	// Always override paths as they come from command arguments
	if len(override.Paths) > 0 {
		merged.Paths = override.Paths
	}

	// Output configuration
	if override.OutputFormat != "" {
		merged.OutputFormat = override.OutputFormat
	}

	if override.OutputWriter != nil {
		merged.OutputWriter = override.OutputWriter
	}

	// Only override if values differ from defaults
	if override.ShowDetails {
		merged.ShowDetails = override.ShowDetails
	}

	// Filtering and sorting - override only when the caller asked for a stricter
	// filter than the default; an unset (zero) override must not clobber the
	// min_complexity that came from the configuration file.
	if override.MinComplexity > config.DefaultMinComplexityFilter {
		merged.MinComplexity = override.MinComplexity
	}

	if override.MaxComplexity != 0 {
		merged.MaxComplexity = override.MaxComplexity
	}

	if override.SortBy != "" && override.SortBy != domain.SortByComplexity {
		merged.SortBy = override.SortBy
	}

	// Complexity thresholds - override if non-default
	if override.LowThreshold != 9 && override.LowThreshold > 0 {
		merged.LowThreshold = override.LowThreshold
	}

	if override.MediumThreshold != 19 && override.MediumThreshold > 0 {
		merged.MediumThreshold = override.MediumThreshold
	}

	// Config path is always from override if provided
	if override.ConfigPath != "" {
		merged.ConfigPath = override.ConfigPath
	}

	return &merged
}

// convertToComplexityRequest converts a Config to ComplexityRequest
func (c *ConfigurationLoaderImpl) convertToComplexityRequest(cfg *config.Config) *domain.ComplexityRequest {
	return &domain.ComplexityRequest{
		// Paths are set by the caller, not from config
		Paths: []string{},

		// Output settings
		OutputFormat: domain.OutputFormat(cfg.Output.Format),
		ShowDetails:  cfg.Output.ShowDetails,
		SortBy:       domain.SortCriteria(cfg.Output.SortBy),

		// Complexity settings
		LowThreshold:    cfg.Complexity.LowThreshold,
		MediumThreshold: cfg.Complexity.MediumThreshold,
		MinComplexity:   cfg.Output.MinComplexity,
		MaxComplexity:   cfg.Complexity.MaxComplexity,

		// Other settings
		Recursive: cfg.Analysis.Recursive,
	}
}

// ValidateConfig validates the configuration
func (c *ConfigurationLoaderImpl) ValidateConfig(req *domain.ComplexityRequest) error {
	// Validate thresholds
	if req.LowThreshold <= 0 {
		return fmt.Errorf("low_threshold must be greater than 0, got %d", req.LowThreshold)
	}

	if req.MediumThreshold <= req.LowThreshold {
		return fmt.Errorf("medium_threshold (%d) must be greater than low_threshold (%d)",
			req.MediumThreshold, req.LowThreshold)
	}

	if req.MinComplexity < 0 {
		return fmt.Errorf("min_complexity cannot be negative, got %d", req.MinComplexity)
	}

	if req.MaxComplexity < 0 {
		return fmt.Errorf("max_complexity cannot be negative, got %d", req.MaxComplexity)
	}

	if req.MaxComplexity > 0 && req.MinComplexity > req.MaxComplexity {
		return fmt.Errorf("min_complexity (%d) cannot be greater than max_complexity (%d)",
			req.MinComplexity, req.MaxComplexity)
	}

	// Validate output format
	validFormats := map[domain.OutputFormat]bool{
		domain.OutputFormatText: true,
		domain.OutputFormatJSON: true,
		domain.OutputFormatHTML: true,
		domain.OutputFormatCSV:  true,
	}

	if !validFormats[req.OutputFormat] {
		return fmt.Errorf("invalid output format: %s (must be one of: text, json, html, csv)",
			req.OutputFormat)
	}

	return nil
}
