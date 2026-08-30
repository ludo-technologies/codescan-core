package analyzer

import (
	"bytes"
	"container/heap"
	"context"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"

	"github.com/ludo-technologies/polyscan/core/apted"
	coreclone "github.com/ludo-technologies/polyscan/core/clone"
	coredomain "github.com/ludo-technologies/polyscan/core/domain"
	corelsh "github.com/ludo-technologies/polyscan/core/lsh"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/constants"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

// CodeLocation represents a location in source code
type CodeLocation struct {
	FilePath  string
	StartLine int
	EndLine   int
	StartCol  int
	EndCol    int
}

// String returns string representation of CodeLocation
func (cl *CodeLocation) String() string {
	return fmt.Sprintf("%s:%d:%d-%d:%d", cl.FilePath, cl.StartLine, cl.StartCol, cl.EndLine, cl.EndCol)
}

// CodeFragment represents a fragment of code
type CodeFragment struct {
	Location   *CodeLocation
	ASTNode    *parser.Node
	TreeNode   *apted.TreeNode
	Content    string   // Original source code content
	Hash       string   // FNV-64a hex hash of Type-1 normalized content; "" when no source content
	Size       int      // Number of AST nodes
	LineCount  int      // Number of source lines
	Complexity int      // Cyclomatic complexity (if applicable)
	Features   []string // Detector-populated clone feature cache for this fragment's tree
}

// NewCodeFragment creates a new code fragment
func NewCodeFragment(location *CodeLocation, astNode *parser.Node, content string) *CodeFragment {
	return &CodeFragment{
		Location:  location,
		ASTNode:   astNode,
		Content:   content,
		Hash:      fragmentHashNormalizer.HashFragmentContent(content),
		Size:      calculateASTSize(astNode),
		LineCount: location.EndLine - location.StartLine + 1,
	}
}

// calculateASTSize calculates the number of nodes in an AST
func calculateASTSize(node *parser.Node) int {
	if node == nil {
		return 0
	}

	size := 1
	for _, child := range parser.OrderedChildren(node) {
		size += calculateASTSize(child)
	}

	return size
}

// CloneDetectorConfig holds configuration for clone detection
type CloneDetectorConfig struct {
	// Minimum number of lines for a code fragment to be considered
	MinLines int

	// Minimum number of AST nodes for a code fragment
	MinNodes int

	// Similarity thresholds for different clone types
	Type1Threshold float64
	Type2Threshold float64
	Type3Threshold float64
	Type4Threshold float64

	// Minimum similarity threshold for clone reporting (user-configurable via --clone-threshold)
	SimilarityThreshold float64

	// Maximum edit distance allowed
	MaxEditDistance float64

	// Whether to ignore differences in literals
	IgnoreLiterals bool

	// Whether to ignore differences in identifiers
	IgnoreIdentifiers bool

	// Cost model to use for APTED
	CostModelType string // "default", "javascript", "weighted"

	// Performance tuning parameters
	MaxClonePairs      int // Maximum pairs to keep in memory
	BatchSizeThreshold int // Minimum fragments to trigger batching
	BatchSizeLarge     int // Batch size for normal projects
	BatchSizeSmall     int // Batch size for large projects
	LargeProjectSize   int // Fragment count threshold for large projects

	// Grouping configuration
	GroupingMode      coreclone.GroupingMode // Default: coreclone.ModeConnected
	GroupingThreshold float64                // Default: Type3Threshold
	KCoreK            int                    // Default: 2

	// LSH Configuration (optional, opt-in)
	UseLSH                 bool    // Enable LSH acceleration
	LSHSimilarityThreshold float64 // Candidate threshold using MinHash similarity
	LSHBands               int     // Number of LSH bands (default: 32)
	LSHRows                int     // Rows per band (default: 4)
	LSHMinHashCount        int     // Number of MinHash functions (default: 128)
	LSHMaxCandidates       int     // Maximum candidates returned per LSH query
}

// DefaultCloneDetectorConfig returns default configuration
func DefaultCloneDetectorConfig() *CloneDetectorConfig {
	return &CloneDetectorConfig{
		MinLines:          5,
		MinNodes:          10,
		Type1Threshold:    constants.DefaultType1CloneThreshold,
		Type2Threshold:    constants.DefaultType2CloneThreshold,
		Type3Threshold:    constants.DefaultType3CloneThreshold,
		Type4Threshold:    constants.DefaultType4CloneThreshold,
		MaxEditDistance:   50.0,
		IgnoreLiterals:    false,
		IgnoreIdentifiers: false,
		CostModelType:     "javascript",
		// Performance parameters
		MaxClonePairs:      10000,
		BatchSizeThreshold: 50,
		BatchSizeLarge:     100,
		BatchSizeSmall:     50,
		LargeProjectSize:   500,

		// Grouping defaults
		GroupingMode:      coreclone.ModeConnected,
		GroupingThreshold: constants.DefaultType4CloneThreshold,
		KCoreK:            2,

		// LSH defaults (opt-in)
		UseLSH:                 false,
		LSHSimilarityThreshold: 0.50,
		LSHBands:               32,
		LSHRows:                4,
		LSHMinHashCount:        128,
		LSHMaxCandidates:       defaultLSHMaxCandidates,
	}
}

// CloneDetector detects code clones using APTED algorithm
type CloneDetector struct {
	// Embed config fields (private to maintain encapsulation)
	cloneDetectorConfig CloneDetectorConfig

	// costModel is retained so per-worker analyzers can be created during
	// concurrent verification; APTED analyzers own mutable scratch buffers and
	// cannot be shared, while the cost model is read-only.
	costModel        apted.CostModel
	analyzer         *apted.APTEDAnalyzer
	converter        *TreeConverter
	textualAnalyzer  *coreclone.TextualSimilarityAnalyzer
	featureExtractor *coreclone.ASTFeatureExtractor
	pairClassifier   *coreclone.PairClassifier
	fragments        []*CodeFragment
	clonePairs       []*domain.ClonePair
	cloneGroups      []*domain.CloneGroup

	// fragmentClones maps each fragment to its single domain.Clone so the
	// same fragment shares one clone ID across all pairs (required for
	// grouping to recognize shared members).
	fragmentClones map[*CodeFragment]*domain.Clone
	nextCloneID    int
}

// NewCloneDetector creates a new clone detector with the given configuration
func NewCloneDetector(config *CloneDetectorConfig) *CloneDetector {
	// Create appropriate cost model based on configuration
	var costModel apted.CostModel
	switch config.CostModelType {
	case "default":
		costModel = apted.NewDefaultCostModel()
	case "javascript":
		costModel = NewJavaScriptCostModelWithConfig(config.IgnoreLiterals, config.IgnoreIdentifiers)
	case "weighted":
		baseCostModel := NewJavaScriptCostModelWithConfig(config.IgnoreLiterals, config.IgnoreIdentifiers)
		costModel = apted.NewWeightedCostModel(1.0, 1.0, 0.8, baseCostModel)
	default:
		costModel = NewJavaScriptCostModel()
	}

	analyzer := apted.NewAPTEDAnalyzerWithNormalization(costModel, apted.NormalizeByMax)
	textualAnalyzer := coreclone.NewTextualSimilarityAnalyzer(removeJSComments)
	syntacticAnalyzer := coreclone.NewSyntacticSimilarityAnalyzerWithExtractor(
		coreclone.NewASTFeatureExtractor().
			WithOptions(3, 4, true, false).
			WithPatterns(jsClonePatternNames).
			WithLiteralNames(jsLiteralLikeNames))
	classifier := coreclone.NewPairClassifier(coreclone.ClassifierConfig{
		Type1Threshold: config.Type1Threshold, Type2Threshold: config.Type2Threshold,
		Type3Threshold: config.Type3Threshold, Type4Threshold: config.Type4Threshold,
		EnableType1: true, EnableType2: true, EnableType3: true, EnableType4: true,
		JaccardPreFilterThreshold: 0.10,
	}, textualAnalyzer, syntacticAnalyzer)

	return &CloneDetector{
		cloneDetectorConfig: *config,
		costModel:           costModel,
		analyzer:            analyzer,
		converter:           NewTreeConverter(),
		textualAnalyzer:     textualAnalyzer,
		featureExtractor: coreclone.NewASTFeatureExtractor().
			WithPatterns(jsClonePatternNames).
			WithLiteralNames(jsLiteralLikeNames),
		pairClassifier: classifier,
		fragments:      []*CodeFragment{},
		clonePairs:     []*domain.ClonePair{},
		cloneGroups:    []*domain.CloneGroup{},
	}
}

// SetUseLSH enables or disables LSH acceleration for clone detection
func (cd *CloneDetector) SetUseLSH(enabled bool) {
	cd.cloneDetectorConfig.UseLSH = enabled
}

// SetBatchSizeLarge sets the batch size for normal projects (used in testing)
func (cd *CloneDetector) SetBatchSizeLarge(size int) {
	cd.cloneDetectorConfig.BatchSizeLarge = size
}

// ExtractFragments extracts code fragments from AST nodes
func (cd *CloneDetector) ExtractFragments(astNodes []*parser.Node, filePath string) []*CodeFragment {
	var fragments []*CodeFragment

	for _, node := range astNodes {
		cd.extractFragmentsRecursive(node, filePath, &fragments)
	}

	return fragments
}

// ExtractFragmentsWithSource extracts code fragments from AST nodes with source content.
// Source content is needed for Type-1 clone classification and optional report output.
func (cd *CloneDetector) ExtractFragmentsWithSource(astNodes []*parser.Node, filePath string, sourceCode []byte) []*CodeFragment {
	var fragments []*CodeFragment
	lines := splitLines(sourceCode)

	for _, node := range astNodes {
		cd.extractFragmentsRecursiveWithSource(node, filePath, lines, &fragments)
	}

	return fragments
}

// extractFragmentsRecursiveWithSource recursively extracts fragments with source content
func (cd *CloneDetector) extractFragmentsRecursiveWithSource(node *parser.Node, filePath string, lines [][]byte, fragments *[]*CodeFragment) {
	if node == nil {
		return
	}

	// Check if this node should be considered as a fragment
	if cd.isFragmentCandidate(node) {
		location := &CodeLocation{
			FilePath:  filePath,
			StartLine: node.Location.StartLine,
			EndLine:   node.Location.EndLine,
			StartCol:  node.Location.StartCol,
			EndCol:    node.Location.EndCol,
		}

		// Extract content from source code for textual (Type-1) analysis
		content := ""
		if len(lines) > 0 {
			content = cd.extractSourceContent(lines, &node.Location)
		}

		fragment := NewCodeFragment(location, node, content)

		// Filter fragments based on configuration
		if cd.shouldIncludeFragment(fragment) {
			*fragments = append(*fragments, fragment)
		}
	}

	// Recursively process children
	for _, child := range parser.OrderedChildren(node) {
		cd.extractFragmentsRecursiveWithSource(child, filePath, lines, fragments)
	}
}

// extractSourceContent extracts source code content for a given location
func (cd *CloneDetector) extractSourceContent(lines [][]byte, loc *parser.Location) string {
	if loc == nil || len(lines) == 0 {
		return ""
	}

	if loc.StartLine < 1 || loc.EndLine > len(lines) {
		return ""
	}

	// Extract lines from StartLine to EndLine (1-indexed)
	var result []byte
	for i := loc.StartLine - 1; i < loc.EndLine && i < len(lines); i++ {
		result = append(result, lines[i]...)
		if i < loc.EndLine-1 {
			result = append(result, '\n')
		}
	}

	return string(result)
}

// splitLines splits source code into lines
func splitLines(sourceCode []byte) [][]byte {
	if len(sourceCode) == 0 {
		return nil
	}

	lines := make([][]byte, 0, bytes.Count(sourceCode, []byte{'\n'})+1)
	start := 0
	for idx, b := range sourceCode {
		if b == '\n' {
			lines = append(lines, sourceCode[start:idx])
			start = idx + 1
		}
	}
	if start < len(sourceCode) {
		lines = append(lines, sourceCode[start:])
	}
	return lines
}

// extractFragmentsRecursive recursively extracts fragments from AST
func (cd *CloneDetector) extractFragmentsRecursive(node *parser.Node, filePath string, fragments *[]*CodeFragment) {
	if node == nil {
		return
	}

	// Check if this node should be considered as a fragment
	if cd.isFragmentCandidate(node) {
		location := &CodeLocation{
			FilePath:  filePath,
			StartLine: node.Location.StartLine,
			EndLine:   node.Location.EndLine,
			StartCol:  node.Location.StartCol,
			EndCol:    node.Location.EndCol,
		}

		fragment := NewCodeFragment(location, node, "")

		// Filter fragments based on configuration
		if cd.shouldIncludeFragment(fragment) {
			*fragments = append(*fragments, fragment)
		}
	}

	// Recursively process children
	for _, child := range parser.OrderedChildren(node) {
		cd.extractFragmentsRecursive(child, filePath, fragments)
	}
}

// isFragmentCandidate checks if a node should be considered as a fragment candidate
func (cd *CloneDetector) isFragmentCandidate(node *parser.Node) bool {
	// Consider functions, classes, and compound statements as fragment candidates
	candidateTypes := []parser.NodeType{
		// Function declarations
		parser.NodeFunction,
		parser.NodeFunctionExpression,
		parser.NodeArrowFunction,
		parser.NodeAsyncFunction,
		parser.NodeGeneratorFunction,
		parser.NodeMethodDefinition,
		// Class declarations
		parser.NodeClass,
		parser.NodeClassExpression,
		// Control flow statements
		parser.NodeForStatement,
		parser.NodeForInStatement,
		parser.NodeForOfStatement,
		parser.NodeWhileStatement,
		parser.NodeDoWhileStatement,
		parser.NodeIfStatement,
		parser.NodeTryStatement,
		parser.NodeSwitchStatement,
		parser.NodeWithStatement,
	}

	for _, candidateType := range candidateTypes {
		if node.Type == candidateType {
			return true
		}
	}

	return false
}

// shouldIncludeFragment determines if a fragment should be included in analysis
func (cd *CloneDetector) shouldIncludeFragment(fragment *CodeFragment) bool {
	// Check minimum size requirements
	if fragment.Size < cd.cloneDetectorConfig.MinNodes {
		return false
	}

	if fragment.LineCount < cd.cloneDetectorConfig.MinLines {
		return false
	}

	return true
}

// cancellationCheckInterval is how many comparisons run between context checks.
// Checking a context is cheap but not free, and comparisons are short.
const cancellationCheckInterval = 10

// isCancelled checks if the context is cancelled
func isCancelled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// DetectClones detects clones in the given code fragments.
//
// The fragments are consumed, not just read: each one's ASTNode is released
// once its APTED tree exists, so callers must not read CodeFragment.ASTNode
// after handing fragments to a detector. See prepareFragments.
func (cd *CloneDetector) DetectClones(fragments []*CodeFragment) ([]*domain.ClonePair, []*domain.CloneGroup) {
	return cd.DetectClonesWithContext(context.Background(), fragments)
}

// DetectClonesWithContext detects clones with context support for cancellation.
// It consumes the fragments' ASTNode references; see DetectClones.
func (cd *CloneDetector) DetectClonesWithContext(ctx context.Context, fragments []*CodeFragment) ([]*domain.ClonePair, []*domain.CloneGroup) {
	cd.fragments = fragments
	cd.clonePairs = []*domain.ClonePair{}
	cd.cloneGroups = []*domain.CloneGroup{}
	cd.fragmentClones = make(map[*CodeFragment]*domain.Clone)
	cd.nextCloneID = 0

	// Check for cancellation before starting
	if isCancelled(ctx) {
		return cd.clonePairs, cd.cloneGroups
	}

	// Convert AST fragments to tree nodes
	cd.prepareFragments()

	// Check for cancellation after preparation
	if isCancelled(ctx) {
		return cd.clonePairs, cd.cloneGroups
	}

	// Detect clone pairs with context
	cd.detectClonePairsWithContext(ctx)

	// Check for cancellation before grouping
	if isCancelled(ctx) {
		return cd.clonePairs, cd.cloneGroups
	}

	// Group related clones using configured strategy
	// Clamp threshold to [0,1]
	thr := cd.cloneDetectorConfig.GroupingThreshold
	if thr < 0.0 {
		thr = 0.0
	} else if thr > 1.0 {
		thr = 1.0
	}
	k := cd.cloneDetectorConfig.KCoreK
	if k < 2 {
		k = 2
	}
	strategy := coreclone.NewGroupingStrategy[*domain.Clone](coreclone.GroupingConfig{Mode: cd.cloneDetectorConfig.GroupingMode, Threshold: thr, KCoreK: k})
	cd.groupClonesWithStrategy(strategy)

	return cd.clonePairs, cd.cloneGroups
}

// DetectClonesWithLSH runs a two-stage pipeline using LSH for candidate generation,
// followed by APTED verification on candidates only. Falls back to exhaustive if misconfigured.
// It consumes the fragments' ASTNode references; see DetectClones.
func (cd *CloneDetector) DetectClonesWithLSH(ctx context.Context, fragments []*CodeFragment) ([]*domain.ClonePair, []*domain.CloneGroup) {
	// If not enabled, delegate to standard path
	if cd == nil || !cd.cloneDetectorConfig.UseLSH {
		return cd.DetectClonesWithContext(ctx, fragments)
	}

	cd.fragments = fragments
	cd.clonePairs = []*domain.ClonePair{}
	cd.cloneGroups = []*domain.CloneGroup{}
	cd.fragmentClones = make(map[*CodeFragment]*domain.Clone)
	cd.nextCloneID = 0

	if isCancelled(ctx) {
		return cd.clonePairs, cd.cloneGroups
	}

	// Prepare TreeNodes for APTED and feature extraction
	cd.prepareFragments()

	if isCancelled(ctx) {
		return cd.clonePairs, cd.cloneGroups
	}

	// Stage 1: MinHash signatures from prepared clone features
	hasher := corelsh.NewMinHasher(cd.cloneDetectorConfig.LSHMinHashCount)

	type fragRec struct {
		idx int
		sig *corelsh.MinHashSignature
	}
	records := make([]fragRec, 0, len(cd.fragments))
	sigByIndex := make(map[int]*corelsh.MinHashSignature, len(cd.fragments))
	for i, f := range cd.fragments {
		if f == nil || f.TreeNode == nil {
			continue
		}
		sig := hasher.ComputeSignature(f.Features)
		records = append(records, fragRec{idx: i, sig: sig})
		sigByIndex[i] = sig
	}

	// Edge case: if signatures cannot be built, fallback
	if len(records) <= 1 {
		return cd.DetectClonesWithContext(ctx, fragments)
	}

	// Stage 2: LSH indexing
	lsh := newLSHCandidateIndex(cd.cloneDetectorConfig.LSHBands, cd.cloneDetectorConfig.LSHRows, cd.cloneDetectorConfig.LSHMaxCandidates)
	for _, r := range records {
		_ = lsh.AddFragment(r.idx, r.sig)
	}

	// Stage 3: Candidate generation + APTED verification
	// Use MinHash similarity to filter before expensive APTED
	minhashThreshold := cd.cloneDetectorConfig.LSHSimilarityThreshold
	if minhashThreshold < 0 {
		minhashThreshold = 0
	} else if minhashThreshold > 1 {
		minhashThreshold = 1
	}

	// Stage 3: generate candidate pairs that survive the cheap filters and
	// verify them a chunk at a time. Candidate count grows with fragments times
	// candidates-per-query, which on a large repository runs to orders of
	// magnitude more pairs than are ever reported, so they are never all held
	// at once. Chunks are verified in generation order, which is what keeps
	// pair IDs and clone identities independent of the chunk size.
	seenPairs := make(map[[2]int]struct{})
	chunk := make([][2]int, 0, candidateChunkSize)

	for _, r := range records {
		if isCancelled(ctx) {
			break
		}
		for _, j := range lsh.FindCandidates(r.sig) {
			i := r.idx
			if j == i || j < 0 || i < 0 {
				continue
			}
			// Deduplicate unordered pair (i<j)
			a, b := i, j
			if a > b {
				a, b = b, a
			}
			key := [2]int{a, b}
			if _, ok := seenPairs[key]; ok {
				continue
			}
			seenPairs[key] = struct{}{}

			f1 := cd.fragments[a]
			f2 := cd.fragments[b]
			if cd.isOverlappingLocation(f1.Location, f2.Location) {
				continue
			}
			if f1.TreeNode == nil || f2.TreeNode == nil {
				continue
			}

			// MinHash similarity pre-filter
			if hasher.EstimateJaccardSimilarity(sigByIndex[a], sigByIndex[b]) < minhashThreshold {
				continue
			}

			chunk = append(chunk, key)
			if len(chunk) == candidateChunkSize {
				// APTED verification, the expensive stage, across workers.
				cd.clonePairs = cd.verifyCandidates(ctx, chunk, cd.clonePairs)
				chunk = chunk[:0]
			}
		}
	}
	cd.clonePairs = cd.verifyCandidates(ctx, chunk, cd.clonePairs)

	// Finalize results
	cd.limitAndSortClonePairs(cd.cloneDetectorConfig.MaxClonePairs)

	// Grouping
	thr := cd.cloneDetectorConfig.GroupingThreshold
	if thr < 0.0 {
		thr = 0.0
	} else if thr > 1.0 {
		thr = 1.0
	}
	k := cd.cloneDetectorConfig.KCoreK
	if k < 2 {
		k = 2
	}
	strategy := coreclone.NewGroupingStrategy[*domain.Clone](coreclone.GroupingConfig{Mode: cd.cloneDetectorConfig.GroupingMode, Threshold: thr, KCoreK: k})
	cd.groupClonesWithStrategy(strategy)

	return cd.clonePairs, cd.cloneGroups
}

// prepareFragments converts AST fragments to tree nodes and populates clone features.
func (cd *CloneDetector) prepareFragments() {
	for _, fragment := range cd.fragments {
		if fragment == nil {
			continue
		}
		if fragment.TreeNode == nil && fragment.ASTNode != nil {
			fragment.TreeNode = cd.converter.ConvertAST(fragment.ASTNode)
		}
		if fragment.TreeNode == nil {
			continue
		}
		// Nothing downstream reads the parser AST once the APTED tree exists,
		// and fragments hold it for the whole run. Dropping the last reference
		// lets the parse trees of already-converted files be collected while
		// later files are still being prepared. The converter deliberately does
		// not copy parser nodes into TreeNode.OriginalNode, so this really is
		// the last reference — see TreeConverter.ConvertAST.
		fragment.ASTNode = nil
		apted.PrepareTreeForAPTED(fragment.TreeNode)
		features, _ := cd.featureExtractor.ExtractFeatures(fragment.TreeNode)
		fragment.Features = features
	}
}

// calculateBatchSize determines the optimal batch size based on fragment count
func (cd *CloneDetector) calculateBatchSize(fragmentCount int) int {
	if fragmentCount < cd.cloneDetectorConfig.BatchSizeThreshold {
		return fragmentCount // No batching needed
	}
	if fragmentCount > cd.cloneDetectorConfig.LargeProjectSize {
		return cd.cloneDetectorConfig.BatchSizeSmall
	}
	return cd.cloneDetectorConfig.BatchSizeLarge
}

// detectClonePairsWithContext detects pairs with context support
func (cd *CloneDetector) detectClonePairsWithContext(ctx context.Context) {
	n := len(cd.fragments)

	// Early return for small datasets
	if n <= 1 {
		return
	}

	// Determine if batching is needed based on fragment count and estimated pairs
	estimatedPairs := (n * (n - 1)) / 2
	needsBatching := n > cd.cloneDetectorConfig.BatchSizeThreshold || estimatedPairs > cd.cloneDetectorConfig.MaxClonePairs

	if needsBatching {
		batchSize := cd.calculateBatchSize(n)
		cd.detectClonePairsWithBatchingContext(ctx, cd.cloneDetectorConfig.MaxClonePairs, batchSize)
	} else {
		cd.detectClonePairsStandardWithContext(ctx)
	}

	// Sort and limit final results
	cd.limitAndSortClonePairs(cd.cloneDetectorConfig.MaxClonePairs)
}

// detectClonePairsStandardWithContext uses standard approach with context
func (cd *CloneDetector) detectClonePairsStandardWithContext(ctx context.Context) {
	n := len(cd.fragments)

	pairID := 0
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {

			// Check for cancellation periodically
			if (i*n+j)%cancellationCheckInterval == 0 && isCancelled(ctx) {
				return
			}
			fragment1 := cd.fragments[i]
			fragment2 := cd.fragments[j]

			// Skip if both fragments are from the same location or overlap in the same file
			if cd.isOverlappingLocation(fragment1.Location, fragment2.Location) {
				continue
			}

			// Compute similarity
			pair := cd.compareFragments(fragment1, fragment2, pairID)
			if pair != nil && cd.isSignificantClone(pair) {
				cd.clonePairs = append(cd.clonePairs, pair)
				pairID++
			}
		}
	}
}

