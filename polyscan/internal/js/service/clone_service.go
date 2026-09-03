package service

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/analyzer"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/version"
)

// CloneServiceImpl implements the domain.CloneService interface
type CloneServiceImpl struct {
	config *analyzer.CloneDetectorConfig
}

// NewCloneService creates a new clone detection service
func NewCloneService(config *analyzer.CloneDetectorConfig) *CloneServiceImpl {
	return &CloneServiceImpl{
		config: config,
	}
}

// NewCloneServiceWithDefaults creates a service with default configuration
func NewCloneServiceWithDefaults() *CloneServiceImpl {
	return &CloneServiceImpl{
		config: analyzer.DefaultCloneDetectorConfig(),
	}
}

// extractedFragments is one file's clone-detection input plus the size counters
// the statistics block reports.
type extractedFragments struct {
	fragments []*analyzer.CodeFragment
	lines     int
	nodes     int
}

// extractFileFragments extracts clone fragments from one parsed file. Fragment
// extraction only reads the detector's configuration, so this is safe to run
// for several files at once.
func extractFileFragments(detector *analyzer.CloneDetector, file *ProjectFile) fileAnalysis[*extractedFragments] {
	filePath := file.Path
	if file.ReadErr != nil {
		return fileAnalysis[*extractedFragments]{
			errors: []string{fmt.Sprintf("[%s] Failed to read file: %v", filePath, file.ReadErr)},
		}
	}

	if file.ParseErr != nil {
		return fileAnalysis[*extractedFragments]{
			errors: []string{fmt.Sprintf("[%s] Failed to parse: %v", filePath, file.ParseErr)},
		}
	}

	extracted := &extractedFragments{
		// Source content is carried along for Type-1 textual gating.
		fragments: detector.ExtractFragmentsWithSource(file.AST.Body, filePath, file.Content),
		lines:     countSourceLines(file.Content),
	}
	for _, node := range file.AST.Body {
		extracted.nodes += countASTNodes(node)
	}

	return fileAnalysis[*extractedFragments]{value: extracted}
}

// DetectClones performs clone detection on the given request. Each file is
// read, parsed, and its fragments extracted inside the fan-out; a file's parse
// tree stays reachable only through its fragments, which drop their AST
// references once converted for APTED, so trees are released as detection
// proceeds — use DetectClonesInSnapshot when several analyses should share
// the parse trees.
func (s *CloneServiceImpl) DetectClones(ctx context.Context, req *domain.CloneRequest) (*domain.CloneResponse, error) {
	startTime := time.Now()
	config := s.effectiveConfig(req)
	detector := analyzer.NewCloneDetector(&config)

	results := analyzeProjectFilesFromPaths(ctx, req.Paths,
		func(file *ProjectFile) fileAnalysis[*extractedFragments] {
			return extractFileFragments(detector, file)
		})
	return s.detectFromExtraction(ctx, results, detector, &config, req, startTime)
}

// DetectClonesInSnapshot performs clone detection on already parsed project
// files. The snapshot defines the analyzed file set; req.Paths, when set,
// must name the same files. Fragments reference the snapshot's ASTs only
// until the detector converts them to APTED trees; after that the snapshot is
// the trees' sole owner, so they stay collectable once every analysis sharing
// the snapshot has finished.
func (s *CloneServiceImpl) DetectClonesInSnapshot(ctx context.Context, snapshot *ProjectSnapshot, req *domain.CloneRequest) (*domain.CloneResponse, error) {
	if err := snapshot.validateRequestPaths(req.Paths); err != nil {
		return nil, err
	}

	startTime := time.Now()
	config := s.effectiveConfig(req)
	detector := analyzer.NewCloneDetector(&config)

	results := analyzeFilesConcurrently(ctx, snapshot.Files,
		func(_ context.Context, file *ProjectFile) fileAnalysis[*extractedFragments] {
			return extractFileFragments(detector, file)
		})
	return s.detectFromExtraction(ctx, results, detector, &config, req, startTime)
}

// effectiveConfig applies request-specific thresholds on top of the service config.
func (s *CloneServiceImpl) effectiveConfig(req *domain.CloneRequest) analyzer.CloneDetectorConfig {
	config := *s.config
	if req.MinLines > 0 {
		config.MinLines = req.MinLines
	}
	if req.MinNodes > 0 {
		config.MinNodes = req.MinNodes
	}
	if req.Type1Threshold > 0 {
		config.Type1Threshold = req.Type1Threshold
	}
	if req.Type2Threshold > 0 {
		config.Type2Threshold = req.Type2Threshold
	}
	if req.Type3Threshold > 0 {
		config.Type3Threshold = req.Type3Threshold
	}
	if req.Type4Threshold > 0 {
		config.Type4Threshold = req.Type4Threshold
	}
	if req.MaxEditDistance > 0 {
		config.MaxEditDistance = req.MaxEditDistance
	}
	if req.SimilarityThreshold > 0 {
		config.SimilarityThreshold = req.SimilarityThreshold
	}
	if req.LSHSimilarityThreshold > 0 {
		config.LSHSimilarityThreshold = req.LSHSimilarityThreshold
	}
	if req.LSHBands > 0 {
		config.LSHBands = req.LSHBands
	}
	if req.LSHRows > 0 {
		config.LSHRows = req.LSHRows
	}
	if req.LSHHashes > 0 {
		config.LSHMinHashCount = req.LSHHashes
	}
	config.IgnoreLiterals = req.IgnoreLiterals
	config.IgnoreIdentifiers = req.IgnoreIdentifiers
	return config
}

