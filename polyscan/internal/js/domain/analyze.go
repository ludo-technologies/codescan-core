package domain

import (
	"fmt"
	"math"
	"time"

	coredomain "github.com/ludo-technologies/polyscan/core/domain"
)

// Health Score Calculation Constants
const (
	// Complexity thresholds and penalties
	ComplexityThresholdHigh   = coredomain.ComplexityThresholdHigh
	ComplexityThresholdMedium = coredomain.ComplexityThresholdMedium
	ComplexityThresholdLow    = coredomain.ComplexityThresholdLow
	ComplexityPenaltyHigh     = coredomain.ComplexityPenaltyHigh
	ComplexityPenaltyMedium   = coredomain.ComplexityPenaltyMedium
	ComplexityPenaltyLow      = coredomain.ComplexityPenaltyLow

	// Code duplication thresholds and penalties
	// 0% = perfect, 30% = max penalty (using fragment ratio: clonedFragments/totalFragments)
	DuplicationThresholdHigh   = coredomain.DuplicationThresholdHigh
	DuplicationThresholdMedium = coredomain.DuplicationThresholdMedium
	DuplicationThresholdLow    = coredomain.DuplicationThresholdLow
	DuplicationPenaltyHigh     = coredomain.DuplicationPenaltyHigh
	DuplicationPenaltyMedium   = coredomain.DuplicationPenaltyMedium
	DuplicationPenaltyLow      = coredomain.DuplicationPenaltyLow

	// CBO coupling scoring curve (used by calculateCouplingPenalty)
	// Penalty grows linearly with the weighted ratio of problematic classes
	// and saturates (reaches the max penalty) at CouplingSaturationRatio.
	CouplingMediumWeight    = coredomain.CouplingMediumWeight
	CouplingSaturationRatio = coredomain.CouplingSaturationRatio

	// Maximum penalties
	MaxDeadCodePenalty = coredomain.MaxDeadCodePenalty
	MaxCriticalPenalty = coredomain.MaxCriticalPenalty
	MaxCyclesPenalty   = coredomain.MaxCyclesPenalty
	MaxDepthPenalty    = coredomain.MaxDepthPenalty
	MaxArchPenalty     = coredomain.MaxArchPenalty
	MaxMSDPenalty      = coredomain.MaxMSDPenalty

	// Score display scale - all categories normalized to this base
	MaxScoreBase = coredomain.MaxScoreBase

	// Actual maximum penalty values for normalization
	MaxDependencyPenalty   = coredomain.MaxDependencyPenalty
	MaxArchitecturePenalty = coredomain.MaxArchitecturePenalty

	// Grade thresholds (stricter than before)
	GradeAThreshold = coredomain.GradeAThreshold
	GradeBThreshold = coredomain.GradeBThreshold
	GradeCThreshold = coredomain.GradeCThreshold
	GradeDThreshold = coredomain.GradeDThreshold

	// Score quality thresholds (aligned with grade thresholds)
	ScoreThresholdExcellent = coredomain.ScoreThresholdExcellent
	ScoreThresholdGood      = coredomain.ScoreThresholdGood
	ScoreThresholdFair      = coredomain.ScoreThresholdFair
	// Poor: 0-59 (below ScoreThresholdFair)

	// Parse-error penalty bounds
	MinParseErrorPenalty = coredomain.MinParseErrorPenalty
	MaxParseErrorPenalty = coredomain.MaxParseErrorPenalty

	// Other constants
	MinimumScore                = coredomain.MinimumScore
	HealthyThreshold            = coredomain.HealthyThreshold
	FallbackComplexityThreshold = coredomain.FallbackComplexityThreshold
	FallbackPenalty             = coredomain.FallbackPenalty
)

// Project scale thresholds, expressed as the minimum number of analyzed files
// a project needs to reach each scale label. Below ScaleSmallThreshold a
// project is reported as "Micro".
const (
	ScaleSmallThreshold      = 10
	ScaleMediumThreshold     = 50
	ScaleLargeThreshold      = 200
	ScaleEnterpriseThreshold = 1000
)

