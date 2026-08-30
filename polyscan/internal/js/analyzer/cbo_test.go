package analyzer

import (
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

func TestDefaultCBOAnalyzerConfig(t *testing.T) {
	config := DefaultCBOAnalyzerConfig()

	if config.IncludeBuiltins {
		t.Error("Expected IncludeBuiltins to be false by default")
	}
	if !config.IncludeTypeImports {
		t.Error("Expected IncludeTypeImports to be true by default")
	}
	if config.LowThreshold != 7 {
		t.Errorf("Expected LowThreshold to be 7, got %d", config.LowThreshold)
	}
	if config.MediumThreshold != 14 {
		t.Errorf("Expected MediumThreshold to be 14, got %d", config.MediumThreshold)
	}
}

func TestNewCBOAnalyzer(t *testing.T) {
	// Test with nil config
	analyzer := NewCBOAnalyzer(nil)
	if analyzer == nil {
		t.Fatal("Expected non-nil analyzer")
	}

	// Test with custom config
	config := &CBOAnalyzerConfig{
		IncludeBuiltins: true,
		LowThreshold:    5,
	}
	analyzer = NewCBOAnalyzer(config)
	if analyzer == nil {
		t.Fatal("Expected non-nil analyzer")
	}
}

func TestCBOImportDependencies(t *testing.T) {
	source := `
import React from 'react';
import { useState, useEffect } from 'react';
import lodash from 'lodash';
import { helper } from './utils';
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewCBOAnalyzer(DefaultCBOAnalyzerConfig())
	result, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// Should have 3 unique import dependencies (react, lodash, utils)
	if result.Metrics.ImportDependencies != 3 {
		t.Errorf("Expected 3 import dependencies, got %d", result.Metrics.ImportDependencies)
	}

	// CouplingCount should equal import dependencies in this case
	if result.Metrics.CouplingCount != 3 {
		t.Errorf("Expected CouplingCount 3, got %d", result.Metrics.CouplingCount)
	}
}

func TestCBOInstantiationDependencies(t *testing.T) {
	source := `
import { UserService } from './user-service';
import { Logger } from './logger';

const userService = new UserService();
const logger = new Logger();
const anotherUser = new UserService();
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewCBOAnalyzer(DefaultCBOAnalyzerConfig())
	result, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// Should have 2 unique instantiation dependencies (UserService, Logger)
	if result.Metrics.InstantiationDependencies != 2 {
		t.Errorf("Expected 2 instantiation dependencies, got %d", result.Metrics.InstantiationDependencies)
	}
}

func TestCBOBuiltinClassesNotCounted(t *testing.T) {
	source := `
const arr = new Array();
const date = new Date();
const map = new Map();
const promise = new Promise((resolve) => resolve());
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewCBOAnalyzer(DefaultCBOAnalyzerConfig())
	result, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// Builtin classes should not be counted
	if result.Metrics.InstantiationDependencies != 0 {
		t.Errorf("Expected 0 instantiation dependencies for builtins, got %d", result.Metrics.InstantiationDependencies)
	}
}

func TestCBORiskLevelLow(t *testing.T) {
	source := `
import { helper } from './utils';
import React from 'react';
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewCBOAnalyzer(DefaultCBOAnalyzerConfig())
	result, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// With only 2 dependencies, risk should be low
	if result.RiskLevel != domain.RiskLevelLow {
		t.Errorf("Expected risk level Low, got %v", result.RiskLevel)
	}
}

func TestCBORiskLevelMedium(t *testing.T) {
	source := `
import a from 'a';
import b from 'b';
import c from 'c';
import d from 'd';
import e from 'e';
import f from 'f';
import g from 'g';
import h from 'h';
import i from 'i';
import j from 'j';
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewCBOAnalyzer(DefaultCBOAnalyzerConfig())
	result, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// With 10 dependencies (> 7, <= 14), risk should be medium
	if result.RiskLevel != domain.RiskLevelMedium {
		t.Errorf("Expected risk level Medium, got %v (CBO: %d)", result.RiskLevel, result.Metrics.CouplingCount)
	}
}

func TestCBORiskLevelHigh(t *testing.T) {
	source := `
import a from 'a';
import b from 'b';
import c from 'c';
import d from 'd';
import e from 'e';
import f from 'f';
import g from 'g';
import h from 'h';
import i from 'i';
import j from 'j';
import k from 'k';
import l from 'l';
import m from 'm';
import n from 'n';
import o from 'o';
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewCBOAnalyzer(DefaultCBOAnalyzerConfig())
	result, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// With 15 dependencies (> 14), risk should be high
	if result.RiskLevel != domain.RiskLevelHigh {
		t.Errorf("Expected risk level High, got %v (CBO: %d)", result.RiskLevel, result.Metrics.CouplingCount)
	}
}

func TestCBOExcludeBuiltins(t *testing.T) {
	source := `
import fs from 'fs';
import path from 'path';
import React from 'react';
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// With builtins excluded (default)
	analyzer := NewCBOAnalyzer(DefaultCBOAnalyzerConfig())
	result, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// Only 'react' should be counted (fs and path are builtins)
	if result.Metrics.ImportDependencies != 1 {
		t.Errorf("Expected 1 import dependency (excluding builtins), got %d", result.Metrics.ImportDependencies)
	}
}

func TestCBOIncludeBuiltins(t *testing.T) {
	source := `
import fs from 'fs';
import path from 'path';
import React from 'react';
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// With builtins included
	config := &CBOAnalyzerConfig{
		IncludeBuiltins:    true,
		IncludeTypeImports: true,
		LowThreshold:       3,
		MediumThreshold:    7,
	}
	analyzer := NewCBOAnalyzer(config)
	result, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// All 3 should be counted
	if result.Metrics.ImportDependencies != 3 {
		t.Errorf("Expected 3 import dependencies (including builtins), got %d", result.Metrics.ImportDependencies)
	}
}

func TestCBOEmptyFile(t *testing.T) {
	source := ``

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewCBOAnalyzer(DefaultCBOAnalyzerConfig())
	result, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	if result.Metrics.CouplingCount != 0 {
		t.Errorf("Expected CouplingCount 0 for empty file, got %d", result.Metrics.CouplingCount)
	}
	if result.RiskLevel != domain.RiskLevelLow {
		t.Errorf("Expected risk level Low for empty file, got %v", result.RiskLevel)
	}
}

func TestCBONilAST(t *testing.T) {
	analyzer := NewCBOAnalyzer(DefaultCBOAnalyzerConfig())
	result, err := analyzer.AnalyzeFile(nil, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	if result.FilePath != "test.js" {
		t.Errorf("Expected file path 'test.js', got %q", result.FilePath)
	}
	if result.Metrics.CouplingCount != 0 {
		t.Errorf("Expected CouplingCount 0 for nil AST, got %d", result.Metrics.CouplingCount)
	}
}

func TestCBODependentClassesSorted(t *testing.T) {
	source := `
import z from 'z-package';
import a from 'a-package';
import m from 'm-package';
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewCBOAnalyzer(DefaultCBOAnalyzerConfig())
	result, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// DependentClasses should be sorted alphabetically
	classes := result.Metrics.DependentClasses
	if len(classes) >= 2 {
		for i := 1; i < len(classes); i++ {
			if classes[i-1] > classes[i] {
				t.Errorf("DependentClasses not sorted: %v", classes)
				break
			}
		}
	}
}

func TestExtractModuleName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"/path/to/file.js", "file"},
		{"/path/to/file.ts", "file"},
		{"file.js", "file"},
		{"path/file.tsx", "file"},
		{"index", "index"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := extractModuleName(tc.input)
			if result != tc.expected {
				t.Errorf("extractModuleName(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestNormalizeModuleName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"./utils", "utils"},
		{"../lib/helper", "lib"},
		{"react", "react"},
		{"@types/node", "@types/node"},
		{"lodash/debounce", "lodash"},
		{"@company/shared-lib/utils", "@company/shared-lib"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := normalizeModuleName(tc.input)
			if result != tc.expected {
				t.Errorf("normalizeModuleName(%q) = %q, want %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestIsBuiltinClass(t *testing.T) {
	builtins := []string{"Array", "Object", "Map", "Set", "Promise", "Date", "Error"}
	for _, name := range builtins {
		if !isBuiltinClass(name) {
			t.Errorf("Expected %q to be a builtin class", name)
		}
	}

	nonBuiltins := []string{"MyClass", "UserService", "CustomError", "AppState"}
	for _, name := range nonBuiltins {
		if isBuiltinClass(name) {
			t.Errorf("Expected %q to NOT be a builtin class", name)
		}
	}
}

func TestIsBuiltinObject(t *testing.T) {
	builtins := []string{"console", "process", "JSON", "Math", "window", "document"}
	for _, name := range builtins {
		if !isBuiltinObject(name) {
			t.Errorf("Expected %q to be a builtin object", name)
		}
	}

	nonBuiltins := []string{"myService", "userManager", "appConfig"}
	for _, name := range nonBuiltins {
		if isBuiltinObject(name) {
			t.Errorf("Expected %q to NOT be a builtin object", name)
		}
	}
}

func TestIsPrimitiveType(t *testing.T) {
	primitives := []string{"string", "number", "boolean", "void", "null", "undefined", "any", "unknown"}
	for _, name := range primitives {
		if !isPrimitiveType(name) {
			t.Errorf("Expected %q to be a primitive type", name)
		}
	}

	nonPrimitives := []string{"MyType", "UserInterface", "CustomClass"}
	for _, name := range nonPrimitives {
		if isPrimitiveType(name) {
			t.Errorf("Expected %q to NOT be a primitive type", name)
		}
	}
}

func TestCalculateRiskLevel(t *testing.T) {
	config := DefaultCBOAnalyzerConfig()
	analyzer := NewCBOAnalyzer(config)

	testCases := []struct {
		cbo      int
		expected domain.RiskLevel
	}{
		{0, domain.RiskLevelLow},
		{1, domain.RiskLevelLow},
		{7, domain.RiskLevelLow},
		{8, domain.RiskLevelMedium},
		{10, domain.RiskLevelMedium},
		{14, domain.RiskLevelMedium},
		{15, domain.RiskLevelHigh},
		{20, domain.RiskLevelHigh},
		{100, domain.RiskLevelHigh},
	}

	for _, tc := range testCases {
		result := analyzer.calculateRiskLevel(tc.cbo)
		if result != tc.expected {
			t.Errorf("calculateRiskLevel(%d) = %v, want %v", tc.cbo, result, tc.expected)
		}
	}
}

func TestCBOAttributeAccessDependencies(t *testing.T) {
	source := `
import userService from './user-service';
import logger from './logger';

function doSomething() {
    userService.getUser();
    userService.updateUser();
    logger.log('message');
}
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewCBOAnalyzer(DefaultCBOAnalyzerConfig())
	result, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// Should have 2 attribute access dependencies (user-service and logger)
	if result.Metrics.AttributeAccessDependencies != 2 {
		t.Errorf("Expected 2 attribute access dependencies, got %d", result.Metrics.AttributeAccessDependencies)
	}
}

func TestCBOBuiltinObjectsNotCounted(t *testing.T) {
	source := `
function doSomething() {
    console.log('message');
    JSON.parse('{}');
    Math.random();
}
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	analyzer := NewCBOAnalyzer(DefaultCBOAnalyzerConfig())
	result, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// Builtin objects should not be counted as dependencies
	if result.Metrics.CouplingCount != 0 {
		t.Errorf("Expected 0 coupling count for builtin objects only, got %d", result.Metrics.CouplingCount)
	}
}

func TestCBOCommonJSRequire(t *testing.T) {
	source := `
const fs = require('fs');
const lodash = require('lodash');
const utils = require('./utils');
`

	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	// Without builtins
	analyzer := NewCBOAnalyzer(DefaultCBOAnalyzerConfig())
	result, err := analyzer.AnalyzeFile(ast, "test.js")
	if err != nil {
		t.Fatalf("Failed to analyze: %v", err)
	}

	// Should have 2 dependencies (lodash and utils, excluding fs builtin)
	if result.Metrics.ImportDependencies != 2 {
		t.Errorf("Expected 2 import dependencies, got %d", result.Metrics.ImportDependencies)
	}
}
