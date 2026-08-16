package service

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/ludo-technologies/polyscan/jscan/domain"
)

// reportComplexityResponse builds a complexity response whose risk labels follow
// the thresholds a real run would have applied.
func reportComplexityResponse(low, medium int, complexities ...int) *domain.ComplexityResponse {
	response := &domain.ComplexityResponse{}
	for i, complexity := range complexities {
		risk := domain.RiskLevelLow
		switch {
		case complexity > medium:
			risk = domain.RiskLevelHigh
		case complexity > low:
			risk = domain.RiskLevelMedium
		}
		response.Functions = append(response.Functions, domain.FunctionComplexity{
			Name:      "fn",
			FilePath:  "src/a.ts",
			StartLine: 1,
			EndLine:   10,
			Metrics:   domain.ComplexityMetrics{Complexity: complexity, NestingDepth: i % 3},
			RiskLevel: risk,
		})
	}
	response.Summary.TotalFunctions = len(complexities)
	response.Summary.FilesAnalyzed = 1
	response.Summary.TotalFiles = 1
	return response
}

func TestRiskThresholdsAreRecoveredFromRiskLabels(t *testing.T) {
	tests := []struct {
		name           string
		response       *domain.ComplexityResponse
		wantLow        int
		wantMedium     int
		wantAtLeastOne bool
	}{
		{
			name:       "every band populated",
			response:   reportComplexityResponse(9, 19, 1, 9, 10, 19, 20, 40),
			wantLow:    9,
			wantMedium: 19,
		},
		{
			name: "no medium functions",
			// Nothing lands between the thresholds, so the lowest high-risk
			// function stands in for the missing upper bound.
			response:   reportComplexityResponse(9, 19, 1, 9, 25),
			wantLow:    9,
			wantMedium: 24,
		},
		{
			name:       "no low functions",
			response:   reportComplexityResponse(9, 19, 12, 15),
			wantLow:    11,
			wantMedium: 15,
		},
		{
			name:       "no functions at all",
			response:   reportComplexityResponse(9, 19),
			wantLow:    0,
			wantMedium: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			low, medium := riskThresholds(test.response)
			if low != test.wantLow || medium != test.wantMedium {
				t.Errorf("riskThresholds() = (%d, %d), want (%d, %d)", low, medium, test.wantLow, test.wantMedium)
			}
		})
	}
}

func TestRiskThresholdsWithoutComplexityAnalysis(t *testing.T) {
	if low, medium := riskThresholds(nil); low != 0 || medium != 0 {
		t.Errorf("expected no thresholds without complexity analysis, got (%d, %d)", low, medium)
	}
}

func TestHistogramBinsFollowTheRunsThresholds(t *testing.T) {
	bins := histogramBins(9, 19)

	labels := make([]string, 0, len(bins))
	bands := make([]string, 0, len(bins))
	for _, bin := range bins {
		labels = append(labels, bin.label)
		bands = append(bands, bin.band)
	}

	wantLabels := []string{"1", "2–5", "6–9", "10–19", "20+"}
	if strings.Join(labels, "|") != strings.Join(wantLabels, "|") {
		t.Errorf("bin labels = %v, want %v", labels, wantLabels)
	}
	// A bucket may only carry a risk color when every complexity inside it has
	// that risk, which is what aligning the edges with the thresholds buys.
	wantBands := []string{"", "", "", "warn", "bad"}
	if strings.Join(bands, "|") != strings.Join(wantBands, "|") {
		t.Errorf("bin bands = %v, want %v", bands, wantBands)
	}
}

func TestHistogramBinsCollapseWhenThresholdsAreTight(t *testing.T) {
	bins := histogramBins(1, 2)

	labels := make([]string, 0, len(bins))
	for _, bin := range bins {
		labels = append(labels, bin.label)
	}
	wantLabels := []string{"1", "2", "3+"}
	if strings.Join(labels, "|") != strings.Join(wantLabels, "|") {
		t.Errorf("bin labels = %v, want %v", labels, wantLabels)
	}
}

