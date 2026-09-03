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

// ProjectSnapshot is the file set of one run. It is the parse trees' owner:
// analyses must not mutate the shared AST, and clone detection keeps dropping
// its per-fragment AST references after APTED conversion, so the trees become
// collectable as soon as the last snapshot reference is gone.
//
// A snapshot built with BuildProjectSnapshot reads and parses every file up
// front and keeps the trees, so several analyses can share one set of parse
// trees. One created with NewProjectSnapshot is for a single analysis: each
// file is loaded inside that analysis's fan-out and released as soon as the
// analysis has extracted its results, so peak memory holds only as many parse
// trees as there are workers.
//
// Either way the snapshot records, per file, whether it could be read and
// parsed. That is the run's file accounting, and it is kept here rather than
// in any one analysis so the health score can charge unparsable files
// whichever analyses ran.
type ProjectSnapshot struct {
	Files []*ProjectFile

	// retain keeps each file's parse tree after an analysis pass so the next
	// analysis can share it. A single-analysis snapshot releases each tree
	// instead.
	retain bool
}

// ProjectFile is one file after read and parse, plus the lazily built CFGs the
// CFG-backed analyses (complexity, dead code) share.
type ProjectFile struct {
	Path     string
	Content  []byte
	AST      *parser.Node
	ReadErr  error
	ParseErr error

	loaded   bool
	released bool

	cfgOnce sync.Once
	cfgs    map[string]*analyzer.CFG
	cfgErr  error
}

// NewProjectSnapshot names the files of a single-analysis run without reading
// them. The analysis loads each file as its fan-out reaches it and releases the
// parse tree right after, so the snapshot never holds the whole project at
// once. The entry points reject a second analysis over the same snapshot: the
// trees it would need are gone.
func NewProjectSnapshot(paths []string) *ProjectSnapshot {
	files := make([]*ProjectFile, len(paths))
	for index, path := range paths {
		files[index] = &ProjectFile{Path: path}
	}
	return &ProjectSnapshot{Files: files}
}

// BuildProjectSnapshot reads and parses each file once, in parallel, and keeps
// the parse trees for every analysis that runs over the snapshot. Files come
// out in path order regardless of scheduling, so downstream reports stay
// deterministic. Read and parse failures are recorded per file rather than
// aborting the build: each analysis reports them in its own format.
func BuildProjectSnapshot(ctx context.Context, paths []string) *ProjectSnapshot {
	snapshot := NewProjectSnapshot(paths)
	snapshot.retain = true
	// Loading is the whole job here, so the pass analyzes nothing.
	analyzeSnapshotFiles(ctx, snapshot, func(*ProjectFile) fileAnalysis[struct{}] {
		return fileAnalysis[struct{}]{}
	})
	return snapshot
}

// analyzeSnapshotFiles applies analyze to every file of the snapshot across
// worker goroutines and returns the results in file order. Each file is loaded
// before analyze sees it, so analyze can read Content, AST, ReadErr and
// ParseErr directly, and a single-analysis snapshot releases the file right
// after. Files a cancelled pass never reached record the cancellation as their
// read error, so the accounting stays complete.
func analyzeSnapshotFiles[T any](
	ctx context.Context,
	snapshot *ProjectSnapshot,
	analyze func(*ProjectFile) fileAnalysis[T],
) []fileAnalysis[T] {
	results := analyzeFilesConcurrently(ctx, snapshot.Files,
		func(_ context.Context, file *ProjectFile) fileAnalysis[T] {
			file.load()
			result := analyze(file)
			if !snapshot.retain {
				file.release()
			}
			return result
		})

	for _, file := range snapshot.Files {
		if !file.loaded {
			file.loaded = true
			file.ReadErr = fmt.Errorf("analysis cancelled: %w", context.Cause(ctx))
		}
	}

	return results
}

// Paths returns the snapshot's file paths in analysis order.
func (s *ProjectSnapshot) Paths() []string {
	paths := make([]string, len(s.Files))
	for index, file := range s.Files {
		paths[index] = file.Path
	}
	return paths
}

// Accounting reports the run's file set the way the health score charges it:
// every file the run covered, and those no analysis could use. It reads what
// the analyses learned about each file, so it must be called once they have
// finished; calling it earlier is a programming error and panics.
func (s *ProjectSnapshot) Accounting() domain.FileAccounting {
	accounting := domain.FileAccounting{Total: len(s.Files)}
	for _, file := range s.Files {
		if !file.loaded {
			panic(fmt.Sprintf("file accounting requested before %s was analyzed", file.Path))
		}
		err := file.ReadErr
		if err == nil {
			err = file.ParseErr
		}
		if err == nil {
			continue
		}
		accounting.Skipped++
		accounting.Errors = append(accounting.Errors, fmt.Sprintf("%s: %v", file.Path, err))
	}
	return accounting
}

// validateRequest guards the AnalyzeSnapshot entry points. The snapshot
// defines the analyzed file set, so a request that names paths must name the
// snapshot's files — anything else means the caller built the snapshot from a
// different selection than it thinks it is analyzing. An empty path list
// defers to the snapshot entirely. A single-analysis snapshot has released its
// parse trees after its one pass, so it cannot be analyzed again.
func (s *ProjectSnapshot) validateRequest(paths []string) error {
	if s == nil {
		return domain.NewInvalidInputError("project snapshot cannot be nil", nil)
	}
	for _, file := range s.Files {
		if file.released {
			return domain.NewInvalidInputError(
				fmt.Sprintf("file %s was released after its single analysis and cannot be analyzed again", file.Path), nil)
		}
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

// load reads and parses the file once. Only the fan-out calls it, one call per
// file per pass, so it needs no lock: a retained file is simply already loaded
// on the next pass.
func (f *ProjectFile) load() {
	if f.loaded {
		return
	}
	f.loaded = true

	content, err := os.ReadFile(f.Path)
	if err != nil {
		f.ReadErr = err
		return
	}
	f.Content = content

	ast, err := parser.ParseForLanguage(f.Path, content)
	if err != nil {
		f.ParseErr = err
		return
	}
	f.AST = ast
}

// release drops the file's contents, parse tree and CFGs once its single
// analysis is done with them, keeping only the path and the errors the
// accounting reads.
func (f *ProjectFile) release() {
	f.released = true
	f.Content = nil
	f.AST = nil
	f.cfgs = nil
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
