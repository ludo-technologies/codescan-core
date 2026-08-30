package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/config"
)

func TestInitCommand_BasicConfigCreation(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "jscan-init-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Set up the config path
	configPath := filepath.Join(tmpDir, "jscan.config.json")

	// Run the init command with args
	cmd := initCmd()
	cmd.SetArgs([]string{"--config", configPath})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("init command failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Verify content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	// Check for expected sections
	contentStr := string(content)
	expectedSections := []string{
		"complexity",
		"dead_code",
		"output",
		"analysis",
		"low_threshold",
		"medium_threshold",
	}

	for _, section := range expectedSections {
		if !strings.Contains(contentStr, section) {
			t.Errorf("Config file missing expected section: %s", section)
		}
	}
}

func TestInitCommand_ForceOverwrite(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "jscan-init-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "jscan.config.json")

	// Create an existing file
	existingContent := []byte(`{"existing": true}`)
	if err := os.WriteFile(configPath, existingContent, 0644); err != nil {
		t.Fatalf("Failed to create existing file: %v", err)
	}

	// Try to create without force - should fail
	cmd := initCmd()
	cmd.SetArgs([]string{"--config", configPath})
	err = cmd.Execute()
	if err == nil {
		t.Fatal("Expected error when file exists without --force")
	}

	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Expected 'already exists' error, got: %v", err)
	}

	// Now try with force - should succeed
	cmd = initCmd()
	cmd.SetArgs([]string{"--config", configPath, "--force"})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("init --force failed: %v", err)
	}

	// Verify file was overwritten (should have complexity section now)
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	if !strings.Contains(string(content), "complexity") {
		t.Error("Config file was not overwritten with new content")
	}
}

func TestInitCommand_MinimalConfig(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "jscan-init-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configPath := filepath.Join(tmpDir, "jscan.config.json")

	cmd := initCmd()
	cmd.SetArgs([]string{"--config", configPath, "--minimal"})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("init --minimal failed: %v", err)
	}

	// Verify file was created
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	// Minimal config should be shorter and contain essential sections
	contentStr := string(content)

	if !strings.Contains(contentStr, "complexity") {
		t.Error("Minimal config missing complexity section")
	}

	if !strings.Contains(contentStr, "dead_code") {
		t.Error("Minimal config missing dead_code section")
	}

	// Generated config must be valid JSON
	if !json.Valid(content) {
		t.Error("Minimal config should be valid JSON")
	}
}

func TestInitCommand_CustomOutputPath(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "jscan-init-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Test custom filename
	customPath := filepath.Join(tmpDir, "custom-config.json")

	cmd := initCmd()
	cmd.SetArgs([]string{"--config", customPath})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("init with custom path failed: %v", err)
	}

	// Verify file was created at custom path
	if _, err := os.Stat(customPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created at custom path")
	}
}

func TestInitCommand_InvalidDirectory(t *testing.T) {
	// Try to create config in non-existent directory
	cmd := initCmd()
	cmd.SetArgs([]string{"--config", "/nonexistent/directory/jscan.config.json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("Expected error when directory doesn't exist")
	}

	if !strings.Contains(err.Error(), "directory does not exist") {
		t.Errorf("Expected 'directory does not exist' error, got: %v", err)
	}
}

func TestInitCommand_FullConfigSize(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "jscan-init-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create full config
	fullPath := filepath.Join(tmpDir, "full.json")
	cmd := initCmd()
	cmd.SetArgs([]string{"--config", fullPath})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("init failed: %v", err)
	}

	fullContent, _ := os.ReadFile(fullPath)

	// Create minimal config
	minimalPath := filepath.Join(tmpDir, "minimal.json")
	cmd = initCmd()
	cmd.SetArgs([]string{"--config", minimalPath, "--minimal"})
	err = cmd.Execute()
	if err != nil {
		t.Fatalf("init --minimal failed: %v", err)
	}

	minimalContent, _ := os.ReadFile(minimalPath)

	// Full config should be larger than minimal
	if len(fullContent) <= len(minimalContent) {
		t.Error("Full config should be larger than minimal config")
	}
}