// detectClonePairsWithBatchingContext processes batches with context support
func (cd *CloneDetector) detectClonePairsWithBatchingContext(ctx context.Context, maxPairs, batchSize int) {
	n := len(cd.fragments)

	// Ensure maxPairs has a reasonable minimum value
	if maxPairs <= 0 {
		maxPairs = 10000
	}
	if batchSize <= 0 {
		batchSize = 100
	}

	// Keep only the best pairs, in a heap ordered by ascending similarity so the
	// weakest pair — the one a new pair has to beat — is always at the root.
	topPairs := &weakestFirstPairs{pairs: make([]*domain.ClonePair, 0, maxPairs)}
	minSimilarity := cd.cloneDetectorConfig.Type4Threshold // Use the lowest threshold as minimum

	pairID := 0
	record := func(pair *domain.ClonePair) {
		if pair == nil {
			return
		}
		topPairs.push(pair, maxPairs)
		pairID++
		// Once the heap is full, only pairs stronger than its weakest member can
		// still make the cut, so raise the bar for later comparisons.
		if topPairs.Len() >= maxPairs {
			minSimilarity = topPairs.weakest().Similarity
		}
	}

	// Process in batches to limit memory usage
	for batchStart := 0; batchStart < n; batchStart += batchSize {
		// Check for cancellation at batch start
		if isCancelled(ctx) {
			cd.clonePairs = topPairs.pairs
			return
		}
		batchEnd := batchStart + batchSize
		if batchEnd > n {
			batchEnd = n
		}

		// Process current batch against all previous and current fragments
		for i := batchStart; i < batchEnd; i++ {
			// Compare with fragments in current batch
			for j := i + 1; j < batchEnd; j++ {
				record(cd.tryCreateClonePair(i, j, minSimilarity, pairID))
			}

			// Compare with all previous fragments
			for j := 0; j < batchStart; j++ {
				record(cd.tryCreateClonePair(i, j, minSimilarity, pairID))
			}
		}
	}

	// Replace clone pairs with the best ones found
	cd.clonePairs = topPairs.pairs
}

