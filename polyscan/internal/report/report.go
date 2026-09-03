// Package report merges the generic-engine analysis (Go, Rust, C++) with the
// JavaScript/TypeScript analysis into the response set the unified formatter
// renders, so every language shares one report, one JSON shape and one health
// score.
package report

import (
	"path/filepath"
	"sort"
	"time"

	coredomain "github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/analysis"
	"github.com/ludo-technologies/polyscan/polyscan/internal/clone"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/version"
)

// Combine merges the generic-engine report with the JavaScript/TypeScript
// result into the shape the output formatter renders. Complexity and clone
// results merge across languages; dead code, coupling and dependencies exist
// only for JavaScript/TypeScript and pass through. The file accounting adds
// up across both sides, so the health score charges every unparsable file
// whichever analyses ran. Either input may be nil when its side found no
// files.
func Combine(generic *analysis.Report, javascript *js.Result) (domain.AnalysisResults, error) {
	results := domain.AnalysisResults{}
	if javascript != nil {
		results.Files = javascript.Files
		results.Complexity = javascript.Complexity
		results.DeadCode = javascript.DeadCode
		results.Clone = javascript.Clones
		results.CBO = javascript.CBO
		results.Deps = javascript.Deps
	}
	if generic != nil {
		complexity, err := genericComplexity(generic)
		if err != nil {
			return domain.AnalysisResults{}, err
		}
		results.Files.Add(domain.FileAccounting{
			Total:   generic.Files.Total,
			Skipped: generic.Files.Skipped,
			Errors:  generic.Errors,
		})
		results.Complexity = mergeComplexity(complexity, results.Complexity)
		results.Clone = mergeClones(genericClones(generic), results.Clone)
	}
	return results, nil
}

// genericComplexity converts the generic engine's complexity analysis into
// the formatter's complexity response, including the per-module and
// per-directory rollups the report's hotspot table reads.
func genericComplexity(report *analysis.Report) (*domain.ComplexityResponse, error) {
	if report.Complexity == nil {
		return nil, nil
	}

	src := report.Complexity
	functions := make([]domain.FunctionComplexity, 0, len(src.Functions))
	distribution := make(map[int]int, len(src.Functions))
	for _, fn := range src.Functions {
		functions = append(functions, domain.FunctionComplexity{
			Name:        fn.Name,
			FilePath:    fn.FilePath,
			Language:    fn.Language,
			StartLine:   fn.StartLine,
			StartColumn: fn.StartColumn,
			EndLine:     fn.EndLine,
			Metrics:     domain.ComplexityMetrics{Complexity: fn.Complexity, NestingDepth: fn.NestingDepth},
			RiskLevel:   domain.RiskLevel(fn.RiskLevel),
		})
		distribution[fn.Complexity]++
	}

	rollups := domain.AggregateComplexityByModule(functions)
	analyzed := make([]string, 0, len(report.FileLines))
	for path, lines := range report.FileLines {
		key := filepath.Clean(path)
		metrics := rollups[key]
		metrics.LinesOfCode = lines
		rollups[key] = metrics
		analyzed = append(analyzed, path)
	}

	var byDirectory domain.DirectoryComplexityMetricsList
	if len(functions) > 0 {
		root, err := domain.ComplexityDirectoryRoot(analyzed)
		if err != nil {
			return nil, err
		}
		byDirectory, err = domain.AggregateComplexityByDirectory(functions, root)
		if err != nil {
			return nil, err
		}
	}

	return &domain.ComplexityResponse{
		Functions:     functions,
		ByDirectory:   byDirectory,
		ModuleRollups: rollups,
		Summary: domain.ComplexitySummary{
			TotalFunctions:         src.Summary.TotalFunctions,
			FunctionsParsed:        src.Summary.TotalFunctions,
			AverageComplexity:      src.Summary.AverageComplexity,
			MaxComplexity:          src.Summary.MaxComplexity,
			MinComplexity:          src.Summary.MinComplexity,
			FilesAnalyzed:          report.Files.Analyzed,
			TotalFiles:             report.Files.Total,
			SkippedFiles:           report.Files.Skipped,
			LowRiskFunctions:       src.Summary.LowRiskFunctions,
			MediumRiskFunctions:    src.Summary.MediumRiskFunctions,
			HighRiskFunctions:      src.Summary.HighRiskFunctions,
			ComplexityDistribution: distribution,
		},
		Warnings:    append([]string{}, report.Warnings...),
		Errors:      append([]string{}, report.Errors...),
		GeneratedAt: time.Now().Format(time.RFC3339),
		Version:     version.Version,
		// The generic engine classifies risk with the shared defaults; the
		// report reads the thresholds back from here to band its histogram.
		Config: map[string]interface{}{
			"low_threshold":    coredomain.DefaultComplexityLowThreshold,
			"medium_threshold": coredomain.DefaultComplexityMediumThreshold,
		},
	}, nil
}

