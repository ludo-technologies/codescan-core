package domain

// Clone type thresholds (similarity score 0.0-1.0).
//
// Type-2 and Type-3 share one threshold on purpose. The detector reports pairs
// from the Type-3 threshold, so a lower Type-2 threshold would classify pairs
// that are then never reported. What separates the two types is the syntactic
// gate, not the similarity: a pair at or above 0.80 whose normalized trees
// also match is Type-2, otherwise Type-3. Pairs between 0.70 and 0.80 were
// mostly functions that share a shape rather than code, such as two output
// formatters' switch statements, which is why the floor moved up from 0.70.
// The JavaScript/TypeScript analyzer keeps its own thresholds in
// polyscan/internal/js/constants; they were tuned separately and Type-3 is
// off by default there.
const (
	DefaultType1CloneThreshold = 0.85
	DefaultType2CloneThreshold = 0.80
	DefaultType3CloneThreshold = 0.80
	DefaultType4CloneThreshold = 0.65
)

// DFA feature weights for similarity comparison.
const (
	DefaultDFAPairCountWeight   = 0.25
	DefaultDFAChainLengthWeight = 0.20
	DefaultDFACrossBlockWeight  = 0.20
	DefaultDFADefKindWeight     = 0.20
	DefaultDFAUseKindWeight     = 0.15
)

// CFG/DFA combined weights for semantic similarity.
const (
	DefaultCFGFeatureWeight = 0.60
	DefaultDFAFeatureWeight = 0.40
)

// Complexity thresholds for risk assessment.
const (
	DefaultComplexityLowThreshold    = 9
	DefaultComplexityMediumThreshold = 19
)

// CBO (Coupling Between Objects) thresholds.
const (
	DefaultCBOLowThreshold    = 3
	DefaultCBOMediumThreshold = 7
)

// LCOM (Lack of Cohesion of Methods) thresholds.
const (
	DefaultLCOMLowThreshold    = 2
	DefaultLCOMMediumThreshold = 5
)

// Clone detection parameters.
const (
	DefaultCloneMinLines            = 10
	DefaultCloneMinNodes            = 20
	DefaultCloneMaxEditDistance     = 50.0
	DefaultCloneSimilarityThreshold = 0.65
	DefaultCloneGroupingThreshold   = 0.65
)

// LSH (Locality-Sensitive Hashing) parameters.
const (
	DefaultLSHAutoThreshold       = 500
	DefaultLSHSimilarityThreshold = 0.50
	DefaultLSHBands               = 32
	DefaultLSHRows                = 4
	DefaultLSHHashes              = 128
)

// Performance parameters.
const (
	DefaultMaxMemoryMB    = 100
	DefaultBatchSize      = 100
	DefaultMaxGoroutines  = 4
	DefaultTimeoutSeconds = 300
)