// weakestFirstPairs is a bounded min-heap of clone pairs keyed on similarity.
// Keeping the top N pairs by re-sorting on every insert costs O(N log N) per
// accepted pair; a heap makes it O(log N).
type weakestFirstPairs struct {
	pairs []*domain.ClonePair
}

func (h *weakestFirstPairs) Len() int { return len(h.pairs) }

func (h *weakestFirstPairs) Less(i, j int) bool {
	return h.pairs[i].Similarity < h.pairs[j].Similarity
}

func (h *weakestFirstPairs) Swap(i, j int) { h.pairs[i], h.pairs[j] = h.pairs[j], h.pairs[i] }

func (h *weakestFirstPairs) Push(x any) { h.pairs = append(h.pairs, x.(*domain.ClonePair)) }

func (h *weakestFirstPairs) Pop() any {
	last := len(h.pairs) - 1
	pair := h.pairs[last]
	h.pairs = h.pairs[:last]
	return pair
}

// weakest returns the lowest-similarity pair currently retained.
func (h *weakestFirstPairs) weakest() *domain.ClonePair { return h.pairs[0] }

// push adds a pair, evicting the weakest one when the heap is already at limit
// and the newcomer is stronger.
func (h *weakestFirstPairs) push(pair *domain.ClonePair, limit int) {
	if len(h.pairs) < limit {
		heap.Push(h, pair)
		return
	}
	if pair.Similarity > h.weakest().Similarity {
		h.pairs[0] = pair
		heap.Fix(h, 0)
	}
}