// genericClones converts the generic engine's clone report into the
// formatter's clone response. Fragments keep one identity across the pairs
// and groups they appear in, as the formatter expects.
func genericClones(report *analysis.Report) *domain.CloneResponse {
	if report.Clones == nil {
		return nil
	}

	src := report.Clones
	fragments := map[int]*domain.Clone{}
	convert := func(fragment clone.Fragment, cloneType coredomain.CloneType) *domain.Clone {
		if existing, ok := fragments[fragment.ID]; ok {
			return existing
		}
		converted := &domain.Clone{
			ID:       fragment.ID,
			Type:     domain.CloneType(cloneType),
			Language: fragment.Language,
			Location: &domain.CloneLocation{
				FilePath:  fragment.FilePath,
				StartLine: fragment.StartLine,
				EndLine:   fragment.EndLine,
			},
			Content:   fragment.Content,
			Size:      fragment.NodeCount,
			LineCount: fragment.LineCount,
		}
		fragments[fragment.ID] = converted
		return converted
	}

	pairs := make([]*domain.ClonePair, 0, len(src.Pairs))
	for _, pair := range src.Pairs {
		pairs = append(pairs, &domain.ClonePair{
			ID:         pair.ID,
			Clone1:     convert(pair.Fragment1, pair.Type),
			Clone2:     convert(pair.Fragment2, pair.Type),
			Similarity: pair.Similarity,
			Distance:   pair.Distance,
			Type:       domain.CloneType(pair.Type),
			Confidence: pair.Confidence,
		})
	}
	groups := make([]*domain.CloneGroup, 0, len(src.Groups))
	for _, group := range src.Groups {
		converted := &domain.CloneGroup{
			ID:         group.ID,
			Type:       domain.CloneType(group.Type),
			Similarity: group.Similarity,
		}
		for _, fragment := range group.Fragments {
			converted.AddClone(convert(fragment, group.Type))
		}
		groups = append(groups, converted)
	}

	clonesByType := make(map[string]int, len(src.Statistics.ClonesByType))
	for cloneType, count := range src.Statistics.ClonesByType {
		clonesByType[cloneType] = count
	}

	return &domain.CloneResponse{
		Clones:      sortedFragments(fragments),
		ClonePairs:  pairs,
		CloneGroups: groups,
		Statistics: &domain.CloneStatistics{
			TotalFragments:    src.Statistics.TotalFragments,
			TotalClones:       src.Statistics.TotalClones,
			TotalClonePairs:   src.Statistics.TotalClonePairs,
			TotalCloneGroups:  src.Statistics.TotalCloneGroups,
			ClonesByType:      clonesByType,
			AverageSimilarity: src.Statistics.AverageSimilarity,
			LinesAnalyzed:     src.Statistics.LinesAnalyzed,
			FilesAnalyzed:     src.Statistics.FilesAnalyzed,
		},
		Success: true,
	}
}

func sortedFragments(fragments map[int]*domain.Clone) []*domain.Clone {
	sorted := make([]*domain.Clone, 0, len(fragments))
	for _, fragment := range fragments {
		sorted = append(sorted, fragment)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })
	return sorted
}

