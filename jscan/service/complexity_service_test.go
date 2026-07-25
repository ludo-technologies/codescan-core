package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ludo-technologies/polyscan/jscan/domain"
	"github.com/ludo-technologies/polyscan/jscan/internal/config"
)

func TestNewComplexityService(t *testing.T) {
	cfg := &config.ComplexityConfig{
		LowThreshold:    5,
		MediumThreshold: 10,
	}

	service := NewComplexityService(cfg)

	if service == nil {
		t.Fatal("NewComplexityService should not return nil")
	}
	if service.config != cfg {
		t.Error("Service should store config reference")
	}
	if service.progress != nil {
		t.Error("Progress should be nil when not provided")
	}
}

func TestNewComplexityServiceWithProgress(t *testing.T) {
	cfg := &config.ComplexityConfig{
		LowThreshold:    5,
		MediumThreshold: 10,
	}
	pm := NewProgressManager(false) // Use non-interactive mode for tests

	service := NewComplexityServiceWithProgress(cfg, pm)

	if service == nil {
		t.Fatal("NewComplexityServiceWithProgress should not return nil")
	}
	if service.progress == nil {
		t.Error("Progress should not be nil")
	}
}

func TestComplexityService_Analyze_EmptyPaths(t *testing.T) {
	cfg := &config.ComplexityConfig{
		LowThreshold:    5,
		MediumThreshold: 10,
		Enabled:         true,
		ReportUnchanged: true,
	}

	service := NewComplexityService(cfg)

	req := domain.ComplexityRequest{
		Paths: []string{},
	}

	_, err := service.Analyze(context.Background(), req)
	if err == nil {
		t.Error("Should return error for empty paths")
	}
}

func TestComplexityService_Analyze_NonexistentFile(t *testing.T) {
	cfg := &config.ComplexityConfig{
		LowThreshold:    5,
		MediumThreshold: 10,
		Enabled:         true,
		ReportUnchanged: true,
	}

	service := NewComplexityService(cfg)

	req := domain.ComplexityRequest{
		Paths: []string{"/nonexistent/file.js"},
	}

	_, err := service.Analyze(context.Background(), req)
	if err == nil {
		t.Error("Should return error for nonexistent file")
	}
}

func TestComplexityService_Analyze_ValidFile(t *testing.T) {
	// Create a temp JS file
	tempDir := t.TempDir()
	jsFile := filepath.Join(tempDir, "test.js")
	content := `
function simple() {
    return 1;
}

function complex(x) {
    if (x > 0) {
        for (let i = 0; i < 10; i++) {
            console.log(i);
        }
    } else {
        console.log("negative");
    }
}
`
	if err := os.WriteFile(jsFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.ComplexityConfig{
		LowThreshold:    5,
		MediumThreshold: 10,
		Enabled:         true,
		ReportUnchanged: true,
	}

	service := NewComplexityService(cfg)

	req := domain.ComplexityRequest{
		Paths: []string{jsFile},
	}

	resp, err := service.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze should not return error: %v", err)
	}

	if resp == nil {
		t.Fatal("Response should not be nil")
	}

	if len(resp.Functions) == 0 {
		t.Error("Should find functions in the test file")
	}

	if resp.Summary.TotalFunctions == 0 {
		t.Error("Summary should have total functions")
	}

	foundValidLocation := false
	for _, fn := range resp.Functions {
		if fn.StartLine > 0 && fn.EndLine >= fn.StartLine {
			foundValidLocation = true
			break
		}
	}
	if !foundValidLocation {
		t.Error("At least one function should include valid line location metadata")
	}
}