// Project scale labels reported in the analyze summary.
const (
	ScaleMicro      = "Micro"
	ScaleSmall      = "Small"
	ScaleMedium     = "Medium"
	ScaleLarge      = "Large"
	ScaleEnterprise = "Enterprise"
)

// AnalyzeResponse represents the combined results of all analyses
type AnalyzeResponse struct {
	// Analysis results
	Complexity *ComplexityResponse     `json:"complexity,omitempty" yaml:"complexity,omitempty"`
	DeadCode   *DeadCodeResponse       `json:"dead_code,omitempty" yaml:"dead_code,omitempty"`
	Clone      *CloneResponse          `json:"clone,omitempty" yaml:"clone,omitempty"`
	CBO        *CBOResponse            `json:"cbo,omitempty" yaml:"cbo,omitempty"`
	System     *SystemAnalysisResponse `json:"system,omitempty" yaml:"system,omitempty"`

	// Overall summary
	Summary AnalyzeSummary `json:"summary" yaml:"summary"`

	// Metadata
	GeneratedAt time.Time `json:"generated_at" yaml:"generated_at"`
	Duration    int64     `json:"duration_ms" yaml:"duration_ms"`
	Version     string    `json:"version" yaml:"version"`
}

// FileAccounting counts the files a run covered, kept apart from any one
// analysis: every analysis silently excludes the files it cannot parse, so the
// health score charges them separately whichever dimensions ran.
type FileAccounting struct {
	// Total is the number of files the run covered.
	Total int
	// Skipped is the number of files no analysis could use because they could
	// not be read or parsed.
	Skipped int
	// Errors says, per skipped file, why it was skipped.
	Errors []string
}

// Add folds another run's accounting into this one, as when the generic
// engine and the JavaScript pipeline each cover part of a tree.
func (a *FileAccounting) Add(other FileAccounting) {
	a.Total += other.Total
	a.Skipped += other.Skipped
	a.Errors = append(a.Errors, other.Errors...)
}

// AnalysisResults is one run's output: the file set it covered and the
// response of each analysis that ran. An analysis that did not run, or found
// nothing to analyze, leaves its response nil, and the health score leaves
// that dimension out instead of scoring it as clean.
type AnalysisResults struct {
	Files      FileAccounting
	Complexity *ComplexityResponse
	DeadCode   *DeadCodeResponse
	Clone      *CloneResponse
	CBO        *CBOResponse
	Deps       *DependencyGraphResponse
}

