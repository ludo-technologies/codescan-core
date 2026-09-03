package service

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/version"
)

//go:embed templates/analyze/report.html templates/analyze/report.css templates/analyze/report.js
var analyzeTemplateFS embed.FS

var (
	analyzeReportCSS = template.CSS(mustReadTemplateAsset("templates/analyze/report.css"))
	analyzeReportJS  = template.JS(mustReadTemplateAsset("templates/analyze/report.js"))

	analyzeReportTemplate = template.Must(
		template.New("report.html").
			Funcs(analyzeTemplateFuncs()).
			ParseFS(analyzeTemplateFS, "templates/analyze/report.html"),
	)
)

func mustReadTemplateAsset(name string) string {
	data, err := analyzeTemplateFS.ReadFile(name)
	if err != nil {
		panic(fmt.Sprintf("embedded template asset %s: %v", name, err))
	}
	return string(data)
}

func analyzeTemplateFuncs() template.FuncMap {
	return template.FuncMap{
		"join":      strings.Join,
		"add":       func(a, b int) int { return a + b },
		"sub":       func(a, b int) int { return a - b },
		"addf":      func(a, b float64) float64 { return a + b },
		"divf":      func(a, b float64) float64 { return a / b },
		"int":       func(t domain.CloneType) int { return int(t) },
		"percent":   formatPercent,
		"scoreBand": scoreBand,
		// Clone fragments are pointers with an optional location, so every
		// fragment accessor tolerates a missing one rather than panicking
		// halfway through rendering.
		"cloneFile":    cloneFile,
		"cloneLines":   cloneLines,
		"cloneContent": cloneContent,
		"lineSpan":     functionLineSpan,
	}
}

