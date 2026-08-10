package analyzer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/internal/parser"
)

// helper to parse JS source and get module info + AST
func parseAndAnalyze(t *testing.T, source string) (*parser.Node, *domain.ModuleInfo) {
	t.Helper()
	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	ma := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := ma.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze module: %v", err)
	}

	return ast, info
}

// helper to parse TS source and get module info + AST
func parseAndAnalyzeTS(t *testing.T, source string) (*parser.Node, *domain.ModuleInfo) {
	t.Helper()
	p := parser.NewTypeScriptParser()
	defer p.Close()

	ast, err := p.ParseFile("test.ts", []byte(source))
	if err != nil {
		t.Fatalf("Failed to parse TS: %v", err)
	}

	ma := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := ma.AnalyzeFile(ast, "test.ts")
	if err != nil {
		t.Fatalf("Failed to analyze module: %v", err)
	}

	return ast, info
}

// --- Unused Import Tests ---

func TestDetectUnusedImports_AllUsed(t *testing.T) {
	source := `
import { useState, useEffect } from 'react';

const [count, setCount] = useState(0);
useEffect(() => {}, []);
`
	ast, info := parseAndAnalyze(t, source)
	findings := DetectUnusedImports(ast, info, "test.js", []byte(source))

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings when all imports are used, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  finding: %s", f.Description)
		}
	}
}

func TestDetectUnusedImports_OneUnused(t *testing.T) {
	source := `
import { useState, useEffect } from 'react';

const [count, setCount] = useState(0);
`
	ast, info := parseAndAnalyze(t, source)
	findings := DetectUnusedImports(ast, info, "test.js", []byte(source))

	if len(findings) != 1 {
		t.Fatalf("Expected 1 finding for unused useEffect, got %d", len(findings))
	}

	if findings[0].Reason != ReasonUnusedImport {
		t.Errorf("Expected reason %s, got %s", ReasonUnusedImport, findings[0].Reason)
	}
	if findings[0].Severity != SeverityLevelWarning {
		t.Errorf("Expected severity warning, got %s", findings[0].Severity)
	}
}

func TestDetectUnusedImports_DefaultUnused(t *testing.T) {
	source := `
import React from 'react';
import { useState } from 'react';

const x = useState(0);
`
	ast, info := parseAndAnalyze(t, source)
	findings := DetectUnusedImports(ast, info, "test.js", []byte(source))

	// React (default import) should be unused
	found := false
	for _, f := range findings {
		if f.Reason == ReasonUnusedImport {
			found = true
		}
	}
	if !found {
		t.Error("Expected at least one unused import finding for default import 'React'")
	}
}

func TestDetectUnusedImports_SideEffectSkipped(t *testing.T) {
	source := `
import 'polyfill';

console.log('hello');
`
	ast, info := parseAndAnalyze(t, source)
	findings := DetectUnusedImports(ast, info, "test.js", []byte(source))

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for side-effect import, got %d", len(findings))
	}
}

func TestDetectUnusedImports_TypeOnlySkipped(t *testing.T) {
	// When IsTypeOnly is set on the import, it should be skipped.
	// We test this with manually constructed ModuleInfo since the parser
	// does not yet fully detect `import type` syntax.
	ast := &parser.Node{
		Type: parser.NodeProgram,
		Body: []*parser.Node{
			{Type: parser.NodeExpressionStatement, Children: []*parser.Node{
				{Type: parser.NodeIdentifier, Name: "x"},
			}},
		},
	}

	info := &domain.ModuleInfo{
		FilePath: "test.ts",
		Imports: []*domain.Import{
			{
				Source:     "./types",
				SourceType: domain.ModuleTypeRelative,
				ImportType: domain.ImportTypeTypeOnly,
				IsTypeOnly: true,
				Specifiers: []domain.ImportSpecifier{
					{Imported: "Foo", Local: "Foo"},
				},
			},
		},
	}

	findings := DetectUnusedImports(ast, info, "test.ts", nil)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for type-only import, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  finding: %s", f.Description)
		}
	}
}

