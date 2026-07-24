package cbo

import (
	"testing"

	"github.com/ludo-technologies/polyscan/core/domain"
)

func TestComputeCBO_EmptyInput(t *testing.T) {
	results := ComputeCBO(nil, DefaultConfig())
	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}

	results = ComputeCBO([]*ClassInfo{}, DefaultConfig())
	if len(results) != 0 {
		t.Fatalf("expected 0 results for empty slice, got %d", len(results))
	}
}

func TestComputeCBO_NilClassSkipped(t *testing.T) {
	classes := []*ClassInfo{
		nil,
		{Name: "Foo", FilePath: "foo.py", StartLine: 1, EndLine: 10},
		nil,
	}
	results := ComputeCBO(classes, DefaultConfig())
	if len(results) != 1 {
		t.Fatalf("expected 1 result (nil skipped), got %d", len(results))
	}
	if results[0].ClassName != "Foo" {
		t.Fatalf("expected class Foo, got %s", results[0].ClassName)
	}
}

func TestComputeCBO_NoDependencies(t *testing.T) {
	classes := []*ClassInfo{
		{
			Name:      "Isolated",
			FilePath:  "isolated.py",
			StartLine: 1,
			EndLine:   20,
		},
	}
	results := ComputeCBO(classes, DefaultConfig())
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	r := results[0]
	if r.CouplingCount != 0 {
		t.Errorf("expected CBO=0, got %d", r.CouplingCount)
	}
	if r.RiskLevel != domain.RiskLevelLow {
		t.Errorf("expected low risk, got %s", r.RiskLevel)
	}
	if len(r.DependentClasses) != 0 {
		t.Errorf("expected no dependent classes, got %v", r.DependentClasses)
	}
}

func TestComputeCBO_LowRisk(t *testing.T) {
	classes := []*ClassInfo{
		{
			Name:     "Service",
			FilePath: "service.py",
			Dependencies: []ClassDependency{
				{ClassName: "Repo", Kind: DepInstantiation},
				{ClassName: "Logger", Kind: DepTypeHint},
				{ClassName: "Config", Kind: DepImport},
			},
		},
	}
	results := ComputeCBO(classes, DefaultConfig())
	r := results[0]
	if r.CouplingCount != 3 {
		t.Errorf("expected CBO=3, got %d", r.CouplingCount)
	}
	if r.RiskLevel != domain.RiskLevelLow {
		t.Errorf("expected low risk for CBO=3, got %s", r.RiskLevel)
	}
}

func TestComputeCBO_MediumRisk(t *testing.T) {
	classes := []*ClassInfo{
		{
			Name:     "Controller",
			FilePath: "controller.py",
			Dependencies: []ClassDependency{
				{ClassName: "A", Kind: DepImport},
				{ClassName: "B", Kind: DepImport},
				{ClassName: "C", Kind: DepImport},
				{ClassName: "D", Kind: DepImport},
				{ClassName: "E", Kind: DepTypeHint},
			},
		},
	}
	results := ComputeCBO(classes, DefaultConfig())
	r := results[0]
	if r.CouplingCount != 5 {
		t.Errorf("expected CBO=5, got %d", r.CouplingCount)
	}
	if r.RiskLevel != domain.RiskLevelMedium {
		t.Errorf("expected medium risk for CBO=5, got %s", r.RiskLevel)
	}
}

func TestComputeCBO_HighRisk(t *testing.T) {
	deps := make([]ClassDependency, 8)
	for i := range deps {
		deps[i] = ClassDependency{
			ClassName: string(rune('A' + i)),
			Kind:      DepImport,
		}
	}
	classes := []*ClassInfo{
		{
			Name:         "GodClass",
			FilePath:     "god.py",
			Dependencies: deps,
		},
	}
	results := ComputeCBO(classes, DefaultConfig())
	r := results[0]
	if r.CouplingCount != 8 {
		t.Errorf("expected CBO=8, got %d", r.CouplingCount)
	}
	if r.RiskLevel != domain.RiskLevelHigh {
		t.Errorf("expected high risk for CBO=8, got %s", r.RiskLevel)
	}
}

func TestComputeCBO_SelfReferenceExcluded(t *testing.T) {
	classes := []*ClassInfo{
		{
			Name:     "Node",
			FilePath: "node.py",
			Dependencies: []ClassDependency{
				{ClassName: "Node", Kind: DepTypeHint},      // self-ref
				{ClassName: "Node", Kind: DepInstantiation}, // self-ref
				{ClassName: "Other", Kind: DepInheritance},
			},
		},
	}
	results := ComputeCBO(classes, DefaultConfig())
	r := results[0]
	if r.CouplingCount != 1 {
		t.Errorf("expected CBO=1 (self-refs excluded), got %d", r.CouplingCount)
	}
	if len(r.DependentClasses) != 1 || r.DependentClasses[0] != "Other" {
		t.Errorf("expected [Other], got %v", r.DependentClasses)
	}
}

func TestComputeCBO_ExcludePatterns(t *testing.T) {
	classes := []*ClassInfo{
		{Name: "TestHelper", FilePath: "test_helper.py"},
		{Name: "Service", FilePath: "service.py"},
		{Name: "MockRepo", FilePath: "mock_repo.py"},
	}
	config := DefaultConfig()
	config.ExcludePatterns = []string{"Test*", "Mock*"}

	results := ComputeCBO(classes, config)
	if len(results) != 1 {
		t.Fatalf("expected 1 result after exclusion, got %d", len(results))
	}
	if results[0].ClassName != "Service" {
		t.Errorf("expected Service, got %s", results[0].ClassName)
	}
}