func TestBuildReportHistogramCountsAndMarksTheThreshold(t *testing.T) {
	complexity := reportComplexityResponse(9, 19, 1, 1, 3, 12, 25)
	hist := buildReportHistogram(complexity, 9, 19)

	if hist == nil {
		t.Fatal("expected a histogram")
	}
	counts := map[string]int{}
	for _, bin := range hist.Bins {
		counts[bin.Label] = bin.Count
	}
	for label, want := range map[string]int{"1": 2, "2–5": 1, "6–9": 0, "10–19": 1, "20+": 1} {
		if counts[label] != want {
			t.Errorf("bin %q count = %d, want %d", label, counts[label], want)
		}
	}
	if hist.Threshold != "risk from CC 10" {
		t.Errorf("threshold marker = %q, want %q", hist.Threshold, "risk from CC 10")
	}

	facts := map[string]string{}
	for _, fact := range hist.Facts {
		facts[fact.Key] = fact.Value
	}
	if facts["Median function"] != "CC 3, 10 lines" {
		t.Errorf("median fact = %q", facts["Median function"])
	}
}

func TestBuildReportHistogramWithoutFunctions(t *testing.T) {
	if hist := buildReportHistogram(&domain.ComplexityResponse{}, 0, 0); hist != nil {
		t.Error("expected no histogram when no function was analyzed")
	}
}

func TestBuildReportTabsMergeTheAnalysesIntoFive(t *testing.T) {
	summary := &domain.AnalyzeSummary{
		ComplexityEnabled:   true,
		DeadCodeEnabled:     true,
		CloneEnabled:        true,
		CBOEnabled:          true,
		DepsEnabled:         true,
		HighComplexityCount: 2,
		DeadCodeCount:       3,
		CloneGroups:         4,
		HighCouplingClasses: 1,
	}
	deps := &domain.DependencyGraphResponse{
		Analysis: &domain.DependencyAnalysisResult{
			CircularDependencies: &domain.CircularDependencyAnalysis{TotalCycles: 2},
		},
	}

	tabs := buildReportTabs(summary, &domain.ComplexityResponse{}, nil, deps)

	ids := make([]string, 0, len(tabs))
	for _, tab := range tabs {
		ids = append(ids, tab.ID)
	}
	want := []string{"overview", "functions", "duplication", "classes", "architecture"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("tabs = %v, want %v", ids, want)
	}
	if tabs[1].Count != 5 || tabs[1].CountBand != "bad" {
		t.Errorf("functions tab = %+v, want 5 issues banded bad", tabs[1])
	}
	if tabs[4].Count != 2 {
		t.Errorf("architecture tab count = %d, want 2 cycles", tabs[4].Count)
	}
}

func TestBuildReportTabsSkipAnalysesThatDidNotRun(t *testing.T) {
	summary := &domain.AnalyzeSummary{ComplexityEnabled: true}

	tabs := buildReportTabs(summary, &domain.ComplexityResponse{}, nil, nil)

	if len(tabs) != 2 || tabs[1].ID != "functions" {
		t.Errorf("expected only the overview and functions tabs, got %+v", tabs)
	}
}

// A dependency score is computed from the summary alone, but the Architecture
// tab needs the analysis result. The card must not link to a tab that is absent.
func TestDependencyDimensionDoesNotLinkToAMissingTab(t *testing.T) {
	summary := &domain.AnalyzeSummary{DepsEnabled: true, DependencyScore: 80}
	tabs := buildReportTabs(summary, nil, nil, nil)

	dims := buildReportDimensions(summary, tabs)

	if len(dims) != 1 {
		t.Fatalf("expected one dimension, got %d", len(dims))
	}
	if dims[0].Tab != "" {
		t.Errorf("expected no tab link, got %q", dims[0].Tab)
	}
}