func TestDetectUnusedImports_TypeReferenceCountsAsUsage(t *testing.T) {
	source := `
import { Metadata } from "next";

type PageMeta = Metadata;
`
	ast, info := parseAndAnalyzeTS(t, source)
	findings := DetectUnusedImports(ast, info, "test.ts", []byte(source))

	if len(findings) != 0 {
		t.Fatalf("Expected 0 findings when import is used in type position, got %d", len(findings))
	}
}

func TestDetectUnusedImports_ImportTypeLineSkipped(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "sample.ts")
	source := `import type { ScanResult } from "@/types/scan";
export async function getScanResult(id: string): Promise<ScanResult> {
	return {} as ScanResult;
}`
	// Write a plain (non-type-only) import to disk: the detection must judge the
	// content it is handed, never re-read the file, so the finding count below
	// also pins that the analysis reads each file exactly once.
	stale := `import { ScanResult } from "@/types/scan";`
	if err := os.WriteFile(filePath, []byte(stale), 0o600); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	p := parser.NewTypeScriptParser()
	defer p.Close()

	ast, err := p.ParseFile(filePath, []byte(source))
	if err != nil {
		t.Fatalf("Failed to parse TS: %v", err)
	}

	ma := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := ma.AnalyzeFile(ast, filePath)
	if err != nil {
		t.Fatalf("Failed to analyze module: %v", err)
	}

	findings := DetectUnusedImports(ast, info, filePath, []byte(source))
	if len(findings) != 0 {
		t.Fatalf("Expected 0 findings for `import type` line, got %d", len(findings))
	}
}

func TestDetectUnusedImports_NilInputs(t *testing.T) {
	findings := DetectUnusedImports(nil, nil, "test.js", nil)
	if findings != nil {
		t.Errorf("Expected nil findings for nil inputs, got %d", len(findings))
	}
}

func TestDetectUnusedImports_NoImports(t *testing.T) {
	source := `
const x = 1;
const y = 2;
`
	ast, info := parseAndAnalyze(t, source)
	findings := DetectUnusedImports(ast, info, "test.js", []byte(source))

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings when no imports, got %d", len(findings))
	}
}

// --- Unused Export Tests ---

func TestDetectUnusedExports_AllImported(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/utils.js": {
			FilePath: "/src/utils.js",
			Exports: []*domain.Export{
				{
					ExportType: "declaration",
					Name:       "helper",
					Location:   domain.SourceLocation{StartLine: 1, EndLine: 1},
				},
			},
		},
		"/src/app.js": {
			FilePath: "/src/app.js",
			Imports: []*domain.Import{
				{
					Source:     "./utils",
					SourceType: domain.ModuleTypeRelative,
					ImportType: domain.ImportTypeNamed,
					Specifiers: []domain.ImportSpecifier{
						{Imported: "helper", Local: "helper"},
					},
				},
			},
		},
	}
	analyzedFiles := map[string]bool{
		"/src/utils.js": true,
		"/src/app.js":   true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExports(allInfos, graph)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings when export is imported, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  finding: %s", f.Description)
		}
	}
}

func TestDetectUnusedExports_NeverImported(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/utils.js": {
			FilePath: "/src/utils.js",
			Exports: []*domain.Export{
				{
					ExportType: "declaration",
					Name:       "unusedHelper",
					Location:   domain.SourceLocation{StartLine: 5, EndLine: 5},
				},
			},
		},
		"/src/app.js": {
			FilePath: "/src/app.js",
			Imports:  []*domain.Import{},
		},
	}
	analyzedFiles := map[string]bool{
		"/src/utils.js": true,
		"/src/app.js":   true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExports(allInfos, graph)

	if len(findings) != 1 {
		t.Fatalf("Expected 1 finding for unused export, got %d", len(findings))
	}

	if findings[0].Reason != ReasonUnusedExport {
		t.Errorf("Expected reason %s, got %s", ReasonUnusedExport, findings[0].Reason)
	}
	if findings[0].Severity != SeverityLevelInfo {
		t.Errorf("Expected severity info, got %s", findings[0].Severity)
	}
}

