package report

import (
	"embed"
	"fmt"
	"html/template"
	"io"
	"strings"

	"github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/analysis"
	"github.com/ludo-technologies/polyscan/polyscan/internal/clone"
)

//go:embed templates/*
var templates embed.FS

var htmlTemplate = template.Must(
	template.New("report.html").
		Funcs(template.FuncMap{
			"add":      func(a, b int) int { return a + b },
			"sub":      func(a, b int) int { return a - b },
			"addf":     func(a, b float64) float64 { return a + b },
			"divf":     func(a, b float64) float64 { return a / b },
			"int":      func(t domain.CloneType) int { return int(t) },
			"percent":  func(ratio float64) string { return fmt.Sprintf("%.1f%%", ratio*100) },
			"lineSpan": func(fn analysis.Function) int { return fn.EndLine - fn.StartLine + 1 },
			"preview":  preview,
		}).
		ParseFS(templates, "templates/report.html"),
)

var css = func() template.CSS {
	data, err := templates.ReadFile("templates/report.css")
	if err != nil {
		panic(err)
	}
	return template.CSS(data)
}()

// Limits on what the HTML report lists; the JSON report carries everything.
const (
	htmlFunctionLimit = 20
	htmlGroupLimit    = 10
	previewLines      = 8
)

// htmlView is the data handed to the template: the document plus the
// pre-computed blocks, so the template stays free of arithmetic.
type htmlView struct {
	CSS template.CSS
	*Document
	// Functions is the head of the listed functions and ListedFunctions
	// how many passed the filter in total.
	Functions       []analysis.Function
	ListedFunctions int
	Histogram       *histogram
	Duplication     []share
	Groups          []clone.Group
}

// share is one clone type's slice of the pairs.
type share struct {
	Label   string
	Percent float64
	Class   string
}

func writeHTML(w io.Writer, doc *Document) error {
	view := htmlView{CSS: css, Document: doc}
	if doc.Complexity != nil {
		functions := listed(doc)
		view.ListedFunctions = len(functions)
		view.Functions = functions[:min(htmlFunctionLimit, len(functions))]
		view.Histogram = buildHistogram(doc.Complexity.Functions)
	}
	if doc.Clones != nil {
		view.Duplication = buildDuplication(doc.Clones)
		view.Groups = doc.Clones.Groups[:min(htmlGroupLimit, len(doc.Clones.Groups))]
	}
	if err := htmlTemplate.Execute(w, view); err != nil {
		return fmt.Errorf("render HTML report: %w", err)
	}
	return nil
}

func buildDuplication(clones *clone.Report) []share {
	if len(clones.Pairs) == 0 {
		return nil
	}
	var shares []share
	for _, cloneType := range []domain.CloneType{domain.Type1Clone, domain.Type2Clone, domain.Type3Clone} {
		count := clones.Statistics.ClonesByType[cloneType.String()]
		if count == 0 {
			continue
		}
		shares = append(shares, share{
			Label:   fmt.Sprintf("%s %s · %d pairs", cloneType, domain.CloneTypeNames[cloneType], count),
			Percent: float64(count) * 100 / float64(len(clones.Pairs)),
			Class:   fmt.Sprintf("t%d", int(cloneType)),
		})
	}
	return shares
}

// preview returns the first lines of a fragment.
func preview(fragment clone.Fragment) string {
	lines := strings.Split(fragment.Content, "\n")
	if len(lines) <= previewLines {
		return fragment.Content
	}
	return strings.Join(lines[:previewLines], "\n") + "\n..."
}

// Histogram geometry, in the 420x190 viewBox of the chart.
const (
	histLeft      = 48.0
	histRight     = 412.0
	histTop       = 18.0
	histBaseline  = 150.0
	histBarFill   = 0.72
	histTickCount = 3
)

type histogram struct {
	Total int
	Bins  []histogramBin
	Ticks []histogramTick
}

type histogramBin struct {
	Label  string
	Count  int
	X      float64
	Y      float64
	Width  float64
	Height float64
	Band   string
}

type histogramTick struct {
	Label string
	Y     float64
}

// histogramBins follows the risk thresholds: 1 | 2–5 | 6–low | low+1–medium
// | medium+1+, so that each bucket carries a single risk color.
func histogramBins() []histogramBin {
	low, medium := domain.DefaultComplexityLowThreshold, domain.DefaultComplexityMediumThreshold
	return []histogramBin{
		{Label: "1"},
		{Label: "2–5"},
		{Label: fmt.Sprintf("6–%d", low)},
		{Label: fmt.Sprintf("%d–%d", low+1, medium), Band: "warn"},
		{Label: fmt.Sprintf("%d+", medium+1), Band: "bad"},
	}
}

func binIndex(complexity int) int {
	switch {
	case complexity <= 1:
		return 0
	case complexity <= 5:
		return 1
	case complexity <= domain.DefaultComplexityLowThreshold:
		return 2
	case complexity <= domain.DefaultComplexityMediumThreshold:
		return 3
	default:
		return 4
	}
}

// buildHistogram plots the complete analyzed population, whatever the
// report filter lists.
func buildHistogram(functions []analysis.Function) *histogram {
	if len(functions) == 0 {
		return nil
	}
	bins := histogramBins()
	for _, fn := range functions {
		bins[binIndex(fn.Complexity)].Count++
	}

	maxCount := 1
	for _, bin := range bins {
		maxCount = max(maxCount, bin.Count)
	}
	niceMax := niceCeiling(maxCount)
	scale := (histBaseline - histTop) / float64(niceMax)

	slot := (histRight - histLeft) / float64(len(bins))
	barWidth := slot * histBarFill
	hist := &histogram{Total: len(functions)}
	for i, bin := range bins {
		height := float64(bin.Count) * scale
		if bin.Count > 0 && height < 1 {
			height = 1
		}
		bin.X = round1(histLeft + slot*float64(i) + (slot-barWidth)/2)
		bin.Y = round1(histBaseline - height)
		bin.Width = round1(barWidth)
		bin.Height = round1(height)
		hist.Bins = append(hist.Bins, bin)
	}
	for i := 0; i <= histTickCount; i++ {
		value := niceMax * i / histTickCount
		hist.Ticks = append(hist.Ticks, histogramTick{
			Label: fmt.Sprint(value),
			Y:     round1(histBaseline - float64(value)*scale),
		})
	}
	return hist
}

// niceCeiling rounds n up to a tidy axis maximum so ticks land on round
// numbers.
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
