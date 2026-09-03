package service

import (
	"context"
	"runtime"
	"sync"
)

// fileAnalysis is one file's contribution to a multi-file analysis.
type fileAnalysis[T any] struct {
	value    T
	warnings []string
	errors   []string
}

// analyzeFilesConcurrently applies analyze to every file across worker
// goroutines and returns the results in input order. Files are either paths to
// read and parse (snapshot construction) or already parsed ProjectFiles
// (per-analysis work on a shared snapshot).
//
// Per-file work — parsing during snapshot construction, CFG building during
// analysis — is independent per file and is where multi-file analyses spend
// most of their time, so this is where concurrency pays. Results are written to
// a preallocated slot per file rather than appended, so the output never
// depends on how work was scheduled.
//
// analyze must not share mutable state between calls — in particular each call
// needs its own tree-sitter parser, which is not safe for concurrent use.
// A cancelled context stops the remaining work; already-finished results are
// still returned, and callers check ctx themselves to decide how to report it.
func analyzeFilesConcurrently[F, T any](
	ctx context.Context,
	files []F,
	analyze func(context.Context, F) fileAnalysis[T],
) []fileAnalysis[T] {
	results := make([]fileAnalysis[T], len(files))
	if len(files) == 0 {
		return results
	}

	workers := runtime.NumCPU()
	if workers > len(files) {
		workers = len(files)
	}

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for index := offset; index < len(files); index += workers {
				select {
				case <-ctx.Done():
					return
				default:
				}

				results[index] = analyze(ctx, files[index])
			}
		}(worker)
	}
	wg.Wait()

	return results
}