// clonePairMeasurement is what comparing two fragments yields, before the
// detector assigns clone identities and pair IDs. Keeping measurement separate
// from that bookkeeping is what lets comparisons run concurrently.
type clonePairMeasurement struct {
	distance   float64
	similarity float64
	cloneType  domain.CloneType
	confidence float64
}

// measureFragments compares two fragments with the given APTED analyzer and
// reports whether they form a clone pair. It touches no detector state, so
// callers may run it concurrently as long as each has its own analyzer.
// Uses a Jaccard pre-filter on pre-computed features to minimize expensive APTED calls.
func (cd *CloneDetector) measureFragments(analyzer *apted.APTEDAnalyzer, fragment1, fragment2 *CodeFragment) (clonePairMeasurement, bool) {
	if fragment1.TreeNode == nil || fragment2.TreeNode == nil {
		return clonePairMeasurement{}, false
	}

	// Early filtering check
	coreFragment1, coreFragment2 := toCoreFragment(fragment1, 0), toCoreFragment(fragment2, 1)
	if !coreclone.ShouldCompareFragments(coreFragment1, coreFragment2) {
		return clonePairMeasurement{}, false
	}

	// Jaccard pre-filter: reject clear non-clones before expensive APTED work.
	// Only used for rejection — all non-rejected pairs proceed to APTED-based
	// classification for accurate clone typing and distance computation.
	if !cd.pairClassifier.PassesJaccardPreFilter(coreFragment1, coreFragment2) {
		return clonePairMeasurement{}, false
	}

	// Compute edit distance and similarity using APTED algorithm
	distance, similarity := analyzer.ComputeDistanceAndSimilarity(fragment1.TreeNode, fragment2.TreeNode)

	// Determine clone type with textual/syntactic gating
	coreType, similarity := cd.pairClassifier.ClassifyPair(coreFragment1, coreFragment2, similarity)
	cloneType := domain.CloneType(coreType)
	if cloneType == 0 {
		return clonePairMeasurement{}, false // Not a significant clone
	}

	return clonePairMeasurement{
		distance:   distance,
		similarity: similarity,
		cloneType:  cloneType,
		// Calculate confidence based on fragment size and similarity
		confidence: coreclone.CalculateConfidence(coreFragment1, coreFragment2, similarity),
	}, true
}

