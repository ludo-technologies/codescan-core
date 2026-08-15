package analyzer

import (
	"testing"

	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/internal/parser"
)

func TestDefaultModuleAnalyzerConfig(t *testing.T) {
	config := DefaultModuleAnalyzerConfig()

	if !config.IncludeBuiltins {
		t.Error("Expected IncludeBuiltins to be true by default")
	}
	if config.ResolveRelative {
		t.Error("Expected ResolveRelative to be false by default")
	}
	if !config.IncludeTypeImports {
		t.Error("Expected IncludeTypeImports to be true by default")
	}
	if len(config.AliasPatterns) != 2 {
		t.Errorf("Expected 2 default alias patterns, got %d", len(config.AliasPatterns))
	}
}

func TestNewModuleAnalyzer(t *testing.T) {
	// Test with nil config
	analyzer := NewModuleAnalyzer(nil)
	if analyzer == nil {
		t.Fatal("Expected non-nil analyzer")
	}

	// Test with custom config
	config := &ModuleAnalyzerConfig{
		IncludeBuiltins: false,
	}
	analyzer = NewModuleAnalyzer(config)
	if analyzer == nil {
		t.Fatal("Expected non-nil analyzer")
	}
}

func TestClassifyModuleSource(t *testing.T) {
	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())

	testCases := []struct {
		source   string
		expected domain.ModuleType
	}{
		// Relative imports
		{"./utils", domain.ModuleTypeRelative},
		{"../lib/helper", domain.ModuleTypeRelative},
		{"./components/Button", domain.ModuleTypeRelative},

		// Absolute imports
		{"/usr/lib/module", domain.ModuleTypeAbsolute},
		{"/home/user/project/lib", domain.ModuleTypeAbsolute},

		// Node.js builtins
		{"node:fs", domain.ModuleTypeBuiltin},
		{"node:path", domain.ModuleTypeBuiltin},
		{"fs", domain.ModuleTypeBuiltin},
		{"path", domain.ModuleTypeBuiltin},
		{"crypto", domain.ModuleTypeBuiltin},
		{"http", domain.ModuleTypeBuiltin},

		// Alias patterns
		{"@/components/Button", domain.ModuleTypeAlias},
		{"~/utils/helper", domain.ModuleTypeAlias},

		// Package imports
		{"react", domain.ModuleTypePackage},
		{"lodash", domain.ModuleTypePackage},
		{"@types/node", domain.ModuleTypePackage},
		{"lodash/debounce", domain.ModuleTypePackage},
		{"@company/shared-lib", domain.ModuleTypePackage},
	}

	for _, tc := range testCases {
		t.Run(tc.source, func(t *testing.T) {
			result := analyzer.classifyModuleSource(tc.source)
			if result != tc.expected {
				t.Errorf("classifyModuleSource(%q) = %v, want %v", tc.source, result, tc.expected)
			}
		})
	}
}

func TestES6DefaultImport(t *testing.T) {
	source := `import React from 'react';`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	if len(info.Imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(info.Imports))
	}

	imp := info.Imports[0]
	if imp.Source != "react" {
		t.Errorf("Expected source 'react', got %q", imp.Source)
	}
	if imp.SourceType != domain.ModuleTypePackage {
		t.Errorf("Expected source type 'package', got %v", imp.SourceType)
	}
	if imp.ImportType != domain.ImportTypeDefault {
		t.Errorf("Expected import type 'default', got %v", imp.ImportType)
	}
}

func TestES6NamedImports(t *testing.T) {
	source := `import { useState, useEffect } from 'react';`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	if len(info.Imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(info.Imports))
	}

	imp := info.Imports[0]
	if imp.Source != "react" {
		t.Errorf("Expected source 'react', got %q", imp.Source)
	}
	if imp.ImportType != domain.ImportTypeNamed {
		t.Errorf("Expected import type 'named', got %v", imp.ImportType)
	}
}

func TestES6NamespaceImport(t *testing.T) {
	source := `import * as React from 'react';`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	if len(info.Imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(info.Imports))
	}

	imp := info.Imports[0]
	if imp.Source != "react" {
		t.Errorf("Expected source 'react', got %q", imp.Source)
	}
	if imp.ImportType != domain.ImportTypeNamespace {
		t.Errorf("Expected import type 'namespace', got %v", imp.ImportType)
	}
}

func TestES6SideEffectImport(t *testing.T) {
	source := `import './styles.css';`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	if len(info.Imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(info.Imports))
	}

	imp := info.Imports[0]
	if imp.Source != "./styles.css" {
		t.Errorf("Expected source './styles.css', got %q", imp.Source)
	}
	if imp.SourceType != domain.ModuleTypeRelative {
		t.Errorf("Expected source type 'relative', got %v", imp.SourceType)
	}
	if imp.ImportType != domain.ImportTypeSideEffect {
		t.Errorf("Expected import type 'side_effect', got %v", imp.ImportType)
	}
}