// AnalyzeSummary provides an overall summary of all analyses
type AnalyzeSummary struct {
	// File statistics
	TotalFiles    int `json:"total_files" yaml:"total_files"`
	AnalyzedFiles int `json:"analyzed_files" yaml:"analyzed_files"`
	SkippedFiles  int `json:"skipped_files" yaml:"skipped_files"`

	// Analysis status
	ComplexityEnabled bool `json:"complexity_enabled" yaml:"complexity_enabled"`
	DeadCodeEnabled   bool `json:"dead_code_enabled" yaml:"dead_code_enabled"`
	CloneEnabled      bool `json:"clone_enabled" yaml:"clone_enabled"`
	CBOEnabled        bool `json:"cbo_enabled" yaml:"cbo_enabled"`

	// System-level (module dependencies & architecture) summary used for scoring
	DepsEnabled               bool    `json:"deps_enabled" yaml:"deps_enabled"`
	ArchEnabled               bool    `json:"arch_enabled" yaml:"arch_enabled"`
	DepsTotalModules          int     `json:"deps_total_modules" yaml:"deps_total_modules"`
	DepsModulesInCycles       int     `json:"deps_modules_in_cycles" yaml:"deps_modules_in_cycles"`
	DepsMaxDepth              int     `json:"deps_max_depth" yaml:"deps_max_depth"`
	DepsMainSequenceDeviation float64 `json:"deps_main_sequence_deviation" yaml:"deps_main_sequence_deviation"`
	ArchCompliance            float64 `json:"arch_compliance" yaml:"arch_compliance"`

	// Key metrics
	// TotalFunctions is the complete analyzed population that every complexity
	// metric below describes, including the health score. Report filters change
	// which functions are listed, never what is scored.
	TotalFunctions int `json:"total_functions" yaml:"total_functions"`
	// FunctionsParsed is retained for output compatibility and matches TotalFunctions.
	FunctionsParsed       int     `json:"functions_parsed" yaml:"functions_parsed"`
	AverageComplexity     float64 `json:"average_complexity" yaml:"average_complexity"`
	HighComplexityCount   int     `json:"high_complexity_count" yaml:"high_complexity_count"`
	MediumComplexityCount int     `json:"medium_complexity_count" yaml:"medium_complexity_count"`

	DeadCodeCount    int `json:"dead_code_count" yaml:"dead_code_count"`
	CriticalDeadCode int `json:"critical_dead_code" yaml:"critical_dead_code"`
	WarningDeadCode  int `json:"warning_dead_code" yaml:"warning_dead_code"`
	InfoDeadCode     int `json:"info_dead_code" yaml:"info_dead_code"`

	TotalClones     int     `json:"total_clones" yaml:"total_clones"`
	ClonePairs      int     `json:"clone_pairs" yaml:"clone_pairs"`
	CloneGroups     int     `json:"clone_groups" yaml:"clone_groups"`
	CodeDuplication float64 `json:"code_duplication_percentage" yaml:"code_duplication_percentage"`

	CBOClasses            int     `json:"cbo_classes" yaml:"cbo_classes"`
	HighCouplingClasses   int     `json:"high_coupling_classes" yaml:"high_coupling_classes"`     // CBO > 7 (High Risk)
	MediumCouplingClasses int     `json:"medium_coupling_classes" yaml:"medium_coupling_classes"` // 3 < CBO ≤ 7 (Medium Risk)
	AverageCoupling       float64 `json:"average_coupling" yaml:"average_coupling"`

	// Project scale
	// TotalLOC is the number of lines the clone analysis read; it is 0 when
	// clone analysis is disabled.
	TotalLOC     int    `json:"total_loc" yaml:"total_loc"`
	ProjectScale string `json:"project_scale" yaml:"project_scale"` // Micro, Small, Medium, Large, Enterprise

	// Overall health score (0-100)
	HealthScore int    `json:"health_score" yaml:"health_score"`
	Grade       string `json:"grade" yaml:"grade"` // A, B, C, D, F

	// Individual category scores (0-100)
	ComplexityScore   int `json:"complexity_score" yaml:"complexity_score"`
	DeadCodeScore     int `json:"dead_code_score" yaml:"dead_code_score"`
	DuplicationScore  int `json:"duplication_score" yaml:"duplication_score"`
	CouplingScore     int `json:"coupling_score" yaml:"coupling_score"`
	DependencyScore   int `json:"dependency_score" yaml:"dependency_score"`
	ArchitectureScore int `json:"architecture_score" yaml:"architecture_score"`
}