// compareFragments compares two fragments and returns a clone pair if similar.
func (cd *CloneDetector) compareFragments(fragment1, fragment2 *CodeFragment, pairID int) *domain.ClonePair {
	measurement, ok := cd.measureFragments(cd.analyzer, fragment1, fragment2)
	if !ok {
		return nil
	}
	return cd.buildClonePair(measurement, fragment1, fragment2, pairID)
}

// buildClonePair attaches clone identities to a measurement. The same fragment
// always maps to the same clone, so this must run on a single goroutine.
func (cd *CloneDetector) buildClonePair(measurement clonePairMeasurement, fragment1, fragment2 *CodeFragment, pairID int) *domain.ClonePair {
	return &domain.ClonePair{
		ID:         pairID,
		Clone1:     cd.fragmentToClone(fragment1),
		Clone2:     cd.fragmentToClone(fragment2),
		Similarity: measurement.similarity,
		Distance:   measurement.distance,
		Type:       measurement.cloneType,
		Confidence: measurement.confidence,
	}
}

// candidateChunkSize bounds how many candidate pairs are verified at once.
// Large enough that every worker has substantial work per chunk, small enough
// that the per-candidate bookkeeping stays a few megabytes however many
// candidates the index produces.
const candidateChunkSize = 1 << 16