func TestDetectUnusedExports_ReExportSkipped(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/utils.js": {
			FilePath: "/src/utils.js",
			Exports: []*domain.Export{
				{
					ExportType: "named",
					Source:     "./other", // re-export
					Name:       "foo",
					Specifiers: []domain.ExportSpecifier{
						{Local: "foo", Exported: "foo"},
					},
					Location: domain.SourceLocation{StartLine: 1, EndLine: 1},
				},
			},
		},
	}
	analyzedFiles := map[string]bool{
		"/src/utils.js": true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExports(allInfos, graph)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for re-export, got %d", len(findings))
	}
}

func TestDetectUnusedExports_IndexFileSkipped(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/index.js": {
			FilePath: "/src/index.js",
			Exports: []*domain.Export{
				{
					ExportType: "declaration",
					Name:       "App",
					Location:   domain.SourceLocation{StartLine: 1, EndLine: 1},
				},
			},
		},
	}
	analyzedFiles := map[string]bool{
		"/src/index.js": true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExports(allInfos, graph)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for index file exports, got %d", len(findings))
	}
}

func TestDetectUnusedExports_TestFileSkipped(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/utils.test.js": {
			FilePath: "/src/utils.test.js",
			Exports: []*domain.Export{
				{
					ExportType: "declaration",
					Name:       "testHelper",
					Location:   domain.SourceLocation{StartLine: 1, EndLine: 1},
				},
			},
		},
	}
	analyzedFiles := map[string]bool{
		"/src/utils.test.js": true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExports(allInfos, graph)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for test file exports, got %d", len(findings))
	}
}

func TestDetectUnusedExports_SpecFileSkipped(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/utils.spec.ts": {
			FilePath: "/src/utils.spec.ts",
			Exports: []*domain.Export{
				{
					ExportType: "declaration",
					Name:       "testHelper",
					Location:   domain.SourceLocation{StartLine: 1, EndLine: 1},
				},
			},
		},
	}
	analyzedFiles := map[string]bool{
		"/src/utils.spec.ts": true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExports(allInfos, graph)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for spec file exports, got %d", len(findings))
	}
}

func TestDetectUnusedExports_NilInput(t *testing.T) {
	graph := BuildImportGraph(nil, nil)
	findings := DetectUnusedExports(nil, graph)
	if findings != nil {
		t.Errorf("Expected nil findings for nil input, got %d", len(findings))
	}
}

// --- Path Resolution Tests ---

func TestResolveImportPath_BasicResolution(t *testing.T) {
	knownFiles := map[string]bool{
		"/src/utils.js": true,
		"/src/app.js":   true,
	}

	resolved := resolveImportPath("/src/app.js", "./utils", knownFiles)
	if resolved != "/src/utils.js" {
		t.Errorf("Expected /src/utils.js, got %s", resolved)
	}
}

func TestResolveImportPath_WithExtension(t *testing.T) {
	knownFiles := map[string]bool{
		"/src/utils.ts": true,
	}

	resolved := resolveImportPath("/src/app.ts", "./utils", knownFiles)
	if resolved != "/src/utils.ts" {
		t.Errorf("Expected /src/utils.ts, got %s", resolved)
	}
}

func TestResolveImportPath_IndexFile(t *testing.T) {
	knownFiles := map[string]bool{
		"/src/components/index.ts": true,
	}

	resolved := resolveImportPath("/src/app.ts", "./components", knownFiles)
	if resolved != "/src/components/index.ts" {
		t.Errorf("Expected /src/components/index.ts, got %s", resolved)
	}
}

func TestResolveImportPath_ParentDirectory(t *testing.T) {
	knownFiles := map[string]bool{
		"/src/utils.js": true,
	}

	resolved := resolveImportPath("/src/sub/app.js", "../utils", knownFiles)
	if resolved != "/src/utils.js" {
		t.Errorf("Expected /src/utils.js, got %s", resolved)
	}
}

func TestResolveImportPath_NonRelative(t *testing.T) {
	knownFiles := map[string]bool{
		"/node_modules/react/index.js": true,
	}

	resolved := resolveImportPath("/src/app.js", "react", knownFiles)
	if resolved != "" {
		t.Errorf("Expected empty string for non-relative import, got %s", resolved)
	}
}

