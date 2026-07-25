package app

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/internal/version"
)

// AnalyzeConfig holds configuration for the analyze use case
type AnalyzeConfig struct {
	EnableComplexity bool
	EnableDeadCode   bool

	// Complexity options
	MinComplexity   int
	MaxComplexity   int
	LowThreshold    int
	MediumThreshold int

	// Dead code options
	MinSeverity domain.DeadCodeSeverity

	// Output options
	OutputFormat domain.OutputFormat
	OutputWriter io.Writer
	OutputPath   string
	NoOpen       bool

	// File options
	Recursive       bool
	IncludePatterns []string
	ExcludePatterns []string
}

// DefaultAnalyzeConfig returns default configuration
func DefaultAnalyzeConfig() AnalyzeConfig {
	return AnalyzeConfig{
		EnableComplexity: true,
		EnableDeadCode:   true,
		MinComplexity:    0,
		MaxComplexity:    0,
		LowThreshold:     9,
		MediumThreshold:  19,
		MinSeverity:      domain.DeadCodeSeverityWarning,
		OutputFormat:     domain.OutputFormatText,
		Recursive:        true,
	}
}

// AnalyzeUseCase orchestrates comprehensive analysis
type AnalyzeUseCase struct {
	complexityUseCase *ComplexityUseCase
	deadCodeUseCase   *DeadCodeUseCase
	fileHelper        *FileHelper
}

// NewAnalyzeUseCase creates a new analyze use case
func NewAnalyzeUseCase(
	complexityUseCase *ComplexityUseCase,
	deadCodeUseCase *DeadCodeUseCase,
) *AnalyzeUseCase {
	return &AnalyzeUseCase{
		complexityUseCase: complexityUseCase,
		deadCodeUseCase:   deadCodeUseCase,
		fileHelper:        NewFileHelper(),
	}
}

// AnalyzeResult holds the results of comprehensive analysis
type AnalyzeResult struct {
	Complexity *domain.ComplexityResponse
	DeadCode   *domain.DeadCodeResponse
	Summary    *domain.AnalyzeSummary
	Duration   time.Duration
}

// Execute performs comprehensive analysis
func (uc *AnalyzeUseCase) Execute(ctx context.Context, config AnalyzeConfig, paths []string) (*AnalyzeResult, error) {
	startTime := time.Now()

	// Collect files
	files, err := ResolveFilePaths(
		uc.fileHelper,
		paths,
		config.Recursive,
		config.IncludePatterns,
		config.ExcludePatterns,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to collect JavaScript/TypeScript files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no JavaScript/TypeScript files found in the specified paths")
	}

	result := &AnalyzeResult{
		Summary: &domain.AnalyzeSummary{
			TotalFiles:    len(files),
			AnalyzedFiles: len(files),
		},
	}

	// Run complexity analysis
	if config.EnableComplexity && uc.complexityUseCase != nil {
		complexityReq := domain.ComplexityRequest{
			Paths:           files,
			MinComplexity:   config.MinComplexity,
			MaxComplexity:   config.MaxComplexity,
			LowThreshold:    config.LowThreshold,
			MediumThreshold: config.MediumThreshold,
			SortBy:          domain.SortByComplexity,
			Recursive:       config.Recursive,
		}

		response, err := uc.complexityUseCase.Execute(ctx, complexityReq)
		if err == nil {
			result.Complexity = response
			result.Summary.ComplexityEnabled = true
			result.Summary.TotalFunctions = response.Summary.TotalFunctions
			result.Summary.FunctionsParsed = response.Summary.FunctionsParsed
			result.Summary.AverageComplexity = response.Summary.AverageComplexity
			result.Summary.HighComplexityCount = response.Summary.HighRiskFunctions
			result.Summary.MediumComplexityCount = response.Summary.MediumRiskFunctions
		}
	}

	// Run dead code analysis
	if config.EnableDeadCode && uc.deadCodeUseCase != nil {
		deadCodeReq := domain.DeadCodeRequest{
			Paths:           files,
			MinSeverity:     config.MinSeverity,
			Recursive:       config.Recursive,
			IncludePatterns: config.IncludePatterns,
			ExcludePatterns: config.ExcludePatterns,
		}

		response, err := uc.deadCodeUseCase.Execute(ctx, deadCodeReq)
		if err == nil {
			result.DeadCode = response
			result.Summary.DeadCodeEnabled = true
			result.Summary.DeadCodeCount = response.Summary.TotalFindings
			result.Summary.CriticalDeadCode = response.Summary.CriticalFindings
			result.Summary.WarningDeadCode = response.Summary.WarningFindings
			result.Summary.InfoDeadCode = response.Summary.InfoFindings
		}
	}

	// Calculate health score
	_ = result.Summary.CalculateHealthScore()

	result.Duration = time.Since(startTime)

	return result, nil
}

// ToAnalyzeResponse converts AnalyzeResult to domain.AnalyzeResponse
func (r *AnalyzeResult) ToAnalyzeResponse() *domain.AnalyzeResponse {
	return &domain.AnalyzeResponse{
		Complexity:  r.Complexity,
		DeadCode:    r.DeadCode,
		Summary:     *r.Summary,
		GeneratedAt: time.Now(),
		Duration:    r.Duration.Milliseconds(),
		Version:     version.Version,
	}
}

// AnalyzeUseCaseBuilder builds an AnalyzeUseCase
type AnalyzeUseCaseBuilder struct {
	complexityUseCase *ComplexityUseCase
	deadCodeUseCase   *DeadCodeUseCase
	fileHelper        *FileHelper
}

// NewAnalyzeUseCaseBuilder creates a new builder
func NewAnalyzeUseCaseBuilder() *AnalyzeUseCaseBuilder {
	return &AnalyzeUseCaseBuilder{}
}

// WithComplexityUseCase sets the complexity use case
func (b *AnalyzeUseCaseBuilder) WithComplexityUseCase(uc *ComplexityUseCase) *AnalyzeUseCaseBuilder {
	b.complexityUseCase = uc
	return b
}

// WithDeadCodeUseCase sets the dead code use case
func (b *AnalyzeUseCaseBuilder) WithDeadCodeUseCase(uc *DeadCodeUseCase) *AnalyzeUseCaseBuilder {
	b.deadCodeUseCase = uc
	return b
}

// WithFileHelper sets the file helper
func (b *AnalyzeUseCaseBuilder) WithFileHelper(fh *FileHelper) *AnalyzeUseCaseBuilder {
	b.fileHelper = fh
	return b
}

// Build creates the AnalyzeUseCase
func (b *AnalyzeUseCaseBuilder) Build() (*AnalyzeUseCase, error) {
	uc := &AnalyzeUseCase{
		complexityUseCase: b.complexityUseCase,
		deadCodeUseCase:   b.deadCodeUseCase,
		fileHelper:        b.fileHelper,
	}

	if uc.fileHelper == nil {
		uc.fileHelper = NewFileHelper()
	}

	return uc, nil
}
