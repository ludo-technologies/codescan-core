// Package js runs the jscan JavaScript/TypeScript analyses as one pipeline,
// shared by the jscan CLI and polyscan analyze. The packages below it are the
// jscan implementation; this file is the entry point both commands use to
// load configuration, collect files and run the analyses.
package js

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/app"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/config"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/service"
)

// ConfigDocsURL documents which configuration keys change behavior.
const ConfigDocsURL = "https://jscan.codescan.dev/configuration/#which-keys-take-effect-today"

// Selection selects the analyses to run.
type Selection struct {
	Complexity bool
	DeadCode   bool
	Clones     bool
	CBO        bool
	Deps       bool
}

// AllAnalyses selects every analysis, the jscan default.
func AllAnalyses() Selection {
	return Selection{Complexity: true, DeadCode: true, Clones: true, CBO: true, Deps: true}
}

func (s Selection) count() int {
	count := 0
	for _, selected := range []bool{s.Complexity, s.DeadCode, s.Clones, s.CBO, s.Deps} {
		if selected {
			count++
		}
	}
	return count
}

// Result holds each analysis response next to its error. An analysis that
// was not selected leaves both nil; one that failed leaves the response nil
// and the error set, and the others still stand.
type Result struct {
	Complexity    *domain.ComplexityResponse
	ComplexityErr error
	DeadCode      *domain.DeadCodeResponse
	DeadCodeErr   error
	Clones        *domain.CloneResponse
	ClonesErr     error
	CBO           *domain.CBOResponse
	CBOErr        error
	Deps          *domain.DependencyGraphResponse
	DepsErr       error
}

// LoadConfig loads the configuration a command should run with and reports
// the keys the file sets that reach no behavior.
//
// The report goes to warn rather than stdout, which commands reserve for the
// results themselves, and it is written for every format: a key that quietly
// does nothing is exactly what the user needs to hear about.
func LoadConfig(configPath, targetPath string, warn io.Writer) (*config.Config, error) {
	result, err := config.Load(configPath, targetPath)
	if err != nil {
		return nil, err
	}

	if len(result.IgnoredKeys) > 0 {
		noun := "keys"
		if len(result.IgnoredKeys) == 1 {
			noun = "key"
		}
		fmt.Fprintf(warn, "Warning: %s sets %d %s that no command reads: %s\n",
			result.Path, len(result.IgnoredKeys), noun, strings.Join(result.IgnoredKeys, ", "))
		fmt.Fprintf(warn, "  See %s\n", ConfigDocsURL)
	}

	return result.Config, nil
}

// ContainsFiles reports whether any JavaScript/TypeScript file exists under
// the paths. Configuration plays no part: polyscan analyze asks before the
// JavaScript configuration is even discovered, so a tree with no JavaScript
// never loads — or fails on — one.
func ContainsFiles(paths []string) (bool, error) {
	files, err := app.NewFileHelper().CollectJSFiles(paths, true, nil, nil)
	if err != nil {
		return false, err
	}
	return len(files) > 0, nil
}

// CollectFiles collects the JavaScript/TypeScript files under each path,
// honoring the configuration's include and exclude patterns.
func CollectFiles(paths []string, cfg *config.Config) ([]string, error) {
	helper := app.NewFileHelper()
	var files []string
	for _, path := range paths {
		pathFiles, err := helper.CollectJSFiles([]string{path}, cfg.Analysis.Recursive, cfg.Analysis.IncludePatterns, cfg.Analysis.ExcludePatterns)
		if err != nil {
			return nil, fmt.Errorf("failed to collect files from %s: %w", path, err)
		}
		files = append(files, pathFiles...)
	}
	return files, nil
}