func TestGetFullConfigTemplate(t *testing.T) {
	tests := []struct {
		projectType config.ProjectType
		strictness  config.Strictness
		wantLow     string
		wantMedium  string
		wantMax     string
	}{
		{
			projectType: config.ProjectTypeGeneric,
			strictness:  config.StrictnessStandard,
			wantLow:     `"low_threshold": 10`,
			wantMedium:  `"medium_threshold": 20`,
			wantMax:     `"max_complexity": 0`,
		},
		{
			projectType: config.ProjectTypeReact,
			strictness:  config.StrictnessStrict,
			wantLow:     `"low_threshold": 5`,
			wantMedium:  `"medium_threshold": 10`,
			wantMax:     `"max_complexity": 15`,
		},
		{
			projectType: config.ProjectTypeVue,
			strictness:  config.StrictnessRelaxed,
			wantLow:     `"low_threshold": 15`,
			wantMedium:  `"medium_threshold": 30`,
			wantMax:     `"max_complexity": 0`,
		},
	}

	for _, tt := range tests {
		t.Run(string(tt.projectType)+"_"+string(tt.strictness), func(t *testing.T) {
			template := config.GetFullConfigTemplate(tt.projectType, tt.strictness)

			if !strings.Contains(template, tt.wantLow) {
				t.Errorf("Template missing expected lowThreshold: %s", tt.wantLow)
			}

			if !strings.Contains(template, tt.wantMedium) {
				t.Errorf("Template missing expected mediumThreshold: %s", tt.wantMedium)
			}

			if !strings.Contains(template, tt.wantMax) {
				t.Errorf("Template missing expected maxComplexity: %s", tt.wantMax)
			}
		})
	}
}

func TestGetMinimalConfigTemplate(t *testing.T) {
	template := config.GetMinimalConfigTemplate()

	// Check essential sections exist
	requiredSections := []string{
		"complexity",
		"dead_code",
		"analysis",
		"low_threshold",
		"medium_threshold",
		"include_patterns",
		"exclude_patterns",
	}

	for _, section := range requiredSections {
		if !strings.Contains(template, section) {
			t.Errorf("Minimal template missing required section: %s", section)
		}
	}

	// Verify it's smaller than full template
	fullTemplate := config.GetFullConfigTemplate(config.ProjectTypeGeneric, config.StrictnessStandard)
	if len(template) >= len(fullTemplate) {
		t.Error("Minimal template should be smaller than full template")
	}
}

func TestProjectPresets(t *testing.T) {
	presets := config.GetProjectPresets()

	// Verify all project types have presets
	projectTypes := []config.ProjectType{
		config.ProjectTypeGeneric,
		config.ProjectTypeReact,
		config.ProjectTypeVue,
		config.ProjectTypeNodeBackend,
	}

	for _, pt := range projectTypes {
		preset, ok := presets[pt]
		if !ok {
			t.Errorf("Missing preset for project type: %s", pt)
			continue
		}

		if len(preset.IncludePatterns) == 0 {
			t.Errorf("Project type %s has no include patterns", pt)
		}

		if len(preset.ExcludePatterns) == 0 {
			t.Errorf("Project type %s has no exclude patterns", pt)
		}

		// All should exclude node_modules
		hasNodeModules := false
		for _, pattern := range preset.ExcludePatterns {
			if strings.Contains(pattern, "node_modules") {
				hasNodeModules = true
				break
			}
		}
		if !hasNodeModules {
			t.Errorf("Project type %s should exclude node_modules", pt)
		}
	}
}

func TestStrictnessPresets(t *testing.T) {
	presets := config.GetStrictnessPresets()

	// Verify all strictness levels have presets
	strictnessLevels := []config.Strictness{
		config.StrictnessRelaxed,
		config.StrictnessStandard,
		config.StrictnessStrict,
	}

	for _, s := range strictnessLevels {
		preset, ok := presets[s]
		if !ok {
			t.Errorf("Missing preset for strictness: %s", s)
			continue
		}

		if preset.LowThreshold <= 0 {
			t.Errorf("Strictness %s has invalid lowThreshold: %d", s, preset.LowThreshold)
		}

		if preset.MediumThreshold <= preset.LowThreshold {
			t.Errorf("Strictness %s: mediumThreshold (%d) should be > lowThreshold (%d)",
				s, preset.MediumThreshold, preset.LowThreshold)
		}
	}

	// Verify strictness ordering (relaxed > standard > strict thresholds)
	relaxed := presets[config.StrictnessRelaxed]
	standard := presets[config.StrictnessStandard]
	strict := presets[config.StrictnessStrict]

	if relaxed.LowThreshold <= standard.LowThreshold {
		t.Error("Relaxed should have higher thresholds than standard")
	}

	if standard.LowThreshold <= strict.LowThreshold {
		t.Error("Standard should have higher thresholds than strict")
	}

	// Strict should have maxComplexity set
	if strict.MaxComplexity <= 0 {
		t.Error("Strict mode should have maxComplexity enforcement")
	}
}