func TestResolveImportPath_NotFound(t *testing.T) {
	knownFiles := map[string]bool{
		"/src/app.js": true,
	}

	resolved := resolveImportPath("/src/app.js", "./nonexistent", knownFiles)
	if resolved != "" {
		t.Errorf("Expected empty string for unresolved path, got %s", resolved)
	}
}

// --- Entry Point / Test File Helpers ---

func TestIsEntryPointFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/src/index.ts", true},
		{"/src/index.js", true},
		{"/src/index.tsx", true},
		{"/src/main.ts", true},
		{"/src/app.js", true},
		{"/src/server.js", true},
		{"/src/utils.ts", false},
		{"/src/helper.js", false},
	}

	for _, tc := range tests {
		result := isEntryPointFile(tc.path)
		if result != tc.expected {
			t.Errorf("isEntryPointFile(%q) = %v, want %v", tc.path, result, tc.expected)
		}
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/src/utils.test.ts", true},
		{"/src/utils.spec.js", true},
		{"/src/__tests__/utils.js", true},
		{"/src/utils.ts", false},
		{"/src/app.js", false},
	}

	for _, tc := range tests {
		result := isTestFile(tc.path)
		if result != tc.expected {
			t.Errorf("isTestFile(%q) = %v, want %v", tc.path, result, tc.expected)
		}
	}
}

// --- Integration-style Test ---

func TestDetectUnusedExports_DefaultExport(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/component.js": {
			FilePath: "/src/component.js",
			Exports: []*domain.Export{
				{
					ExportType: "default",
					Name:       "MyComponent",
					Location:   domain.SourceLocation{StartLine: 10, EndLine: 10},
				},
			},
		},
		"/src/app.js": {
			FilePath: "/src/app.js",
			Imports: []*domain.Import{
				{
					Source:     "./component",
					SourceType: domain.ModuleTypeRelative,
					ImportType: domain.ImportTypeDefault,
					Specifiers: []domain.ImportSpecifier{
						{Imported: "default", Local: "MyComponent"},
					},
				},
			},
		},
	}
	analyzedFiles := map[string]bool{
		"/src/component.js": true,
		"/src/app.js":       true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExports(allInfos, graph)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings when default export is imported, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  finding: %s", f.Description)
		}
	}
}

func TestDetectUnusedImports_ReExportedNotFlagged(t *testing.T) {
	source := `
import { foo } from './a';
export { foo };
`
	ast, info := parseAndAnalyze(t, source)
	findings := DetectUnusedImports(ast, info, "test.js", []byte(source))

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings when import is re-exported, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  finding: %s", f.Description)
		}
	}
}

func TestDetectUnusedImports_ExportDefaultUsesImport(t *testing.T) {
	source := `
import { foo } from './a';
export default foo;
`
	ast, info := parseAndAnalyze(t, source)
	findings := DetectUnusedImports(ast, info, "test.js", []byte(source))

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings when import is used in export default, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  finding: %s", f.Description)
		}
	}
}

func TestResolveImportPath_MjsExtension(t *testing.T) {
	knownFiles := map[string]bool{
		"/src/utils.mjs": true,
	}

	resolved := resolveImportPath("/src/app.js", "./utils", knownFiles)
	if resolved != "/src/utils.mjs" {
		t.Errorf("Expected /src/utils.mjs, got %q", resolved)
	}
}

func TestResolveImportPath_CtsExtension(t *testing.T) {
	knownFiles := map[string]bool{
		"/src/helper.cts": true,
	}

	resolved := resolveImportPath("/src/app.ts", "./helper", knownFiles)
	if resolved != "/src/helper.cts" {
		t.Errorf("Expected /src/helper.cts, got %q", resolved)
	}
}

func TestResolveImportPath_MtsExtension(t *testing.T) {
	knownFiles := map[string]bool{
		"/src/lib.mts": true,
	}

	resolved := resolveImportPath("/src/app.ts", "./lib", knownFiles)
	if resolved != "/src/lib.mts" {
		t.Errorf("Expected /src/lib.mts, got %q", resolved)
	}
}

// --- Orphan Files Tests ---

