package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/report"
)

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := rootCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestAnalyzeText(t *testing.T) {
	out, err := run(t, "analyze", "../../testdata/go/sample.go")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Server.Handle: 8") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestAnalyzeJSON(t *testing.T) {
	out, err := run(t, "analyze", "--format", "json", "--min-complexity", "5", "../../testdata/go")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	var doc report.Document
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	if len(doc.Complexity.Functions) != 3 || doc.Complexity.Summary.TotalFunctions != 10 || doc.Files.Skipped != 0 {
		t.Errorf("listed %d functions of %d, want 3 of 10", len(doc.Complexity.Functions), doc.Complexity.Summary.TotalFunctions)
	}
	if doc.Clones == nil || doc.Clones.Statistics.TotalClonePairs != 3 {
		t.Errorf("clones = %+v, want 3 pairs", doc.Clones)
	}
}

func TestAnalyzeSelect(t *testing.T) {
	out, err := run(t, "analyze", "--select", "clone", "../../testdata/go/clones")
	if err != nil {
		t.Fatalf("analyze: %v\n%s", err, out)
	}
	if strings.Contains(out, "Complexity Analysis") || !strings.Contains(out, "Clone Detection") {
		t.Errorf("unexpected output:\n%s", out)
	}
}

func TestAnalyzeRejectsBadArguments(t *testing.T) {
	for _, args := range [][]string{
		{"analyze"},
		{"analyze", "--select", "deadcode", "../../testdata/go"},
		{"analyze", "--format", "html", "../../testdata/go"},
		{"analyze", "--min-complexity", "0", "../../testdata/go"},
		{"analyze", "does-not-exist"},
	} {
		if _, err := run(t, args...); err == nil {
			t.Errorf("%v: expected an error", args)
		}
	}
}

func TestVersion(t *testing.T) {
	out, err := run(t, "version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "polyscan version ") {
		t.Errorf("unexpected output %q", out)
	}
}
