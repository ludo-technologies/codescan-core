package app

import (
	"context"
	"fmt"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	servicepkg "github.com/ludo-technologies/polyscan/polyscan/internal/js/service"
)

// DeadCodeUseCase orchestrates the dead code analysis workflow.
type DeadCodeUseCase struct {
	service    domain.DeadCodeService
	fileHelper *FileHelper
}

// NewDeadCodeUseCase creates a new dead code use case.
func NewDeadCodeUseCase() *DeadCodeUseCase {
	return &DeadCodeUseCase{
		service:    servicepkg.NewDeadCodeService(),
		fileHelper: NewFileHelper(),
	}
}

// Execute performs the complete dead code analysis workflow.
func (uc *DeadCodeUseCase) Execute(ctx context.Context, req domain.DeadCodeRequest) (*domain.DeadCodeResponse, error) {
	applyDeadCodeDefaults(&req)

	if err := req.Validate(); err != nil {
		return nil, domain.NewInvalidInputError("invalid request", err)
	}

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

	req.Paths = files

	resp, err := uc.service.Analyze(ctx, req)
	if err != nil {
		return nil, domain.NewAnalysisError("dead code analysis failed", err)
	}

	return resp, nil
}

// AnalyzeFile analyzes a single file for dead code.
func (uc *DeadCodeUseCase) AnalyzeFile(ctx context.Context, filePath string, req domain.DeadCodeRequest) (*domain.FileDeadCode, error) {
	applyDeadCodeDefaults(&req)

	if !uc.fileHelper.IsValidJSFile(filePath) {
		return nil, domain.NewInvalidInputError(fmt.Sprintf("not a valid JavaScript/TypeScript file: %s", filePath), nil)
	}

	exists, err := uc.fileHelper.FileExists(filePath)
	if err != nil {
		return nil, domain.NewFileNotFoundError(filePath, err)
	}
	if !exists {
		return nil, domain.NewFileNotFoundError(filePath, fmt.Errorf("file does not exist"))
	}

	resp, err := uc.service.AnalyzeFile(ctx, filePath, req)
	if err != nil {
		return nil, domain.NewAnalysisError("dead code analysis failed", err)
	}

	return resp, nil
}

func applyDeadCodeDefaults(req *domain.DeadCodeRequest) {
	if req.OutputFormat == "" {
		req.OutputFormat = domain.OutputFormatText
	}
	if req.MinSeverity == "" {
		req.MinSeverity = domain.DeadCodeSeverityInfo
	}
	if req.SortBy == "" {
		req.SortBy = domain.DeadCodeSortBySeverity
	}
}

// DeadCodeUseCaseBuilder provides a builder pattern for creating DeadCodeUseCase.
type DeadCodeUseCaseBuilder struct {
	service    domain.DeadCodeService
	fileHelper *FileHelper
}

// NewDeadCodeUseCaseBuilder creates a new builder.
func NewDeadCodeUseCaseBuilder() *DeadCodeUseCaseBuilder {
	return &DeadCodeUseCaseBuilder{}
}

// WithService sets the dead code service.
func (b *DeadCodeUseCaseBuilder) WithService(service domain.DeadCodeService) *DeadCodeUseCaseBuilder {
	b.service = service
	return b
}

// WithFileHelper sets the file helper.
func (b *DeadCodeUseCaseBuilder) WithFileHelper(fileHelper *FileHelper) *DeadCodeUseCaseBuilder {
	b.fileHelper = fileHelper
	return b
}

// Build creates the DeadCodeUseCase with the configured dependencies.
func (b *DeadCodeUseCaseBuilder) Build() (*DeadCodeUseCase, error) {
	uc := &DeadCodeUseCase{
		service:    b.service,
		fileHelper: b.fileHelper,
	}

	if uc.service == nil {
		uc.service = servicepkg.NewDeadCodeService()
	}
	if uc.fileHelper == nil {
		uc.fileHelper = NewFileHelper()
	}

	return uc, nil
}
