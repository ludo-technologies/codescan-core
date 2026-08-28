// Package analysis collects source files, dispatches each to its language
// and aggregates the per-function results into a report.
package analysis

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"

	"github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/core/source"
	"github.com/ludo-technologies/polyscan/polyscan/internal/clone"
	"github.com/ludo-technologies/polyscan/polyscan/internal/engine"
	"github.com/ludo-technologies/polyscan/polyscan/internal/lang"
)

// Options selects the analyses to run.
type Options struct {
	Complexity bool
	Clones     bool
}

// Function is the complexity result for one function.
type Function struct {
	Name        string `json:"name"`
	FilePath    string `json:"file_path"`
	Language    string `json:"language"`
	StartLine   int    `json:"start_line"`
	StartColumn int    `json:"start_column"`
	EndLine     int    `json:"end_line"`
	// Complexity is the McCabe cyclomatic complexity.
	Complexity int `json:"complexity"`
	// Decisions breaks the decision points down by kind. The kinds are
	// defined per language.
	Decisions map[string]int   `json:"decisions"`
	RiskLevel domain.RiskLevel `json:"risk_level"`
}

// ComplexitySummary aggregates every analyzed function. Report filters only
// limit which functions are listed; they never change these numbers.
type ComplexitySummary struct {
	TotalFunctions    int     `json:"total_functions"`
	AverageComplexity float64 `json:"average_complexity"`
	MaxComplexity     int     `json:"max_complexity"`
	MinComplexity     int     `json:"min_complexity"`

	LowRiskFunctions    int `json:"low_risk_functions"`
	MediumRiskFunctions int `json:"medium_risk_functions"`
	HighRiskFunctions   int `json:"high_risk_functions"`
}

// Complexity is the complexity analysis of a set of paths.
type Complexity struct {
	// Functions is sorted by descending complexity, then by location.
	Functions []Function        `json:"functions"`
	Summary   ComplexitySummary `json:"summary"`
}

// Files counts the files a run covered. Skipped files could not be read
// and are absent from every metric; partial files had a syntax error, and
// the functions containing it are absent. A consumer must read this before
// trusting the aggregates.
type Files struct {
	Total    int `json:"total"`
	Analyzed int `json:"analyzed"`
	Partial  int `json:"partial"`
	Skipped  int `json:"skipped"`
}

// Report is the complete analysis of a set of paths.
type Report struct {
	Files      Files         `json:"files"`
	Complexity *Complexity   `json:"complexity,omitempty"`
	Clones     *clone.Report `json:"clone,omitempty"`
	// Warnings lists, per partial file, its syntax error.
	Warnings []string `json:"warnings,omitempty"`
	// Errors lists, per skipped file, why it was skipped.
	Errors []string `json:"errors,omitempty"`
}

// Analyze collects every supported source file under paths and runs the
// selected analyses on it. A file that cannot be read is skipped and
// reported in Errors, a file with a syntax error is analyzed without the
// functions that contain it and reported in Warnings, and finding no
// supported file at all is an error.
func Analyze(paths []string, options Options) (*Report, error) {
	files, err := source.CollectFiles(paths, source.FileFilter{
		IncludePatterns: lang.IncludePatterns(),
		Recursive:       true,
	})
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no supported source files found")
	}

	report := &Report{Files: Files{Total: len(files)}}
	if options.Complexity {
		report.Complexity = &Complexity{Functions: []Function{}}
	}
	detectors := map[*engine.Language]*clone.Detector{}

	for _, file := range files {
		language, ok := lang.ByPath(file)
		if !ok {
			// CollectFiles only returns registered extensions.
			panic(fmt.Sprintf("no language for %s", file))
		}
		display := displayPath(file)
		result, err := analyzeFile(language, file)
		if err != nil {
			report.Files.Skipped++
			report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", display, err))
			continue
		}
		report.Files.Analyzed++
		if result.SyntaxError != nil {
			report.Files.Partial++
			report.Warnings = append(report.Warnings, fmt.Sprintf("%s: %v; functions containing it were not analyzed", display, result.SyntaxError))
		}
		functions := result.Functions

		if options.Complexity {
			for _, fn := range functions {
				report.Complexity.Functions = append(report.Complexity.Functions, newFunction(fn, language, display))
			}
		}
		if options.Clones && !language.IsTestFile(display) {
			detector, ok := detectors[language]
			if !ok {
				detector = clone.NewDetector(language.Clone, clone.DefaultConfig())
				detectors[language] = detector
			}
			for _, fn := range functions {
				if !fn.IsTest {
					detector.Add(fn, display)
				}
			}
		}
	}

	if options.Complexity {
		sortFunctions(report.Complexity.Functions)
		report.Complexity.Summary = summarize(report.Complexity.Functions)
	}
	if options.Clones {
		report.Clones = detectClones(detectors)
	}
	return report, nil
}

func analyzeFile(language *engine.Language, path string) (*engine.Result, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return language.Analyze(content)
}

func newFunction(fn engine.Function, language *engine.Language, display string) Function {
	return Function{
		Name:        fn.Name,
		FilePath:    display,
		Language:    language.Name,
		StartLine:   fn.StartLine,
		StartColumn: fn.StartColumn,
		EndLine:     fn.EndLine,
		Complexity:  fn.Complexity,
		Decisions:   fn.Decisions,
		RiskLevel:   RiskLevel(fn.Complexity),
	}
}

