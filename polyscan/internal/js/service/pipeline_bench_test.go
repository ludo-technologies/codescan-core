package service

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/config"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

// corpusEnvVar names the directory holding the JavaScript/TypeScript corpus the
// pipeline benchmarks run against. Benchmarks skip when it is unset so `go test
// -bench=.` stays fast for everyone who has not opted in.
//
//	JSCAN_BENCH_CORPUS=/path/to/repo go test -run='^$' -bench=BenchmarkPipeline \
//	    -benchmem -cpuprofile=cpu.prof ./service/
const corpusEnvVar = "JSCAN_BENCH_CORPUS"

var benchSourceExtensions = []string{".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts"}

// benchCorpusPaths collects every analyzable source file under the corpus
// directory, sorted so repeated runs analyze the files in the same order.
func benchCorpusPaths(b *testing.B) []string {
	b.Helper()

	root := os.Getenv(corpusEnvVar)
	if root == "" {
		b.Skipf("set %s to a JavaScript/TypeScript source tree to run this benchmark", corpusEnvVar)
	}

	var paths []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "node_modules" || strings.HasPrefix(entry.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		for _, candidate := range benchSourceExtensions {
			if ext == candidate {
				paths = append(paths, path)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		b.Fatalf("walking corpus %s: %v", root, err)
	}
	if len(paths) == 0 {
		b.Fatalf("no JavaScript/TypeScript files found under %s", root)
	}

	sort.Strings(paths)
	return paths
}

// reportThroughput records lines-of-source per second, the metric the
// performance targets in docs/performance.md are stated in.
func reportThroughput(b *testing.B, paths []string) {
	b.Helper()

	lines := 0
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines += countSourceLines(content)
	}
	b.ReportMetric(float64(lines*b.N)/b.Elapsed().Seconds(), "LOC/s")
}

func BenchmarkPipelineClone(b *testing.B) {
	paths := benchCorpusPaths(b)
	service := NewCloneServiceWithDefaults()

	req := domain.DefaultCloneRequest()
	req.Paths = paths
	req.OutputWriter = io.Discard

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.DetectClones(context.Background(), req); err != nil {
			b.Fatalf("clone detection failed: %v", err)
		}
	}
	b.StopTimer()
	reportThroughput(b, paths)
}

func BenchmarkPipelineComplexity(b *testing.B) {
	paths := benchCorpusPaths(b)
	service := NewComplexityService(&config.DefaultConfig().Complexity)

	req := domain.ComplexityRequest{Paths: paths, OutputWriter: io.Discard}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Analyze(context.Background(), req); err != nil {
			b.Fatalf("complexity analysis failed: %v", err)
		}
	}
	b.StopTimer()
	reportThroughput(b, paths)
}

func BenchmarkPipelineDeadCode(b *testing.B) {
	paths := benchCorpusPaths(b)
	service := NewDeadCodeService()

	req := domain.DeadCodeRequest{Paths: paths, OutputWriter: io.Discard}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Analyze(context.Background(), req); err != nil {
			b.Fatalf("dead code analysis failed: %v", err)
		}
	}
	b.StopTimer()
	reportThroughput(b, paths)
}

func BenchmarkPipelineDeps(b *testing.B) {
	paths := benchCorpusPaths(b)
	service := NewDependencyGraphServiceWithDefaults()

	req := domain.DefaultDependencyGraphRequest()
	req.Paths = paths

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Analyze(context.Background(), *req); err != nil {
			b.Fatalf("dependency analysis failed: %v", err)
		}
	}
	b.StopTimer()
	reportThroughput(b, paths)
}

func BenchmarkPipelineCBO(b *testing.B) {
	paths := benchCorpusPaths(b)
	service := NewCBOServiceWithDefaults()

	req := domain.DefaultCBORequest()
	req.Paths = paths
	req.OutputWriter = io.Discard

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Analyze(context.Background(), *req); err != nil {
			b.Fatalf("CBO analysis failed: %v", err)
		}
	}
	b.StopTimer()
	reportThroughput(b, paths)
}
