// Package clone detects duplicated functions with the tree edit distance and
// classification algorithms of core/clone, over the trees the engine builds.
package clone

import (
	"runtime"
	"sort"
	"strconv"
	"sync"

	"github.com/ludo-technologies/polyscan/core/apted"
	coreclone "github.com/ludo-technologies/polyscan/core/clone"
	"github.com/ludo-technologies/polyscan/core/domain"
	"github.com/ludo-technologies/polyscan/core/lsh"
	"github.com/ludo-technologies/polyscan/polyscan/internal/engine"
)

// Config tunes clone detection.
type Config struct {
	// MinLines and MinNodes drop fragments too small to be meaningful clones.
	MinLines int
	MinNodes int
	// Type thresholds classify a pair by its structural similarity, with
	// Type-1 additionally requiring identical text and Type-2 a matching
	// normalized syntax tree. Type-4 is not reported: core reserves it for
	// semantic analysis, which polyscan does not run, and below the Type-3
	// threshold structural similarity alone mostly finds functions that
	// merely share a skeleton.
	Type1Threshold float64
	Type2Threshold float64
	Type3Threshold float64
	// SimilarityThreshold is the lowest similarity reported.
	SimilarityThreshold float64
	// MaxEditDistance drops pairs whose raw edit distance exceeds it. Zero
	// disables the limit.
	MaxEditDistance float64
	// MaxPairs bounds the reported pairs to the strongest ones.
	MaxPairs int
	// Grouping merges pairs into groups.
	Grouping coreclone.GroupingConfig
	// LSH configures candidate generation. It is used when the number of
	// all fragment pairs exceeds MaxPairs, in which case only pairs whose
	// MinHash signatures collide are compared.
	LSH LSHConfig
}

// LSHConfig configures the MinHash index used on large inputs.
type LSHConfig struct {
	SimilarityThreshold float64
	Bands               int
	Rows                int
	Hashes              int
	MaxCandidates       int
}

// DefaultConfig returns the thresholds shared by every polyscan analyzer.
func DefaultConfig() Config {
	return Config{
		MinLines:            domain.DefaultCloneMinLines,
		MinNodes:            domain.DefaultCloneMinNodes,
		Type1Threshold:      domain.DefaultType1CloneThreshold,
		Type2Threshold:      domain.DefaultType2CloneThreshold,
		Type3Threshold:      domain.DefaultType3CloneThreshold,
		SimilarityThreshold: domain.DefaultType3CloneThreshold,
		MaxEditDistance:     domain.DefaultCloneMaxEditDistance,
		MaxPairs:            10000,
		Grouping: coreclone.GroupingConfig{
			Mode:      coreclone.ModeConnected,
			Threshold: domain.DefaultType3CloneThreshold,
			KCoreK:    2,
		},
		LSH: LSHConfig{
			SimilarityThreshold: domain.DefaultLSHSimilarityThreshold,
			Bands:               domain.DefaultLSHBands,
			Rows:                domain.DefaultLSHRows,
			Hashes:              domain.DefaultLSHHashes,
			MaxCandidates:       1024,
		},
	}
}

// Pair is a reported clone pair.
type Pair struct {
	ID         int              `json:"id"`
	Type       domain.CloneType `json:"type"`
	Similarity float64          `json:"similarity"`
	Distance   float64          `json:"distance"`
	Confidence float64          `json:"confidence"`
	Fragment1  Fragment         `json:"fragment1"`
	Fragment2  Fragment         `json:"fragment2"`
}

// Group is a set of fragments that are clones of each other.
type Group struct {
	ID         int              `json:"id"`
	Type       domain.CloneType `json:"type"`
	Similarity float64          `json:"similarity"`
	Fragments  []Fragment       `json:"fragments"`
}

// Fragment locates one function that takes part in a clone.
type Fragment struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
	LineCount int    `json:"line_count"`
	NodeCount int    `json:"node_count"`
}

// Statistics summarizes a detection run.
type Statistics struct {
	// TotalFragments counts the functions large enough to be compared.
	TotalFragments int `json:"total_fragments"`
	// TotalClones counts the distinct fragments that appear in a pair or
	// group.
	TotalClones       int            `json:"total_clones"`
	TotalClonePairs   int            `json:"total_clone_pairs"`
	TotalCloneGroups  int            `json:"total_clone_groups"`
	ClonesByType      map[string]int `json:"clones_by_type"`
	AverageSimilarity float64        `json:"average_similarity"`
}