func TestConfigTemplateIsValidJSON(t *testing.T) {
	template := config.GetFullConfigTemplate(config.ProjectTypeGeneric, config.StrictnessStandard)

	if !json.Valid([]byte(template)) {
		t.Fatal("Full template should be valid JSON")
	}

	unexpectedLegacyKeys := []string{
		"showDetails",
		"minSeverity",
		"include\":",
		"exclude\":",
	}

	for _, key := range unexpectedLegacyKeys {
		if strings.Contains(template, key) {
			t.Errorf("Template should not contain legacy key format: %s", key)
		}
	}
}

func TestReactProjectPresetHasNextExclusion(t *testing.T) {
	presets := config.GetProjectPresets()
	reactPreset := presets[config.ProjectTypeReact]

	hasNextDir := false
	for _, pattern := range reactPreset.ExcludePatterns {
		if strings.Contains(pattern, ".next") {
			hasNextDir = true
			break
		}
	}

	if !hasNextDir {
		t.Error("React preset should exclude .next directory")
	}
}

func TestVueProjectPresetHasNuxtExclusion(t *testing.T) {
	presets := config.GetProjectPresets()
	vuePreset := presets[config.ProjectTypeVue]

	hasNuxtDir := false
	for _, pattern := range vuePreset.ExcludePatterns {
		if strings.Contains(pattern, ".nuxt") {
			hasNuxtDir = true
			break
		}
	}

	if !hasNuxtDir {
		t.Error("Vue preset should exclude .nuxt directory")
	}

	// Vue preset should include .vue files
	hasVueFiles := false
	for _, pattern := range vuePreset.IncludePatterns {
		if strings.Contains(pattern, ".vue") {
			hasVueFiles = true
			break
		}
	}

	if !hasVueFiles {
		t.Error("Vue preset should include .vue files")
	}
}

func TestNodeBackendPresetHasMjsCjs(t *testing.T) {
	presets := config.GetProjectPresets()
	nodePreset := presets[config.ProjectTypeNodeBackend]

	hasMjs := false
	hasCjs := false
	for _, pattern := range nodePreset.IncludePatterns {
		if strings.Contains(pattern, ".mjs") {
			hasMjs = true
		}
		if strings.Contains(pattern, ".cjs") {
			hasCjs = true
		}
	}

	if !hasMjs {
		t.Error("Node backend preset should include .mjs files")
	}

	if !hasCjs {
		t.Error("Node backend preset should include .cjs files")
	}
}

func TestInitCmd_FlagsExist(t *testing.T) {
	cmd := initCmd()

	// Check that all expected flags exist
	expectedFlags := []string{"config", "force", "minimal", "interactive"}
	for _, flagName := range expectedFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Missing expected flag: --%s", flagName)
		}
	}

	// Check short flags
	shortFlags := map[string]string{
		"c": "config",
		"f": "force",
		"i": "interactive",
	}

	for short, long := range shortFlags {
		flag := cmd.Flags().ShorthandLookup(short)
		if flag == nil {
			t.Errorf("Missing short flag -%s for --%s", short, long)
		}
	}
}

func TestInitCmd_DefaultConfigPath(t *testing.T) {
	cmd := initCmd()

	// Verify default config path
	configFlag := cmd.Flags().Lookup("config")
	if configFlag == nil {
		t.Fatal("config flag not found")
	}

	if configFlag.DefValue != "jscan.config.json" {
		t.Errorf("Expected default config path to be 'jscan.config.json', got '%s'", configFlag.DefValue)
	}
}