// detectFromExtraction runs detection over per-file extraction results, in
// input order, and assembles the response both entry points share.
func (s *CloneServiceImpl) detectFromExtraction(ctx context.Context, results []fileAnalysis[*extractedFragments], detector *analyzer.CloneDetector, config *analyzer.CloneDetectorConfig, req *domain.CloneRequest, startTime time.Time) (*domain.CloneResponse, error) {
	if ctx.Err() != nil {
		return nil, fmt.Errorf("clone detection cancelled: %w", ctx.Err())
	}

	var allFragments []*analyzer.CodeFragment
	filesAnalyzed := 0
	linesAnalyzed := 0
	nodesAnalyzed := 0
	var errors []string

	for _, result := range results {
		errors = append(errors, result.errors...)
		if result.value == nil {
			continue
		}

		allFragments = append(allFragments, result.value.fragments...)
		filesAnalyzed++
		linesAnalyzed += result.value.lines
		nodesAnalyzed += result.value.nodes
	}

	if len(allFragments) == 0 {
		// No fragments found, return empty response (or partial failure details)
		response := &domain.CloneResponse{
			Clones:      []*domain.Clone{},
			ClonePairs:  []*domain.ClonePair{},
			CloneGroups: []*domain.CloneGroup{},
			Statistics: &domain.CloneStatistics{
				TotalFragments:    0,
				TotalClones:       0,
				TotalClonePairs:   0,
				TotalCloneGroups:  0,
				ClonesByType:      make(map[string]int),
				AverageSimilarity: 0,
				LinesAnalyzed:     linesAnalyzed,
				NodesAnalyzed:     nodesAnalyzed,
				FilesAnalyzed:     filesAnalyzed,
			},
			Duration: time.Since(startTime).Milliseconds(),
			Success:  len(errors) == 0,
		}
		if len(errors) > 0 {
			response.Error = strings.Join(errors, "; ")
			return response, fmt.Errorf("clone analysis failed for %d file(s)", len(errors))
		}
		return response, nil
	}

	// Determine whether to use LSH based on configuration and estimated exact-pair cost.
	useLSH := domain.ShouldUseLSHWithPairEstimate(req.LSHEnabled, len(allFragments), req.LSHAutoThreshold, config.MaxClonePairs)
	if useLSH {
		detector.SetUseLSH(true)
	}

	// Detect clones
	var clonePairs []*domain.ClonePair
	var cloneGroups []*domain.CloneGroup

	if useLSH {
		clonePairs, cloneGroups = detector.DetectClonesWithLSH(ctx, allFragments)
	} else {
		clonePairs, cloneGroups = detector.DetectClonesWithContext(ctx, allFragments)
	}

	// Filter results based on request criteria (clone types, similarity range)
	clonePairs = filterClonePairs(clonePairs, req)
	cloneGroups = filterCloneGroups(cloneGroups, req)

	// Build statistics
	statistics := s.buildStatistics(clonePairs, cloneGroups, filesAnalyzed, linesAnalyzed)
	statistics.TotalFragments = len(allFragments)
	statistics.NodesAnalyzed = nodesAnalyzed

	// Sort clone pairs by similarity (descending), breaking ties on source
	// location so equally similar pairs keep the same order on every run.
	sort.Slice(clonePairs, func(i, j int) bool {
		return domain.ClonePairPrecedes(clonePairs[i], clonePairs[j])
	})

	// Extract unique clones represented by either pairs or groups.
	clones := s.extractUniqueClones(clonePairs, cloneGroups)

	response := &domain.CloneResponse{
		Clones:      clones,
		ClonePairs:  clonePairs,
		CloneGroups: cloneGroups,
		Statistics:  statistics,
		Duration:    time.Since(startTime).Milliseconds(),
		Success:     len(errors) == 0,
	}
	if len(errors) > 0 {
		response.Error = strings.Join(errors, "; ")
		return response, fmt.Errorf("clone analysis completed with %d file error(s)", len(errors))
	}
	return response, nil
}

// DetectClonesInFiles performs clone detection on specific files
func (s *CloneServiceImpl) DetectClonesInFiles(ctx context.Context, filePaths []string, req *domain.CloneRequest) (*domain.CloneResponse, error) {
	singleReq := *req
	singleReq.Paths = filePaths
	return s.DetectClones(ctx, &singleReq)
}

// ComputeSimilarity computes similarity between two code fragments
func (s *CloneServiceImpl) ComputeSimilarity(ctx context.Context, fragment1, fragment2 string) (float64, error) {
	// This would require parsing both fragments and computing APTED distance
	// For now, return a placeholder
	return 0.0, fmt.Errorf("ComputeSimilarity not yet implemented")
}

