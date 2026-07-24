package domain

// Clone type thresholds (similarity score 0.0-1.0).
const (
	DefaultType1CloneThreshold = 0.85
	DefaultType2CloneThreshold = 0.75
	DefaultType3CloneThreshold = 0.70
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
