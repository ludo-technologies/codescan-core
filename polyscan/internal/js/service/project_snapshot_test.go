package service

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/analyzer"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/config"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

func writeSnapshotFixture(t *testing.T, name, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}
	return path
}

func TestBuildProjectSnapshot_ParsesFilesInPathOrder(t *testing.T) {
	first := writeSnapshotFixture(t, "a.js", `function a() { return 1; }`)
	second := writeSnapshotFixture(t, "b.ts", `function b(): number { return 2; }`)
	paths := []string{first, second}

	snapshot := BuildProjectSnapshot(context.Background(), paths)

	if len(snapshot.Files) != len(paths) {
		t.Fatalf("expected %d files, got %d", len(paths), len(snapshot.Files))
	}
	for index, file := range snapshot.Files {
		if file.Path != paths[index] {
			t.Errorf("Files[%d].Path = %q, expected %q", index, file.Path, paths[index])
		}
		if !file.Parsed() {
			t.Errorf("Files[%d] should be parsed: readErr=%v parseErr=%v", index, file.ReadErr, file.ParseErr)
		}
	}
}

func TestBuildProjectSnapshot_RecordsReadErrorPerFile(t *testing.T) {
	valid := writeSnapshotFixture(t, "a.js", `function a() { return 1; }`)
	missing := filepath.Join(t.TempDir(), "missing.js")

	snapshot := BuildProjectSnapshot(context.Background(), []string{missing, valid})

	if snapshot.Files[0].ReadErr == nil {
		t.Error("missing file should carry a read error")
	}
	if snapshot.Files[0].Parsed() {
		t.Error("missing file should not report as parsed")
	}
	if !snapshot.Files[1].Parsed() {
		t.Error("valid file should still be parsed when a sibling fails")
	}
}

func TestBuildProjectSnapshot_CancelledContext(t *testing.T) {
	path := writeSnapshotFixture(t, "a.js", `function a() { return 1; }`)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	snapshot := BuildProjectSnapshot(ctx, []string{path})

	if len(snapshot.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(snapshot.Files))
	}
	if snapshot.Files[0].ReadErr == nil {
		t.Error("cancelled build should record an error on unprocessed files")
	}
}

func TestProjectFile_CFGsBuiltOnceAndShared(t *testing.T) {
	path := writeSnapshotFixture(t, "a.js", `function a() { if (a) { return 1; } return 0; }`)

	snapshot := BuildProjectSnapshot(context.Background(), []string{path})
	file := snapshot.Files[0]

	const callers = 8
	results := make([]map[string]*analyzer.CFG, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			cfgs, err := file.CFGs()
			if err != nil {
				t.Errorf("CFGs() failed: %v", err)
				return
			}
			results[slot] = cfgs
		}(i)
	}
	wg.Wait()

	for i := 1; i < callers; i++ {
		if len(results[i]) != len(results[0]) {
			t.Fatalf("caller %d saw %d CFGs, caller 0 saw %d", i, len(results[i]), len(results[0]))
		}
		for name, cfg := range results[i] {
			if results[0][name] != cfg {
				t.Errorf("caller %d got a different CFG instance for %q", i, name)
			}
		}
	}
}

func TestProjectFile_CFGsReportsParseError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.js")

	snapshot := BuildProjectSnapshot(context.Background(), []string{missing})

	if _, err := snapshot.Files[0].CFGs(); err == nil {
		t.Error("CFGs() should surface the file's read error")
	}
}

func complexityTestConfig() *config.ComplexityConfig {
	return &config.ComplexityConfig{
		LowThreshold:    5,
		MediumThreshold: 10,
		Enabled:         true,
		ReportUnchanged: true,
	}
}

// TestAnalyzeSnapshot_MatchesStandaloneAnalyze pins that analyzing via a shared
// snapshot produces the same results as the standalone path that builds its own.
func TestAnalyzeSnapshot_MatchesStandaloneAnalyze(t *testing.T) {
	path := writeSnapshotFixture(t, "a.js", `
function trivial(a) { return a; }
function branching(a) {
  if (a) { return 1; }
  return 0;
}
`)
	paths := []string{path}

	svc := NewComplexityService(complexityTestConfig())
	req := domain.ComplexityRequest{Paths: paths, SortBy: domain.SortByComplexity}

	standalone, err := svc.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("standalone analysis failed: %v", err)
	}

	snapshot := BuildProjectSnapshot(context.Background(), paths)
	shared, err := svc.AnalyzeSnapshot(context.Background(), snapshot, req)
	if err != nil {
		t.Fatalf("snapshot analysis failed: %v", err)
	}

	if len(shared.Functions) != len(standalone.Functions) {
		t.Fatalf("snapshot path found %d functions, standalone found %d",
			len(shared.Functions), len(standalone.Functions))
	}
	for i := range shared.Functions {
		if shared.Functions[i] != standalone.Functions[i] {
			t.Errorf("function %d differs: snapshot=%+v standalone=%+v",
				i, shared.Functions[i], standalone.Functions[i])
		}
	}
	if !reflect.DeepEqual(shared.Summary, standalone.Summary) {
		t.Errorf("summaries differ: snapshot=%+v standalone=%+v", shared.Summary, standalone.Summary)
	}
}