// verifyCandidates runs APTED verification over candidate fragment pairs across
// worker goroutines, then appends the surviving pairs to pairs on a single
// goroutine in candidate order. Pair IDs and clone identities therefore do not
// depend on how the work was scheduled or on how candidates were chunked.
//
// That guarantee covers runs that finish. A worker that observes a cancelled
// context abandons the rest of its stripe while other workers may still be
// completing theirs, so the partial results of a cancelled run do depend on
// scheduling. Callers that surface partial output must not present it as
// reproducible.
func (cd *CloneDetector) verifyCandidates(ctx context.Context, candidates [][2]int, pairs []*domain.ClonePair) []*domain.ClonePair {
	if len(candidates) == 0 {
		return pairs
	}

	measurements := make([]clonePairMeasurement, len(candidates))
	accepted := make([]bool, len(candidates))

	workers := runtime.NumCPU()
	if workers > len(candidates) {
		workers = len(candidates)
	}

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(offset int) {
			defer wg.Done()
			// Each worker needs its own analyzer: APTED reuses scratch buffers
			// across comparisons.
			analyzer := apted.NewAPTEDAnalyzerWithNormalization(cd.costModel, apted.NormalizeByMax)
			for index := offset; index < len(candidates); index += workers {
				if index%cancellationCheckInterval == 0 && isCancelled(ctx) {
					return
				}
				pair := candidates[index]
				measurements[index], accepted[index] = cd.measureFragments(
					analyzer, cd.fragments[pair[0]], cd.fragments[pair[1]])
			}
		}(worker)
	}
	wg.Wait()

	for index, candidate := range candidates {
		if !accepted[index] {
			continue
		}
		fragment1, fragment2 := cd.fragments[candidate[0]], cd.fragments[candidate[1]]
		pair := cd.buildClonePair(measurements[index], fragment1, fragment2, len(pairs))
		if cd.isSignificantClone(pair) {
			pairs = append(pairs, pair)
		}
	}
	return pairs
}

