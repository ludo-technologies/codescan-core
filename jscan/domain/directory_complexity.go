package domain

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// directoryComplexityAccumulator carries the running sums a directory average
// needs; only the finished metrics are handed back to callers.
type directoryComplexityAccumulator struct {
	metrics           DirectoryComplexityMetrics
	totalComplexity   int
	totalNestingDepth int
}

// ComplexityDirectoryRoot returns the deepest directory that contains every
// analyzed file, which is the root the reported directory paths are relative
// to. Files participate through their parent directory; the scope is never
// widened past what the caller actually asked to analyze.
func ComplexityDirectoryRoot(files []string) (string, error) {
	if len(files) == 0 {
		return "", fmt.Errorf("at least one analyzed file is required")
	}

	var root string
	for _, file := range files {
		identity, err := absoluteDirectory(file)
		if err != nil {
			return "", err
		}

		if root == "" {
			root = identity
			continue
		}
		if filepath.VolumeName(root) != filepath.VolumeName(identity) {
			return "", fmt.Errorf("analyzed files do not share a filesystem volume")
		}
		root = commonDirectory(root, identity)
	}
	return root, nil
}

// AggregateComplexityByDirectory groups the reported function population by its
// direct project-root-relative directory. This is the only owner of directory
// grouping and directory-level complexity arithmetic.
func AggregateComplexityByDirectory(functions []FunctionComplexity, projectRoot string) (DirectoryComplexityMetricsList, error) {
	if len(functions) == 0 {
		return DirectoryComplexityMetricsList{}, nil
	}

	rootIdentity, err := absolutePath(projectRoot)
	if err != nil {
		return nil, err
	}

	directories := make(map[string]*directoryComplexityAccumulator)
	for _, function := range functions {
		fileIdentity, err := absolutePath(function.FilePath)
		if err != nil {
			return nil, err
		}
		relativePath, err := filepath.Rel(rootIdentity, fileIdentity)
		if err != nil {
			return nil, fmt.Errorf("make function file path %q relative to root %q: %w", function.FilePath, projectRoot, err)
		}
		if pathEscapesRoot(relativePath) {
			return nil, fmt.Errorf("function file path %q is outside complexity directory root %q", function.FilePath, projectRoot)
		}

		directoryPath := filepath.Dir(relativePath)
		accumulator := directories[directoryPath]
		if accumulator == nil {
			accumulator = &directoryComplexityAccumulator{
				metrics: DirectoryComplexityMetrics{DirectoryPath: directoryPath},
			}
			directories[directoryPath] = accumulator
		}
		accumulator.addFunction(function)
	}

	result := make(DirectoryComplexityMetricsList, 0, len(directories))
	for _, directory := range directories {
		directory.finishAverages()
		result = append(result, directory.metrics)
	}
	sortDirectoryComplexity(result)
	return result, nil
}

func (a *directoryComplexityAccumulator) addFunction(function FunctionComplexity) {
	a.metrics.FunctionCount++
	a.totalComplexity += function.Metrics.Complexity
	a.totalNestingDepth += function.Metrics.NestingDepth
	if function.Metrics.Complexity > a.metrics.MaxComplexity {
		a.metrics.MaxComplexity = function.Metrics.Complexity
	}
	if function.Metrics.NestingDepth > a.metrics.MaxNestingDepth {
		a.metrics.MaxNestingDepth = function.Metrics.NestingDepth
	}
	if function.RiskLevel == RiskLevelHigh {
		a.metrics.HighRiskFunctionCount++
	}
}

func (a *directoryComplexityAccumulator) finishAverages() {
	count := float64(a.metrics.FunctionCount)
	a.metrics.AverageComplexity = float64(a.totalComplexity) / count
	a.metrics.AverageNestingDepth = float64(a.totalNestingDepth) / count
}

// sortDirectoryComplexity ranks the worst directories first so that a truncated
// report still shows the ones worth acting on.
func sortDirectoryComplexity(directories DirectoryComplexityMetricsList) {
	sort.Slice(directories, func(i, j int) bool {
		left, right := directories[i], directories[j]
		if left.HighRiskFunctionCount != right.HighRiskFunctionCount {
			return left.HighRiskFunctionCount > right.HighRiskFunctionCount
		}
		if left.MaxComplexity != right.MaxComplexity {
			return left.MaxComplexity > right.MaxComplexity
		}
		if left.AverageComplexity != right.AverageComplexity {
			return left.AverageComplexity > right.AverageComplexity
		}
		return left.DirectoryPath < right.DirectoryPath
	})
}

// commonDirectory returns the deepest directory containing both arguments.
func commonDirectory(left, right string) string {
	for {
		relative, err := filepath.Rel(left, right)
		if err == nil && !pathEscapesRoot(relative) {
			return left
		}
		parent := filepath.Dir(left)
		if parent == left {
			return left
		}
		left = parent
	}
}

func pathEscapesRoot(relativePath string) bool {
	return relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator))
}

// absolutePath resolves a caller-facing path to the identity used for grouping.
// Paths are compared as identities only; the caller's own spelling is what gets
// reported back.
func absolutePath(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve path %q: %w", path, err)
	}
	return absolute, nil
}

func absoluteDirectory(file string) (string, error) {
	absolute, err := absolutePath(file)
	if err != nil {
		return "", err
	}
	return filepath.Dir(absolute), nil
}