// TestAnalyzeSnapshot_RejectsMismatchedPaths pins the snapshot entry points'
// contract: the snapshot defines the analyzed file set, so a request naming
// different files must fail instead of silently analyzing the snapshot.
func TestAnalyzeSnapshot_RejectsMismatchedPaths(t *testing.T) {
	path := writeSnapshotFixture(t, "a.js", `function a() { return 1; }`)
	snapshot := BuildProjectSnapshot(context.Background(), []string{path})

	svc := NewComplexityService(complexityTestConfig())
	req := domain.ComplexityRequest{Paths: []string{"other.js"}}

	if _, err := svc.AnalyzeSnapshot(context.Background(), snapshot, req); err == nil {
		t.Error("paths not in the snapshot should be rejected")
	}

	tooMany := domain.ComplexityRequest{Paths: []string{path, "extra.js"}}
	if _, err := svc.AnalyzeSnapshot(context.Background(), snapshot, tooMany); err == nil {
		t.Error("a request naming more paths than the snapshot holds should be rejected")
	}

	matching := domain.ComplexityRequest{Paths: []string{path}}
	if _, err := svc.AnalyzeSnapshot(context.Background(), snapshot, matching); err != nil {
		t.Errorf("matching paths should be accepted: %v", err)
	}
}

func TestAnalyzeSnapshot_NilSnapshot(t *testing.T) {
	svc := NewComplexityService(complexityTestConfig())

	if _, err := svc.AnalyzeSnapshot(context.Background(), nil, domain.ComplexityRequest{}); err == nil {
		t.Error("nil snapshot should be rejected")
	}
	if _, err := AnalyzeDeadCodeSnapshot(context.Background(), nil, domain.DeadCodeRequest{}); err == nil {
		t.Error("nil snapshot should be rejected")
	}
	if _, err := NewCBOServiceWithDefaults().AnalyzeSnapshot(context.Background(), nil, domain.CBORequest{}); err == nil {
		t.Error("nil snapshot should be rejected")
	}
	if _, err := NewDependencyGraphServiceWithDefaults().AnalyzeSnapshot(context.Background(), nil, domain.DependencyGraphRequest{}); err == nil {
		t.Error("nil snapshot should be rejected")
	}
	if _, err := NewCloneServiceWithDefaults().DetectClonesInSnapshot(context.Background(), nil, domain.DefaultCloneRequest()); err == nil {
		t.Error("nil snapshot should be rejected")
	}
}

const brokenSnapshotFixture = `function broken( {`

func TestNewProjectSnapshot_AccountsForFilesWhileReleasingThem(t *testing.T) {
	valid := writeSnapshotFixture(t, "a.js", `function a() { return 1; }`)
	broken := writeSnapshotFixture(t, "b.js", brokenSnapshotFixture)
	missing := filepath.Join(t.TempDir(), "missing.js")
	snapshot := NewProjectSnapshot([]string{valid, broken, missing})

	response, err := NewComplexityService(complexityTestConfig()).AnalyzeSnapshot(context.Background(), snapshot, domain.ComplexityRequest{})
	if err != nil {
		t.Fatalf("analysis failed: %v", err)
	}
	if response.Summary.FilesAnalyzed != 1 || response.Summary.SkippedFiles != 2 {
		t.Errorf("summary = %+v, want 1 analyzed and 2 skipped", response.Summary)
	}

	accounting := snapshot.Accounting()
	if accounting.Total != 3 || accounting.Skipped != 2 || len(accounting.Errors) != 2 {
		t.Fatalf("accounting = %+v, want 3 files with 2 skipped", accounting)
	}
	if !strings.HasPrefix(accounting.Errors[0], broken+": syntax error") {
		t.Errorf("parse failure not reported per file: %q", accounting.Errors[0])
	}
	if !strings.HasPrefix(accounting.Errors[1], missing+": ") {
		t.Errorf("read failure not reported per file: %q", accounting.Errors[1])
	}
	for _, file := range snapshot.Files {
		if file.AST != nil || file.Content != nil {
			t.Errorf("%s still holds its parse tree after its single analysis", file.Path)
		}
	}
}

func TestNewProjectSnapshot_RejectsASecondAnalysis(t *testing.T) {
	path := writeSnapshotFixture(t, "a.js", `function a() { return 1; }`)
	snapshot := NewProjectSnapshot([]string{path})
	svc := NewComplexityService(complexityTestConfig())
	if _, err := svc.AnalyzeSnapshot(context.Background(), snapshot, domain.ComplexityRequest{}); err != nil {
		t.Fatalf("first analysis failed: %v", err)
	}

	if _, err := svc.AnalyzeSnapshot(context.Background(), snapshot, domain.ComplexityRequest{}); err == nil {
		t.Error("a second analysis over a released snapshot must be rejected")
	}
	if _, err := NewDependencyGraphServiceWithDefaults().AnalyzeSnapshot(context.Background(), NewProjectSnapshot([]string{path}), domain.DependencyGraphRequest{}); err == nil {
		t.Error("dependency analysis over a single-analysis snapshot must be rejected")
	}
}

func TestProjectSnapshot_AccountingBeforeAnalysisPanics(t *testing.T) {
	snapshot := NewProjectSnapshot([]string{"unloaded.js"})
	defer func() {
		if recover() == nil {
			t.Error("accounting before the analysis loaded the files must panic")
		}
	}()
	snapshot.Accounting()
}

func TestBuildProjectSnapshot_AccountingIsAvailableAfterBuild(t *testing.T) {
	valid := writeSnapshotFixture(t, "a.js", `function a() { return 1; }`)
	broken := writeSnapshotFixture(t, "b.js", brokenSnapshotFixture)

	accounting := BuildProjectSnapshot(context.Background(), []string{valid, broken}).Accounting()

	if accounting.Total != 2 || accounting.Skipped != 1 {
		t.Errorf("accounting = %+v, want 2 files with 1 skipped", accounting)
	}
}
