package app

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

// ComplexityUseCase orchestrates the complexity analysis workflow
type ComplexityUseCase struct {
	service    domain.ComplexityService
	fileHelper *FileHelper
}

// NewComplexityUseCase creates a new complexity use case
func NewComplexityUseCase(service domain.ComplexityService) *ComplexityUseCase {
	return &ComplexityUseCase{
		service:    service,
		fileHelper: NewFileHelper(),
	}
}

// Execute performs the complete complexity analysis workflow
func (uc *ComplexityUseCase) Execute(ctx context.Context, req domain.ComplexityRequest) (*domain.ComplexityResponse, error) {
	// Validate input
	if err := uc.validateRequest(req); err != nil {
		return nil, domain.NewInvalidInputError("invalid request", err)
	}

	// Resolve file paths
	files, err := ResolveFilePaths(
		uc.fileHelper,
		req.Paths,
		req.Recursive,
		req.IncludePatterns,
		req.ExcludePatterns,
	)
	if err != nil {
		return nil, domain.NewFileNotFoundError("failed to collect files", err)
	}

	if len(files) == 0 {
		return nil, domain.NewInvalidInputError("no JavaScript/TypeScript files found in the specified paths", nil)
	}

	// Update request with collected files
	req.Paths = files

	// Perform analysis
	response, err := uc.service.Analyze(ctx, req)
	if err != nil {
		return nil, domain.NewAnalysisError("complexity analysis failed", err)
	}

	return response, nil
}

// AnalyzeFile analyzes a single file
func (uc *ComplexityUseCase) AnalyzeFile(ctx context.Context, filePath string, req domain.ComplexityRequest) (*domain.ComplexityResponse, error) {
	// Validate file
	if !uc.fileHelper.IsValidJSFile(filePath) {
		return nil, domain.NewInvalidInputError(fmt.Sprintf("not a valid JavaScript/TypeScript file: %s", filePath), nil)
	}

	// Check if file exists
	exists, err := uc.fileHelper.FileExists(filePath)
	if err != nil {
		return nil, domain.NewFileNotFoundError(filePath, err)
	}
	if !exists {
		return nil, domain.NewFileNotFoundError(filePath, fmt.Errorf("file does not exist"))
	}

	// Update request with single file path
	req.Paths = []string{filePath}

	// Perform analysis
	return uc.service.Analyze(ctx, req)
}

// validateRequest validates the complexity request
func (uc *ComplexityUseCase) validateRequest(req domain.ComplexityRequest) error {
	if len(req.Paths) == 0 {
		return fmt.Errorf("no input paths specified")
	}

	if req.MinComplexity < 0 {
		return fmt.Errorf("minimum complexity cannot be negative")
	}

	if req.MaxComplexity < 0 {
		return fmt.Errorf("maximum complexity cannot be negative")
	}

	if req.MaxComplexity > 0 && req.MinComplexity > req.MaxComplexity {
		return fmt.Errorf("minimum complexity cannot be greater than maximum complexity")
	}

	if req.LowThreshold <= 0 {
		req.LowThreshold = 9
	}

	if req.MediumThreshold <= 0 {
		req.MediumThreshold = 19
	}

	if req.MediumThreshold <= req.LowThreshold {
		return fmt.Errorf("medium threshold must be greater than low threshold")
	}

	return nil
}

// ComplexityUseCaseBuilder provides a builder pattern for creating ComplexityUseCase
type ComplexityUseCaseBuilder struct {
	service    domain.ComplexityService
	fileHelper *FileHelper
}

// NewComplexityUseCaseBuilder creates a new builder
func NewComplexityUseCaseBuilder() *ComplexityUseCaseBuilder {
	return &ComplexityUseCaseBuilder{}
}

// WithService sets the complexity service
func (b *ComplexityUseCaseBuilder) WithService(service domain.ComplexityService) *ComplexityUseCaseBuilder {
	b.service = service
	return b
}

// WithFileHelper sets the file helper
func (b *ComplexityUseCaseBuilder) WithFileHelper(fileHelper *FileHelper) *ComplexityUseCaseBuilder {
	b.fileHelper = fileHelper
	return b
}

// Build creates the ComplexityUseCase with the configured dependencies
func (b *ComplexityUseCaseBuilder) Build() (*ComplexityUseCase, error) {
	if b.service == nil {
		return nil, fmt.Errorf("complexity service is required")
	}

	uc := &ComplexityUseCase{
		service:    b.service,
		fileHelper: b.fileHelper,
	}

	if uc.fileHelper == nil {
		uc.fileHelper = NewFileHelper()
	}

	return uc, nil
}

// UseCaseOptions provides configuration options for the use case
type UseCaseOptions struct {
	EnableProgress   bool
	ProgressInterval time.Duration
	MaxConcurrency   int
	TimeoutPerFile   time.Duration
}

// DefaultUseCaseOptions returns default options
func DefaultUseCaseOptions() UseCaseOptions {
	return UseCaseOptions{
		EnableProgress:   true,
		ProgressInterval: 100 * time.Millisecond,
		MaxConcurrency:   4,
		TimeoutPerFile:   30 * time.Second,
	}
}

// WriteOutput is a helper interface for writing output
type WriteOutput interface {
	Write(writer io.Writer, response *domain.ComplexityResponse, format domain.OutputFormat) error
}
