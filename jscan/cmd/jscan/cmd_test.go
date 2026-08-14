package main

import (
	"testing"
)

func TestAnalyzeCmd_FlagsExist(t *testing.T) {
	cmd := analyzeCmd()

	expectedFlags := []string{"select", "format", "json", "text", "html", "no-open", "output", "config"}
	for _, flagName := range expectedFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Missing expected flag: --%s", flagName)
		}
	}
}

func TestAnalyzeCmd_ShortFlags(t *testing.T) {
	cmd := analyzeCmd()

	shortFlags := map[string]string{
		"s": "select",
		"f": "format",
		"o": "output",
		"c": "config",
	}

	for short, long := range shortFlags {
		flag := cmd.Flags().ShorthandLookup(short)
		if flag == nil {
			t.Errorf("Missing short flag -%s for --%s", short, long)
		}
	}
}

func TestAnalyzeCmd_DefaultValues(t *testing.T) {
	cmd := analyzeCmd()

	formatFlag := cmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Fatal("format flag not found")
	}
	if formatFlag.DefValue != "html" {
		t.Errorf("Expected default format to be 'html', got '%s'", formatFlag.DefValue)
	}

	selectFlag := cmd.Flags().Lookup("select")
	if selectFlag == nil {
		t.Fatal("select flag not found")
	}
	// Default is all analyses
	if selectFlag.DefValue != "[complexity,deadcode,clone,cbo,deps]" {
		t.Errorf("Expected default select to be '[complexity,deadcode,clone,cbo,deps]', got '%s'", selectFlag.DefValue)
	}
}

func TestAnalyzeCmd_NoPathsError(t *testing.T) {
	cmd := analyzeCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when no paths specified")
	}
}

func TestCheckCmd_FlagsExist(t *testing.T) {
	cmd := checkCmd()

	expectedFlags := []string{"max-complexity", "allow-dead-code", "allow-circular-deps", "max-cycles", "select", "verbose", "json", "config"}
	for _, flagName := range expectedFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Missing expected flag: --%s", flagName)
		}
	}
}

func TestCheckCmd_ShortFlags(t *testing.T) {
	cmd := checkCmd()

	shortFlags := map[string]string{
		"s": "select",
		"v": "verbose",
		"c": "config",
	}

	for short, long := range shortFlags {
		flag := cmd.Flags().ShorthandLookup(short)
		if flag == nil {
			t.Errorf("Missing short flag -%s for --%s", short, long)
		}
	}
}

func TestCheckCmd_NoPathsError(t *testing.T) {
	cmd := checkCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when no paths specified")
	}
}

func TestCheckExitError_Error(t *testing.T) {
	err := &CheckExitError{Code: 1, Message: "test error"}
	if err.Error() != "test error" {
		t.Errorf("Error() should return message, got '%s'", err.Error())
	}
}

func TestDepsCmd_FlagsExist(t *testing.T) {
	cmd := depsCmd()

	expectedFlags := []string{"format", "output", "config", "dot", "include-external", "include-types", "no-cycles", "max-depth", "min-coupling", "no-legend", "rank-dir"}
	for _, flagName := range expectedFlags {
		flag := cmd.Flags().Lookup(flagName)
		if flag == nil {
			t.Errorf("Missing expected flag: --%s", flagName)
		}
	}
}

func TestDepsCmd_ShortFlags(t *testing.T) {
	cmd := depsCmd()

	shortFlags := map[string]string{
		"f": "format",
		"o": "output",
		"c": "config",
	}

	for short, long := range shortFlags {
		flag := cmd.Flags().ShorthandLookup(short)
		if flag == nil {
			t.Errorf("Missing short flag -%s for --%s", short, long)
		}
	}
}

func TestDepsCmd_DefaultValues(t *testing.T) {
	cmd := depsCmd()

	formatFlag := cmd.Flags().Lookup("format")
	if formatFlag == nil {
		t.Fatal("format flag not found")
	}
	if formatFlag.DefValue != "text" {
		t.Errorf("Expected default format to be 'text', got '%s'", formatFlag.DefValue)
	}
}

func TestDepsCmd_NoPathsError(t *testing.T) {
	cmd := depsCmd()
	cmd.SetArgs([]string{})

	err := cmd.Execute()
	if err == nil {
		t.Error("Expected error when no paths specified")
	}
}

func TestVersionCmd_FlagsExist(t *testing.T) {
	cmd := versionCmd()

	if cmd == nil {
		t.Fatal("versionCmd should not return nil")
	}

	verboseFlag := cmd.Flags().Lookup("verbose")
	if verboseFlag == nil {
		t.Error("Missing expected flag: --verbose")
	}
}

func TestVersionCmd_ShortFlag(t *testing.T) {
	cmd := versionCmd()

	flag := cmd.Flags().ShorthandLookup("v")
	if flag == nil {
		t.Error("Missing short flag -v for --verbose")
	}
}

func TestVersionCmd_Execute(t *testing.T) {
	cmd := versionCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Errorf("versionCmd.Execute() error = %v", err)
	}
}

func TestVersionCmd_ExecuteVerbose(t *testing.T) {
	cmd := versionCmd()
	cmd.SetArgs([]string{"--verbose"})
	if err := cmd.Execute(); err != nil {
		t.Errorf("versionCmd.Execute(--verbose) error = %v", err)
	}
}