// WriteHTML writes the analysis result as a self-contained HTML report.
func (f *OutputFormatterImpl) WriteHTML(
	results domain.AnalysisResults,
	writer io.Writer,
	duration time.Duration,
) error {
	if results.Clone != nil {
		if results.Clone.Statistics == nil {
			results.Clone.Statistics = &domain.CloneStatistics{}
		}
		clonePairs := make([]*domain.ClonePair, 0, len(results.Clone.ClonePairs))
		for _, pair := range results.Clone.ClonePairs {
			if pair != nil {
				clonePairs = append(clonePairs, pair)
			}
		}
		results.Clone.ClonePairs = clonePairs
	}

	view := buildAnalyzeReportView(results, duration)
	if err := analyzeReportTemplate.Execute(writer, view); err != nil {
		return fmt.Errorf("failed to render HTML report: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// View model
// ---------------------------------------------------------------------------

const (
	reportHotspotLimit     = 8
	reportScoreRingCircumf = 351.86 // 2 * pi * r for r=56
)

// analyzeReportView is the data handed to the report template. It carries the
// raw responses so the detail tabs can reach them, plus the pre-computed
// overview blocks so the template stays free of arithmetic.
type analyzeReportView struct {
	CSS template.CSS
	JS  template.JS

	GeneratedAt time.Time
	Duration    int64
	Version     string

	Complexity    *domain.ComplexityResponse
	DeadCode      *domain.DeadCodeResponse
	Clone         *domain.CloneResponse
	CBO           *domain.CBOResponse
	Deps          *domain.DependencyGraphResponse
	ModuleQuality []domain.ModuleQualityMetrics
	Summary       *domain.AnalyzeSummary

	ProjectName string
	ProjectPath string
	ScoreBand   string
	RingOffset  float64

	Tabs        []reportTab
	Verdict     reportVerdict
	Facts       []reportFact
	Dimensions  []reportDimension
	Hotspots    []reportHotspot
	Histogram   *reportHistogram
	Duplication *reportDuplication
	Classes     *reportClasses
	Structure   *reportStructure

	ProjectScale    string
	SkippedFiles    int
	ShowFunctions   bool
	ShowDeadColumn  bool
	ShowCloneColumn bool
}

type reportTab struct {
	ID        string
	Label     string
	Count     int
	CountBand string // "", "warn", "bad"
}

type reportVerdict struct {
	Headline string
	Body     []reportSegment
}

// reportSegment is a run of verdict text; Strong runs are emphasized.
type reportSegment struct {
	Text   string
	Strong bool
}

type reportFact struct {
	Value string
	Label string
}

type reportDimension struct {
	Name  string
	Score int
	Band  string
	Left  string
	Right string
	Tab   string
}

type reportHotspot struct {
	Dir       string
	File      string
	Lines     string
	Functions int
	MaxCC     int
	MaxCCPct  int
	MaxCCBand string
	HighRisk  int
	DeadCode  int
	Clones    int
}

type reportHistogram struct {
	Total      string
	Bins       []reportHistogramBin
	Ticks      []reportHistogramTick
	ThresholdX float64
	Threshold  string
	Facts      []reportKV
}

type reportHistogramBin struct {
	Label  string
	Count  int
	X      float64
	Y      float64
	Width  float64
	Height float64
	Band   string // "", "warn", "bad"
}

type reportHistogramTick struct {
	Label string
	Y     float64
}

type reportKV struct {
	Key   string
	Value string
	Band  string
	Mono  bool
}

type reportDuplication struct {
	Percent   float64
	Fragments int
	Types     []reportShare
	Facts     []reportKV
}

type reportShare struct {
	Label   string
	Percent float64
	Class   string
}

type reportClasses struct {
	Total int
	Facts []reportKV
}

type reportStructure struct {
	Cycles int
	Facts  []reportKV
}

// scoreBand maps a 0-100 score onto the report's three semantic colors.
func scoreBand(score int) string {
	switch {
	case score >= domain.ScoreThresholdGood:
		return "ok"
	case score >= domain.ScoreThresholdFair:
		return "watch"
	default:
		return "poor"
	}
}

func buildAnalyzeReportView(
	results domain.AnalysisResults,
	duration time.Duration,
) *analyzeReportView {
	// Reuse the shared summary and rollup builders so the report can never
	// disagree with the text, JSON, or CSV output about a score.
	summary := BuildAnalyzeSummary(results)
	moduleQuality := BuildModuleQuality(results.Complexity, results.DeadCode, results.Deps)

	view := &analyzeReportView{
		CSS:             analyzeReportCSS,
		JS:              analyzeReportJS,
		GeneratedAt:     time.Now(),
		Duration:        duration.Milliseconds(),
		Version:         version.Version,
		Complexity:      results.Complexity,
		DeadCode:        results.DeadCode,
		Clone:           results.Clone,
		CBO:             results.CBO,
		Deps:            results.Deps,
		ModuleQuality:   moduleQuality,
		Summary:         summary,
		ScoreBand:       scoreBand(summary.HealthScore),
		RingOffset:      reportScoreRingCircumf * (1 - float64(clampScore(summary.HealthScore))/100),
		ProjectScale:    FormatProjectScale(summary),
		SkippedFiles:    summary.SkippedFiles,
		ShowFunctions:   reportHasFunctions(summary, results.Complexity, moduleQuality),
		ShowDeadColumn:  summary.DeadCodeEnabled,
		ShowCloneColumn: summary.CloneEnabled,
	}
	risk := readComplexityRisk(results.Complexity)
	view.ProjectName, view.ProjectPath = reportProject(moduleQuality, results.Complexity)
	view.Tabs = buildReportTabs(summary, results.Complexity, moduleQuality, results.Deps)
	view.Dimensions = buildReportDimensions(summary, view.Tabs)
	view.Verdict = buildReportVerdict(summary, view.Dimensions)
	view.Facts = buildReportFacts(summary, moduleQuality)
	view.Hotspots = buildReportHotspots(moduleQuality, results.Clone, risk)
	view.Histogram = buildReportHistogram(results.Complexity, risk)
	view.Duplication = buildReportDuplication(summary, results.Clone)
	view.Classes = buildReportClasses(summary, results.CBO)
	view.Structure = buildReportStructure(results.Deps)
	return view
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

// reportProject derives a project label from the analyzed file paths: the
// responses carry no explicit root, so the common directory of everything that
// was analyzed is the closest thing to one. Paths are made absolute first so
// a relative target such as "polyscan/" yields a real directory instead of a
// path that merely repeats the project name.
func reportProject(
	moduleQuality []domain.ModuleQualityMetrics,
	complexity *domain.ComplexityResponse,
) (name, root string) {
	files := make([]string, 0, len(moduleQuality))
	for _, module := range moduleQuality {
		files = append(files, module.FilePath)
	}
	if len(files) == 0 && complexity != nil {
		for _, function := range complexity.Functions {
			files = append(files, function.FilePath)
		}
	}
	for i, file := range files {
		absolute, err := filepath.Abs(file)
		if err != nil {
			return "", ""
		}
		files[i] = absolute
	}
	root = commonDirectory(files)
	if root == "" || root == "." || root == string(filepath.Separator) {
		return "", ""
	}
	return filepath.Base(root), abbreviateHome(root)
}

// abbreviateHome swaps the user's home directory for "~" so the report header
// stays short and does not leak the account name when the file is shared.
func abbreviateHome(p string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" || !strings.HasPrefix(p, home) {
		return p
	}
	return "~" + strings.TrimPrefix(p, home)
}

func commonDirectory(files []string) string {
	if len(files) == 0 {
		return ""
	}
	separator := string(filepath.Separator)
	prefix := strings.Split(filepath.Dir(files[0]), separator)
	for _, file := range files[1:] {
		parts := strings.Split(filepath.Dir(file), separator)
		n := 0
		for n < len(prefix) && n < len(parts) && prefix[n] == parts[n] {
			n++
		}
		prefix = prefix[:n]
		if len(prefix) == 0 {
			return ""
		}
	}
	return strings.Join(prefix, separator)
}

// reportHasFunctions reports whether the Functions tab has anything to show:
// the function-level analyses or the per-file rollups derived from them.
func reportHasFunctions(
	summary *domain.AnalyzeSummary,
	complexity *domain.ComplexityResponse,
	moduleQuality []domain.ModuleQualityMetrics,
) bool {
	if summary.ComplexityEnabled || summary.DeadCodeEnabled || len(moduleQuality) > 0 {
		return true
	}
	return complexity != nil && len(complexity.ByDirectory) > 0
}

func buildReportTabs(
	summary *domain.AnalyzeSummary,
	complexity *domain.ComplexityResponse,
	moduleQuality []domain.ModuleQualityMetrics,
	deps *domain.DependencyGraphResponse,
) []reportTab {
	tabs := []reportTab{{ID: "overview", Label: "Overview"}}

	if reportHasFunctions(summary, complexity, moduleQuality) {
		tab := reportTab{ID: "functions", Label: "Functions", Count: summary.HighComplexityCount + summary.DeadCodeCount}
		switch {
		case summary.HighComplexityCount > 0 || summary.CriticalDeadCode > 0:
			tab.CountBand = "bad"
		case tab.Count > 0:
			tab.CountBand = "warn"
		}
		tabs = append(tabs, tab)
	}
	if summary.CloneEnabled {
		tab := reportTab{ID: "duplication", Label: "Duplication", Count: summary.CloneGroups}
		if tab.Count == 0 {
			tab.Count = summary.ClonePairs
		}
		if tab.Count > 0 {
			tab.CountBand = "warn"
		}
		tabs = append(tabs, tab)
	}
	if summary.CBOEnabled {
		tab := reportTab{ID: "classes", Label: "Classes", Count: summary.HighCouplingClasses}
		if tab.Count > 0 {
			tab.CountBand = "bad"
		}
		tabs = append(tabs, tab)
	}
	if reportHasArchitecture(deps) {
		tab := reportTab{ID: "architecture", Label: "Architecture"}
		if deps.Analysis.CircularDependencies != nil {
			tab.Count = deps.Analysis.CircularDependencies.TotalCycles
		}
		if tab.Count > 0 {
			tab.CountBand = "bad"
		}
		tabs = append(tabs, tab)
	}
	return tabs
}

// reportHasArchitecture reports whether dependency analysis produced the result
// the Architecture tab renders. The graph alone is not enough: every block on
// that tab reads from the analysis.
func reportHasArchitecture(deps *domain.DependencyGraphResponse) bool {
	return deps != nil && deps.Analysis != nil
}

func buildReportDimensions(summary *domain.AnalyzeSummary, tabs []reportTab) []reportDimension {
	rendered := make(map[string]bool, len(tabs))
	for _, tab := range tabs {
		rendered[tab.ID] = true
	}

	var dims []reportDimension
	// A dimension can be scored while its detail tab is absent (dependency
	// scoring runs off the summary, but the tab needs the analysis result), so
	// only link cards whose target tab is actually rendered.
	add := func(name string, score int, left, right, tab string) {
		if !rendered[tab] {
			tab = ""
		}
		dims = append(dims, reportDimension{Name: name, Score: score, Band: scoreBand(score), Left: left, Right: right, Tab: tab})
	}

	if summary.ComplexityEnabled {
		add("Complexity", summary.ComplexityScore,
			fmt.Sprintf("avg CC %.2f", summary.AverageComplexity),
			fmt.Sprintf("%d high-risk", summary.HighComplexityCount), "functions")
	}
	if summary.DeadCodeEnabled {
		add("Dead code", summary.DeadCodeScore,
			pluralize(summary.DeadCodeCount, "finding", "findings"),
			fmt.Sprintf("%d critical", summary.CriticalDeadCode), "functions")
	}
	if summary.CloneEnabled {
		add("Duplication", summary.DuplicationScore,
			fmt.Sprintf("%.1f%% of fragments", summary.CodeDuplication),
			pluralize(summary.CloneGroups, "group", "groups"), "duplication")
	}
	if summary.CBOEnabled {
		add("Coupling", summary.CouplingScore,
			fmt.Sprintf("avg CBO %.1f", summary.AverageCoupling),
			fmt.Sprintf("%d of %d high", summary.HighCouplingClasses, summary.CBOClasses), "classes")
	}
	if summary.DepsEnabled {
		cycles := "no cycles"
		if summary.DepsModulesInCycles > 0 {
			cycles = fmt.Sprintf("%d modules in cycles", summary.DepsModulesInCycles)
		}
		add("Dependencies", summary.DependencyScore, cycles, fmt.Sprintf("depth %d", summary.DepsMaxDepth), "architecture")
	}
	return dims
}

func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func buildReportVerdict(summary *domain.AnalyzeSummary, dims []reportDimension) reportVerdict {
	verdict := reportVerdict{Headline: gradeHeadline(summary.Grade)}

	var clean, weak []reportDimension
	for _, dim := range dims {
		switch {
		case dim.Score >= domain.ScoreThresholdExcellent:
			clean = append(clean, dim)
		case dim.Score < domain.ScoreThresholdGood:
			weak = append(weak, dim)
		}
	}
	sort.SliceStable(weak, func(i, j int) bool { return weak[i].Score < weak[j].Score })

	text := func(s string) { verdict.Body = append(verdict.Body, reportSegment{Text: s}) }
	strong := func(s string) { verdict.Body = append(verdict.Body, reportSegment{Text: s, Strong: true}) }

	if skipped := summary.SkippedFiles; skipped > 0 {
		strong(fmt.Sprintf("%s of %d could not be parsed", pluralize(skipped, "file", "files"), summary.TotalFiles))
		text(" and were skipped; the health score is penalized for them. ")
	}

	switch {
	case len(dims) == 0:
		text("No analyses were enabled for this run.")
		return verdict
	case len(clean) == len(dims):
		files := pluralize(summary.TotalFiles, "file", "files")
		if len(dims) == 1 {
			text(fmt.Sprintf("%s scores %d/100 across %s.", joinNames(lowerNames(dims)), dims[0].Score, files))
		} else {
			text(fmt.Sprintf("All %d dimensions score %d or above across %s.", len(dims), domain.ScoreThresholdExcellent, files))
		}
		return verdict
	case len(clean) > 0:
		text(joinNames(lowerNames(clean)) + " " + isAre(len(clean)) + " clean. ")
	}

	if len(weak) == 0 {
		text(fmt.Sprintf("No dimension scores below %d.", domain.ScoreThresholdGood))
		return verdict
	}
	if len(weak) > 3 {
		weak = weak[:3]
	}
	if len(clean) > 0 {
		text("Most of the remaining debt is in ")
	} else {
		text("Most of the debt is in ")
	}
	for i, dim := range weak {
		if i > 0 {
			if i == len(weak)-1 {
				text(" and ")
			} else {
				text(", ")
			}
		}
		strong(strings.ToLower(dim.Name))
		text(fmt.Sprintf(" (%s, %s)", dim.Left, dim.Right))
	}
	text(".")
	return verdict
}

func gradeHeadline(grade string) string {
	switch strings.ToUpper(grade) {
	case "A":
		return "Healthy codebase"
	case "B":
		return "Good shape overall"
	case "C":
		return "Fair, with clear debt to pay down"
	case "D":
		return "Quality needs attention"
	case "F":
		return "Serious quality problems"
	default:
		// CalculateHealthScore rejected the summary and graded it N/A.
		return "Health score unavailable"
	}
}

func lowerNames(dims []reportDimension) []string {
	names := make([]string, 0, len(dims))
	for _, dim := range dims {
		names = append(names, strings.ToLower(dim.Name))
	}
	return names
}

// joinNames renders "A", "A and B", or "A, B, and C" with the first name
// capitalized for sentence position.
func joinNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	first := strings.ToUpper(names[0][:1]) + names[0][1:]
	switch len(names) {
	case 1:
		return first
	case 2:
		return first + " and " + names[1]
	default:
		return first + ", " + strings.Join(names[1:len(names)-1], ", ") + ", and " + names[len(names)-1]
	}
}

func isAre(n int) string {
	if n == 1 {
		return "is"
	}
	return "are"
}

func buildReportFacts(summary *domain.AnalyzeSummary, moduleQuality []domain.ModuleQualityMetrics) []reportFact {
	var facts []reportFact
	lines := 0
	for _, module := range moduleQuality {
		lines += module.LinesOfCode
	}
	if lines == 0 {
		lines = summary.TotalLOC
	}
	if lines > 0 {
		facts = append(facts, reportFact{Value: formatThousands(lines), Label: "lines"})
	}
	facts = append(facts, reportFact{Value: formatThousands(summary.TotalFiles), Label: "files"})
	if summary.ComplexityEnabled {
		facts = append(facts, reportFact{Value: formatThousands(summary.TotalFunctions), Label: "functions"})
	}
	if summary.CBOEnabled {
		facts = append(facts, reportFact{Value: formatThousands(summary.CBOClasses), Label: "classes"})
	}
	return facts
}

// formatPercent renders a 0-1 ratio as a percentage. Similarity is reported
// this way everywhere in the report: a bare 0.95 reads as neither a score nor
// a share.
func formatPercent(ratio float64) string {
	return fmt.Sprintf("%.1f%%", ratio*100)
}

func formatThousands(n int) string {
	digits := fmt.Sprintf("%d", n)
	if len(digits) <= 3 {
		return digits
	}
	var builder strings.Builder
	head := len(digits) % 3
	if head > 0 {
		builder.WriteString(digits[:head])
	}
	for i := head; i < len(digits); i += 3 {
		if builder.Len() > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(digits[i : i+3])
	}
	return builder.String()
}

func buildReportHotspots(
	moduleQuality []domain.ModuleQualityMetrics,
	clone *domain.CloneResponse,
	risk complexityRisk,
) []reportHotspot {
	if len(moduleQuality) == 0 {
		return nil
	}
	clonesByFile := countClonesByFile(clone)

	modules := make([]domain.ModuleQualityMetrics, len(moduleQuality))
	copy(modules, moduleQuality)
	sort.SliceStable(modules, func(i, j int) bool {
		a, b := modules[i], modules[j]
		if a.HighRiskFunctionCount != b.HighRiskFunctionCount {
			return a.HighRiskFunctionCount > b.HighRiskFunctionCount
		}
		if a.MaxComplexity != b.MaxComplexity {
			return a.MaxComplexity > b.MaxComplexity
		}
		if a.DeadCodeFindingCount != b.DeadCodeFindingCount {
			return a.DeadCodeFindingCount > b.DeadCodeFindingCount
		}
		if clonesByFile[a.FilePath] != clonesByFile[b.FilePath] {
			return clonesByFile[a.FilePath] > clonesByFile[b.FilePath]
		}
		return a.LinesOfCode > b.LinesOfCode
	})
	if len(modules) > reportHotspotLimit {
		modules = modules[:reportHotspotLimit]
	}

	maxCC := 1
	for _, module := range modules {
		maxCC = max(maxCC, module.MaxComplexity)
	}

	rows := make([]reportHotspot, 0, len(modules))
	for _, module := range modules {
		dir, file := filepath.Split(module.FilePath)
		rows = append(rows, reportHotspot{
			Dir:       dir,
			File:      file,
			Lines:     formatThousands(module.LinesOfCode),
			Functions: module.AnalyzedFunctionCount,
			MaxCC:     module.MaxComplexity,
			MaxCCPct:  module.MaxComplexity * 100 / maxCC,
			MaxCCBand: risk.band(module.MaxComplexity),
			HighRisk:  module.HighRiskFunctionCount,
			DeadCode:  module.DeadCodeFindingCount,
			Clones:    clonesByFile[module.FilePath],
		})
	}
	return rows
}

// complexityRisk is the pair of thresholds the run was configured with: the
// highest complexity still rated low risk, and the highest still rated medium.
// Known is false when the response did not report them, and then nothing is
// banded rather than banded against a default the project may have overridden.
type complexityRisk struct {
	Low    int
	Medium int
	Known  bool
}

// readComplexityRisk takes the thresholds from the config the analysis
// reported. Inferring them from the risk labels of the listed functions would
// be wrong whenever a report filter removed the functions that sit on a
// boundary.
func readComplexityRisk(complexity *domain.ComplexityResponse) complexityRisk {
	if complexity == nil {
		return complexityRisk{}
	}
	config, ok := complexity.Config.(map[string]interface{})
	if !ok {
		return complexityRisk{}
	}
	low, lowOK := config["low_threshold"].(int)
	medium, mediumOK := config["medium_threshold"].(int)
	if !lowOK || !mediumOK || low < 1 || medium < low {
		return complexityRisk{}
	}
	return complexityRisk{Low: low, Medium: medium, Known: true}
}

func (risk complexityRisk) band(complexity int) string {
	switch {
	case !risk.Known:
		return ""
	case complexity > risk.Medium:
		return "bad"
	case complexity > risk.Low:
		return "warn"
	default:
		return ""
	}
}

// countClonesByFile counts clone fragments per file. Groups are authoritative
// when present; otherwise pairs are counted, each fragment once.
func countClonesByFile(clone *domain.CloneResponse) map[string]int {
	counts := make(map[string]int)
	if clone == nil {
		return counts
	}
	if len(clone.CloneGroups) > 0 {
		for _, group := range clone.CloneGroups {
			if group == nil {
				continue
			}
			for _, fragment := range group.Clones {
				if fragment != nil && fragment.Location != nil {
					counts[fragment.Location.FilePath]++
				}
			}
		}
		return counts
	}
	// A fragment can sit in several pairs; count it once, keyed by its span, to
	// match how CloneStatistics.TotalClones deduplicates.
	seen := make(map[string]struct{})
	for _, pair := range clone.ClonePairs {
		for _, fragment := range []*domain.Clone{pair.Clone1, pair.Clone2} {
			if fragment == nil || fragment.Location == nil {
				continue
			}
			key := fmt.Sprintf("%s:%d-%d", fragment.Location.FilePath, fragment.Location.StartLine, fragment.Location.EndLine)
			if _, duplicate := seen[key]; duplicate {
				continue
			}
			seen[key] = struct{}{}
			counts[fragment.Location.FilePath]++
		}
	}
	return counts
}

// Histogram geometry (SVG user units).
const (
	histLeft       = 44.0
	histRight      = 412.0
	histTop        = 30.0
	histBaseline   = 150.0
	histBarFill    = 0.72
	histTickCount  = 3
	histLabelSpace = 6.0
)

type histogramBin struct {
	label string
	upper int    // inclusive; 0 means open-ended
	band  string // "", "warn", "bad"
}

// histogramBins builds buckets that follow the run's own risk thresholds:
// 1 | 2–5 | 6–low | low+1–medium | medium+1+, collapsing the buckets the
// thresholds make empty (a low threshold of 1 removes both middle buckets).
// Bucket edges that fall on the thresholds are what lets a bucket carry a
// single risk color honestly.
func histogramBins(risk complexityRisk) []histogramBin {
	if risk.Medium <= risk.Low {
		risk.Medium = risk.Low + 1
	}
	candidates := []int{1, risk.Low, risk.Medium}
	if risk.Low > 5 {
		candidates = []int{1, 5, risk.Low, risk.Medium}
	}
	uppers := []int{}
	for _, upper := range candidates {
		if len(uppers) == 0 || upper > uppers[len(uppers)-1] {
			uppers = append(uppers, upper)
		}
	}
	bins := make([]histogramBin, 0, len(uppers)+1)
	previous := 0
	for _, upper := range uppers {
		bin := histogramBin{upper: upper, band: risk.band(upper)}
		if upper == previous+1 {
			bin.label = fmt.Sprintf("%d", upper)
		} else {
			bin.label = fmt.Sprintf("%d–%d", previous+1, upper)
		}
		bins = append(bins, bin)
		previous = upper
	}
	return append(bins, histogramBin{label: fmt.Sprintf("%d+", previous+1), band: "bad"})
}

// buildReportHistogram plots the complete analyzed population from the
// distribution the summary carries. complexity.Functions is the filtered
// report list, so counting it would contradict the function total, the median,
// and every score in the same report.
func buildReportHistogram(complexity *domain.ComplexityResponse, risk complexityRisk) *reportHistogram {
	if complexity == nil || !risk.Known {
		return nil
	}
	distribution := complexity.Summary.ComplexityDistribution
	total := 0
	for _, count := range distribution {
		total += count
	}
	if total == 0 {
		return nil
	}
	defs := histogramBins(risk)

	counts := make([]int, len(defs))
	for cc, count := range distribution {
		counts[binIndex(defs, cc)] += count
	}

	maxCount := 1
	for _, count := range counts {
		maxCount = max(maxCount, count)
	}
	niceMax := niceCeiling(maxCount)
	scale := (histBaseline - histTop) / float64(niceMax)

	slot := (histRight - histLeft) / float64(len(defs))
	barWidth := slot * histBarFill
	hist := &reportHistogram{Total: formatThousands(total)}
	for i, def := range defs {
		height := float64(counts[i]) * scale
		if counts[i] > 0 && height < 1 {
			height = 1
		}
		x := histLeft + slot*float64(i) + (slot-barWidth)/2
		hist.Bins = append(hist.Bins, reportHistogramBin{
			Label:  def.label,
			Count:  counts[i],
			X:      round1(x),
			Y:      round1(histBaseline - height),
			Width:  round1(barWidth),
			Height: round1(height),
			Band:   def.band,
		})
		if def.band == "warn" && hist.Threshold == "" {
			hist.ThresholdX = round1(histLeft + slot*float64(i) - histLabelSpace/2)
			hist.Threshold = fmt.Sprintf("risk from CC %d", risk.Low+1)
		}
	}
	for i := 0; i <= histTickCount; i++ {
		value := niceMax * i / histTickCount
		hist.Ticks = append(hist.Ticks, reportHistogramTick{
			Label: formatThousands(value),
			Y:     round1(histBaseline - float64(value)*scale),
		})
	}

	hist.Facts = append(hist.Facts, reportKV{
		Key:   "Median complexity",
		Value: fmt.Sprintf("CC %s", formatMedian(medianOfDistribution(distribution, total))),
	})
	// The two facts below need a function each, and complexity.Functions is the
	// filtered report list. Naming the worst of a filtered list as the worst of
	// the project would be wrong, so they are reported only when the list is
	// the whole population.
	if len(complexity.Functions) == total {
		deepest, longest := extremeFunctions(complexity.Functions)
		// A zero maximum says nothing — either nothing nests or, for the
		// languages the generic engine covers, nesting is not measured.
		if deepest.Metrics.NestingDepth > 0 {
			hist.Facts = append(hist.Facts, reportKV{
				Key:   "Deepest nesting",
				Value: fmt.Sprintf("%d levels (%s)", deepest.Metrics.NestingDepth, deepest.Name),
			})
		}
		hist.Facts = append(hist.Facts, reportKV{
			Key:   "Longest function",
			Value: fmt.Sprintf("%d lines (%s)", functionLineSpan(*longest), longest.Name),
		})
	}
	return hist
}

// extremeFunctions returns the most deeply nested and the longest function.
// The caller guarantees a non-empty population.
func extremeFunctions(functions []domain.FunctionComplexity) (deepest, longest *domain.FunctionComplexity) {
	for i := range functions {
		function := &functions[i]
		if deepest == nil || function.Metrics.NestingDepth > deepest.Metrics.NestingDepth {
			deepest = function
		}
		if longest == nil || functionLineSpan(*function) > functionLineSpan(*longest) {
			longest = function
		}
	}
	return deepest, longest
}

func binIndex(bins []histogramBin, cc int) int {
	for i, def := range bins {
		if def.upper == 0 || cc <= def.upper {
			return i
		}
	}
	return len(bins) - 1
}

// functionLineSpan is how many source lines a function occupies. jscan has no
// SLOC metric, so the span between its first and last line is the closest
// honest measure of length.
func functionLineSpan(function domain.FunctionComplexity) int {
	if function.EndLine < function.StartLine {
		return 0
	}
	return function.EndLine - function.StartLine + 1
}

// niceCeiling rounds n up to a tidy axis maximum so ticks land on round numbers.
func niceCeiling(n int) int {
	if n <= 3 {
		return 3
	}
	magnitude := 1
	for n/magnitude >= 10 {
		magnitude *= 10
	}
	for _, step := range []int{1, 2, 3, 5, 6, 10} {
		if candidate := step * magnitude; candidate >= n {
			return candidate
		}
	}
	return magnitude * 10
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

// medianOfDistribution finds the median of a population counted by value,
// walking the values in order until half the population is behind it.
func medianOfDistribution(distribution map[int]int, total int) float64 {
	if total == 0 {
		return 0
	}
	values := make([]int, 0, len(distribution))
	for value := range distribution {
		values = append(values, value)
	}
	sort.Ints(values)

	lower, upper := (total-1)/2, total/2
	seen, low := 0, 0
	for _, value := range values {
		seen += distribution[value]
		if low == 0 && seen > lower {
			low = value
		}
		if seen > upper {
			return float64(low+value) / 2
		}
	}
	return float64(low)
}

// formatMedian prints whole medians without a decimal and half values with one.
func formatMedian(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func buildReportDuplication(summary *domain.AnalyzeSummary, clone *domain.CloneResponse) *reportDuplication {
	if !summary.CloneEnabled || clone == nil || clone.Statistics == nil {
		return nil
	}
	stats := clone.Statistics
	dup := &reportDuplication{Percent: summary.CodeDuplication, Fragments: stats.TotalFragments}

	byType := make(map[domain.CloneType]int)
	files := make(map[string]struct{})
	unit := "fragments"
	if len(clone.CloneGroups) > 0 {
		for _, group := range clone.CloneGroups {
			if group == nil {
				continue
			}
			for _, fragment := range group.Clones {
				if fragment == nil {
					continue
				}
				byType[group.Type]++
				if fragment.Location != nil {
					files[fragment.Location.FilePath] = struct{}{}
				}
			}
		}
	} else {
		unit = "pairs"
		for _, pair := range clone.ClonePairs {
			byType[pair.Type]++
			for _, fragment := range []*domain.Clone{pair.Clone1, pair.Clone2} {
				if fragment != nil && fragment.Location != nil {
					files[fragment.Location.FilePath] = struct{}{}
				}
			}
		}
	}
	total := 0
	for _, count := range byType {
		total += count
	}
	for _, cloneType := range []domain.CloneType{domain.Type1Clone, domain.Type2Clone, domain.Type3Clone, domain.Type4Clone} {
		count := byType[cloneType]
		if count == 0 {
			continue
		}
		dup.Types = append(dup.Types, reportShare{
			Label:   fmt.Sprintf("%s %s · %d %s", cloneType.String(), cloneTypeNoun(cloneType), count, unit),
			Percent: float64(count) * 100 / float64(total),
			Class:   fmt.Sprintf("t%d", int(cloneType)),
		})
	}

	dup.Facts = []reportKV{
		{Key: "Clone groups", Value: formatThousands(stats.TotalCloneGroups)},
		{Key: "Clone pairs", Value: formatThousands(stats.TotalClonePairs)},
	}
	if stats.TotalClonePairs > 0 || stats.TotalCloneGroups > 0 {
		dup.Facts = append(dup.Facts, reportKV{Key: "Avg similarity", Value: formatPercent(stats.AverageSimilarity)})
	}
	if stats.FilesAnalyzed > 0 {
		dup.Facts = append(dup.Facts, reportKV{Key: "Files with clones", Value: fmt.Sprintf("%d of %d", len(files), stats.FilesAnalyzed)})
	}
	return dup
}

func cloneTypeNoun(cloneType domain.CloneType) string {
	switch cloneType {
	case domain.Type1Clone:
		return "identical"
	case domain.Type2Clone:
		return "renamed"
	case domain.Type3Clone:
		return "modified"
	case domain.Type4Clone:
		return "semantic"
	default:
		return ""
	}
}

func cloneFile(fragment *domain.Clone) string {
	if fragment == nil || fragment.Location == nil {
		return "unknown"
	}
	return fragment.Location.FilePath
}

func cloneLines(fragment *domain.Clone) string {
	if fragment == nil || fragment.Location == nil {
		return "—"
	}
	return fmt.Sprintf("%d-%d", fragment.Location.StartLine, fragment.Location.EndLine)
}

// cloneContent returns the first few lines of a fragment, or "" when the run
// did not capture source content.
func cloneContent(fragment *domain.Clone) string {
	if fragment == nil || fragment.Content == "" {
		return ""
	}
	const maxLines = 8
	lines := strings.Split(fragment.Content, "\n")
	if len(lines) <= maxLines {
		return fragment.Content
	}
	return strings.Join(lines[:maxLines], "\n") + "\n..."
}

func buildReportClasses(summary *domain.AnalyzeSummary, cbo *domain.CBOResponse) *reportClasses {
	if !summary.CBOEnabled || cbo == nil {
		return nil
	}
	classes := &reportClasses{
		Total: cbo.Summary.TotalClasses,
		Facts: []reportKV{
			{Key: "High coupling", Value: formatThousands(cbo.Summary.HighRiskClasses), Band: warnIfPositive(cbo.Summary.HighRiskClasses)},
			{Key: "Medium coupling", Value: formatThousands(cbo.Summary.MediumRiskClasses)},
			{Key: "Average CBO", Value: fmt.Sprintf("%.2f", cbo.Summary.AverageCBO)},
		},
	}
	if top := mostCoupledClass(cbo.Classes); top != nil {
		classes.Facts = append(classes.Facts, reportKV{
			Key:   "Most coupled",
			Value: fmt.Sprintf("%s (%d)", top.Name, top.Metrics.CouplingCount),
			Mono:  true,
		})
	}
	return classes
}

func warnIfPositive(n int) string {
	if n > 0 {
		return "warn"
	}
	return "good"
}

func mostCoupledClass(classes []domain.ClassCoupling) *domain.ClassCoupling {
	var top *domain.ClassCoupling
	for i := range classes {
		if top == nil || classes[i].Metrics.CouplingCount > top.Metrics.CouplingCount {
			top = &classes[i]
		}
	}
	return top
}

func buildReportStructure(deps *domain.DependencyGraphResponse) *reportStructure {
	if !reportHasArchitecture(deps) {
		return nil
	}
	analysis := deps.Analysis
	structure := &reportStructure{
		Facts: []reportKV{
			{Key: "Modules / edges", Value: fmt.Sprintf("%d / %d", analysis.TotalModules, analysis.TotalDependencies)},
			{Key: "Max dependency depth", Value: formatThousands(analysis.MaxDepth)},
		},
	}
	if analysis.CircularDependencies != nil {
		structure.Cycles = analysis.CircularDependencies.TotalCycles
	}
	if analysis.CouplingAnalysis != nil {
		structure.Facts = append(structure.Facts,
			reportKV{Key: "Avg instability", Value: fmt.Sprintf("%.2f", analysis.CouplingAnalysis.AverageInstability)},
			reportKV{Key: "Main sequence deviation", Value: fmt.Sprintf("%.2f", analysis.CouplingAnalysis.MainSequenceDeviation)},
		)
	}
	return structure
}