// Run executes the selected analyses over files in parallel and collects
// every response and error into one Result.
//
// With several analyses selected, every file is parsed once and the parse
// trees are shared across them. Nothing references the snapshot after the
// goroutines below finish, so the shared trees become collectable as soon as
// the analyses are done with them — clone detection drops its per-fragment
// AST references itself once fragments are converted for APTED. A single
// selected analysis has nobody to share with and skips the snapshot: the
// services' own entry points then release each file as they go, which holds
// far fewer parse trees at once.
func Run(ctx context.Context, files []string, cfg *config.Config, selected Selection) *Result {
	var snapshot *service.ProjectSnapshot
	if selected.count() > 1 {
		snapshot = service.BuildProjectSnapshot(ctx, files, nil)
	}

	result := &Result{}
	var wg sync.WaitGroup
	var mu sync.Mutex

	if selected.Complexity {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := runComplexity(ctx, snapshot, files, cfg)
			mu.Lock()
			result.Complexity, result.ComplexityErr = resp, err
			mu.Unlock()
		}()
	}

	if selected.DeadCode {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := runDeadCode(ctx, snapshot, files, cfg)
			mu.Lock()
			result.DeadCode, result.DeadCodeErr = resp, err
			mu.Unlock()
		}()
	}

	if selected.Clones {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := runClones(ctx, snapshot, files)
			mu.Lock()
			result.Clones, result.ClonesErr = resp, err
			mu.Unlock()
		}()
	}

	if selected.CBO {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := runCBO(ctx, snapshot, files)
			mu.Lock()
			result.CBO, result.CBOErr = resp, err
			mu.Unlock()
		}()
	}

	if selected.Deps {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp, err := runDeps(ctx, snapshot, files)
			mu.Lock()
			result.Deps, result.DepsErr = resp, err
			mu.Unlock()
		}()
	}

	wg.Wait()
	return result
}

// runComplexity runs complexity analysis without progress tracking, over the
// shared snapshot when one exists.
func runComplexity(ctx context.Context, snapshot *service.ProjectSnapshot, files []string, cfg *config.Config) (*domain.ComplexityResponse, error) {
	svc := service.NewComplexityService(&cfg.Complexity)

	req := domain.ComplexityRequest{
		Paths:           files,
		LowThreshold:    cfg.Complexity.LowThreshold,
		MediumThreshold: cfg.Complexity.MediumThreshold,
		MinComplexity:   cfg.Output.MinComplexity,
		SortBy:          domain.SortCriteria(cfg.Output.SortBy),
	}

	if snapshot != nil {
		return svc.AnalyzeSnapshot(ctx, snapshot, req)
	}
	return svc.Analyze(ctx, req)
}

// runDeadCode runs dead code analysis without progress tracking, over the
// shared snapshot when one exists.
func runDeadCode(ctx context.Context, snapshot *service.ProjectSnapshot, files []string, cfg *config.Config) (*domain.DeadCodeResponse, error) {
	if snapshot != nil {
		return service.AnalyzeDeadCodeSnapshot(ctx, snapshot, DeadCodeRequest(files, cfg))
	}
	return service.AnalyzeDeadCode(ctx, DeadCodeRequest(files, cfg))
}

// DeadCodeRequest builds the dead code request every entry point shares.
func DeadCodeRequest(files []string, cfg *config.Config) domain.DeadCodeRequest {
	return domain.DeadCodeRequest{
		Paths:       files,
		MinSeverity: domain.DeadCodeSeverity(cfg.DeadCode.MinSeverity),
		SortBy:      domain.DeadCodeSortCriteria(cfg.DeadCode.SortBy),
	}
}

// runClones runs clone detection without progress tracking, over the shared
// snapshot when one exists.
func runClones(ctx context.Context, snapshot *service.ProjectSnapshot, files []string) (*domain.CloneResponse, error) {
	svc := service.NewCloneServiceWithDefaults()

	req := domain.DefaultCloneRequest()
	req.Paths = files

	if snapshot != nil {
		return svc.DetectClonesInSnapshot(ctx, snapshot, req)
	}
	return svc.DetectClones(ctx, req)
}

// runCBO runs CBO analysis without progress tracking, over the shared
// snapshot when one exists.
func runCBO(ctx context.Context, snapshot *service.ProjectSnapshot, files []string) (*domain.CBOResponse, error) {
	svc := service.NewCBOServiceWithDefaults()

	req := domain.CBORequest{
		Paths: files,
	}

	if snapshot != nil {
		return svc.AnalyzeSnapshot(ctx, snapshot, req)
	}
	return svc.Analyze(ctx, req)
}

// runDeps runs dependency analysis without progress tracking, over the
// shared snapshot when one exists.
func runDeps(ctx context.Context, snapshot *service.ProjectSnapshot, files []string) (*domain.DependencyGraphResponse, error) {
	svc := service.NewDependencyGraphServiceWithDefaults()

	req := domain.DependencyGraphRequest{
		Paths:        files,
		DetectCycles: domain.BoolPtr(true),
	}

	if snapshot != nil {
		return svc.AnalyzeSnapshot(ctx, snapshot, req)
	}
	return svc.Analyze(ctx, req)
}