func TestBuildReportVerdictNamesTheWeakestDimensions(t *testing.T) {
	summary := &domain.AnalyzeSummary{Grade: "C", TotalFiles: 40}
	dims := []reportDimension{
		{Name: "Complexity", Score: 95, Left: "avg CC 2.00", Right: "0 high-risk"},
		{Name: "Duplication", Score: 30, Left: "18.0% of fragments", Right: "9 groups"},
		{Name: "Dead code", Score: 55, Left: "12 findings", Right: "1 critical"},
	}

	verdict := buildReportVerdict(summary, dims)

	if verdict.Headline != "Fair, with clear debt to pay down" {
		t.Errorf("headline = %q", verdict.Headline)
	}
	body := verdictText(verdict)
	if !strings.Contains(body, "Complexity is clean.") {
		t.Errorf("expected the clean dimension to be named: %q", body)
	}
	// Weakest first, so the worst score leads.
	if !strings.Contains(body, "duplication (18.0% of fragments, 9 groups) and dead code") {
		t.Errorf("expected the weak dimensions ordered worst first: %q", body)
	}
}

func TestBuildReportVerdictReportsSkippedFiles(t *testing.T) {
	summary := &domain.AnalyzeSummary{Grade: "B", TotalFiles: 10, SkippedFiles: 2}

	verdict := buildReportVerdict(summary, []reportDimension{{Name: "Complexity", Score: 92}})

	body := verdictText(verdict)
	if !strings.Contains(body, "2 files of 10 could not be parsed") {
		t.Errorf("expected the parse failures in the verdict: %q", body)
	}
	if !strings.Contains(body, "the health score is penalized for them") {
		t.Errorf("expected the penalty to be explained: %q", body)
	}
}

func TestBuildReportVerdictWithoutAnyAnalysis(t *testing.T) {
	verdict := buildReportVerdict(&domain.AnalyzeSummary{Grade: "F"}, nil)

	if body := verdictText(verdict); body != "No analyses were enabled for this run." {
		t.Errorf("verdict = %q", body)
	}
}

func verdictText(verdict reportVerdict) string {
	var builder strings.Builder
	for _, segment := range verdict.Body {
		builder.WriteString(segment.Text)
	}
	return builder.String()
}

func TestBuildReportHotspotsRankByRiskThenComplexity(t *testing.T) {
	modules := []domain.ModuleQualityMetrics{
		{FilePath: "src/quiet.ts", ModuleComplexityMetrics: domain.ModuleComplexityMetrics{LinesOfCode: 10, MaxComplexity: 2}},
		{FilePath: "src/big.ts", ModuleComplexityMetrics: domain.ModuleComplexityMetrics{LinesOfCode: 900, MaxComplexity: 14}},
		{FilePath: "src/risky.ts", ModuleComplexityMetrics: domain.ModuleComplexityMetrics{LinesOfCode: 100, MaxComplexity: 30, HighRiskFunctionCount: 3}},
	}
	clone := &domain.CloneResponse{
		CloneGroups: []*domain.CloneGroup{{
			Clones: []*domain.Clone{
				{Location: &domain.CloneLocation{FilePath: "src/big.ts", StartLine: 1, EndLine: 20}},
				{Location: &domain.CloneLocation{FilePath: "src/quiet.ts", StartLine: 1, EndLine: 20}},
			},
		}},
	}

	hotspots := buildReportHotspots(modules, clone, 9, 19)

	if len(hotspots) != 3 {
		t.Fatalf("expected 3 hotspots, got %d", len(hotspots))
	}
	if hotspots[0].File != "risky.ts" || hotspots[1].File != "big.ts" {
		t.Errorf("unexpected ranking: %s then %s", hotspots[0].File, hotspots[1].File)
	}
	if hotspots[0].MaxCCBand != "bad" || hotspots[1].MaxCCBand != "warn" || hotspots[2].MaxCCBand != "" {
		t.Errorf("unexpected complexity bands: %q, %q, %q", hotspots[0].MaxCCBand, hotspots[1].MaxCCBand, hotspots[2].MaxCCBand)
	}
	if hotspots[1].Clones != 1 {
		t.Errorf("expected the clone fragment counted against src/big.ts, got %d", hotspots[1].Clones)
	}
	if hotspots[0].MaxCCPct != 100 {
		t.Errorf("expected the worst module's bar to be full, got %d%%", hotspots[0].MaxCCPct)
	}
}

