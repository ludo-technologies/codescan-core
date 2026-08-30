package service

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/analyzer"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

// ProjectSnapshot holds every analyzed file read and parsed exactly once, so
// the analyses that used to parse the project independently can share one set
// of parse trees. The snapshot is the parse trees' owner: analyses must not
// mutate the shared AST, and clone detection keeps dropping its per-fragment
// AST references after APTED conversion, so the trees become collectable as
// soon as the last snapshot reference is gone.
type ProjectSnapshot struct {
	Files []*ProjectFile
}

// ProjectFile is one file after read and parse, plus the lazily built CFGs the
// CFG-backed analyses (complexity, dead code) share.
type ProjectFile struct {
	Path     string
	Content  []byte
	AST      *parser.Node
	ReadErr  error
	ParseErr error

	cfgOnce sync.Once
	cfgs    map[string]*analyzer.CFG
	cfgErr  error
}

// BuildProjectSnapshot reads and parses each file once, in parallel. Files come
// out in path order regardless of scheduling, so downstream reports stay
// deterministic. Read and parse failures are recorded per file rather than
// aborting the build: each analysis reports them in its own format.
func BuildProjectSnapshot(ctx context.Context, paths []string, progress domain.TaskProgress) *ProjectSnapshot {
	results := analyzeFilesConcurrently(ctx, paths, progress,
		func(_ context.Context, path string) fileAnalysis[*ProjectFile] {
			return fileAnalysis[*ProjectFile]{value: buildProjectFile(path)}
		})

	files := make([]*ProjectFile, len(paths))
	for index, result := range results {
		if result.value != nil {
			files[index] = result.value
			continue
		}
		// The worker never reached this slot: the context was cancelled.
		files[index] = &ProjectFile{
			Path:    paths[index],
			ReadErr: fmt.Errorf("analysis cancelled: %w", context.Cause(ctx)),
		}
	}

	return &ProjectSnapshot{Files: files}
}

// analyzeProjectFilesFromPaths reads, parses, and analyzes each path inside the
// fan-out, so every analysis sees the same *ProjectFile shape as the snapshot
// path while each file's parse tree is released as soon as analyze returns.
// This is the memory-lean pipeline for a single analysis; a run that shares
// files across several analyses builds a ProjectSnapshot instead.
func analyzeProjectFilesFromPaths[T any](
	ctx context.Context,
	paths []string,
	progress domain.TaskProgress,
	analyze func(*ProjectFile) fileAnalysis[T],
) []fileAnalysis[T] {
	return analyzeFilesConcurrently(ctx, paths, progress,
		func(_ context.Context, path string) fileAnalysis[T] {
			return analyze(buildProjectFile(path))
		})
}

// Paths returns the snapshot's file paths in analysis order.
func (s *ProjectSnapshot) Paths() []string {
	paths := make([]string, len(s.Files))
	for index, file := range s.Files {
		paths[index] = file.Path
	}
	return paths
}

// validateRequestPaths guards the AnalyzeSnapshot entry points: the snapshot
// defines the analyzed file set, so a request that names paths must name the
// snapshot's files — anything else means the caller built the snapshot from a
// different selection than it thinks it is analyzing. An empty path list
// defers to the snapshot entirely.
func (s *ProjectSnapshot) validateRequestPaths(paths []string) error {
	if s == nil {
		return domain.NewInvalidInputError("project snapshot cannot be nil", nil)
	}
	if len(paths) == 0 {
		return nil
	}
	if len(paths) != len(s.Files) {
		return domain.NewInvalidInputError(
			fmt.Sprintf("request names %d paths but the project snapshot holds %d files", len(paths), len(s.Files)), nil)
	}

	inSnapshot := make(map[string]bool, len(s.Files))
	for _, file := range s.Files {
		inSnapshot[file.Path] = true
	}
	for _, path := range paths {
		if !inSnapshot[path] {
			return domain.NewInvalidInputError(
				fmt.Sprintf("request path %s is not in the project snapshot", path), nil)
		}
	}
	return nil
}

func buildProjectFile(path string) *ProjectFile {
	file := &ProjectFile{Path: path}

	content, err := os.ReadFile(path)
	if err != nil {
		file.ReadErr = err
		return file
	}
	file.Content = content

	ast, err := parser.ParseForLanguage(path, content)
	if err != nil {
		file.ParseErr = err
		return file
	}
	file.AST = ast

	return file
}

// Parsed reports whether the file has a valid parse tree to analyze.
func (f *ProjectFile) Parsed() bool {
	return f != nil && f.ReadErr == nil && f.ParseErr == nil && f.AST != nil
}

// CFGs builds the file's CFGs once and shares the result across analyses. It is
// safe to call from several analyses concurrently; the CFGs are built by
// whichever caller gets there first and must be treated as read-only.
func (f *ProjectFile) CFGs() (map[string]*analyzer.CFG, error) {
	if f.ReadErr != nil {
		return nil, f.ReadErr
	}
	if f.ParseErr != nil {
		return nil, f.ParseErr
	}
	if f.AST == nil {
		return nil, fmt.Errorf("file %s has no parse tree", f.Path)
	}

	f.cfgOnce.Do(func() {
		f.cfgs, f.cfgErr = analyzer.NewCFGBuilder().BuildAll(f.AST)
	})

	return f.cfgs, f.cfgErr
}
