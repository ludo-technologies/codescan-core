package service

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/internal/analyzer"
	"github.com/ludo-technologies/polyscan/jscan/internal/parser"
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