func TestComputeCBO_DependencyBreakdown(t *testing.T) {
	classes := []*ClassInfo{
		{
			Name:     "Handler",
			FilePath: "handler.py",
			Dependencies: []ClassDependency{
				{ClassName: "Base", Kind: DepInheritance},
				{ClassName: "Logger", Kind: DepTypeHint},
				{ClassName: "Logger", Kind: DepInstantiation},
				{ClassName: "Config", Kind: DepImport},
				{ClassName: "Config", Kind: DepAttributeAccess},
			},
		},
	}
	results := ComputeCBO(classes, DefaultConfig())
	r := results[0]

	// 3 unique classes: Base, Logger, Config
	if r.CouplingCount != 3 {
		t.Errorf("expected CBO=3, got %d", r.CouplingCount)
	}

	expected := map[DependencyKind]int{
		DepInheritance:     1,
		DepTypeHint:        1,
		DepInstantiation:   1,
		DepImport:          1,
		DepAttributeAccess: 1,
	}
	for kind, count := range expected {
		if r.DependencyBreakdown[kind] != count {
			t.Errorf("expected %s=%d, got %d", kind, count, r.DependencyBreakdown[kind])
		}
	}
}

func TestComputeCBO_DependentClassesSorted(t *testing.T) {
	classes := []*ClassInfo{
		{
			Name:     "X",
			FilePath: "x.py",
			Dependencies: []ClassDependency{
				{ClassName: "Zebra", Kind: DepImport},
				{ClassName: "Apple", Kind: DepImport},
				{ClassName: "Mango", Kind: DepImport},
			},
		},
	}
	results := ComputeCBO(classes, DefaultConfig())
	deps := results[0].DependentClasses

	if len(deps) != 3 {
		t.Fatalf("expected 3 dependent classes, got %d", len(deps))
	}
	if deps[0] != "Apple" || deps[1] != "Mango" || deps[2] != "Zebra" {
		t.Errorf("expected [Apple Mango Zebra], got %v", deps)
	}
}

func TestAssessRisk_BoundaryValues(t *testing.T) {
	config := DefaultConfig() // low=3, medium=7

	tests := []struct {
		cbo      int
		expected domain.RiskLevel
	}{
		{0, domain.RiskLevelLow},
		{3, domain.RiskLevelLow},    // exactly at low threshold
		{4, domain.RiskLevelMedium}, // one above low threshold
		{7, domain.RiskLevelMedium}, // exactly at medium threshold
		{8, domain.RiskLevelHigh},   // one above medium threshold
		{100, domain.RiskLevelHigh},
	}

	for _, tt := range tests {
		got := AssessRisk(tt.cbo, config)
		if got != tt.expected {
			t.Errorf("AssessRisk(%d): expected %s, got %s", tt.cbo, tt.expected, got)
		}
	}
}

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		expected bool
	}{
		{"TestHelper", []string{"Test*"}, true},
		{"MockRepo", []string{"Mock*"}, true},
		{"Service", []string{"Test*", "Mock*"}, false},
		{"myclass", []string{"MyClass"}, true}, // case-insensitive exact
		{"MyClass", []string{"MyClass"}, true},
		{"FooBar", []string{"Foo*"}, true},
		{"FooBar", []string{"*Bar"}, true},
		{"FooBar", []string{"Baz*"}, false},
		{"A", []string{}, false},
	}

	for _, tt := range tests {
		got := MatchesPattern(tt.name, tt.patterns)
		if got != tt.expected {
			t.Errorf("MatchesPattern(%q, %v): expected %v, got %v",
				tt.name, tt.patterns, tt.expected, got)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if config.LowThreshold != 3 {
		t.Errorf("expected LowThreshold=3, got %d", config.LowThreshold)
	}
	if config.MediumThreshold != 7 {
		t.Errorf("expected MediumThreshold=7, got %d", config.MediumThreshold)
	}
	if config.ExcludePatterns != nil {
		t.Errorf("expected nil ExcludePatterns, got %v", config.ExcludePatterns)
	}
}

func TestDependencyKind_String(t *testing.T) {
	tests := []struct {
		kind     DependencyKind
		expected string
	}{
		{DepInheritance, "inheritance"},
		{DepTypeHint, "type_hint"},
		{DepInstantiation, "instantiation"},
		{DepAttributeAccess, "attribute_access"},
		{DepImport, "import"},
		{DependencyKind(99), "unknown"},
	}

	for _, tt := range tests {
		got := tt.kind.String()
		if got != tt.expected {
			t.Errorf("DependencyKind(%d).String(): expected %q, got %q", tt.kind, tt.expected, got)
		}
	}
}

func TestComputeCBO_ResultFieldsPopulated(t *testing.T) {
	classes := []*ClassInfo{
		{
			Name:       "MyClass",
			FilePath:   "my_class.py",
			StartLine:  10,
			EndLine:    50,
			IsAbstract: true,
			Dependencies: []ClassDependency{
				{ClassName: "Base", Kind: DepInheritance},
			},
		},
	}
	results := ComputeCBO(classes, DefaultConfig())
	r := results[0]

	if r.ClassName != "MyClass" {
		t.Errorf("expected ClassName=MyClass, got %s", r.ClassName)
	}
	if r.FilePath != "my_class.py" {
		t.Errorf("expected FilePath=my_class.py, got %s", r.FilePath)
	}
	if r.StartLine != 10 {
		t.Errorf("expected StartLine=10, got %d", r.StartLine)
	}
	if r.EndLine != 50 {
		t.Errorf("expected EndLine=50, got %d", r.EndLine)
	}
	if !r.IsAbstract {
		t.Error("expected IsAbstract=true")
	}
}
