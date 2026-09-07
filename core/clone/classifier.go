package clone

import (
	"math"

	"github.com/ludo-technologies/polyscan/core/domain"
)

// ClassifierConfig holds configuration for clone pair classification.
type ClassifierConfig struct {
	Type1Threshold float64
	Type2Threshold float64
	Type3Threshold float64
	Type4Threshold float64
	EnableType1    bool
	EnableType2    bool
	EnableType3    bool
	EnableType4    bool
	// JaccardPreFilterThreshold is the feature Jaccard similarity below which
	// pairs are rejected before expensive structural analysis. Only used for
	// rejection — all non-rejected pairs proceed to structural classification.
	// Zero disables the pre-filter.
	JaccardPreFilterThreshold float64
}

// DefaultClassifierConfig returns the default classifier configuration.
func DefaultClassifierConfig() ClassifierConfig {
	return ClassifierConfig{
		Type1Threshold:            domain.DefaultType1CloneThreshold,
		Type2Threshold:            domain.DefaultType2CloneThreshold,
		Type3Threshold:            domain.DefaultType3CloneThreshold,
		Type4Threshold:            domain.DefaultType4CloneThreshold,
		EnableType1:               true,
		EnableType2:               true,
		EnableType3:               true,
		EnableType4:               true,
		JaccardPreFilterThreshold: 0.10,
	}
}

// PairClassifier classifies clone pairs from structural similarity, gating
// Type-1 on exact textual match and Type-2 on syntactic (normalized AST)
// similarity.
type PairClassifier struct {
	config    ClassifierConfig
	textual   *TextualSimilarityAnalyzer
	syntactic *SyntacticSimilarityAnalyzer
}

// NewPairClassifier creates a pair classifier. A nil textual analyzer disables
// the Type-1 gate (no pair can be confirmed as Type-1); a nil syntactic
// analyzer disables the Type-2 gate.
func NewPairClassifier(config ClassifierConfig, textual *TextualSimilarityAnalyzer, syntactic *SyntacticSimilarityAnalyzer) *PairClassifier {
	return &PairClassifier{
		config:    config,
		textual:   textual,
		syntactic: syntactic,
	}
}

// ClassifyPair classifies a clone pair from its precomputed structural
// (APTED) similarity. Returns the clone type (0 when the pair is not a
// significant clone) and the possibly capped similarity actually used for
// classification.
func (c *PairClassifier) ClassifyPair(f1, f2 *CodeFragment, structuralSimilarity float64) (domain.CloneType, float64) {
	if c.config.EnableType1 && c.textual != nil &&
		structuralSimilarity >= c.config.Type1Threshold && c.textual.IsExactMatch(f1, f2) {
		return domain.Type1Clone, structuralSimilarity
	}

	capped := c.CapNonTextualSimilarity(structuralSimilarity)
	if c.config.EnableType2 && c.syntactic != nil && capped >= c.config.Type2Threshold {
		syntacticSimilarity := c.syntactic.ComputeSimilarity(f1, f2)
		if syntacticSimilarity >= c.config.Type2Threshold {
			return domain.Type2Clone, math.Min(capped, syntacticSimilarity)
		}
	}
	if c.config.EnableType3 && capped >= c.config.Type3Threshold {
		return domain.Type3Clone, capped
	}
	if c.config.EnableType4 && capped >= c.config.Type4Threshold {
		return domain.Type4Clone, capped
	}

	return 0, capped
}

// CapNonTextualSimilarity caps similarity just below the Type-1 threshold so
// that pairs without an exact textual match never report a Type-1-level
// similarity. ClassifyPair applies it internally; callers that classify pairs
// outside ClassifyPair (e.g. a semantic Type-4 path) should apply it to the
// similarity they report so values stay comparable across clone types.
func (c *PairClassifier) CapNonTextualSimilarity(similarity float64) float64 {
	if similarity < c.config.Type1Threshold {
		return similarity
	}

	capped := math.Nextafter(c.config.Type1Threshold, 0)
	if capped < c.config.Type2Threshold {
		return c.config.Type2Threshold
	}
	return capped
}

// PassesJaccardPreFilter reports whether a pair survives the cheap feature
// Jaccard rejection filter. Pairs without pre-computed features always pass.
func (c *PairClassifier) PassesJaccardPreFilter(f1, f2 *CodeFragment) bool {
	if c.config.JaccardPreFilterThreshold <= 0 {
		return true
	}
	if f1 == nil || f2 == nil || len(f1.Features) == 0 || len(f2.Features) == 0 {
		return true
	}
	return JaccardSimilarity(f1.Features, f2.Features) >= c.config.JaccardPreFilterThreshold
}

// ShouldCompareFragments applies cheap size/line prefilters: fragments whose
// node counts or line counts differ too much cannot be clones.
func ShouldCompareFragments(f1, f2 *CodeFragment) bool {
	// Early filtering: Skip if size difference is too large (>50%)
	sizeDiff := math.Abs(float64(f1.NodeCount - f2.NodeCount))
	avgSize := float64(f1.NodeCount+f2.NodeCount) / 2.0
	if avgSize > 0 && sizeDiff/avgSize > 0.5 {
		return false // Too different in size to be clones
	}

	// Early filtering: Skip if line count difference is too large
	lineDiff := math.Abs(float64(f1.LineCount - f2.LineCount))
	if lineDiff > float64(f1.LineCount)*0.5 && lineDiff > float64(f2.LineCount)*0.5 {
		return false // Too different in line count
	}

	return true
}

// CalculateConfidence calculates confidence in a clone pair from similarity,
// fragment size, and complexity agreement.
func CalculateConfidence(f1, f2 *CodeFragment, similarity float64) float64 {
	confidence := similarity

	// Increase confidence for larger fragments
	avgSize := float64(f1.NodeCount+f2.NodeCount) / 2.0
	sizeBonus := math.Min(avgSize/100.0, 0.2) // Up to 20% bonus for large fragments
	confidence += sizeBonus

	// Increase confidence if both fragments have similar complexity
	if f1.Complexity > 0 && f2.Complexity > 0 {
		complexityRatio := float64(min(f1.Complexity, f2.Complexity)) /
			float64(max(f1.Complexity, f2.Complexity))
		confidence += complexityRatio * 0.1 // Up to 10% bonus for similar complexity
	}

	if confidence > 1.0 {
		confidence = 1.0
	}

	return confidence
}

// LocationsOverlap reports whether two locations overlap in the same file
// (inclusive line ranges). Detectors use this to reject same-file pairs of
// overlapping fragments, which describe containment rather than duplication.
func LocationsOverlap(a, b ItemLocation) bool {
	if a.FilePath != b.FilePath {
		return false
	}
	// Ranges overlap unless one ends before the other starts.
	return !(a.EndLine < b.StartLine || b.EndLine < a.StartLine)
}
