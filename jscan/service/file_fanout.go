package service

import (
	"context"
	"runtime"
	"sync"

	"github.com/ludo-technologies/polyscan/jscan/domain"
)

// fileAnalysis is one file's contribution to a multi-file analysis.
type fileAnalysis[T any] struct {
	value    T
	warnings []string
	errors   []string
}

// analyzeFilesConcurrently applies analyze to every path across worker
// goroutines and returns the results in path order.
//
// Reading and parsing a file is independent per file and is where multi-file
// analyses spend most of their time, so this is where concurrency pays. Results
// are written to a preallocated slot per path rather than appended, so the
// output never depends on how work was scheduled.
//
// analyze must not share mutable state between calls — in particular each call
// needs its own tree-sitter parser, which is not safe for concurrent use.
// A cancelled context stops the remaining work; already-finished results are
// still returned, and callers check ctx themselves to decide how to report it.
func analyzeFilesConcurrently[T any](
	ctx context.Context,
	paths []string,
	progress domain.TaskProgress,
	analyze func(context.Context, string) fileAnalysis[T],
) []fileAnalysis[T] {
	results := make([]fileAnalysis[T], len(paths))
	if len(paths) == 0 {
		return results
	}

	workers := runtime.NumCPU()
	if workers > len(paths) {
		workers = len(paths)
	}

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			for index := offset; index < len(paths); index += workers {
				select {
				case <-ctx.Done():
					return
				default:
				}

				results[index] = analyze(ctx, paths[index])
				if progress != nil {
					progress.Increment(1)
				}
			}
		}(worker)
	}
	wg.Wait()

	return results
}
