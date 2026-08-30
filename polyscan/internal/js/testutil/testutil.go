// Package testutil provides helper functions for testing jscan components
package testutil

import (
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

// CreateTestAST creates a test AST from JavaScript source code
func CreateTestAST(t *testing.T, source string) *parser.Node {
	t.Helper()
	p := parser.NewParser()
	defer p.Close()

	ast, err := p.ParseString(source)
	if err != nil {
		t.Fatalf("Failed to parse test code: %v", err)
	}
	return ast
}

// CreateTestASTNoFail creates a test AST, returning nil on error instead of failing
func CreateTestASTNoFail(source string) (*parser.Node, error) {
	p := parser.NewParser()
	defer p.Close()
	return p.ParseString(source)
}

// AssertNoError fails the test if err is not nil
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// AssertError fails the test if err is nil
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("Expected error but got nil")
	}
}

// AssertEqual fails the test if expected != actual
func AssertEqual(t *testing.T, expected, actual any) {
	t.Helper()
	if expected != actual {
		t.Errorf("Expected %v, got %v", expected, actual)
	}
}

// AssertTrue fails the test if condition is false
func AssertTrue(t *testing.T, condition bool, msg string) {
	t.Helper()
	if !condition {
		t.Error(msg)
	}
}

// AssertFalse fails the test if condition is true
func AssertFalse(t *testing.T, condition bool, msg string) {
	t.Helper()
	if condition {
		t.Error(msg)
	}
}

// AssertNotNil fails the test if value is nil
func AssertNotNil(t *testing.T, value any) {
	t.Helper()
	if value == nil {
		t.Error("Expected non-nil value")
	}
}

// AssertNil fails the test if value is not nil
func AssertNil(t *testing.T, value any) {
	t.Helper()
	if value != nil {
		t.Errorf("Expected nil, got %v", value)
	}
}

// FindFunctionInAST finds a function node by name in the AST
func FindFunctionInAST(ast *parser.Node, name string) *parser.Node {
	var found *parser.Node
	ast.Walk(func(n *parser.Node) bool {
		if n.IsFunction() && n.Name == name {
			found = n
			return false
		}
		return true
	})
	return found
}

// CountFunctionsInAST counts the number of functions in an AST
func CountFunctionsInAST(ast *parser.Node) int {
	count := 0
	ast.Walk(func(n *parser.Node) bool {
		if n.IsFunction() {
			count++
		}
		return true
	})
	return count
}

// CountNodesOfType counts nodes of a specific type in an AST
func CountNodesOfType(ast *parser.Node, nodeType parser.NodeType) int {
	count := 0
	ast.Walk(func(n *parser.Node) bool {
		if n.Type == nodeType {
			count++
		}
		return true
	})
	return count
}