// fragmentToClone converts a CodeFragment to a domain.Clone.
// The same fragment always maps to the same clone instance and ID so that
// grouping strategies can recognize shared members across pairs.
func (cd *CloneDetector) fragmentToClone(fragment *CodeFragment) *domain.Clone {
	if cd.fragmentClones == nil {
		cd.fragmentClones = make(map[*CodeFragment]*domain.Clone)
	}
	if clone, ok := cd.fragmentClones[fragment]; ok {
		return clone
	}
	clone := cd.newCloneFromFragment(fragment, cd.nextCloneID)
	cd.nextCloneID++
	cd.fragmentClones[fragment] = clone
	return clone
}

func (cd *CloneDetector) newCloneFromFragment(fragment *CodeFragment, id int) *domain.Clone {
	return &domain.Clone{
		ID: id,
		Location: &domain.CloneLocation{
			FilePath:  fragment.Location.FilePath,
			StartLine: fragment.Location.StartLine,
			EndLine:   fragment.Location.EndLine,
			StartCol:  fragment.Location.StartCol,
			EndCol:    fragment.Location.EndCol,
		},
		Content:    fragment.Content,
		Hash:       fragment.Hash,
		Size:       fragment.Size,
		LineCount:  fragment.LineCount,
		Complexity: fragment.Complexity,
	}
}

// isSignificantClone determines if a clone pair is significant enough to report
func (cd *CloneDetector) isSignificantClone(pair *domain.ClonePair) bool {
	// Check minimum similarity threshold
	// Use SimilarityThreshold if set, otherwise fall back to Type4Threshold
	minThreshold := cd.cloneDetectorConfig.SimilarityThreshold
	if minThreshold <= 0 {
		minThreshold = cd.cloneDetectorConfig.Type4Threshold
	}
	if pair.Similarity < minThreshold {
		return false
	}

	// Check maximum distance threshold (0 means no limit)
	if pair.Type != domain.Type4Clone && cd.cloneDetectorConfig.MaxEditDistance > 0 && pair.Distance > cd.cloneDetectorConfig.MaxEditDistance {
		return false
	}

	// Additional filtering based on fragment characteristics
	minSize := math.Min(float64(pair.Clone1.Size), float64(pair.Clone2.Size))
	return minSize >= float64(cd.cloneDetectorConfig.MinNodes)
}