// Validate checks if the summary contains valid values
func (s *AnalyzeSummary) Validate() error {
	// File accounting: the skipped count drives the parse-error penalty, so a
	// nonsensical value must not silently produce a plausible score.
	if s.SkippedFiles < 0 {
		return fmt.Errorf("SkippedFiles cannot be negative: %d", s.SkippedFiles)
	}

	if s.SkippedFiles > s.TotalFiles {
		return fmt.Errorf("SkippedFiles (%d) cannot exceed TotalFiles (%d)", s.SkippedFiles, s.TotalFiles)
	}

	// Basic range checks
	if s.AverageComplexity < 0 {
		return fmt.Errorf("AverageComplexity cannot be negative: %f", s.AverageComplexity)
	}

	if s.CodeDuplication < 0 || s.CodeDuplication > 100 {
		return fmt.Errorf("CodeDuplication must be 0-100: %f", s.CodeDuplication)
	}

	// Architecture compliance check (when enabled)
	if s.ArchEnabled {
		if s.ArchCompliance < 0 || s.ArchCompliance > 1 {
			return fmt.Errorf("ArchCompliance must be 0-1, got %f", s.ArchCompliance)
		}
	}

	// Dependency metrics check (when enabled)
	if s.DepsEnabled {
		if s.DepsMainSequenceDeviation < 0 || s.DepsMainSequenceDeviation > 1 {
			return fmt.Errorf("DepsMainSequenceDeviation must be 0-1, got %f", s.DepsMainSequenceDeviation)
		}

		if s.DepsTotalModules > 0 && s.DepsModulesInCycles > s.DepsTotalModules {
			return fmt.Errorf("DepsModulesInCycles (%d) cannot exceed DepsTotalModules (%d)",
				s.DepsModulesInCycles, s.DepsTotalModules)
		}
	}

	// CBO checks
	if s.CBOClasses > 0 {
		if s.HighCouplingClasses > s.CBOClasses {
			return fmt.Errorf("HighCouplingClasses (%d) cannot exceed CBOClasses (%d)",
				s.HighCouplingClasses, s.CBOClasses)
		}
		if s.MediumCouplingClasses > s.CBOClasses {
			return fmt.Errorf("MediumCouplingClasses (%d) cannot exceed CBOClasses (%d)",
				s.MediumCouplingClasses, s.CBOClasses)
		}
		if (s.HighCouplingClasses + s.MediumCouplingClasses) > s.CBOClasses {
			return fmt.Errorf("HighCouplingClasses + MediumCouplingClasses (%d) cannot exceed CBOClasses (%d)",
				s.HighCouplingClasses+s.MediumCouplingClasses, s.CBOClasses)
		}
	}

	return nil
}

// calculateParseErrorPenalty charges the health score for files that could not
// be analyzed at all (max MaxParseErrorPenalty)
func (s *AnalyzeSummary) calculateParseErrorPenalty() int {
	return coredomain.ParseErrorPenalty(s.SkippedFiles, s.TotalFiles)
}

// calculateComplexityPenalty calculates the penalty for complexity (max 20)
// Uses ratio of high/medium complexity functions (ESLint-aligned: high > 20, medium 10-20)
// Weight: High = 1.0, Medium = 0.5
// Reaches max penalty when 30% or more functions are problematic
func (s *AnalyzeSummary) calculateComplexityPenalty() int {
	if s.TotalFunctions == 0 {
		return 0
	}

	// Weighted ratio of problematic functions
	weighted := float64(s.HighComplexityCount) + 0.5*float64(s.MediumComplexityCount)
	ratio := weighted / float64(s.TotalFunctions)

	return coredomain.LinearPenalty(ratio, 0, 0.05)
}

// calculateDeadCodePenalty calculates the penalty for dead code (max 20)
// Uses per-file rate of weighted findings so that large repos are not unfairly penalized.
// Weights: Critical=1.0, Warning=0.5, Info=0.2
// The rate (weightedFindings / totalFiles) is mapped linearly to 0–20,
// reaching the maximum penalty at a rate of 3.0 findings per file.
func (s *AnalyzeSummary) calculateDeadCodePenalty() int {
	weightedDeadCode := float64(s.CriticalDeadCode)*1.0 +
		float64(s.WarningDeadCode)*0.5 +
		float64(s.InfoDeadCode)*0.2

	if weightedDeadCode <= 0 {
		return 0
	}

	files := s.TotalFiles
	if files < 1 {
		files = 1
	}

	// Per-file rate: how many weighted findings per file
	rate := weightedDeadCode / float64(files)

	return coredomain.LinearPenalty(rate, 0, 3.0)
}

// calculateDuplicationPenalty calculates the penalty for code duplication (max 20)
// Uses continuous linear function based on defined thresholds
func (s *AnalyzeSummary) calculateDuplicationPenalty() int {
	return coredomain.DuplicationPenalty(s.CodeDuplication)
}

