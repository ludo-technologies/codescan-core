package js

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeConfigFile(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "jscan.config.json")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	return path
}

// TestLoadCommandConfig_WarnsAboutIgnoredKeys covers the point of #41: a key
// that is accepted and then ignored has to say so.
func TestLoadCommandConfig_WarnsAboutIgnoredKeys(t *testing.T) {
	path := writeConfigFile(t, `{
  "complexity": {"low_threshold": 5, "medium_threshold": 10},
  "dead_code": {"context_lines": 5},
  "output": {"show_details": true}
}`)

	var warnings bytes.Buffer
	cfg, err := LoadConfig(path, "", &warnings)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if cfg.Complexity.LowThreshold != 5 {
		t.Errorf("low_threshold = %d, expected the file's value 5", cfg.Complexity.LowThreshold)
	}

	out := warnings.String()
	for _, key := range []string{"dead_code.context_lines", "output.show_details"} {
		if !strings.Contains(out, key) {
			t.Errorf("warning should name %s, got:\n%s", key, out)
		}
	}
	if strings.Contains(out, "complexity.low_threshold") {
		t.Errorf("warning should not name an applied key, got:\n%s", out)
	}
	if !strings.Contains(out, ConfigDocsURL) {
		t.Errorf("warning should point at the documentation, got:\n%s", out)
	}
}

// TestLoadCommandConfig_SilentWhenEveryKeyApplies keeps the warning from
// becoming noise that users learn to ignore.
func TestLoadCommandConfig_SilentWhenEveryKeyApplies(t *testing.T) {
	path := writeConfigFile(t, `{
  "complexity": {"low_threshold": 5, "medium_threshold": 10},
  "analysis": {"exclude_patterns": ["dist"]}
}`)

	var warnings bytes.Buffer
	if _, err := LoadConfig(path, "", &warnings); err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}
	if warnings.Len() != 0 {
		t.Errorf("expected no warning, got:\n%s", warnings.String())
	}
}

// TestLoadCommandConfig_InvalidFileFails keeps validation failures loud: they
// are errors, not warnings.
func TestLoadCommandConfig_InvalidFileFails(t *testing.T) {
	path := writeConfigFile(t, `{"complexity": {"low_threshold": 10, "medium_threshold": 5}}`)

	var warnings bytes.Buffer
	if _, err := LoadConfig(path, "", &warnings); err == nil {
		t.Error("expected an error for a config whose thresholds are inverted")
	}
}