// groupClonesWithStrategy groups clone pairs using a pluggable strategy.
func (cd *CloneDetector) groupClonesWithStrategy(strategy coreclone.GroupingStrategy[*domain.Clone]) {
	if strategy == nil {
		cd.cloneGroups = []*domain.CloneGroup{}
		return
	}
	corePairs := make([]*coreclone.ItemPair[*domain.Clone], 0, len(cd.clonePairs))
	originals := make(map[*coreclone.ItemPair[*domain.Clone]]*domain.ClonePair, len(cd.clonePairs))
	for _, pair := range cd.clonePairs {
		if pair == nil || pair.Clone1 == nil || pair.Clone2 == nil {
			continue
		}
		converted := &coreclone.ItemPair[*domain.Clone]{Item1: pair.Clone1, Item2: pair.Clone2, Similarity: pair.Similarity, PairType: coredomain.CloneType(pair.Type)}
		corePairs = append(corePairs, converted)
		originals[converted] = pair
	}
	memberResult := coreclone.DedupeStrictSubsetGroupMembers(strategy.GroupItems(corePairs), corePairs)
	groupResult := coreclone.DedupeCoveredGroups(memberResult.Groups)
	groups := coreclone.FilterGroupsWithoutBackingPairs(groupResult.Groups, corePairs)
	for key := range memberResult.Suppressed {
		groupResult.Suppressed[key] = struct{}{}
	}
	corePairs = coreclone.FilterPairsWithSuppressedMembers(corePairs, groupResult.Suppressed)
	corePairs = coreclone.FilterSuppressedPairs(corePairs, groupResult.SuppressedPairs)
	cd.clonePairs = cd.clonePairs[:0]
	for _, pair := range corePairs {
		cd.clonePairs = append(cd.clonePairs, originals[pair])
	}
	cd.cloneGroups = make([]*domain.CloneGroup, 0, len(groups))
	for _, group := range groups {
		cd.cloneGroups = append(cd.cloneGroups, &domain.CloneGroup{ID: group.ID, Clones: group.Items, Type: domain.CloneType(group.GroupType), Similarity: group.Similarity, Size: len(group.Items)})
	}
}

func toCoreFragment(fragment *CodeFragment, id int) *coreclone.CodeFragment {
	if fragment == nil {
		return nil
	}
	result := &coreclone.CodeFragment{ID: id, Content: fragment.Content, Hash: fragment.Hash, ASTNode: fragment.TreeNode, NodeCount: fragment.Size, LineCount: fragment.LineCount, Complexity: fragment.Complexity, Features: fragment.Features}
	if fragment.Location != nil {
		result.FilePath = fragment.Location.FilePath
		result.StartLine = fragment.Location.StartLine
		result.EndLine = fragment.Location.EndLine
		result.StartCol = fragment.Location.StartCol
		result.EndCol = fragment.Location.EndCol
	}
	return result
}

// isSameLocation checks if two locations refer to the same code
func (cd *CloneDetector) isSameLocation(loc1, loc2 *CodeLocation) bool {
	return loc1.FilePath == loc2.FilePath &&
		loc1.StartLine == loc2.StartLine &&
		loc1.EndLine == loc2.EndLine
}

// isOverlappingLocation checks if two locations from the same file overlap
// (one contains or partially contains the other)
func (cd *CloneDetector) isOverlappingLocation(loc1, loc2 *CodeLocation) bool {
	if loc1.FilePath != loc2.FilePath {
		return false
	}
	// Check if ranges overlap: NOT (loc1 ends before loc2 starts OR loc2 ends before loc1 starts)
	return !(loc1.EndLine < loc2.StartLine || loc2.EndLine < loc1.StartLine)
}

// GetStatistics returns clone detection statistics
func (cd *CloneDetector) GetStatistics() map[string]interface{} {
	stats := make(map[string]interface{})

	stats["total_fragments"] = len(cd.fragments)
	stats["total_clone_pairs"] = len(cd.clonePairs)
	stats["total_clone_groups"] = len(cd.cloneGroups)

	// Count by clone type
	typeCounts := make(map[string]int)
	for _, pair := range cd.clonePairs {
		typeCounts[pair.Type.String()]++
	}
	stats["clone_types"] = typeCounts

	// Average similarity
	if len(cd.clonePairs) > 0 {
		totalSim := 0.0
		for _, pair := range cd.clonePairs {
			totalSim += pair.Similarity
		}
		stats["average_similarity"] = totalSim / float64(len(cd.clonePairs))
	}

	return stats
}

// tryCreateClonePair attempts to create a clone pair if it meets similarity threshold
func (cd *CloneDetector) tryCreateClonePair(i, j int, minSimilarity float64, pairID int) *domain.ClonePair {
	fragment1 := cd.fragments[i]
	fragment2 := cd.fragments[j]

	// Skip if both fragments are from the same location or overlap in the same file
	if cd.isOverlappingLocation(fragment1.Location, fragment2.Location) {
		return nil
	}

	// Quick similarity check before expensive computation
	if fragment1.TreeNode == nil || fragment2.TreeNode == nil {
		return nil
	}

	// Full similarity computation (compareFragments already calls shouldCompareFragments)
	pair := cd.compareFragments(fragment1, fragment2, pairID)
	if pair != nil && cd.isSignificantClone(pair) && pair.Similarity >= minSimilarity {
		return pair
	}
	return nil
}

// limitAndSortClonePairs ensures final results are sorted and limited
func (cd *CloneDetector) limitAndSortClonePairs(maxPairs int) {
	// Sort clone pairs by similarity (descending), breaking ties on source
	// location. The truncation below makes the tie-break load-bearing: without
	// it, which of a run of equally similar pairs survives the cut would depend
	// on the order candidates happened to be generated in.
	sort.Slice(cd.clonePairs, func(i, j int) bool {
		return domain.ClonePairPrecedes(cd.clonePairs[i], cd.clonePairs[j])
	})

	// Limit the number of pairs to prevent memory issues
	if len(cd.clonePairs) > maxPairs {
		cd.clonePairs = cd.clonePairs[:maxPairs]
	}
}

// Helper functions
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