func TestDetectOrphanFiles_BasicOrphan(t *testing.T) {
	// A→B→C chain, D is orphan (no one imports it, it imports nothing from the chain)
	allInfos := map[string]*domain.ModuleInfo{
		"/src/app.js": {
			FilePath: "/src/app.js",
			Imports: []*domain.Import{
				{
					Source:     "./utils",
					SourceType: domain.ModuleTypeRelative,
					ImportType: domain.ImportTypeNamed,
					Specifiers: []domain.ImportSpecifier{
						{Imported: "helper", Local: "helper"},
					},
				},
			},
		},
		"/src/utils.js": {
			FilePath: "/src/utils.js",
			Imports: []*domain.Import{
				{
					Source:     "./lib",
					SourceType: domain.ModuleTypeRelative,
					ImportType: domain.ImportTypeNamed,
					Specifiers: []domain.ImportSpecifier{
						{Imported: "doStuff", Local: "doStuff"},
					},
				},
			},
		},
		"/src/lib.js": {
			FilePath: "/src/lib.js",
		},
		"/src/orphan.js": {
			FilePath: "/src/orphan.js",
			Imports: []*domain.Import{
				{
					Source:     "./orphan-dep",
					SourceType: domain.ModuleTypeRelative,
					ImportType: domain.ImportTypeNamed,
					Specifiers: []domain.ImportSpecifier{
						{Imported: "x", Local: "x"},
					},
				},
			},
		},
		"/src/orphan-dep.js": {
			FilePath: "/src/orphan-dep.js",
		},
	}
	analyzedFiles := map[string]bool{
		"/src/app.js":        true,
		"/src/utils.js":      true,
		"/src/lib.js":        true,
		"/src/orphan.js":     true,
		"/src/orphan-dep.js": true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectOrphanFiles(allInfos, graph)

	// app.js is a root (entry point by name), utils.js and lib.js are reachable from app.
	// orphan.js is also a root (no one imports it), orphan-dep.js is reachable from orphan.
	// So there should be no orphans (all files are reachable from some root).
	if len(findings) != 0 {
		t.Errorf("Expected 0 orphan findings (all reachable from roots), got %d", len(findings))
		for _, f := range findings {
			t.Logf("  finding: %s - %s", f.FilePath, f.Description)
		}
	}
}

func TestDetectOrphanFiles_EntryPointNotOrphan(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/index.js": {
			FilePath: "/src/index.js",
		},
		"/src/main.js": {
			FilePath: "/src/main.js",
		},
		"/src/server.ts": {
			FilePath: "/src/server.ts",
		},
	}
	analyzedFiles := map[string]bool{
		"/src/index.js":  true,
		"/src/main.js":   true,
		"/src/server.ts": true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectOrphanFiles(allInfos, graph)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for entry point files, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  finding: %s", f.Description)
		}
	}
}

func TestDetectOrphanFiles_TestFileSkipped(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/index.js": {
			FilePath: "/src/index.js",
		},
		"/src/utils.test.js": {
			FilePath: "/src/utils.test.js",
		},
	}
	analyzedFiles := map[string]bool{
		"/src/index.js":      true,
		"/src/utils.test.js": true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectOrphanFiles(allInfos, graph)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings when only test files are unreachable, got %d", len(findings))
	}
}

func TestDetectOrphanFiles_ConfigFileSkipped(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/index.js": {
			FilePath: "/src/index.js",
		},
		"/jest.config.js": {
			FilePath: "/jest.config.js",
		},
		"/vitest.setup.ts": {
			FilePath: "/vitest.setup.ts",
		},
	}
	analyzedFiles := map[string]bool{
		"/src/index.js":    true,
		"/jest.config.js":  true,
		"/vitest.setup.ts": true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectOrphanFiles(allInfos, graph)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for config/setup files, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  finding: %s", f.Description)
		}
	}
}

func TestDetectOrphanFiles_AllConnected(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/index.js": {
			FilePath: "/src/index.js",
			Imports: []*domain.Import{
				{
					Source:     "./app",
					SourceType: domain.ModuleTypeRelative,
					ImportType: domain.ImportTypeDefault,
					Specifiers: []domain.ImportSpecifier{
						{Imported: "default", Local: "App"},
					},
				},
			},
		},
		"/src/app.js": {
			FilePath: "/src/app.js",
			Imports: []*domain.Import{
				{
					Source:     "./utils",
					SourceType: domain.ModuleTypeRelative,
					ImportType: domain.ImportTypeNamed,
					Specifiers: []domain.ImportSpecifier{
						{Imported: "helper", Local: "helper"},
					},
				},
			},
		},
		"/src/utils.js": {
			FilePath: "/src/utils.js",
		},
	}
	analyzedFiles := map[string]bool{
		"/src/index.js": true,
		"/src/app.js":   true,
		"/src/utils.js": true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectOrphanFiles(allInfos, graph)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings when all files are connected, got %d", len(findings))
	}
}