// buildStatistics builds clone detection statistics
func (s *CloneServiceImpl) buildStatistics(pairs []*domain.ClonePair, groups []*domain.CloneGroup, filesAnalyzed, linesAnalyzed int) *domain.CloneStatistics {
	stats := &domain.CloneStatistics{
		TotalClonePairs:  len(pairs),
		TotalCloneGroups: len(groups),
		ClonesByType:     make(map[string]int),
		FilesAnalyzed:    filesAnalyzed,
		LinesAnalyzed:    linesAnalyzed,
	}

	// Count clones by type and calculate average similarity
	totalSimilarity := 0.0
	uniqueClones := make(map[string]bool)

	for _, pair := range pairs {
		stats.ClonesByType[pair.Type.String()]++
		totalSimilarity += pair.Similarity

		// Track unique clone locations
		if pair.Clone1 != nil && pair.Clone1.Location != nil {
			key := pair.Clone1.Location.String()
			uniqueClones[key] = true
		}
		if pair.Clone2 != nil && pair.Clone2.Location != nil {
			key := pair.Clone2.Location.String()
			uniqueClones[key] = true
		}
	}
	for _, group := range groups {
		if group == nil {
			continue
		}
		for _, clone := range group.Clones {
			if clone != nil && clone.Location != nil {
				uniqueClones[clone.Location.String()] = true
			}
		}
	}

	stats.TotalClones = len(uniqueClones)
	if len(pairs) > 0 {
		stats.AverageSimilarity = totalSimilarity / float64(len(pairs))
	}

	return stats
}

// extractUniqueClones extracts unique clones represented by pairs or groups.
func (s *CloneServiceImpl) extractUniqueClones(pairs []*domain.ClonePair, groups []*domain.CloneGroup) []*domain.Clone {
	seen := make(map[string]*domain.Clone)
	var clones []*domain.Clone

	for _, pair := range pairs {
		if pair.Clone1 != nil && pair.Clone1.Location != nil {
			key := pair.Clone1.Location.String()
			if _, exists := seen[key]; !exists {
				seen[key] = pair.Clone1
				clones = append(clones, pair.Clone1)
			}
		}
		if pair.Clone2 != nil && pair.Clone2.Location != nil {
			key := pair.Clone2.Location.String()
			if _, exists := seen[key]; !exists {
				seen[key] = pair.Clone2
				clones = append(clones, pair.Clone2)
			}
		}
	}
	for _, group := range groups {
		if group == nil {
			continue
		}
		for _, clone := range group.Clones {
			if clone == nil || clone.Location == nil {
				continue
			}
			key := clone.Location.String()
			if _, exists := seen[key]; !exists {
				seen[key] = clone
				clones = append(clones, clone)
			}
		}
	}

	return clones
}

// cloneTypeEnabled reports whether the given type is in the enabled set.
// An empty set means no type filtering (all types pass).
func cloneTypeEnabled(t domain.CloneType, enabled []domain.CloneType) bool {
	if len(enabled) == 0 {
		return true
	}
	for _, e := range enabled {
		if t == e {
			return true
		}
	}
	return false
}

// similarityInRange reports whether the similarity is within the requested range.
// MaxSimilarity <= 0 means no upper bound.
func similarityInRange(similarity float64, req *domain.CloneRequest) bool {
	if similarity < req.MinSimilarity {
		return false
	}
	if req.MaxSimilarity > 0 && similarity > req.MaxSimilarity {
		return false
	}
	return true
}

// filterClonePairs filters clone pairs based on request criteria
func filterClonePairs(pairs []*domain.ClonePair, req *domain.CloneRequest) []*domain.ClonePair {
	filtered := make([]*domain.ClonePair, 0, len(pairs))
	for _, pair := range pairs {
		if pair == nil {
			continue
		}
		if !similarityInRange(pair.Similarity, req) {
			continue
		}
		if !cloneTypeEnabled(pair.Type, req.CloneTypes) {
			continue
		}
		filtered = append(filtered, pair)
	}
	return filtered
}

// filterCloneGroups filters clone groups based on request criteria
func filterCloneGroups(groups []*domain.CloneGroup, req *domain.CloneRequest) []*domain.CloneGroup {
	filtered := make([]*domain.CloneGroup, 0, len(groups))
	for _, group := range groups {
		if group == nil {
			continue
		}
		if !similarityInRange(group.Similarity, req) {
			continue
		}
		if !cloneTypeEnabled(group.Type, req.CloneTypes) {
			continue
		}
		filtered = append(filtered, group)
	}
	return filtered
}

// countASTNodes counts all structural nodes in an AST subtree
func countASTNodes(node *parser.Node) int {
	if node == nil {
		return 0
	}
	count := 1
	for _, child := range parser.OrderedChildren(node) {
		count += countASTNodes(child)
	}
	return count
}

// GetVersion returns the current version for response metadata
func (s *CloneServiceImpl) GetVersion() string {
	return version.Version
}