func TestComplexityService_Analyze_ContextCancellation(t *testing.T) {
	cfg := &config.ComplexityConfig{
		LowThreshold:    5,
		MediumThreshold: 10,
		Enabled:         true,
	}

	service := NewComplexityService(cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := domain.ComplexityRequest{
		Paths: []string{"test.js"},
	}

	_, err := service.Analyze(ctx, req)
	if err == nil {
		t.Error("Should return error when context is cancelled")
	}
}

func TestComplexityService_AnalyzeFile(t *testing.T) {
	// Create a temp JS file
	tempDir := t.TempDir()
	jsFile := filepath.Join(tempDir, "test.js")
	content := `function test() { return 1; }`
	if err := os.WriteFile(jsFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.ComplexityConfig{
		LowThreshold:    5,
		MediumThreshold: 10,
		Enabled:         true,
		ReportUnchanged: true,
	}

	service := NewComplexityService(cfg)

	resp, err := service.AnalyzeFile(context.Background(), jsFile, domain.ComplexityRequest{})
	if err != nil {
		t.Fatalf("AnalyzeFile should not return error: %v", err)
	}

	if resp == nil {
		t.Fatal("Response should not be nil")
	}
}

func TestComplexityService_filterFunctions(t *testing.T) {
	cfg := &config.ComplexityConfig{
		LowThreshold:    5,
		MediumThreshold: 10,
		Enabled:         true,
		ReportUnchanged: false, // Don't report unchanged (complexity = 1)
	}

	service := NewComplexityService(cfg)

	functions := []domain.FunctionComplexity{
		{Name: "simple", Metrics: domain.ComplexityMetrics{Complexity: 1}},
		{Name: "medium", Metrics: domain.ComplexityMetrics{Complexity: 5}},
		{Name: "complex", Metrics: domain.ComplexityMetrics{Complexity: 15}},
	}

	req := domain.ComplexityRequest{
		MinComplexity: 3,
		MaxComplexity: 10,
	}

	filtered := service.filterFunctions(functions, req)

	// Should filter:
	// - simple (complexity 1 < min 3)
	// - complex (complexity 15 > max 10)
	if len(filtered) != 1 {
		t.Errorf("Should have 1 filtered function, got %d", len(filtered))
	}

	if len(filtered) > 0 && filtered[0].Name != "medium" {
		t.Errorf("Filtered function should be 'medium', got '%s'", filtered[0].Name)
	}
}

func TestComplexityService_filterFunctions_ReportUnchanged(t *testing.T) {
	cfg := &config.ComplexityConfig{
		ReportUnchanged: true,
	}

	service := NewComplexityService(cfg)

	functions := []domain.FunctionComplexity{
		{Name: "simple", Metrics: domain.ComplexityMetrics{Complexity: 1}},
	}

	req := domain.ComplexityRequest{}

	filtered := service.filterFunctions(functions, req)

	if len(filtered) != 1 {
		t.Errorf("Should include unchanged function when ReportUnchanged is true")
	}
}

func TestComplexityService_sortFunctions_ByComplexity(t *testing.T) {
	cfg := &config.ComplexityConfig{}
	service := NewComplexityService(cfg)

	functions := []domain.FunctionComplexity{
		{Name: "a", Metrics: domain.ComplexityMetrics{Complexity: 5}},
		{Name: "b", Metrics: domain.ComplexityMetrics{Complexity: 15}},
		{Name: "c", Metrics: domain.ComplexityMetrics{Complexity: 10}},
	}

	sorted := service.sortFunctions(functions, domain.SortByComplexity)

	// Should be sorted descending by complexity
	if sorted[0].Metrics.Complexity != 15 {
		t.Error("First should have highest complexity")
	}
	if sorted[1].Metrics.Complexity != 10 {
		t.Error("Second should have medium complexity")
	}
	if sorted[2].Metrics.Complexity != 5 {
		t.Error("Third should have lowest complexity")
	}
}

func TestComplexityService_sortFunctions_ByName(t *testing.T) {
	cfg := &config.ComplexityConfig{}
	service := NewComplexityService(cfg)

	functions := []domain.FunctionComplexity{
		{Name: "charlie"},
		{Name: "alpha"},
		{Name: "beta"},
	}

	sorted := service.sortFunctions(functions, domain.SortByName)

	if sorted[0].Name != "alpha" {
		t.Errorf("First should be 'alpha', got '%s'", sorted[0].Name)
	}
	if sorted[1].Name != "beta" {
		t.Errorf("Second should be 'beta', got '%s'", sorted[1].Name)
	}
	if sorted[2].Name != "charlie" {
		t.Errorf("Third should be 'charlie', got '%s'", sorted[2].Name)
	}
}

func TestComplexityService_sortFunctions_ByRisk(t *testing.T) {
	cfg := &config.ComplexityConfig{}
	service := NewComplexityService(cfg)

	functions := []domain.FunctionComplexity{
		{Name: "low", RiskLevel: domain.RiskLevelLow},
		{Name: "high", RiskLevel: domain.RiskLevelHigh},
		{Name: "medium", RiskLevel: domain.RiskLevelMedium},
	}

	sorted := service.sortFunctions(functions, domain.SortByRisk)

	// Should be sorted: high, medium, low
	if sorted[0].RiskLevel != domain.RiskLevelHigh {
		t.Error("First should be high risk")
	}
	if sorted[1].RiskLevel != domain.RiskLevelMedium {
		t.Error("Second should be medium risk")
	}
	if sorted[2].RiskLevel != domain.RiskLevelLow {
		t.Error("Third should be low risk")
	}
}

func TestComplexityService_sortFunctions_Default(t *testing.T) {
	cfg := &config.ComplexityConfig{}
	service := NewComplexityService(cfg)

	functions := []domain.FunctionComplexity{
		{Name: "a", Metrics: domain.ComplexityMetrics{Complexity: 5}},
		{Name: "b", Metrics: domain.ComplexityMetrics{Complexity: 15}},
	}

	// Unknown sort criteria should default to complexity
	sorted := service.sortFunctions(functions, domain.SortCriteria("unknown"))

	if sorted[0].Metrics.Complexity != 15 {
		t.Error("Default sort should be by complexity descending")
	}
}

func TestComplexityService_generateSummary_Empty(t *testing.T) {
	cfg := &config.ComplexityConfig{}
	service := NewComplexityService(cfg)

	summary := service.generateSummary([]domain.FunctionComplexity{}, 0, domain.ComplexityRequest{}, 0)

	if summary.TotalFunctions != 0 {
		t.Error("Empty functions should have 0 total")
	}
	if summary.FilesAnalyzed != 0 {
		t.Error("Should have 0 files analyzed")
	}
}

func TestComplexityService_generateSummary_WithFunctions(t *testing.T) {
	cfg := &config.ComplexityConfig{}
	service := NewComplexityService(cfg)

	functions := []domain.FunctionComplexity{
		{Name: "a", Metrics: domain.ComplexityMetrics{Complexity: 5}, RiskLevel: domain.RiskLevelLow},
		{Name: "b", Metrics: domain.ComplexityMetrics{Complexity: 15}, RiskLevel: domain.RiskLevelMedium},
		{Name: "c", Metrics: domain.ComplexityMetrics{Complexity: 25}, RiskLevel: domain.RiskLevelHigh},
	}

	summary := service.generateSummary(functions, 2, domain.ComplexityRequest{}, 5)

	if summary.TotalFunctions != 3 {
		t.Errorf("TotalFunctions should be 3, got %d", summary.TotalFunctions)
	}
	if summary.FunctionsParsed != 5 {
		t.Errorf("FunctionsParsed should be 5, got %d", summary.FunctionsParsed)
	}
	if summary.FilesAnalyzed != 2 {
		t.Errorf("FilesAnalyzed should be 2, got %d", summary.FilesAnalyzed)
	}
	if summary.MinComplexity != 5 {
		t.Errorf("MinComplexity should be 5, got %d", summary.MinComplexity)
	}
	if summary.MaxComplexity != 25 {
		t.Errorf("MaxComplexity should be 25, got %d", summary.MaxComplexity)
	}

	expectedAvg := 15.0 // (5+15+25)/3
	if summary.AverageComplexity != expectedAvg {
		t.Errorf("AverageComplexity should be %.2f, got %.2f", expectedAvg, summary.AverageComplexity)
	}

	if summary.LowRiskFunctions != 1 {
		t.Errorf("LowRiskFunctions should be 1, got %d", summary.LowRiskFunctions)
	}
	if summary.MediumRiskFunctions != 1 {
		t.Errorf("MediumRiskFunctions should be 1, got %d", summary.MediumRiskFunctions)
	}
	if summary.HighRiskFunctions != 1 {
		t.Errorf("HighRiskFunctions should be 1, got %d", summary.HighRiskFunctions)
	}
}

func TestComplexityService_buildConfigForResponse(t *testing.T) {
	cfg := &config.ComplexityConfig{
		LowThreshold:    5,
		MediumThreshold: 10,
		MaxComplexity:   50,
	}
	service := NewComplexityService(cfg)

	req := domain.ComplexityRequest{
		SortBy:        domain.SortByName,
		MinComplexity: 3,
	}

	configMap := service.buildConfigForResponse(req)

	if configMap["low_threshold"] != 5 {
		t.Error("low_threshold should be 5")
	}
	if configMap["medium_threshold"] != 10 {
		t.Error("medium_threshold should be 10")
	}
	if configMap["max_complexity"] != 50 {
		t.Error("max_complexity should be 50")
	}
	if configMap["sort_by"] != domain.SortByName {
		t.Error("sort_by should be 'name'")
	}
	if configMap["min_complexity"] != 3 {
		t.Error("min_complexity should be 3")
	}
}

func TestComplexityService_readFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.txt")
	content := "test content"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.ComplexityConfig{}
	service := NewComplexityService(cfg)

	data, err := service.readFile(testFile)
	if err != nil {
		t.Fatalf("readFile should not return error: %v", err)
	}

	if string(data) != content {
		t.Errorf("Content should be '%s', got '%s'", content, string(data))
	}
}

func TestComplexityService_readFile_NonExistent(t *testing.T) {
	cfg := &config.ComplexityConfig{}
	service := NewComplexityService(cfg)

	_, err := service.readFile("/nonexistent/file.txt")
	if err == nil {
		t.Error("readFile should return error for nonexistent file")
	}
}

func TestComplexityService_Analyze_WithProgress(t *testing.T) {
	// Create a temp JS file
	tempDir := t.TempDir()
	jsFile := filepath.Join(tempDir, "test.js")
	content := `function test() { return 1; }`
	if err := os.WriteFile(jsFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.ComplexityConfig{
		LowThreshold:    5,
		MediumThreshold: 10,
		Enabled:         true,
		ReportUnchanged: true,
	}

	pm := NewProgressManager(false) // Use non-interactive mode for tests
	service := NewComplexityServiceWithProgress(cfg, pm)

	req := domain.ComplexityRequest{
		Paths: []string{jsFile},
	}

	resp, err := service.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze should not return error: %v", err)
	}

	if resp == nil {
		t.Fatal("Response should not be nil")
	}
}

func TestComplexityService_Analyze_ResponseFields(t *testing.T) {
	// Create a temp JS file
	tempDir := t.TempDir()
	jsFile := filepath.Join(tempDir, "test.js")
	content := `function test() { return 1; }`
	if err := os.WriteFile(jsFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	cfg := &config.ComplexityConfig{
		LowThreshold:    5,
		MediumThreshold: 10,
		Enabled:         true,
		ReportUnchanged: true,
	}

	service := NewComplexityService(cfg)

	req := domain.ComplexityRequest{
		Paths: []string{jsFile},
	}

	resp, err := service.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze should not return error: %v", err)
	}

	// Verify response has all expected fields
	if resp.GeneratedAt == "" {
		t.Error("GeneratedAt should not be empty")
	}

	// Verify GeneratedAt is a valid RFC3339 timestamp
	_, err = time.Parse(time.RFC3339, resp.GeneratedAt)
	if err != nil {
		t.Errorf("GeneratedAt should be valid RFC3339: %v", err)
	}

	if resp.Version == "" {
		t.Error("Version should not be empty")
	}

	if resp.Config == nil {
		t.Error("Config should not be nil")
	}
}