func TestDetectOrphanFiles_NilInput(t *testing.T) {
	graph := BuildImportGraph(nil, nil)
	findings := DetectOrphanFiles(nil, graph)
	if findings != nil {
		t.Errorf("Expected nil findings for nil input, got %d", len(findings))
	}
}

// --- Unused Exported Functions Tests ---

func TestDetectUnusedExportedFunctions_UnusedFunction(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/utils.js": {
			FilePath: "/src/utils.js",
			Exports: []*domain.Export{
				{
					ExportType:  "declaration",
					Declaration: "FunctionDeclaration",
					Name:        "unusedFunc",
					Location:    domain.SourceLocation{StartLine: 1, EndLine: 3},
				},
			},
		},
		"/src/app.js": {
			FilePath: "/src/app.js",
			Imports:  []*domain.Import{},
		},
	}
	analyzedFiles := map[string]bool{
		"/src/utils.js": true,
		"/src/app.js":   true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExportedFunctions(allInfos, graph)

	if len(findings) != 1 {
		t.Fatalf("Expected 1 finding for unused exported function, got %d", len(findings))
	}
	if findings[0].Reason != ReasonUnusedExportedFunction {
		t.Errorf("Expected reason %s, got %s", ReasonUnusedExportedFunction, findings[0].Reason)
	}
	if findings[0].Severity != SeverityLevelWarning {
		t.Errorf("Expected severity warning, got %s", findings[0].Severity)
	}
}

func TestDetectUnusedExportedFunctions_UsedFunction(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/utils.js": {
			FilePath: "/src/utils.js",
			Exports: []*domain.Export{
				{
					ExportType:  "declaration",
					Declaration: "FunctionDeclaration",
					Name:        "usedFunc",
					Location:    domain.SourceLocation{StartLine: 1, EndLine: 3},
				},
			},
		},
		"/src/app.js": {
			FilePath: "/src/app.js",
			Imports: []*domain.Import{
				{
					Source:     "./utils",
					SourceType: domain.ModuleTypeRelative,
					ImportType: domain.ImportTypeNamed,
					Specifiers: []domain.ImportSpecifier{
						{Imported: "usedFunc", Local: "usedFunc"},
					},
				},
			},
		},
	}
	analyzedFiles := map[string]bool{
		"/src/utils.js": true,
		"/src/app.js":   true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExportedFunctions(allInfos, graph)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings when exported function is imported, got %d", len(findings))
	}
}

func TestDetectUnusedExportedFunctions_ConstNotTargeted(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/utils.js": {
			FilePath: "/src/utils.js",
			Exports: []*domain.Export{
				{
					ExportType:  "declaration",
					Declaration: "const",
					Name:        "MY_CONST",
					Location:    domain.SourceLocation{StartLine: 1, EndLine: 1},
				},
			},
		},
		"/src/app.js": {
			FilePath: "/src/app.js",
			Imports:  []*domain.Import{},
		},
	}
	analyzedFiles := map[string]bool{
		"/src/utils.js": true,
		"/src/app.js":   true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExportedFunctions(allInfos, graph)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for const export (not function/class), got %d", len(findings))
	}
}

func TestDetectUnusedExportedFunctions_EntryPointSkipped(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/index.js": {
			FilePath: "/src/index.js",
			Exports: []*domain.Export{
				{
					ExportType:  "declaration",
					Declaration: "FunctionDeclaration",
					Name:        "main",
					Location:    domain.SourceLocation{StartLine: 1, EndLine: 3},
				},
			},
		},
	}
	analyzedFiles := map[string]bool{
		"/src/index.js": true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExportedFunctions(allInfos, graph)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for entry point file exports, got %d", len(findings))
	}
}

