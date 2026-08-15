package domain

import "math"

// Health score calculation constants shared by all language analyzers.
// Grade computation must match across tools; language-specific penalty
// formulas (complexity, dead code) live in each analyzer and compose with
// the shared calculators below.
const (
	// Complexity thresholds and penalties
	ComplexityThresholdHigh   = 20
	ComplexityThresholdMedium = 10
	ComplexityThresholdLow    = 5
	ComplexityPenaltyHigh     = 20
	ComplexityPenaltyMedium   = 12
	ComplexityPenaltyLow      = 6

	// Code duplication thresholds and penalties
	// 0% = perfect, 30% = max penalty (using fragment ratio: clonedFragments/totalFragments)
	DuplicationThresholdHigh   = 30.0
	DuplicationThresholdMedium = 15.0
	DuplicationThresholdLow    = 0.0
	DuplicationPenaltyHigh     = 20
	DuplicationPenaltyMedium   = 12
	DuplicationPenaltyLow      = 6

	// CBO coupling scoring curve (used by CouplingPenalty)
	// Penalty grows linearly with the weighted ratio of problematic classes
	// and saturates (reaches the max penalty) at CouplingSaturationRatio.
	CouplingMediumWeight    = 0.3  // Medium-risk classes count 0.3 vs High = 1.0
	CouplingSaturationRatio = 0.40 // weighted ratio at which the penalty maxes out

	// Maximum penalties
	MaxDeadCodePenalty = 20
	MaxCriticalPenalty = 10
	MaxCyclesPenalty   = 10
	MaxDepthPenalty    = 3
	MaxArchPenalty     = 12
	MaxMSDPenalty      = 3

	// Score display scale - all categories normalized to this base
	MaxScoreBase = 20

	// Actual maximum penalty values for normalization
	MaxDependencyPenalty   = MaxCyclesPenalty + MaxDepthPenalty + MaxMSDPenalty // 16
	MaxArchitecturePenalty = MaxArchPenalty                                     // 12

	// Grade thresholds
	GradeAThreshold = 90
	GradeBThreshold = 75
	GradeCThreshold = 60
	GradeDThreshold = 45

	// Score quality thresholds (aligned with grade thresholds)
	ScoreThresholdExcellent = 90 // Excellent: 90-100
	ScoreThresholdGood      = 75 // Good: 75-89
	ScoreThresholdFair      = 60 // Fair: 60-74
	// Poor: 0-59 (below ScoreThresholdFair)

	// Files that fail to parse are absent from every metric, so without a
	// penalty they score better than working code. The bounds are anchored to
	// the grade thresholds: a single unanalyzable file forfeits an A, and a
	// target where nothing parses cannot rank above F.
	MinParseErrorPenalty = 100 - GradeAThreshold + 1
	MaxParseErrorPenalty = 100 - GradeDThreshold + 1

	// Other constants
	MinimumScore                = 0 // Allow truly low scores for severely problematic code
	HealthyThreshold            = 70
	FallbackComplexityThreshold = 10
	FallbackPenalty             = 5
)

// LinearPenalty maps a value onto a 0..MaxScoreBase penalty that starts at 0
// when value <= start and grows linearly to the maximum at saturation.
// A NaN value is treated as missing data and yields no penalty.
func LinearPenalty(value, start, saturation float64) int {
	if math.IsNaN(value) || value <= start {
		return 0
	}
	if saturation <= start {
		return MaxScoreBase
	}

	penalty := (value - start) / (saturation - start) * float64(MaxScoreBase)
	if penalty > float64(MaxScoreBase) {
		penalty = float64(MaxScoreBase)
	}

	return int(math.Round(penalty))
}

// DuplicationPenalty calculates the penalty for code duplication (max 20).
// Linear: 0% duplication = 0 penalty, DuplicationThresholdHigh (30%) = max.
// A NaN percentage is treated as missing data and yields no penalty.
func DuplicationPenalty(duplicationPercent float64) int {
	if math.IsNaN(duplicationPercent) || duplicationPercent <= DuplicationThresholdLow {
		return 0
	}

	penaltyRange := DuplicationThresholdHigh - DuplicationThresholdLow
	penalty := (duplicationPercent - DuplicationThresholdLow) / penaltyRange * 20.0
	if penalty > 20.0 {
		penalty = 20.0
	}

	return int(math.Round(penalty))
}

// ParseErrorPenalty charges the health score for files that could not be
// analyzed at all. Such a file yields no functions, no dead code, no clones and
// no coupling, so without this term corrupting a file raises the score. The
// penalty grows with the unanalyzed fraction and never drops below
// MinParseErrorPenalty, so one broken file in a large tree still costs a grade.
func ParseErrorPenalty(skippedFiles, totalFiles int) int {
	if skippedFiles <= 0 || totalFiles <= 0 {
		return 0
	}

	ratio := float64(skippedFiles) / float64(totalFiles)
	if ratio > 1 {
		ratio = 1
	}

	return max(int(math.Round(ratio*float64(MaxParseErrorPenalty))), MinParseErrorPenalty)
}