// mergeComplexity merges the generic and JavaScript complexity responses into
// one population. The JavaScript configuration wins where the responses
// cannot both be represented (thresholds, sort criteria): it is the only one
// a user can change, and the generic side runs on the same defaults.
func mergeComplexity(generic, javascript *domain.ComplexityResponse) *domain.ComplexityResponse {
	if generic == nil {
		return javascript
	}
	if javascript == nil {
		return generic
	}

	functions := make([]domain.FunctionComplexity, 0, len(generic.Functions)+len(javascript.Functions))
	functions = append(functions, generic.Functions...)
	functions = append(functions, javascript.Functions...)
	sort.SliceStable(functions, func(i, j int) bool {
		a, b := functions[i], functions[j]
		if a.Metrics.Complexity != b.Metrics.Complexity {
			return a.Metrics.Complexity > b.Metrics.Complexity
		}
		if a.FilePath != b.FilePath {
			return a.FilePath < b.FilePath
		}
		return a.StartLine < b.StartLine
	})

	rollups := make(map[string]domain.ModuleComplexityMetrics, len(generic.ModuleRollups)+len(javascript.ModuleRollups))
	for path, metrics := range generic.ModuleRollups {
		rollups[path] = metrics
	}
	for path, metrics := range javascript.ModuleRollups {
		rollups[path] = metrics
	}

	return &domain.ComplexityResponse{
		Functions:     functions,
		ByDirectory:   mergeDirectories(generic.ByDirectory, javascript.ByDirectory),
		ModuleRollups: rollups,
		Summary:       mergeComplexitySummaries(generic.Summary, javascript.Summary),
		Warnings:      mergeStrings(generic.Warnings, javascript.Warnings),
		Errors:        mergeStrings(generic.Errors, javascript.Errors),
		GeneratedAt:   javascript.GeneratedAt,
		Version:       javascript.Version,
		Config:        javascript.Config,
	}
}

func mergeComplexitySummaries(a, b domain.ComplexitySummary) domain.ComplexitySummary {
	merged := domain.ComplexitySummary{
		TotalFunctions:      a.TotalFunctions + b.TotalFunctions,
		FunctionsParsed:     a.FunctionsParsed + b.FunctionsParsed,
		MaxComplexity:       max(a.MaxComplexity, b.MaxComplexity),
		FilesAnalyzed:       a.FilesAnalyzed + b.FilesAnalyzed,
		TotalFiles:          a.TotalFiles + b.TotalFiles,
		SkippedFiles:        a.SkippedFiles + b.SkippedFiles,
		LowRiskFunctions:    a.LowRiskFunctions + b.LowRiskFunctions,
		MediumRiskFunctions: a.MediumRiskFunctions + b.MediumRiskFunctions,
		HighRiskFunctions:   a.HighRiskFunctions + b.HighRiskFunctions,
	}
	switch {
	case a.TotalFunctions == 0:
		merged.MinComplexity = b.MinComplexity
	case b.TotalFunctions == 0:
		merged.MinComplexity = a.MinComplexity
	default:
		merged.MinComplexity = min(a.MinComplexity, b.MinComplexity)
	}
	if merged.TotalFunctions > 0 {
		merged.AverageComplexity = (a.AverageComplexity*float64(a.TotalFunctions) +
			b.AverageComplexity*float64(b.TotalFunctions)) / float64(merged.TotalFunctions)
	}
	if a.ComplexityDistribution != nil || b.ComplexityDistribution != nil {
		merged.ComplexityDistribution = map[int]int{}
		for complexity, count := range a.ComplexityDistribution {
			merged.ComplexityDistribution[complexity] += count
		}
		for complexity, count := range b.ComplexityDistribution {
			merged.ComplexityDistribution[complexity] += count
		}
	}
	return merged
}

// mergeDirectories joins the per-directory rollups, folding together rows
// whose relative paths coincide: each side reports directories relative to
// its own analyzed root, so the same label can appear in both.
func mergeDirectories(a, b domain.DirectoryComplexityMetricsList) domain.DirectoryComplexityMetricsList {
	byPath := map[string]domain.DirectoryComplexityMetrics{}
	for _, row := range a {
		byPath[row.DirectoryPath] = row
	}
	for _, row := range b {
		existing, ok := byPath[row.DirectoryPath]
		if !ok {
			byPath[row.DirectoryPath] = row
			continue
		}
		total := existing.FunctionCount + row.FunctionCount
		merged := domain.DirectoryComplexityMetrics{
			DirectoryPath:         row.DirectoryPath,
			FunctionCount:         total,
			MaxComplexity:         max(existing.MaxComplexity, row.MaxComplexity),
			HighRiskFunctionCount: existing.HighRiskFunctionCount + row.HighRiskFunctionCount,
			MaxNestingDepth:       max(existing.MaxNestingDepth, row.MaxNestingDepth),
		}
		if total > 0 {
			merged.AverageComplexity = (existing.AverageComplexity*float64(existing.FunctionCount) +
				row.AverageComplexity*float64(row.FunctionCount)) / float64(total)
			merged.AverageNestingDepth = (existing.AverageNestingDepth*float64(existing.FunctionCount) +
				row.AverageNestingDepth*float64(row.FunctionCount)) / float64(total)
		}
		byPath[row.DirectoryPath] = merged
	}

	merged := make(domain.DirectoryComplexityMetricsList, 0, len(byPath))
	for _, row := range byPath {
		merged = append(merged, row)
	}
	sort.Slice(merged, func(i, j int) bool {
		if merged[i].AverageComplexity != merged[j].AverageComplexity {
			return merged[i].AverageComplexity > merged[j].AverageComplexity
		}
		return merged[i].DirectoryPath < merged[j].DirectoryPath
	})
	return merged
}