func TestDetectUnusedExportedFunctions_TestFileSkipped(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/utils.test.js": {
			FilePath: "/src/utils.test.js",
			Exports: []*domain.Export{
				{
					ExportType:  "declaration",
					Declaration: "FunctionDeclaration",
					Name:        "testHelper",
					Location:    domain.SourceLocation{StartLine: 1, EndLine: 1},
				},
			},
		},
	}
	analyzedFiles := map[string]bool{
		"/src/utils.test.js": true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExportedFunctions(allInfos, graph)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings for test file exports, got %d", len(findings))
	}
}

func TestDetectUnusedExportedFunctions_DefaultExportFunction(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/component.js": {
			FilePath: "/src/component.js",
			Exports: []*domain.Export{
				{
					ExportType:  "default",
					Declaration: "FunctionDeclaration",
					Name:        "MyComponent",
					Location:    domain.SourceLocation{StartLine: 1, EndLine: 10},
				},
			},
		},
		"/src/app.js": {
			FilePath: "/src/app.js",
			Imports:  []*domain.Import{},
		},
	}
	analyzedFiles := map[string]bool{
		"/src/component.js": true,
		"/src/app.js":       true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExportedFunctions(allInfos, graph)

	if len(findings) != 1 {
		t.Fatalf("Expected 1 finding for unused default export function, got %d", len(findings))
	}
	if findings[0].Reason != ReasonUnusedExportedFunction {
		t.Errorf("Expected reason %s, got %s", ReasonUnusedExportedFunction, findings[0].Reason)
	}
}

func TestDetectUnusedExportedFunctions_ClassExport(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/service.js": {
			FilePath: "/src/service.js",
			Exports: []*domain.Export{
				{
					ExportType:  "declaration",
					Declaration: "ClassDeclaration",
					Name:        "UserService",
					Location:    domain.SourceLocation{StartLine: 1, EndLine: 20},
				},
			},
		},
		"/src/app.js": {
			FilePath: "/src/app.js",
			Imports:  []*domain.Import{},
		},
	}
	analyzedFiles := map[string]bool{
		"/src/service.js": true,
		"/src/app.js":     true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExportedFunctions(allInfos, graph)

	if len(findings) != 1 {
		t.Fatalf("Expected 1 finding for unused exported class, got %d", len(findings))
	}
}

func TestDetectUnusedExportedFunctions_NilInput(t *testing.T) {
	graph := BuildImportGraph(nil, nil)
	findings := DetectUnusedExportedFunctions(nil, graph)
	if findings != nil {
		t.Errorf("Expected nil findings for nil input, got %d", len(findings))
	}
}

// --- Config File Helper Tests ---

func TestIsConfigFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/jest.config.js", true},
		{"/vitest.config.ts", true},
		{"/vitest.setup.ts", true},
		{"/src/utils.js", false},
		{"/src/app.js", false},
		{"/webpack.config.js", true},
		{"/babel.config.json", true},
	}

	for _, tc := range tests {
		result := isConfigFile(tc.path)
		if result != tc.expected {
			t.Errorf("isConfigFile(%q) = %v, want %v", tc.path, result, tc.expected)
		}
	}
}

func TestDetectUnusedExports_NamespaceImportCoversAll(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/src/utils.js": {
			FilePath: "/src/utils.js",
			Exports: []*domain.Export{
				{
					ExportType: "declaration",
					Name:       "foo",
					Location:   domain.SourceLocation{StartLine: 1, EndLine: 1},
				},
				{
					ExportType: "declaration",
					Name:       "bar",
					Location:   domain.SourceLocation{StartLine: 2, EndLine: 2},
				},
			},
		},
		"/src/app.js": {
			FilePath: "/src/app.js",
			Imports: []*domain.Import{
				{
					Source:     "./utils",
					SourceType: domain.ModuleTypeRelative,
					ImportType: domain.ImportTypeNamespace,
					Specifiers: []domain.ImportSpecifier{
						{Imported: "*", Local: "utils"},
					},
				},
			},
		},
	}
	analyzedFiles := map[string]bool{
		"/src/utils.js": true,
		"/src/app.js":   true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExports(allInfos, graph)

	if len(findings) != 0 {
		t.Errorf("Expected 0 findings when namespace import covers all exports, got %d", len(findings))
	}
}