// Report is the result of a detection run.
type Report struct {
	Pairs      []Pair     `json:"pairs"`
	Groups     []Group    `json:"groups"`
	Statistics Statistics `json:"statistics"`
}

// Detector detects clones among fragments of one language.
type Detector struct {
	config     Config
	costModel  apted.CostModel
	extractor  *coreclone.ASTFeatureExtractor
	textual    *coreclone.TextualSimilarityAnalyzer
	classifier *coreclone.PairClassifier

	fragments []*coreclone.CodeFragment
	names     []string
}

// NewDetector creates a detector for the language described by spec.
func NewDetector(spec engine.CloneSpec, config Config) *Detector {
	extractor := func() *coreclone.ASTFeatureExtractor {
		return coreclone.NewASTFeatureExtractor().
			WithPatterns(spec.Patterns).
			WithLiteralNames(append(append([]string{}, spec.Identifiers...), spec.Literals...))
	}
	// The engine already removed comments from the fragment content, so the
	// textual analyzer needs no comment stripper of its own.
	textual := coreclone.NewTextualSimilarityAnalyzer(nil)
	syntactic := coreclone.NewSyntacticSimilarityAnalyzerWithExtractor(extractor().WithOptions(3, 4, true, false))
	classifier := coreclone.NewPairClassifier(coreclone.ClassifierConfig{
		Type1Threshold: config.Type1Threshold, Type2Threshold: config.Type2Threshold,
		Type3Threshold: config.Type3Threshold,
		EnableType1:    true, EnableType2: true, EnableType3: true,
		JaccardPreFilterThreshold: 0.10,
	}, textual, syntactic)

	return &Detector{
		config:     config,
		costModel:  newCostModel(spec),
		extractor:  extractor(),
		textual:    textual,
		classifier: classifier,
	}
}

// Add registers a function as a fragment when it is large enough. The
// display name is kept for the report.
func (d *Detector) Add(fn engine.Function, filePath string) {
	nodeCount := fn.Tree.Size()
	lineCount := fn.EndLine - fn.StartLine + 1
	if nodeCount < d.config.MinNodes || lineCount < d.config.MinLines {
		return
	}
	apted.PrepareTreeForAPTED(fn.Tree)
	features, err := d.extractor.ExtractFeatures(fn.Tree)
	if err != nil {
		// The extractor only fails on a nil tree, which Add never passes.
		panic(err)
	}
	d.fragments = append(d.fragments, &coreclone.CodeFragment{
		ID:         len(d.fragments),
		FilePath:   filePath,
		StartLine:  fn.StartLine,
		EndLine:    fn.EndLine,
		StartCol:   fn.StartColumn,
		EndCol:     fn.EndColumn,
		Content:    fn.Content,
		Hash:       d.textual.HashFragmentContent(fn.Content),
		ASTNode:    fn.Tree,
		NodeCount:  nodeCount,
		LineCount:  lineCount,
		Complexity: fn.Complexity,
		Features:   features,
	})
	d.names = append(d.names, fn.Name)
}

// candidateChunkSize bounds how many candidate pairs are verified at once,
// so that the candidate bookkeeping stays a few megabytes however many
// candidates the index produces.
const candidateChunkSize = 1 << 16

// Detect compares the registered fragments and reports the clone pairs and
// groups among them.
func (d *Detector) Detect() *Report {
	var pairs []*pair
	chunk := make([][2]int, 0, candidateChunkSize)
	flush := func() {
		pairs = append(pairs, d.verify(chunk)...)
		chunk = chunk[:0]
		// The strongest MaxPairs of a prefix are among the strongest
		// MaxPairs overall, so trimming early bounds memory on clone-rich
		// input without changing the result.
		if len(pairs) > 2*d.config.MaxPairs {
			pairs = strongest(pairs, d.config.MaxPairs)
		}
	}
	d.eachCandidate(func(i, j int) {
		chunk = append(chunk, [2]int{i, j})
		if len(chunk) == candidateChunkSize {
			flush()
		}
	})
	flush()
	pairs = strongest(pairs, d.config.MaxPairs)
	pairs, groups := d.group(pairs)
	return d.report(pairs, groups)
}