func mergeStrings(a, b []string) []string {
	if len(a)+len(b) == 0 {
		return nil
	}
	merged := make([]string, 0, len(a)+len(b))
	merged = append(merged, a...)
	return append(merged, b...)
}

// mergeClones merges the generic and JavaScript clone responses. Fragment IDs
// of the second response are rebased to stay unique, and pairs and groups are
// re-ranked and renumbered across languages the way each side ranks its own.
func mergeClones(generic, javascript *domain.CloneResponse) *domain.CloneResponse {
	if generic == nil {
		return javascript
	}
	if javascript == nil {
		return generic
	}

	offset := 0
	for _, fragment := range generic.Clones {
		offset = max(offset, fragment.ID+1)
	}
	for _, fragment := range javascript.Clones {
		fragment.ID += offset
	}

	merged := &domain.CloneResponse{
		Clones:     append(append([]*domain.Clone{}, generic.Clones...), javascript.Clones...),
		ClonePairs: append(append([]*domain.ClonePair{}, generic.ClonePairs...), javascript.ClonePairs...),
		CloneGroups: append(append([]*domain.CloneGroup{}, generic.CloneGroups...),
			javascript.CloneGroups...),
		Statistics: mergeCloneStatistics(generic.Statistics, javascript.Statistics),
		Duration:   generic.Duration + javascript.Duration,
		Success:    generic.Success && javascript.Success,
		Error:      firstNonEmpty(generic.Error, javascript.Error),
	}

	sort.SliceStable(merged.ClonePairs, func(i, j int) bool {
		return domain.ClonePairPrecedes(merged.ClonePairs[i], merged.ClonePairs[j])
	})
	sort.SliceStable(merged.CloneGroups, func(i, j int) bool {
		a, b := merged.CloneGroups[i], merged.CloneGroups[j]
		if a.Similarity != b.Similarity {
			return a.Similarity > b.Similarity
		}
		return len(a.Clones) > len(b.Clones)
	})
	for i, pair := range merged.ClonePairs {
		pair.ID = i
	}
	for i, group := range merged.CloneGroups {
		group.ID = i
	}
	return merged
}

func mergeCloneStatistics(a, b *domain.CloneStatistics) *domain.CloneStatistics {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	merged := &domain.CloneStatistics{
		TotalFragments:   a.TotalFragments + b.TotalFragments,
		TotalClones:      a.TotalClones + b.TotalClones,
		TotalClonePairs:  a.TotalClonePairs + b.TotalClonePairs,
		TotalCloneGroups: a.TotalCloneGroups + b.TotalCloneGroups,
		ClonesByType:     map[string]int{},
		LinesAnalyzed:    a.LinesAnalyzed + b.LinesAnalyzed,
		FilesAnalyzed:    a.FilesAnalyzed + b.FilesAnalyzed,
		NodesAnalyzed:    a.NodesAnalyzed + b.NodesAnalyzed,
	}
	for cloneType, count := range a.ClonesByType {
		merged.ClonesByType[cloneType] += count
	}
	for cloneType, count := range b.ClonesByType {
		merged.ClonesByType[cloneType] += count
	}
	if merged.TotalClonePairs > 0 {
		merged.AverageSimilarity = (a.AverageSimilarity*float64(a.TotalClonePairs) +
			b.AverageSimilarity*float64(b.TotalClonePairs)) / float64(merged.TotalClonePairs)
	}
	return merged
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
