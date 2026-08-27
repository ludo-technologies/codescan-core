// Package report writes an analysis report as text or JSON.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/analysis"
)

// Document is the report together with the metadata of the run that
// produced it.
type Document struct {
	Version     string           `json:"version"`
	GeneratedAt time.Time        `json:"generated_at"`
	DurationMs  int64            `json:"duration_ms"`
	Complexity  *analysis.Report `json:"complexity"`
	// MinComplexity is the filter applied to the listed functions. The
	// summary always covers every analyzed function.
	MinComplexity int `json:"-"`
}

// Write renders the document in the given format.
func Write(w io.Writer, doc *Document, format domain.OutputFormat) error {
	switch format {
	case domain.OutputFormatJSON:
		return writeJSON(w, doc)
	case domain.OutputFormatText:
		return writeText(w, doc)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func writeJSON(w io.Writer, doc *Document) error {
	filtered := *doc.Complexity
	filtered.Functions = listed(doc)
	out := *doc
	out.Complexity = &filtered

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

func writeText(w io.Writer, doc *Document) error {
	summary := doc.Complexity.Summary
	fmt.Fprintf(w, "\n=== Complexity Analysis ===\n\n")
	fmt.Fprintf(w, "Generated: %s\n", doc.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "Version: %s\n\n", doc.Version)

	fmt.Fprintf(w, "Summary:\n")
	fmt.Fprintf(w, "  Files analyzed: %d\n", summary.FilesAnalyzed)
	if summary.SkippedFiles > 0 {
		fmt.Fprintf(w, "  Files skipped: %d\n", summary.SkippedFiles)
	}
	fmt.Fprintf(w, "  Total functions: %d\n", summary.TotalFunctions)
	fmt.Fprintf(w, "  Average complexity: %.2f\n", summary.AverageComplexity)
	fmt.Fprintf(w, "  Max complexity: %d\n", summary.MaxComplexity)
	fmt.Fprintf(w, "  Min complexity: %d\n\n", summary.MinComplexity)

	fmt.Fprintf(w, "Risk Distribution:\n")
	fmt.Fprintf(w, "  High risk: %d\n", summary.HighRiskFunctions)
	fmt.Fprintf(w, "  Medium risk: %d\n", summary.MediumRiskFunctions)
	fmt.Fprintf(w, "  Low risk: %d\n", summary.LowRiskFunctions)

	functions := listed(doc)
	if len(functions) > 0 {
		if doc.MinComplexity > 1 {
			fmt.Fprintf(w, "\nFunctions (complexity >= %d):\n", doc.MinComplexity)
		} else {
			fmt.Fprintf(w, "\nFunctions:\n")
		}
		for _, fn := range functions {
			indicator := ""
			switch fn.RiskLevel {
			case domain.RiskLevelHigh:
				indicator = " [HIGH]"
			case domain.RiskLevelMedium:
				indicator = " [MEDIUM]"
			}
			fmt.Fprintf(w, "  %s: %d%s\n", fn.Name, fn.Complexity, indicator)
			fmt.Fprintf(w, "    File: %s:%d-%d\n", fn.FilePath, fn.StartLine, fn.EndLine)
		}
	}

	if len(doc.Complexity.Errors) > 0 {
		fmt.Fprintf(w, "\nErrors:\n")
		for _, e := range doc.Complexity.Errors {
			fmt.Fprintf(w, "  - %s\n", e)
		}
	}
	return nil
}

// listed returns the functions that pass the report filter, in the order
// the analysis sorted them.
func listed(doc *Document) []analysis.Function {
	functions := []analysis.Function{}
	for _, fn := range doc.Complexity.Functions {
		if fn.Complexity >= doc.MinComplexity {
			functions = append(functions, fn)
		}
	}
	return functions
}
