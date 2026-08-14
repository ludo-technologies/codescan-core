package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func resetAnalyzeFlags() {
	selectAnalyses = []string{"complexity", "deadcode", "clone", "cbo", "deps"}
	outputFormat = "html"
	configPath = ""
	jsonOutput = false
	htmlOutput = false
	textOutput = false
	noOpenBrowser = false
	outputPath = ""
}

func writeSampleJSFile(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	filePath := filepath.Join(dir, "index.js")
	content := `
function calculateTotal(items) {
  let total = 0;
  for (const item of items) {
    if (item.price > 0) {
      total += item.price;
    }
  }
  return total;
}
`
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write sample file: %v", err)
	}
	return dir, filePath
}

func captureStdoutAndStderr(f func() error) (stdout string, stderr string, err error) {
	origStdout := os.Stdout
	origStderr := os.Stderr

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()

	os.Stdout = wOut
	os.Stderr = wErr

	outChan := make(chan string)
	errChan := make(chan string)

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		outChan <- buf.String()
	}()

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		errChan <- buf.String()
	}()

	err = f()

	_ = wOut.Close()
	_ = wErr.Close()

	os.Stdout = origStdout
	os.Stderr = origStderr

	stdout = <-outChan
	stderr = <-errChan

	_ = rOut.Close()
	_ = rErr.Close()

	return stdout, stderr, err
}

func TestAnalyzeCmd_FormatFlagHelpText(t *testing.T) {
	cmd := analyzeCmd()
	formatFlag := cmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Fatal("expected format flag to exist")
	}
	if !strings.Contains(formatFlag.Usage, "yaml") || !strings.Contains(formatFlag.Usage, "csv") {
		t.Errorf("expected format flag usage to mention yaml and csv, got %q", formatFlag.Usage)
	}
}

func TestAnalyzeCmd_InvalidFormatErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		errPattern string
	}{
		{
			name:       "unrecognized format foo",
			args:       []string{"--format", "foo"},
			errPattern: `invalid format "foo", must be one of: html, json, text, yaml, csv`,
		},
		{
			name:       "uppercase JSON",
			args:       []string{"--format", "JSON"},
			errPattern: `invalid format "JSON", must be one of: html, json, text, yaml, csv`,
		},
		{
			name:       "typo jsonn",
			args:       []string{"--format", "jsonn"},
			errPattern: `invalid format "jsonn", must be one of: html, json, text, yaml, csv`,
		},
		{
			name:       "invalid format with json flag",
			args:       []string{"--json", "--format", "invalid"},
			errPattern: `invalid format "invalid", must be one of: html, json, text, yaml, csv`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetAnalyzeFlags()
			dir, _ := writeSampleJSFile(t)

			cmd := analyzeCmd()
			fullArgs := append(tt.args, dir)
			cmd.SetArgs(fullArgs)

			_, _, err := captureStdoutAndStderr(func() error {
				return cmd.Execute()
			})

			if err == nil {
				t.Fatalf("expected error for args %v, got nil", fullArgs)
			}
			if !strings.Contains(err.Error(), tt.errPattern) {
				t.Errorf("expected error containing %q, got %q", tt.errPattern, err.Error())
			}
		})
	}
}

func TestAnalyzeCmd_FormatYAML(t *testing.T) {
	resetAnalyzeFlags()
	dir, _ := writeSampleJSFile(t)

	cmd := analyzeCmd()
	cmd.SetArgs([]string{"--format", "yaml", dir})

	stdout, stderr, err := captureStdoutAndStderr(func() error {
		return cmd.Execute()
	})

	if err != nil {
		t.Fatalf("analyze --format yaml failed: %v", err)
	}

	// Verify stdout is valid YAML and not corrupted by terminal messages
	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(stdout), &parsed); err != nil {
		t.Fatalf("stdout is not valid YAML: %v\nOutput was:\n%s", err, stdout)
	}

	if _, ok := parsed["version"]; !ok {
		t.Errorf("YAML output missing version field: %s", stdout)
	}
	if _, ok := parsed["complexity"]; !ok {
		t.Errorf("YAML output missing complexity field: %s", stdout)
	}

	// Verify score summary is written to stderr
	if !strings.Contains(stderr, "Health Score:") {
		t.Errorf("expected stderr to contain score summary, got:\n%s", stderr)
	}
}

func TestAnalyzeCmd_FormatCSV(t *testing.T) {
	resetAnalyzeFlags()
	dir, _ := writeSampleJSFile(t)

	cmd := analyzeCmd()
	cmd.SetArgs([]string{"--format", "csv", dir})

	stdout, stderr, err := captureStdoutAndStderr(func() error {
		return cmd.Execute()
	})

	if err != nil {
		t.Fatalf("analyze --format csv failed: %v", err)
	}

	// Verify stdout contains CSV header
	if !strings.Contains(stdout, "type,file,function,start_line,end_line") {
		t.Errorf("stdout does not contain expected CSV header, got:\n%s", stdout)
	}

	// Verify stdout contains CSV data
	if !strings.Contains(stdout, "calculateTotal") {
		t.Errorf("stdout does not contain function name in CSV, got:\n%s", stdout)
	}

	// Verify score summary is written to stderr
	if !strings.Contains(stderr, "Health Score:") {
		t.Errorf("expected stderr to contain score summary, got:\n%s", stderr)
	}
}

func TestAnalyzeCmd_FormatJSON(t *testing.T) {
	resetAnalyzeFlags()
	dir, _ := writeSampleJSFile(t)

	cmd := analyzeCmd()
	cmd.SetArgs([]string{"--format", "json", dir})

	stdout, stderr, err := captureStdoutAndStderr(func() error {
		return cmd.Execute()
	})

	if err != nil {
		t.Fatalf("analyze --format json failed: %v", err)
	}

	if !strings.Contains(stdout, `"complexity"`) {
		t.Errorf("expected JSON output on stdout, got:\n%s", stdout)
	}

	if !strings.Contains(stderr, "Health Score:") {
		t.Errorf("expected stderr to contain score summary, got:\n%s", stderr)
	}
}

func TestAnalyzeCmd_FormatText(t *testing.T) {
	resetAnalyzeFlags()
	dir, _ := writeSampleJSFile(t)

	cmd := analyzeCmd()
	cmd.SetArgs([]string{"--format", "text", dir})

	stdout, _, err := captureStdoutAndStderr(func() error {
		return cmd.Execute()
	})

	if err != nil {
		t.Fatalf("analyze --format text failed: %v", err)
	}

	if !strings.Contains(stdout, "Complexity Analysis") {
		t.Errorf("expected text output on stdout, got:\n%s", stdout)
	}
}