func TestCountClonesByFileCountsEachFragmentOnce(t *testing.T) {
	fragment := &domain.Clone{Location: &domain.CloneLocation{FilePath: "src/a.ts", StartLine: 1, EndLine: 20}}
	other := &domain.Clone{Location: &domain.CloneLocation{FilePath: "src/b.ts", StartLine: 5, EndLine: 25}}
	third := &domain.Clone{Location: &domain.CloneLocation{FilePath: "src/c.ts", StartLine: 5, EndLine: 25}}

	// The same fragment appears in two pairs; groups are absent, so the pair
	// list is the only source and must not double-count it.
	counts := countClonesByFile(&domain.CloneResponse{
		ClonePairs: []*domain.ClonePair{
			{Clone1: fragment, Clone2: other},
			{Clone1: fragment, Clone2: third},
			{Clone1: nil, Clone2: nil},
		},
	})

	if counts["src/a.ts"] != 1 {
		t.Errorf("expected the shared fragment counted once, got %d", counts["src/a.ts"])
	}
	if counts["src/b.ts"] != 1 || counts["src/c.ts"] != 1 {
		t.Errorf("expected one fragment per partner file, got %+v", counts)
	}
}

func TestWriteHTMLRendersTheOverviewFromAnalysisData(t *testing.T) {
	complexity := reportComplexityResponse(9, 19, 1, 4, 12, 30)
	complexity.Summary.TotalFiles = 4
	complexity.Summary.FilesAnalyzed = 3
	complexity.Summary.SkippedFiles = 1
	complexity.ModuleRollups = map[string]domain.ModuleComplexityMetrics{
		"src/a.ts": {LinesOfCode: 240, AnalyzedFunctionCount: 4, AverageComplexity: 11.75, MaxComplexity: 30, HighRiskFunctionCount: 1},
	}

	var buf bytes.Buffer
	formatter := NewOutputFormatter()
	if err := formatter.WriteHTML(complexity, nil, nil, nil, nil, &buf, 1200*time.Millisecond); err != nil {
		t.Fatalf("WriteHTML failed: %v", err)
	}

	report := buf.String()
	for _, expected := range []string{
		"HEALTH SCORE",
		"Score breakdown",
		"Complexity distribution",
		"Hotspot files",
		`data-tab="functions"`,
		"1 file of 4 could not be parsed",
		"3 of 4 files analyzed, 1 skipped",
		"1200ms",
	} {
		if !strings.Contains(report, expected) {
			t.Errorf("expected the report to contain %q", expected)
		}
	}
	// Tabs for analyses that did not run must not be rendered at all.
	for _, absent := range []string{`id="duplication"`, `id="classes"`, `id="architecture"`} {
		if strings.Contains(report, absent) {
			t.Errorf("expected no %s panel when the analysis did not run", absent)
		}
	}
}

func TestWriteHTMLAnnouncesDeadCodeTruncation(t *testing.T) {
	findings := make([]domain.DeadCodeFinding, 0, 25)
	for i := 0; i < 25; i++ {
		findings = append(findings, domain.DeadCodeFinding{
			FunctionName: "fn",
			Location:     domain.DeadCodeLocation{FilePath: "src/a.ts", StartLine: i + 1, EndLine: i + 2},
			Severity:     domain.DeadCodeSeverityWarning,
			Reason:       "unreachable code",
		})
	}
	deadCode := &domain.DeadCodeResponse{
		Files: []domain.FileDeadCode{{
			FilePath:  "src/a.ts",
			Functions: []domain.FunctionDeadCode{{Name: "fn", Findings: findings}},
		}},
		Summary: domain.DeadCodeSummary{TotalFiles: 1, TotalFindings: 25, WarningFindings: 25, FilesWithDeadCode: 1},
	}

	var buf bytes.Buffer
	formatter := NewOutputFormatter()
	if err := formatter.WriteHTML(nil, deadCode, nil, nil, nil, &buf, time.Second); err != nil {
		t.Fatalf("WriteHTML failed: %v", err)
	}

	report := buf.String()
	if !strings.Contains(report, "Showing 20 of 25 findings") {
		t.Error("expected the truncated dead code table to say so")
	}
	if strings.Count(report, `<span class="pill sev-warning">`) != 20 {
		t.Errorf("expected exactly 20 findings rendered, got %d", strings.Count(report, `<span class="pill sev-warning">`))
	}
}