func TestDetectUnusedExports_AliasImportResolvesTarget(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/repo/src/utils/math.ts": {
			FilePath: "/repo/src/utils/math.ts",
			Exports: []*domain.Export{
				{
					ExportType: "declaration",
					Name:       "sum",
					Location:   domain.SourceLocation{StartLine: 1, EndLine: 1},
				},
			},
		},
		"/repo/src/app.ts": {
			FilePath: "/repo/src/app.ts",
			Imports: []*domain.Import{
				{
					Source:     "@/utils/math",
					SourceType: domain.ModuleTypeAlias,
					ImportType: domain.ImportTypeNamed,
					Specifiers: []domain.ImportSpecifier{
						{Imported: "sum", Local: "sum"},
					},
				},
			},
		},
	}
	analyzedFiles := map[string]bool{
		"/repo/src/utils/math.ts": true,
		"/repo/src/app.ts":        true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExports(allInfos, graph)
	if len(findings) != 0 {
		t.Errorf("Expected 0 findings when export is imported through alias, got %d", len(findings))
	}
}

func TestDetectUnusedExportedFunctions_AliasImportResolvesTarget(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/repo/src/service/user.ts": {
			FilePath: "/repo/src/service/user.ts",
			Exports: []*domain.Export{
				{
					ExportType:  "declaration",
					Declaration: "FunctionDeclaration",
					Name:        "loadUser",
					Location:    domain.SourceLocation{StartLine: 1, EndLine: 3},
				},
			},
		},
		"/repo/src/app.ts": {
			FilePath: "/repo/src/app.ts",
			Imports: []*domain.Import{
				{
					Source:     "@/service/user",
					SourceType: domain.ModuleTypeAlias,
					ImportType: domain.ImportTypeNamed,
					Specifiers: []domain.ImportSpecifier{
						{Imported: "loadUser", Local: "loadUser"},
					},
				},
			},
		},
	}
	analyzedFiles := map[string]bool{
		"/repo/src/service/user.ts": true,
		"/repo/src/app.ts":          true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExportedFunctions(allInfos, graph)
	if len(findings) != 0 {
		t.Errorf("Expected 0 findings when exported function is imported through alias, got %d", len(findings))
	}
}

func TestDetectUnusedExportedFunctions_NextPageReservedExportsSkipped(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/repo/src/app/scan/[id]/page.tsx": {
			FilePath: "/repo/src/app/scan/[id]/page.tsx",
			Exports: []*domain.Export{
				{
					ExportType:  "declaration",
					Declaration: "AsyncFunctionDeclaration",
					Name:        "generateMetadata",
					Location:    domain.SourceLocation{StartLine: 8, EndLine: 43},
				},
				{
					ExportType:  "default",
					Declaration: "AsyncFunctionDeclaration",
					Name:        "ScanPage",
					Location:    domain.SourceLocation{StartLine: 45, EndLine: 59},
				},
			},
		},
	}
	analyzedFiles := map[string]bool{
		"/repo/src/app/scan/[id]/page.tsx": true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExportedFunctions(allInfos, graph)
	if len(findings) != 0 {
		t.Fatalf("Expected 0 findings for Next.js reserved page exports, got %d", len(findings))
	}
}

func TestDetectUnusedExports_NextPageDefaultExportSkipped(t *testing.T) {
	allInfos := map[string]*domain.ModuleInfo{
		"/repo/src/app/page.tsx": {
			FilePath: "/repo/src/app/page.tsx",
			Exports: []*domain.Export{
				{
					ExportType: "default",
					Name:       "Home",
					Location:   domain.SourceLocation{StartLine: 3, EndLine: 34},
				},
			},
		},
	}
	analyzedFiles := map[string]bool{
		"/repo/src/app/page.tsx": true,
	}

	graph := BuildImportGraph(allInfos, analyzedFiles)
	findings := DetectUnusedExports(allInfos, graph)
	if len(findings) != 0 {
		t.Fatalf("Expected 0 findings for Next.js page default export, got %d", len(findings))
	}
}