// detectClones runs every language's detector and merges the results into
// one report. Fragments of different languages are never compared.
func detectClones(detectors map[*engine.Language]*clone.Detector) *clone.Report {
	merged := &clone.Report{
		Pairs:      []clone.Pair{},
		Groups:     []clone.Group{},
		Statistics: clone.Statistics{ClonesByType: map[string]int{}},
	}
	languages := make([]*engine.Language, 0, len(detectors))
	for language := range detectors {
		languages = append(languages, language)
	}
	sort.Slice(languages, func(i, j int) bool { return languages[i].Name < languages[j].Name })

	totalSimilarity := 0.0
	for _, language := range languages {
		report := detectors[language].Detect()
		// Each detector numbers its fragments from zero, so they are
		// rebased to stay unique across languages.
		offset := merged.Statistics.TotalFragments
		rebase := func(fragment *clone.Fragment) {
			fragment.ID += offset
			fragment.Language = language.Name
		}
		for _, pair := range report.Pairs {
			rebase(&pair.Fragment1)
			rebase(&pair.Fragment2)
			merged.Pairs = append(merged.Pairs, pair)
			totalSimilarity += pair.Similarity
		}
		for _, group := range report.Groups {
			for i := range group.Fragments {
				rebase(&group.Fragments[i])
			}
			merged.Groups = append(merged.Groups, group)
		}
		merged.Statistics.TotalFragments += report.Statistics.TotalFragments
		merged.Statistics.TotalClones += report.Statistics.TotalClones
		for cloneType, count := range report.Statistics.ClonesByType {
			merged.Statistics.ClonesByType[cloneType] += count
		}
	}

	rank(merged)
	merged.Statistics.TotalClonePairs = len(merged.Pairs)
	merged.Statistics.TotalCloneGroups = len(merged.Groups)
	if len(merged.Pairs) > 0 {
		merged.Statistics.AverageSimilarity = totalSimilarity / float64(len(merged.Pairs))
	}
	return merged
}

// rank orders pairs and groups across languages the way core/clone orders
// them within one: by similarity, then larger groups first, then by
// location. IDs follow the order.
func rank(report *clone.Report) {
	sort.SliceStable(report.Pairs, func(i, j int) bool {
		a, b := report.Pairs[i], report.Pairs[j]
		if !almostEqual(a.Similarity, b.Similarity) {
			return a.Similarity > b.Similarity
		}
		if a.Fragment1.FilePath != b.Fragment1.FilePath || a.Fragment1.StartLine != b.Fragment1.StartLine {
			return fragmentPrecedes(a.Fragment1, b.Fragment1)
		}
		return fragmentPrecedes(a.Fragment2, b.Fragment2)
	})
	sort.SliceStable(report.Groups, func(i, j int) bool {
		a, b := report.Groups[i], report.Groups[j]
		if !almostEqual(a.Similarity, b.Similarity) {
			return a.Similarity > b.Similarity
		}
		if len(a.Fragments) != len(b.Fragments) {
			return len(a.Fragments) > len(b.Fragments)
		}
		return fragmentPrecedes(a.Fragments[0], b.Fragments[0])
	})
	for i := range report.Pairs {
		report.Pairs[i].ID = i
	}
	for i := range report.Groups {
		report.Groups[i].ID = i
	}
}

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}

func fragmentPrecedes(a, b clone.Fragment) bool {
	if a.FilePath != b.FilePath {
		return a.FilePath < b.FilePath
	}
	return a.StartLine < b.StartLine
}

// RiskLevel classifies a complexity with the thresholds shared by every
// polyscan analyzer.
func RiskLevel(complexity int) domain.RiskLevel {
	switch {
	case complexity <= domain.DefaultComplexityLowThreshold:
		return domain.RiskLevelLow
	case complexity <= domain.DefaultComplexityMediumThreshold:
		return domain.RiskLevelMedium
	default:
		return domain.RiskLevelHigh
	}
}

func sortFunctions(functions []Function) {
	sort.SliceStable(functions, func(i, j int) bool {
		a, b := functions[i], functions[j]
		if a.Complexity != b.Complexity {
			return a.Complexity > b.Complexity
		}
		if a.FilePath != b.FilePath {
			return a.FilePath < b.FilePath
		}
		return a.StartLine < b.StartLine
	})
}

func summarize(functions []Function) ComplexitySummary {
	summary := ComplexitySummary{TotalFunctions: len(functions)}
	if len(functions) == 0 {
		return summary
	}
	total := 0
	summary.MinComplexity = functions[0].Complexity
	for _, fn := range functions {
		total += fn.Complexity
		summary.MaxComplexity = max(summary.MaxComplexity, fn.Complexity)
		summary.MinComplexity = min(summary.MinComplexity, fn.Complexity)
		switch fn.RiskLevel {
		case domain.RiskLevelLow:
			summary.LowRiskFunctions++
		case domain.RiskLevelMedium:
			summary.MediumRiskFunctions++
		case domain.RiskLevelHigh:
			summary.HighRiskFunctions++
		}
	}
	summary.AverageComplexity = float64(total) / float64(len(functions))
	return summary
}

// displayPath shortens an absolute path to one relative to the working
// directory when the file lies under it.
func displayPath(path string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	rel, err := filepath.Rel(cwd, path)
	if err != nil || !filepath.IsLocal(rel) {
		return path
	}
	return rel
}
