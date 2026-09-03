package service

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

// reportComplexityResponse builds a complexity response the way the complexity
// service does: risk labels drawn from the thresholds, a distribution over the
// whole analyzed population, and the thresholds in the reported config.
func reportComplexityResponse(low, medium int, complexities ...int) *domain.ComplexityResponse {
	response := &domain.ComplexityResponse{
		Config: map[string]interface{}{"low_threshold": low, "medium_threshold": medium},
	}
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
		if response.Summary.ComplexityDistribution == nil {
			response.Summary.ComplexityDistribution = map[int]int{}
		}
		response.Summary.ComplexityDistribution[complexity]++
	}
	response.Summary.TotalFunctions = len(complexities)
	response.Summary.FilesAnalyzed = 1
	response.Summary.TotalFiles = 1
	return response
}

func TestComplexityRiskComesFromTheReportedConfig(t *testing.T) {
	tests := []struct {
		name     string
		response *domain.ComplexityResponse
		want     complexityRisk
	}{
		{
			name:     "thresholds reported",
			response: reportComplexityResponse(9, 19, 1, 12),
			want:     complexityRisk{Low: 9, Medium: 19, Known: true},
		},
		{
			// Inferring the thresholds from the listed functions would answer
			// 11 and 15 here, and band CC 12 as low risk when the run rated it
			// medium.
			name: "report filters removed the boundary functions",
			response: &domain.ComplexityResponse{
				Config: map[string]interface{}{"low_threshold": 9, "medium_threshold": 19},
				Functions: []domain.FunctionComplexity{
					{Metrics: domain.ComplexityMetrics{Complexity: 12}, RiskLevel: domain.RiskLevelMedium},
					{Metrics: domain.ComplexityMetrics{Complexity: 15}, RiskLevel: domain.RiskLevelMedium},
				},
			},
			want: complexityRisk{Low: 9, Medium: 19, Known: true},
		},
		{
			name:     "no config reported",
			response: &domain.ComplexityResponse{},
			want:     complexityRisk{},
		},
		{
			name: "thresholds out of order",
			response: &domain.ComplexityResponse{
				Config: map[string]interface{}{"low_threshold": 20, "medium_threshold": 5},
			},
			want: complexityRisk{},
		},
		{
			name: "thresholds of another type",
			response: &domain.ComplexityResponse{
				Config: map[string]interface{}{"low_threshold": "9", "medium_threshold": "19"},
			},
			want: complexityRisk{},
		},
		{
			name:     "no complexity analysis",
			response: nil,
			want:     complexityRisk{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := readComplexityRisk(test.response); got != test.want {
				t.Errorf("readComplexityRisk() = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestHistogramBinsFollowTheRunsThresholds(t *testing.T) {
	bins := histogramBins(complexityRisk{Low: 9, Medium: 19, Known: true})

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
	bins := histogramBins(complexityRisk{Low: 1, Medium: 2, Known: true})

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
	hist := buildReportHistogram(complexity, complexityRisk{Low: 9, Medium: 19, Known: true})

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

	facts := reportFacts(hist)
	if facts["Median complexity"] != "CC 3" {
		t.Errorf("median fact = %q", facts["Median complexity"])
	}
	if facts["Longest function"] == "" || facts["Deepest nesting"] == "" {
		t.Errorf("expected the per-function facts on a complete list, got %+v", facts)
	}
}

// The histogram, the function total, and every score describe the complete
// analyzed population; the listed functions are whatever the report filters
// left. Counting the list would make the same report contradict itself.
func TestBuildReportHistogramCoversTheFilteredOutFunctions(t *testing.T) {
	complexity := reportComplexityResponse(9, 19, 1, 1, 3, 12, 25)
	// output.min_complexity dropped everything below 10 from the report list.
	complexity.Functions = complexity.Functions[3:]

	hist := buildReportHistogram(complexity, complexityRisk{Low: 9, Medium: 19, Known: true})

	if hist == nil {
		t.Fatal("expected a histogram")
	}
	if hist.Total != "5" {
		t.Errorf("histogram total = %q, want the whole population", hist.Total)
	}
	plotted := 0
	for _, bin := range hist.Bins {
		plotted += bin.Count
	}
	if plotted != 5 {
		t.Errorf("plotted %d functions, want the whole population of 5", plotted)
	}

	facts := reportFacts(hist)
	if facts["Median complexity"] != "CC 3" {
		t.Errorf("median fact = %q, want the median of the whole population", facts["Median complexity"])
	}
	// Naming the longest of a filtered list as the longest of the project
	// would be wrong, so the per-function facts drop out instead.
	if _, named := facts["Longest function"]; named {
		t.Errorf("expected no per-function facts on a filtered list, got %+v", facts)
	}
}

func TestBuildReportHistogramWithoutTheDataItNeeds(t *testing.T) {
	withoutFunctions := reportComplexityResponse(9, 19)
	if hist := buildReportHistogram(withoutFunctions, complexityRisk{Low: 9, Medium: 19, Known: true}); hist != nil {
		t.Error("expected no histogram when no function was analyzed")
	}
	// Without the thresholds the buckets have no honest edges to fall on.
	withoutConfig := reportComplexityResponse(9, 19, 1, 12)
	withoutConfig.Config = nil
	if hist := buildReportHistogram(withoutConfig, readComplexityRisk(withoutConfig)); hist != nil {
		t.Error("expected no histogram when the run did not report its thresholds")
	}
}

func reportFacts(hist *reportHistogram) map[string]string {
	facts := map[string]string{}
	for _, fact := range hist.Facts {
		facts[fact.Key] = fact.Value
	}
	return facts
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

	hotspots := buildReportHotspots(modules, clone, complexityRisk{Low: 9, Medium: 19, Known: true})

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
	complexity.ModuleRollups = map[string]domain.ModuleComplexityMetrics{
		"src/a.ts": {LinesOfCode: 240, AnalyzedFunctionCount: 4, AverageComplexity: 11.75, MaxComplexity: 30, HighRiskFunctionCount: 1},
	}

	var buf bytes.Buffer
	formatter := NewOutputFormatter()
	results := domain.AnalysisResults{Files: domain.FileAccounting{Total: 4, Skipped: 1}, Complexity: complexity}
	if err := formatter.WriteHTML(results, &buf, 1200*time.Millisecond); err != nil {
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
	if err := formatter.WriteHTML(domain.AnalysisResults{DeadCode: deadCode}, &buf, time.Second); err != nil {
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

func TestReportProject_RelativePathsResolveToTheRealDirectory(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	modules := []domain.ModuleQualityMetrics{
		{FilePath: filepath.Join("sample", "a.go")},
		{FilePath: filepath.Join("sample", "nested", "b.go")},
	}

	name, root := reportProject(modules, nil)

	if name != "sample" {
		t.Fatalf("name = %q, want %q", name, "sample")
	}
	want := abbreviateHome(filepath.Join(cwd, "sample"))
	if root != want {
		t.Fatalf("root = %q, want %q", root, want)
	}
}