// CouplingPenalty calculates the penalty for class coupling (max 20) from the
// weighted ratio of problematic classes (High = 1.0, Medium = CouplingMediumWeight),
// saturating at CouplingSaturationRatio.
func CouplingPenalty(highCouplingClasses, mediumCouplingClasses, totalClasses int) int {
	if totalClasses <= 0 {
		return 0
	}

	weightedProblematicClasses := float64(highCouplingClasses) + (CouplingMediumWeight * float64(mediumCouplingClasses))
	ratio := weightedProblematicClasses / float64(totalClasses)
	if ratio < 0 {
		ratio = 0
	}

	penalty := ratio / CouplingSaturationRatio * 20.0
	if penalty > 20.0 {
		penalty = 20.0
	}

	return int(math.Round(penalty))
}

// DependencyPenalty calculates the penalty for module dependencies
// (max 16: cycles=10, depth=3, main sequence deviation=3).
func DependencyPenalty(totalModules, modulesInCycles, maxDepth int, mainSequenceDeviation float64) int {
	penalty := 0

	// Cycles penalty (max 10): uses larger of proportion-based and log-scaled floor.
	// The log-scaled floor ensures that circular dependencies always contribute
	// a meaningful penalty, even in large codebases where the proportion is small.
	if totalModules > 0 && modulesInCycles > 0 {
		ratio := float64(modulesInCycles) / float64(totalModules)
		if ratio > 1 {
			ratio = 1
		}
		proportionPenalty := float64(MaxCyclesPenalty) * ratio
		logFloor := math.Log2(float64(modulesInCycles) + 1)
		cyclePenalty := math.Max(logFloor, proportionPenalty)
		if cyclePenalty > float64(MaxCyclesPenalty) {
			cyclePenalty = float64(MaxCyclesPenalty)
		}
		penalty += int(math.Round(cyclePenalty))
	}

	// Depth penalty (max 3): excess over expected depth ~ O(log N)
	if totalModules > 0 {
		expected := int(math.Max(3, math.Ceil(math.Log2(float64(totalModules)+1))+1))
		excess := maxDepth - expected
		if excess < 0 {
			excess = 0
		}
		if excess > MaxDepthPenalty {
			excess = MaxDepthPenalty
		}
		penalty += excess
	}

	// Main sequence deviation penalty (max 3)
	if mainSequenceDeviation > 0 {
		msd := mainSequenceDeviation
		if msd > 1 {
			msd = 1
		}
		penalty += int(math.Round(msd * float64(MaxMSDPenalty)))
	}

	return penalty
}

// ArchitecturePenalty calculates the penalty for architecture compliance
// (max 12). Compliance is a 0..1 ratio. A NaN compliance is treated as
// missing data and yields no penalty.
func ArchitecturePenalty(compliance float64) int {
	if math.IsNaN(compliance) {
		return 0
	}
	if compliance < 0 {
		compliance = 0
	}
	if compliance > 1 {
		compliance = 1
	}
	return int(math.Round(float64(MaxArchPenalty) * (1 - compliance)))
}

// NormalizeToScoreBase normalizes a penalty value to the MaxScoreBase scale
// (0-20) so all category scores use a consistent display scale.
func NormalizeToScoreBase(penalty int, maxPenalty int) int {
	if maxPenalty == 0 {
		return 0
	}
	normalized := int(math.Round(float64(penalty) / float64(maxPenalty) * float64(MaxScoreBase)))
	if normalized < 0 {
		normalized = 0
	}
	if normalized > MaxScoreBase {
		normalized = MaxScoreBase
	}
	return normalized
}

// PenaltyToScore converts a penalty value to a 0-100 score.
func PenaltyToScore(penalty int, maxPenalty int) int {
	if maxPenalty == 0 {
		return 100
	}
	score := 100 - int(math.Round(float64(penalty)*100.0/float64(maxPenalty)))
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score
}

// HealthScoreFromPenalties returns 100 minus the sum of all penalties,
// floored at MinimumScore and capped at 100 (negative penalties cannot
// raise the score above a clean result).
func HealthScoreFromPenalties(penalties ...int) int {
	score := 100
	for _, p := range penalties {
		score -= p
	}
	if score < MinimumScore {
		score = MinimumScore
	}
	if score > 100 {
		score = 100
	}
	return score
}

// GradeFromScore maps a health score to a letter grade. This mapping must
// stay identical across all language analyzers.
func GradeFromScore(score int) string {
	switch {
	case score >= GradeAThreshold:
		return "A"
	case score >= GradeBThreshold:
		return "B"
	case score >= GradeCThreshold:
		return "C"
	case score >= GradeDThreshold:
		return "D"
	default:
		return "F"
	}
}

// IsHealthyScore reports whether a health score is considered healthy.
func IsHealthyScore(score int) bool {
	return score >= HealthyThreshold
}