func TestCommonJSRequire(t *testing.T) {
	source := `const fs = require('fs');`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	if len(info.Imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(info.Imports))
	}

	imp := info.Imports[0]
	if imp.Source != "fs" {
		t.Errorf("Expected source 'fs', got %q", imp.Source)
	}
	if imp.SourceType != domain.ModuleTypeBuiltin {
		t.Errorf("Expected source type 'builtin', got %v", imp.SourceType)
	}
	if imp.ImportType != domain.ImportTypeRequire {
		t.Errorf("Expected import type 'require', got %v", imp.ImportType)
	}
}

func TestDynamicImport(t *testing.T) {
	source := `const module = import('./lazy-module');`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	if len(info.Imports) != 1 {
		t.Fatalf("Expected 1 import, got %d", len(info.Imports))
	}

	imp := info.Imports[0]
	if !imp.IsDynamic {
		t.Error("Expected import to be dynamic")
	}
	if imp.ImportType != domain.ImportTypeDynamic {
		t.Errorf("Expected import type 'dynamic', got %v", imp.ImportType)
	}
	if imp.Source != "./lazy-module" {
		t.Errorf("Expected source './lazy-module', got %q", imp.Source)
	}
	if imp.SourceType != domain.ModuleTypeRelative {
		t.Errorf("Expected source type 'relative', got %v", imp.SourceType)
	}
}

func TestDynamicImportForms(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   []string
	}{
		{
			name:   "awaited",
			source: "async function load() { return await import('./awaited.js'); }",
			want:   []string{"./awaited.js"},
		},
		{
			name:   "returned from exported function",
			source: "export function b() { return import('./returned.js'); }",
			want:   []string{"./returned.js"},
		},
		{
			name:   "chained with then",
			source: "import('./chained.js').then(m => m.run());",
			want:   []string{"./chained.js"},
		},
		{
			name:   "template literal without substitution",
			source: "const m = import(`./template.js`);",
			want:   []string{"./template.js"},
		},
		{
			name:   "template literal with substitution is not resolvable",
			source: "const m = import(`./locales/${lang}.js`);",
			want:   nil,
		},
		{
			name:   "package specifier",
			source: "const m = import('lodash');",
			want:   []string{"lodash"},
		},
		{
			name:   "several in one file",
			source: "import('./one.js');\nimport('./two.js').catch(() => {});\n",
			want:   []string{"./one.js", "./two.js"},
		},
		{
			name:   "import.meta is not an import",
			source: "const url = import.meta.url;",
			want:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := parser.NewParser()
			defer p.Close()

			ast, err := p.ParseString(tt.source)
			if err != nil {
				t.Fatalf("Failed to parse: %v", err)
			}

			analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
			info, err := analyzer.AnalyzeFile(ast, "test.js")
			if err != nil {
				t.Fatalf("Failed to analyze: %v", err)
			}

			var got []string
			for _, imp := range info.Imports {
				if !imp.IsDynamic {
					t.Errorf("Expected every detected import to be dynamic, got %v for %q", imp.ImportType, imp.Source)
				}
				got = append(got, imp.Source)
			}

			if len(got) != len(tt.want) {
				t.Fatalf("Expected sources %v, got %v", tt.want, got)
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("Expected source %q at index %d, got %q", want, i, got[i])
				}
			}
		})
	}
}

func TestExportNamedDeclaration(t *testing.T) {
	source := `export function hello() { return 'world'; }`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	if len(info.Exports) != 1 {
		t.Fatalf("Expected 1 export, got %d", len(info.Exports))
	}

	exp := info.Exports[0]
	if exp.ExportType != "named" {
		t.Errorf("Expected export type 'named', got %q", exp.ExportType)
	}
}

func TestExportDefaultDeclaration(t *testing.T) {
	source := `export default function() { return 'world'; }`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// Must have at least one export
	if len(info.Exports) == 0 {
		t.Fatal("Expected at least 1 export, got 0")
	}

	// Check that a default export is found
	foundDefault := false
	for _, exp := range info.Exports {
		if exp.ExportType == "default" {
			foundDefault = true
			break
		}
	}

	if !foundDefault {
		t.Errorf("Expected to find default export, got: %+v", info.Exports)
	}
}

func TestReExport(t *testing.T) {
	source := `export { foo, bar } from './module';`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	if len(info.Exports) < 1 {
		t.Fatalf("Expected at least 1 export, got %d", len(info.Exports))
	}

	exp := info.Exports[0]
	if exp.Source != "./module" {
		t.Errorf("Expected re-export source './module', got %q", exp.Source)
	}
}