// eachCandidate visits every fragment index pair worth comparing, as (i, j)
// with i < j, each pair once, in a deterministic order. Every pair is a
// candidate until the count exceeds MaxPairs; past that, only pairs whose
// MinHash signatures collide in the LSH index and estimate at least the LSH
// similarity threshold are.
func (d *Detector) eachCandidate(visit func(i, j int)) {
	n := len(d.fragments)
	if n*(n-1)/2 <= d.config.MaxPairs {
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				visit(i, j)
			}
		}
		return
	}

	hasher := lsh.NewMinHasher(d.config.LSH.Hashes)
	index := lsh.NewLSHIndex(d.config.LSH.Bands, d.config.LSH.Rows)
	signatures := make([]*lsh.MinHashSignature, n)
	for i, fragment := range d.fragments {
		signatures[i] = hasher.ComputeSignature(fragment.Features)
		if err := index.AddFragment(strconv.Itoa(i), signatures[i]); err != nil {
			panic(err) // Only a duplicate ID fails, and IDs are sequential.
		}
	}
	similar := func(i, j int) bool {
		return hasher.EstimateJaccardSimilarity(signatures[i], signatures[j]) >= d.config.LSH.SimilarityThreshold
	}

	// Sharing a bucket is symmetric, so an unordered pair shows up in both
	// members' queries and is visited from the lower index. A query that
	// hits MaxCandidates lists the lowest indexes of its buckets, though,
	// and can stop short of a higher index, so the candidates of capped
	// queries are kept and the higher index visits the pair instead when
	// the lower one missed it.
	capped := map[int]map[int]struct{}{}
	for i := range d.fragments {
		ids := index.FindCandidatesLimit(signatures[i], d.config.LSH.MaxCandidates)
		var mine map[int]struct{}
		if len(ids) == d.config.LSH.MaxCandidates {
			mine = make(map[int]struct{}, len(ids))
		}
		for _, id := range ids {
			j, err := strconv.Atoi(id)
			if err != nil {
				panic(err)
			}
			if mine != nil {
				mine[j] = struct{}{}
			}
			switch {
			case j == i:
			case j > i:
				if similar(i, j) {
					visit(i, j)
				}
			default:
				if listed, ok := capped[j]; ok {
					if _, seen := listed[i]; !seen && similar(j, i) {
						visit(j, i)
					}
				}
			}
		}
		if mine != nil {
			capped[i] = mine
		}
	}
}

// strongest returns the limit strongest pairs in order.
func strongest(pairs []*pair, limit int) []*pair {
	sort.Slice(pairs, func(i, j int) bool { return pairPrecedes(pairs[i], pairs[j]) })
	if len(pairs) > limit {
		return pairs[:limit]
	}
	return pairs
}

// measurement is what comparing two fragments yields.
type measurement struct {
	distance   float64
	similarity float64
	cloneType  domain.CloneType
	confidence float64
}

// pair is a candidate that classified as a clone.
type pair struct {
	measurement
	fragment1 *coreclone.CodeFragment
	fragment2 *coreclone.CodeFragment
}

// verify runs the tree edit distance over the candidates on every CPU and
// returns the pairs that classify as clones, in candidate order so that the
// result does not depend on scheduling.
func (d *Detector) verify(candidates [][2]int) []*pair {
	if len(candidates) == 0 {
		return nil
	}
	measurements := make([]measurement, len(candidates))
	accepted := make([]bool, len(candidates))

	workers := min(runtime.NumCPU(), len(candidates))
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			// The analyzer reuses scratch buffers across comparisons, so
			// each worker needs its own.
			analyzer := apted.NewAPTEDAnalyzerWithNormalization(d.costModel, apted.NormalizeByMax)
			for index := offset; index < len(candidates); index += workers {
				candidate := candidates[index]
				measurements[index], accepted[index] = d.measure(analyzer, d.fragments[candidate[0]], d.fragments[candidate[1]])
			}
		}(worker)
	}
	wg.Wait()

	var pairs []*pair
	for index, candidate := range candidates {
		if accepted[index] {
			pairs = append(pairs, &pair{
				measurement: measurements[index],
				fragment1:   d.fragments[candidate[0]],
				fragment2:   d.fragments[candidate[1]],
			})
		}
	}
	return pairs
}

// measure compares two fragments. It touches no detector state, so callers
// may run it concurrently as long as each has its own analyzer.
func (d *Detector) measure(analyzer *apted.APTEDAnalyzer, f1, f2 *coreclone.CodeFragment) (measurement, bool) {
	if coreclone.LocationsOverlap(f1.ItemLocation(), f2.ItemLocation()) {
		return measurement{}, false
	}
	if !coreclone.ShouldCompareFragments(f1, f2) || !d.classifier.PassesJaccardPreFilter(f1, f2) {
		return measurement{}, false
	}

	distance, similarity := analyzer.ComputeDistanceAndSimilarity(f1.ASTNode, f2.ASTNode)
	cloneType, similarity := d.classifier.ClassifyPair(f1, f2, similarity)
	if cloneType == 0 || similarity < d.config.SimilarityThreshold {
		return measurement{}, false
	}
	if d.config.MaxEditDistance > 0 && distance > d.config.MaxEditDistance {
		return measurement{}, false
	}
	return measurement{
		distance:   distance,
		similarity: similarity,
		cloneType:  cloneType,
		confidence: coreclone.CalculateConfidence(f1, f2, similarity),
	}, true
}

