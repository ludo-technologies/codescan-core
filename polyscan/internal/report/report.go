// Package report writes an analysis report as text or JSON.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/analysis"
	"github.com/ludo-technologies/polyscan/polyscan/internal/clone"
)

// Document is the report together with the metadata of the run that
// produced it.
type Document struct {
	Version     string    `json:"version"`
	GeneratedAt time.Time `json:"generated_at"`
	DurationMs  int64     `json:"duration_ms"`
	*analysis.Report
	// MinComplexity is the filter applied to the listed functions. The
	// summary always covers every analyzed function.
	MinComplexity int `json:"-"`
}

// textPairLimit bounds the clone pairs the text report lists; the JSON
// report carries them all.
const textPairLimit = 10

// Write renders the document in the given format.
func Write(w io.Writer, doc *Document, format domain.OutputFormat) error {
	switch format {
	case domain.OutputFormatJSON:
		return writeJSON(w, doc)
	case domain.OutputFormatText:
		writeText(w, doc)
		return nil
	case domain.OutputFormatHTML:
		return writeHTML(w, doc)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func writeJSON(w io.Writer, doc *Document) error {
	out := *doc
	if doc.Complexity != nil {
		filtered := *doc.Complexity
		filtered.Functions = listed(doc)
		report := *doc.Report
		report.Complexity = &filtered
		out.Report = &report
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(out)
}

func writeText(w io.Writer, doc *Document) {
	fmt.Fprintf(w, "\n=== polyscan ===\n\n")
	fmt.Fprintf(w, "Generated: %s\n", doc.GeneratedAt.Format(time.RFC3339))
	fmt.Fprintf(w, "Version: %s\n", doc.Version)
	fmt.Fprintf(w, "Files analyzed: %d\n", doc.Files.Analyzed)
	if doc.Files.Partial > 0 {
		fmt.Fprintf(w, "Files with syntax errors: %d\n", doc.Files.Partial)
	}
	if doc.Files.Skipped > 0 {
		fmt.Fprintf(w, "Files skipped: %d\n", doc.Files.Skipped)
	}

	if doc.Complexity != nil {
		writeComplexityText(w, doc)
	}
	if doc.Clones != nil {
		writeCloneText(w, doc.Clones)
	}

	if len(doc.Warnings) > 0 {
		fmt.Fprintf(w, "\nWarnings:\n")
		for _, warning := range doc.Warnings {
			fmt.Fprintf(w, "  - %s\n", warning)
		}
	}
	if len(doc.Errors) > 0 {
		fmt.Fprintf(w, "\nErrors:\n")
		for _, e := range doc.Errors {
			fmt.Fprintf(w, "  - %s\n", e)
		}
	}
}

func writeComplexityText(w io.Writer, doc *Document) {
	summary := doc.Complexity.Summary
	fmt.Fprintf(w, "\n=== Complexity Analysis ===\n\n")
	fmt.Fprintf(w, "Summary:\n")
	fmt.Fprintf(w, "  Total functions: %d\n", summary.TotalFunctions)
	fmt.Fprintf(w, "  Average complexity: %.2f\n", summary.AverageComplexity)
	fmt.Fprintf(w, "  Max complexity: %d\n", summary.MaxComplexity)
	fmt.Fprintf(w, "  Min complexity: %d\n\n", summary.MinComplexity)

	fmt.Fprintf(w, "Risk Distribution:\n")
	fmt.Fprintf(w, "  High risk: %d\n", summary.HighRiskFunctions)
	fmt.Fprintf(w, "  Medium risk: %d\n", summary.MediumRiskFunctions)
	fmt.Fprintf(w, "  Low risk: %d\n", summary.LowRiskFunctions)

	functions := listed(doc)
	if len(functions) == 0 {
		return
	}
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

func writeCloneText(w io.Writer, clones *clone.Report) {
	stats := clones.Statistics
	fmt.Fprintf(w, "\n=== Clone Detection ===\n\n")
	fmt.Fprintf(w, "Statistics:\n")
	fmt.Fprintf(w, "  Fragments compared: %d\n", stats.TotalFragments)
	fmt.Fprintf(w, "  Total clone pairs: %d\n", stats.TotalClonePairs)
	fmt.Fprintf(w, "  Total clone groups: %d\n", stats.TotalCloneGroups)
	fmt.Fprintf(w, "  Average similarity: %.2f\n", stats.AverageSimilarity)
	if len(stats.ClonesByType) > 0 {
		fmt.Fprintf(w, "\nClone Types:\n")
		for _, cloneType := range []domain.CloneType{domain.Type1Clone, domain.Type2Clone, domain.Type3Clone, domain.Type4Clone} {
			if count := stats.ClonesByType[cloneType.String()]; count > 0 {
				fmt.Fprintf(w, "  %s: %d\n", cloneType, count)
			}
		}
	}

	if len(clones.Groups) > 0 {
		fmt.Fprintf(w, "\nClone Groups:\n")
		for _, group := range clones.Groups {
			fmt.Fprintf(w, "  Group %d: %s, %d fragments, %.1f%% similar\n", group.ID+1, group.Type, len(group.Fragments), group.Similarity*100)
			for _, fragment := range group.Fragments {
				fmt.Fprintf(w, "    %s (%s:%d-%d)\n", fragment.Name, fragment.FilePath, fragment.StartLine, fragment.EndLine)
			}
		}
	}

	if len(clones.Pairs) == 0 {
		fmt.Fprintf(w, "\nNo code clones detected.\n")
		return
	}
	fmt.Fprintf(w, "\nTop Clone Pairs:\n")
	for _, pair := range clones.Pairs[:min(textPairLimit, len(clones.Pairs))] {
		fmt.Fprintf(w, "  %s: %s (%s:%d-%d) <-> %s (%s:%d-%d) (%.1f%% similar)\n",
			pair.Type,
			pair.Fragment1.Name, pair.Fragment1.FilePath, pair.Fragment1.StartLine, pair.Fragment1.EndLine,
			pair.Fragment2.Name, pair.Fragment2.FilePath, pair.Fragment2.StartLine, pair.Fragment2.EndLine,
			pair.Similarity*100)
	}
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
