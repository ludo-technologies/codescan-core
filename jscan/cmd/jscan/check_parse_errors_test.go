package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeParseErrorFixture creates a directory holding one file jscan can analyze
// and one it cannot, which is the case the gate exists for: the broken file is
// dropped from every metric, so the remaining file alone decides the verdict.
func writeParseErrorFixture(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	good := "function ok(x) {\n  if (x) { return 1; }\n  return 0;\n}\n"
	if err := os.WriteFile(filepath.Join(dir, "good.js"), []byte(good), 0644); err != nil {
		t.Fatalf("failed to write good.js: %v", err)
	}
	broken := "function broken( {\n  return ***;\n"
	if err := os.WriteFile(filepath.Join(dir, "broken.js"), []byte(broken), 0644); err != nil {
		t.Fatalf("failed to write broken.js: %v", err)
	}
	return dir
}

// runCheckCmd executes check with args, restoring the package-level flag state
// the cobra command writes into so that tests stay independent of each other.
func runCheckCmd(t *testing.T, args ...string) error {
	t.Helper()

	t.Cleanup(func() {
		checkAllowParseErrors = false
		checkAllowDeadCode = false
		checkSelectAnalyses = checkAnalyses
	})

	cmd := checkCmd()
	cmd.SetArgs(args)
	cmd.SetOut(os.Stderr)
	return cmd.Execute()
}

func exitCode(t *testing.T, err error) int {
	t.Helper()

	if err == nil {
		return 0
	}
	var exitErr *CheckExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected a CheckExitError, got %T: %v", err, err)
	}
	return exitErr.Code
}

func TestCheck_FailsOnFilesThatCannotBeParsed(t *testing.T) {
	dir := writeParseErrorFixture(t)

	err := runCheckCmd(t, "--select", "complexity", dir)

	if code := exitCode(t, err); code != 2 {
		t.Errorf("an unparseable file must exit 2 (analysis error), got %d (err: %v)", code, err)
	}
	if err != nil && !strings.Contains(err.Error(), "--allow-parse-errors") {
		t.Errorf("the failure should name the escape hatch, got %q", err.Error())
	}
}

func TestCheck_AllowParseErrorsKeepsTheOldBehaviour(t *testing.T) {
	dir := writeParseErrorFixture(t)

	err := runCheckCmd(t, "--select", "complexity", "--allow-parse-errors", "--max-complexity", "10", dir)

	if code := exitCode(t, err); code != 0 {
		t.Errorf("--allow-parse-errors should report and pass, got exit %d (err: %v)", code, err)
	}
}

func TestCheck_UnknownSelectValueIsAnAnalysisError(t *testing.T) {
	dir := writeParseErrorFixture(t)

	err := runCheckCmd(t, "--select", "complexty", dir)

	if code := exitCode(t, err); code != 2 {
		t.Errorf("a misspelled --select value would otherwise select nothing and pass; got exit %d", code)
	}
}

func TestCheck_UnknownFlagIsAnAnalysisError(t *testing.T) {
	dir := writeParseErrorFixture(t)

	err := runCheckCmd(t, "--not-a-flag", dir)

	if code := exitCode(t, err); code != 2 {
		t.Errorf("an unknown flag is invalid input, not a quality verdict; got exit %d", code)
	}
}