// pairPrecedes orders pairs by descending similarity, then by location, so
// that the MaxPairs cut is the same on every run.
func pairPrecedes(a, b *pair) bool {
	if a.similarity != b.similarity {
		return a.similarity > b.similarity
	}
	if a.fragment1.ID != b.fragment1.ID {
		return a.fragment1.ID < b.fragment1.ID
	}
	return a.fragment2.ID < b.fragment2.ID
}

// group merges pairs into groups with the configured strategy and applies
// the core dedupe passes, which can also suppress pairs.
func (d *Detector) group(pairs []*pair) ([]*pair, []*coreclone.ItemGroup[*coreclone.CodeFragment]) {
	items := make([]*coreclone.ItemPair[*coreclone.CodeFragment], 0, len(pairs))
	originals := make(map[*coreclone.ItemPair[*coreclone.CodeFragment]]*pair, len(pairs))
	for _, p := range pairs {
		item := &coreclone.ItemPair[*coreclone.CodeFragment]{
			Item1: p.fragment1, Item2: p.fragment2,
			Similarity: p.similarity, PairType: p.cloneType,
		}
		items = append(items, item)
		originals[item] = p
	}

	strategy := coreclone.NewGroupingStrategy[*coreclone.CodeFragment](d.config.Grouping)
	members := coreclone.DedupeStrictSubsetGroupMembers(strategy.GroupItems(items), items)
	covered := coreclone.DedupeCoveredGroups(members.Groups)
	groups := coreclone.FilterGroupsWithoutBackingPairs(covered.Groups, items)
	for key := range members.Suppressed {
		covered.Suppressed[key] = struct{}{}
	}
	items = coreclone.FilterPairsWithSuppressedMembers(items, covered.Suppressed)
	items = coreclone.FilterSuppressedPairs(items, covered.SuppressedPairs)

	kept := make([]*pair, 0, len(items))
	for _, item := range items {
		kept = append(kept, originals[item])
	}
	return kept, groups
}

func (d *Detector) report(pairs []*pair, groups []*coreclone.ItemGroup[*coreclone.CodeFragment]) *Report {
	report := &Report{
		Pairs:  []Pair{},
		Groups: []Group{},
		Statistics: Statistics{
			TotalFragments:   len(d.fragments),
			TotalClonePairs:  len(pairs),
			TotalCloneGroups: len(groups),
			ClonesByType:     map[string]int{},
		},
	}
	clones := map[int]struct{}{}
	totalSimilarity := 0.0
	for i, p := range pairs {
		report.Pairs = append(report.Pairs, Pair{
			ID:         i,
			Type:       p.cloneType,
			Similarity: p.similarity,
			Distance:   p.distance,
			Confidence: p.confidence,
			Fragment1:  d.fragment(p.fragment1),
			Fragment2:  d.fragment(p.fragment2),
		})
		report.Statistics.ClonesByType[p.cloneType.String()]++
		totalSimilarity += p.similarity
		clones[p.fragment1.ID] = struct{}{}
		clones[p.fragment2.ID] = struct{}{}
	}
	for _, group := range groups {
		out := Group{ID: group.ID, Type: group.GroupType, Similarity: group.Similarity}
		for _, item := range group.Items {
			out.Fragments = append(out.Fragments, d.fragment(item))
			clones[item.ID] = struct{}{}
		}
		report.Groups = append(report.Groups, out)
	}
	report.Statistics.TotalClones = len(clones)
	if len(pairs) > 0 {
		report.Statistics.AverageSimilarity = totalSimilarity / float64(len(pairs))
	}
	return report
}

func (d *Detector) fragment(f *coreclone.CodeFragment) Fragment {
	return Fragment{
		ID:        f.ID,
		Name:      d.names[f.ID],
		FilePath:  f.FilePath,
		StartLine: f.StartLine,
		EndLine:   f.EndLine,
		LineCount: f.LineCount,
		NodeCount: f.NodeCount,
	}
}