// calculateCouplingPenalty calculates the penalty for class coupling (max 20)
// Uses continuous linear function based on weighted ratio of problematic classes
func (s *AnalyzeSummary) calculateCouplingPenalty() int {
	return coredomain.CouplingPenalty(s.HighCouplingClasses, s.MediumCouplingClasses, s.CBOClasses)
}

// calculateDependencyPenalty calculates the penalty for module dependencies (max 16: cycles=10, depth=3, MSD=3)
func (s *AnalyzeSummary) calculateDependencyPenalty() int {
	if !s.DepsEnabled {
		return 0
	}

	return coredomain.DependencyPenalty(
		s.DepsTotalModules,
		s.DepsModulesInCycles,
		s.DepsMaxDepth,
		s.DepsMainSequenceDeviation,
	)
}

// calculateArchitecturePenalty calculates the penalty for architecture compliance (max 12)
func (s *AnalyzeSummary) calculateArchitecturePenalty() int {
	if !s.ArchEnabled {
		return 0
	}

	return coredomain.ArchitecturePenalty(s.ArchCompliance)
}

// normalizeToScoreBase normalizes a penalty value to the MaxScoreBase scale (0-20)
// This ensures all category scores use a consistent display scale
func normalizeToScoreBase(penalty int, maxPenalty int) int {
	return coredomain.NormalizeToScoreBase(penalty, maxPenalty)
}

// penaltyToScore converts a penalty value to a 0-100 score
func penaltyToScore(penalty int, maxPenalty int) int {
	return coredomain.PenaltyToScore(penalty, maxPenalty)
}

// CalculateHealthScore calculates an overall health score based on analysis results
func (s *AnalyzeSummary) CalculateHealthScore() error {
	// Scale only depends on the file count, so it is set even when the rest of
	// the summary fails validation below.
	s.ProjectScale = s.classifyScale()

	// Validate input values first
	if err := s.Validate(); err != nil {
		// Set default values on error
		s.HealthScore = 0
		s.Grade = "N/A"
		s.ComplexityScore = 0
		s.DeadCodeScore = 0
		s.DuplicationScore = 0
		s.CouplingScore = 0
		s.DependencyScore = 0
		s.ArchitectureScore = 0
		return fmt.Errorf("invalid summary data: %w", err)
	}
	// Calculate penalties and corresponding scores
	// Individual scores are normalized to a consistent 20-point scale for display consistency

	complexityPenalty := s.calculateComplexityPenalty()
	s.ComplexityScore = penaltyToScore(complexityPenalty, MaxScoreBase)

	deadCodePenalty := s.calculateDeadCodePenalty()
	s.DeadCodeScore = penaltyToScore(deadCodePenalty, MaxScoreBase)

	duplicationPenalty := s.calculateDuplicationPenalty()
	s.DuplicationScore = penaltyToScore(duplicationPenalty, MaxScoreBase)

	couplingPenalty := s.calculateCouplingPenalty()
	s.CouplingScore = penaltyToScore(couplingPenalty, MaxScoreBase)

	// Dependencies and Architecture need normalization since their max penalties differ from MaxScoreBase
	dependencyPenalty := s.calculateDependencyPenalty()
	normalizedDepPenalty := normalizeToScoreBase(dependencyPenalty, MaxDependencyPenalty)
	s.DependencyScore = penaltyToScore(normalizedDepPenalty, MaxScoreBase)

	architecturePenalty := s.calculateArchitecturePenalty()
	// Use compliance directly as score (98% compliance = 98 points)
	s.ArchitectureScore = int(math.Round(s.ArchCompliance * 100))

	// Unanalyzable files are penalised outside the per-category scores: they
	// have no category of their own, and every category above silently
	// excludes them.
	parseErrorPenalty := s.calculateParseErrorPenalty()

	// The score is computed over the dimensions that ran: each enabled
	// dimension contributes its penalty against its maximum, and a dimension
	// that did not run is left out entirely rather than scored as clean, so a
	// language with only complexity and clone analysis is judged on those.
	totalPenalty, maxPenalty := 0, 0
	charge := func(enabled bool, penalty, budget int) {
		if !enabled {
			return
		}
		totalPenalty += penalty
		maxPenalty += budget
	}
	charge(s.ComplexityEnabled, complexityPenalty, MaxScoreBase)
	charge(s.DeadCodeEnabled, deadCodePenalty, MaxScoreBase)
	charge(s.CloneEnabled, duplicationPenalty, MaxScoreBase)
	charge(s.CBOEnabled, couplingPenalty, MaxScoreBase)
	charge(s.DepsEnabled, dependencyPenalty, MaxDependencyPenalty)
	charge(s.ArchEnabled, architecturePenalty, MaxArchitecturePenalty)

	score := healthScoreFromPenaltyBudget(totalPenalty, maxPenalty, parseErrorPenalty)
	s.HealthScore = score
	s.Grade = coredomain.GradeFromScore(score)

	return nil
}

