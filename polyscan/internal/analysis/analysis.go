// Package analysis collects source files, dispatches each to its language
// and aggregates the per-function results into a report.
package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/core/source"
	"github.com/ludo-technologies/polyscan/polyscan/internal/lang"
)

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

// Summary aggregates every analyzed function. Report filters only limit
// which functions are listed; they never change these numbers.
type Summary struct {
	TotalFunctions    int     `json:"total_functions"`
	AverageComplexity float64 `json:"average_complexity"`
	MaxComplexity     int     `json:"max_complexity"`
	MinComplexity     int     `json:"min_complexity"`
	// FilesAnalyzed counts the files that parsed and contributed to the
	// metrics. TotalFiles counts every file the request covered, and
	// SkippedFiles the ones that could not be read or parsed. Skipped files
	// are absent from every metric here.
	FilesAnalyzed int `json:"files_analyzed"`
	TotalFiles    int `json:"total_files"`
	SkippedFiles  int `json:"skipped_files"`

	LowRiskFunctions    int `json:"low_risk_functions"`
	MediumRiskFunctions int `json:"medium_risk_functions"`
	HighRiskFunctions   int `json:"high_risk_functions"`
}

// Report is the complete complexity analysis of a set of paths.
type Report struct {
	// Functions is sorted by descending complexity, then by location.
	Functions []Function `json:"functions"`
	Summary   Summary    `json:"summary"`
	// Errors lists, per skipped file, why it was skipped.
	Errors []string `json:"errors,omitempty"`
}

// Analyze collects every supported source file under paths and analyzes it.
// A file that cannot be read or parsed is skipped and reported in Errors;
// finding no supported file at all is an error.
func Analyze(paths []string) (*Report, error) {
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

	report := &Report{}
	report.Summary.TotalFiles = len(files)
	for _, file := range files {
		functions, err := analyzeFile(file)
		if err != nil {
			report.Summary.SkippedFiles++
			report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", displayPath(file), err))
			continue
		}
		report.Summary.FilesAnalyzed++
		report.Functions = append(report.Functions, functions...)
	}

	sort.SliceStable(report.Functions, func(i, j int) bool {
		a, b := report.Functions[i], report.Functions[j]
		if a.Complexity != b.Complexity {
			return a.Complexity > b.Complexity
		}
		if a.FilePath != b.FilePath {
			return a.FilePath < b.FilePath
		}
		return a.StartLine < b.StartLine
	})
	summarize(report)
	return report, nil
}

func analyzeFile(path string) ([]Function, error) {
	language, ok := lang.ByPath(path)
	if !ok {
		return nil, fmt.Errorf("unsupported file extension")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	found, err := language.Analyze(content)
	if err != nil {
		return nil, err
	}

	display := displayPath(path)
	functions := make([]Function, 0, len(found))
	for _, fn := range found {
		functions = append(functions, Function{
			Name:        fn.Name,
			FilePath:    display,
			Language:    language.Name,
			StartLine:   fn.StartLine,
			StartColumn: fn.StartColumn,
			EndLine:     fn.EndLine,
			Complexity:  fn.Complexity,
			Decisions:   fn.Decisions,
			RiskLevel:   RiskLevel(fn.Complexity),
		})
	}
	return functions, nil
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

func summarize(report *Report) {
	summary := &report.Summary
	summary.TotalFunctions = len(report.Functions)
	if summary.TotalFunctions == 0 {
		return
	}
	total := 0
	summary.MinComplexity = report.Functions[0].Complexity
	for _, fn := range report.Functions {
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
	summary.AverageComplexity = float64(total) / float64(summary.TotalFunctions)
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