func TestExportAll(t *testing.T) {
	source := `export * from './utils';`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// Must have at least one export
	if len(info.Exports) == 0 {
		t.Fatal("Expected at least 1 export, got 0")
	}

	foundAll := false
	for _, exp := range info.Exports {
		if exp.ExportType == "all" && exp.Source == "./utils" {
			foundAll = true
			break
		}
	}

	if !foundAll {
		t.Errorf("Expected to find 'export all' with source './utils', got: %+v", info.Exports)
	}
}

func TestModuleExports(t *testing.T) {
	source := `module.exports = { foo: 'bar' };`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// Must have at least one export
	if len(info.Exports) == 0 {
		t.Fatal("Expected at least 1 export, got 0")
	}

	foundModuleExports := false
	for _, exp := range info.Exports {
		if exp.Declaration == "module.exports" {
			foundModuleExports = true
			break
		}
	}

	if !foundModuleExports {
		t.Errorf("Expected to find 'module.exports' declaration, got: %+v", info.Exports)
	}
}

func TestMultipleImports(t *testing.T) {
	source := `
import React from 'react';
import { useState } from 'react';
import './styles.css';
const fs = require('fs');
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	if len(info.Imports) < 4 {
		t.Errorf("Expected at least 4 imports, got %d", len(info.Imports))
	}
}

func TestAnalyzeAll(t *testing.T) {
	p := parser.NewParser()
	defer p.Close()

	asts := make(map[string]*parser.Node)

	source1 := `import React from 'react';`
	ast1, _ := p.ParseString(source1)
	asts["file1.js"] = ast1

	source2 := `import { useState } from 'react'; import lodash from 'lodash';`
	ast2, _ := p.ParseString(source2)
	asts["file2.js"] = ast2

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	result, err := analyzer.AnalyzeAll(asts)
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	if result.Summary.TotalFiles != 2 {
		t.Errorf("Expected 2 total files, got %d", result.Summary.TotalFiles)
	}
	if result.Summary.TotalImports < 3 {
		t.Errorf("Expected at least 3 total imports, got %d", result.Summary.TotalImports)
	}
	if result.Summary.UniquePackages < 2 {
		t.Errorf("Expected at least 2 unique packages, got %d", result.Summary.UniquePackages)
	}
}

func TestDependencyBuilding(t *testing.T) {
	source := `
import React from 'react';
import { useState } from 'react';
import './styles.css';
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// Check that dependencies are deduped
	if len(info.Dependencies) > len(info.Imports) {
		t.Error("Dependencies should not exceed imports count (deduplication)")
	}

	// Check that dependencies have proper source types
	for _, dep := range info.Dependencies {
		if dep.Source == "" {
			t.Error("Dependency source should not be empty")
		}
		if dep.SourceType == "" {
			t.Error("Dependency source type should not be empty")
		}
	}
}

func TestRelativeImports(t *testing.T) {
	source := `
import { foo } from './utils';
import { bar } from '../lib/helper';
import { baz } from './components/Button';
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	relativeCount := 0
	for _, imp := range info.Imports {
		if imp.SourceType == domain.ModuleTypeRelative {
			relativeCount++
		}
	}

	if relativeCount != 3 {
		t.Errorf("Expected 3 relative imports, got %d", relativeCount)
	}
}

func TestAliasImports(t *testing.T) {
	source := `
import { Component } from '@/components/Button';
import { utils } from '~/utils/helper';
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	aliasCount := 0
	for _, imp := range info.Imports {
		if imp.SourceType == domain.ModuleTypeAlias {
			aliasCount++
		}
	}

	if aliasCount != 2 {
		t.Errorf("Expected 2 alias imports, got %d", aliasCount)
	}
}

func TestNodeBuiltinImports(t *testing.T) {
	source := `
import fs from 'fs';
import path from 'path';
import { createServer } from 'http';
import crypto from 'node:crypto';
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	builtinCount := 0
	for _, imp := range info.Imports {
		if imp.SourceType == domain.ModuleTypeBuiltin {
			builtinCount++
		}
	}

	if builtinCount != 4 {
		t.Errorf("Expected 4 builtin imports, got %d", builtinCount)
	}
}

func TestEmptyFile(t *testing.T) {
	source := ``

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	if len(info.Imports) != 0 {
		t.Errorf("Expected 0 imports for empty file, got %d", len(info.Imports))
	}
	if len(info.Exports) != 0 {
		t.Errorf("Expected 0 exports for empty file, got %d", len(info.Exports))
	}
}

func TestNilAST(t *testing.T) {
	analyzer := NewModuleAnalyzer(DefaultModuleAnalyzerConfig())
	info, err := analyzer.AnalyzeFile(nil, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	if info.FilePath != "test.js" {
		t.Errorf("Expected file path 'test.js', got %q", info.FilePath)
	}
	if len(info.Imports) != 0 {
		t.Errorf("Expected 0 imports for nil AST, got %d", len(info.Imports))
	}
}