// healthScoreFromPenaltyBudget scores a run against the dimensions that
// actually ran: totalPenalty is the sum of the enabled dimensions' penalties
// and maxPenalty the most those dimensions could have charged. The ratio is
// projected onto the 100-point scale, so a dimension that did not run is left
// out of the score instead of counting as clean. parseErrorPenalty is charged
// unscaled afterwards: its bounds are anchored to the grade thresholds, not
// to any dimension. The result is floored at MinimumScore and capped at 100.
//
// This is polyscan's cross-language scoring rule; adopting it in core (and
// from there in pyscn) is tracked separately, since core is consumed through
// published tags.
func healthScoreFromPenaltyBudget(totalPenalty, maxPenalty, parseErrorPenalty int) int {
	score := 100
	if maxPenalty > 0 {
		if totalPenalty < 0 {
			totalPenalty = 0
		}
		if totalPenalty > maxPenalty {
			totalPenalty = maxPenalty
		}
		score -= int(math.Round(float64(totalPenalty) * 100 / float64(maxPenalty)))
	}
	if parseErrorPenalty > 0 {
		score -= parseErrorPenalty
	}
	if score < MinimumScore {
		score = MinimumScore
	}
	if score > 100 {
		score = 100
	}
	return score
}

// CalculateFallbackScore provides a simple fallback health score calculation
// Used when validation fails to provide a basic score based on available metrics
func (s *AnalyzeSummary) CalculateFallbackScore() int {
	score := 100

	// Complexity penalty
	if s.AverageComplexity > float64(FallbackComplexityThreshold) {
		score -= FallbackComplexityThreshold
	}

	// Dead code penalty
	if s.DeadCodeCount > 0 {
		score -= FallbackPenalty
	}

	// High complexity penalty
	if s.HighComplexityCount > 0 {
		score -= FallbackPenalty
	}

	if score < MinimumScore {
		score = MinimumScore
	}

	return score
}

// classifyScale returns a project scale label based on the number of analyzed files
func (s *AnalyzeSummary) classifyScale() string {
	switch {
	case s.AnalyzedFiles >= ScaleEnterpriseThreshold:
		return ScaleEnterprise
	case s.AnalyzedFiles >= ScaleLargeThreshold:
		return ScaleLarge
	case s.AnalyzedFiles >= ScaleMediumThreshold:
		return ScaleMedium
	case s.AnalyzedFiles >= ScaleSmallThreshold:
		return ScaleSmall
	default:
		return ScaleMicro
	}
}

// GetGradeFromScore maps a health score to a letter grade
func GetGradeFromScore(score int) string {
	return coredomain.GradeFromScore(score)
}

// IsHealthy returns true if the codebase is considered healthy
func (s *AnalyzeSummary) IsHealthy() bool {
	return coredomain.IsHealthyScore(s.HealthScore)
}

// HasIssues returns true if any issues were found
func (s *AnalyzeSummary) HasIssues() bool {
	return s.HighComplexityCount > 0 || s.DeadCodeCount > 0 || s.ClonePairs > 0 || s.HighCouplingClasses > 0
}
