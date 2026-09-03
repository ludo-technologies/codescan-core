package domain

import (
	"math"
	"testing"

	coredomain "github.com/ludo-technologies/polyscan/core/domain"
)

func TestScoringConstantsAndPublicGradeWrapperUseCoreDomain(t *testing.T) {
	if MaxScoreBase != coredomain.MaxScoreBase || GradeAThreshold != coredomain.GradeAThreshold {
		t.Fatal("jscan scoring constants differ from core/domain")
	}
	for _, score := range []int{0, 44, 45, 59, 60, 74, 75, 89, 90, 100} {
		if got, want := GetGradeFromScore(score), coredomain.GradeFromScore(score); got != want {
			t.Errorf("GetGradeFromScore(%d) = %q, want %q", score, got, want)
		}
	}
}

func TestClassifyScale(t *testing.T) {
	tests := []struct {
		name          string
		analyzedFiles int
		wantScale     string
	}{
		{"zero files", 0, ScaleMicro},
		{"9 files (upper Micro)", 9, ScaleMicro},
		{"10 files (lower Small)", 10, ScaleSmall},
		{"49 files (upper Small)", 49, ScaleSmall},
		{"50 files (lower Medium)", 50, ScaleMedium},
		{"199 files (upper Medium)", 199, ScaleMedium},
		{"200 files (lower Large)", 200, ScaleLarge},
		{"999 files (upper Large)", 999, ScaleLarge},
		{"1000 files (Enterprise)", 1000, ScaleEnterprise},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &AnalyzeSummary{
				AnalyzedFiles: tt.analyzedFiles,
			}
			if err := s.CalculateHealthScore(); err != nil {
				t.Fatalf("CalculateHealthScore() error: %v", err)
			}
			if s.ProjectScale != tt.wantScale {
				t.Errorf("ProjectScale = %q, want %q for %d files",
					s.ProjectScale, tt.wantScale, tt.analyzedFiles)
			}
		})
	}
}

func TestClassifyScale_SetEvenWhenValidationFails(t *testing.T) {
	// AverageComplexity is invalid, so CalculateHealthScore bails out early.
	// The scale only depends on the file count and must still be reported.
	s := &AnalyzeSummary{
		AnalyzedFiles:     120,
		AverageComplexity: -1,
	}
	if err := s.CalculateHealthScore(); err == nil {
		t.Fatal("CalculateHealthScore() = nil error, want validation failure")
	}
	if s.ProjectScale != ScaleMedium {
		t.Errorf("ProjectScale = %q, want %q", s.ProjectScale, ScaleMedium)
	}
}

func TestCalculateHealthScore_CyclePenaltyLogFloor(t *testing.T) {
	tests := []struct {
		name              string
		modulesInCycles   int
		totalModules      int
		wantMinDepPenalty int // minimum expected dependency penalty from cycles alone
	}{
		{
			name:              "small ratio still penalised via log floor",
			modulesInCycles:   18,
			totalModules:      587,
			wantMinDepPenalty: 4, // log2(19) ≈ 4.25 → round 4
		},
		{
			name:              "very small ratio still penalised",
			modulesInCycles:   15,
			totalModules:      1500,
			wantMinDepPenalty: 4, // log2(16) = 4.0 → round 4
		},
		{
			name:              "moderate ratio uses log floor",
			modulesInCycles:   6,
			totalModules:      80,
			wantMinDepPenalty: 3, // log2(7) ≈ 2.81 → round 3
		},
		{
			name:              "no cycles no penalty",
			modulesInCycles:   0,
			totalModules:      500,
			wantMinDepPenalty: 0,
		},
		{
			name:              "large ratio uses proportion",
			modulesInCycles:   40,
			totalModules:      80,
			wantMinDepPenalty: 5, // proportion: 10*0.5 = 5, log2(41) ≈ 5.36 → max is 5.36 → round 5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &AnalyzeSummary{
				DepsEnabled:         true,
				DepsTotalModules:    tt.totalModules,
				DepsModulesInCycles: tt.modulesInCycles,
			}
			if err := s.CalculateHealthScore(); err != nil {
				t.Fatalf("CalculateHealthScore() error: %v", err)
			}
			// Dependency penalty = 100 - DependencyScore (mapped to 0-100 via penaltyToScore)
			// We verify the score is low enough to reflect the expected penalty.
			// With only cycle penalty active (depth=0, MSD=0), the normalized penalty
			// is: round(cyclePenalty / 16 * 20), and score = 100 - round(norm * 100 / 20).
			expectedCyclePenalty := 0
			if tt.modulesInCycles > 0 {
				ratio := float64(tt.modulesInCycles) / float64(tt.totalModules)
				if ratio > 1 {
					ratio = 1
				}
				prop := float64(MaxCyclesPenalty) * ratio
				logF := math.Log2(float64(tt.modulesInCycles) + 1)
				expectedCyclePenalty = int(math.Round(math.Min(float64(MaxCyclesPenalty), math.Max(logF, prop))))
			}

			if expectedCyclePenalty < tt.wantMinDepPenalty {
				t.Errorf("expected cycle penalty >= %d, formula gives %d", tt.wantMinDepPenalty, expectedCyclePenalty)
			}

			// The dependency score must reflect the penalty (not 100)
			if tt.modulesInCycles > 0 && s.DependencyScore >= 100 {
				t.Errorf("DependencyScore should be < 100 when cycles exist, got %d", s.DependencyScore)
			}
			if tt.modulesInCycles == 0 && s.DependencyScore != 100 {
				t.Errorf("DependencyScore should be 100 with no cycles and no other dep issues, got %d", s.DependencyScore)
			}
		})
	}
}

func TestCalculateHealthScore_DuplicationPenalty(t *testing.T) {
	tests := []struct {
		name            string
		duplication     float64
		wantDuplication int // expected DuplicationScore
		wantHealth      int
	}{
		// Clone detection is the only enabled dimension, so the health score
		// is the duplication score: the missing dimensions are left out of
		// the score rather than scored as clean.
		{
			// 0-30% scale: 1/30*20 = 0.67 -> 1 penalty
			name:            "low duplication penalised from zero",
			duplication:     1.0,
			wantDuplication: 95,
			wantHealth:      95,
		},
		{
			// 5/30*20 = 3.33 -> 3 penalty
			name:            "medium duplication",
			duplication:     5.0,
			wantDuplication: 85,
			wantHealth:      85,
		},
		{
			// 30% reaches the max penalty (20)
			name:            "max penalty at threshold high",
			duplication:     30.0,
			wantDuplication: 0,
			wantHealth:      0,
		},
		{
			name:            "no duplication no penalty",
			duplication:     0.0,
			wantDuplication: 100,
			wantHealth:      100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &AnalyzeSummary{CloneEnabled: true, CodeDuplication: tt.duplication}
			if err := s.CalculateHealthScore(); err != nil {
				t.Fatalf("CalculateHealthScore() error: %v", err)
			}
			if s.DuplicationScore != tt.wantDuplication {
				t.Errorf("DuplicationScore = %d, want %d", s.DuplicationScore, tt.wantDuplication)
			}
			if s.HealthScore != tt.wantHealth {
				t.Errorf("HealthScore = %d, want %d", s.HealthScore, tt.wantHealth)
			}
		})
	}
}

func TestCalculateHealthScore_CouplingCalibration(t *testing.T) {
	// Softened CBO curve: a repo with a healthy average CBO and ~10% high-coupling
	// classes should not floor the coupling score.
	s := &AnalyzeSummary{
		CBOEnabled:            true,
		CBOClasses:            100,
		HighCouplingClasses:   10, // 10%
		MediumCouplingClasses: 20,
		// weighted = 10 + 0.3*20 = 16; ratio = 0.16; penalty = 0.16/0.40*20 = 8
	}
	if err := s.CalculateHealthScore(); err != nil {
		t.Fatalf("CalculateHealthScore() error: %v", err)
	}
	if s.CouplingScore != 60 { // 100 - (8/20)*100
		t.Errorf("CouplingScore = %d, want 60", s.CouplingScore)
	}
	// Coupling is the only enabled dimension, so it is the health score.
	if s.HealthScore != 60 || s.Grade != "C" {
		t.Errorf("HealthScore = %d (%s), want 60 (C)", s.HealthScore, s.Grade)
	}
}

// TestCalculateHealthScore_MissingDimensionsLeftOut pins the cross-language
// scoring rule: a language with only complexity and clone analysis is judged
// on those two dimensions, and the missing ones are left out of the score
// rather than scored as clean.
func TestCalculateHealthScore_MissingDimensionsLeftOut(t *testing.T) {
	s := &AnalyzeSummary{
		ComplexityEnabled: true,
		CloneEnabled:      true,
		TotalFunctions:    100,
		TotalFiles:        10,
		AnalyzedFiles:     10,
		// 20 high + 8*0.5 medium = 24% weighted ratio -> penalty 16 of 20
		HighComplexityCount:   20,
		MediumComplexityCount: 8,
		// 15% duplication -> penalty 10 of 20
		CodeDuplication: 15.0,
	}
	if err := s.CalculateHealthScore(); err != nil {
		t.Fatalf("CalculateHealthScore() error: %v", err)
	}
	// 26 of a 40-point budget: 100 - round(26*100/40) = 35. Scored as clean
	// missing dimensions would have given 100 - 26 = 74 instead.
	if s.HealthScore != 35 || s.Grade != "F" {
		t.Errorf("HealthScore = %d (%s), want 35 (F)", s.HealthScore, s.Grade)
	}
}

// TestCalculateHealthScore_ComplexityPenaltyCurve pins the complexity curve
// from #96: a handful of complex functions in a mostly clean codebase costs a
// few points, and only a codebase where 30% of the functions are high risk
// takes the full penalty.
func TestCalculateHealthScore_ComplexityPenaltyCurve(t *testing.T) {
	tests := []struct {
		name           string
		total          int
		high, medium   int
		wantComplexity int
	}{
		// ky: 128 functions, 5 high and 8 medium, a weighted ratio of 7%.
		{name: "ky", total: 128, high: 5, medium: 8, wantComplexity: 75},
		{name: "one high function in twenty", total: 20, high: 1, wantComplexity: 85},
		// Below half a point the linear curve rounds to 0; one point is charged instead.
		{name: "one high function in 134 costs a point", total: 134, high: 1, wantComplexity: 95},
		{name: "one medium function in 1000 costs a point", total: 1000, medium: 1, wantComplexity: 95},
		{name: "no risky functions is clean", total: 1000, wantComplexity: 100},
		{name: "half of the functions medium", total: 100, medium: 50, wantComplexity: 15},
		{name: "30% high saturates", total: 100, high: 30, wantComplexity: 0},
		{name: "beyond saturation stays at the floor", total: 100, high: 80, medium: 20, wantComplexity: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &AnalyzeSummary{
				ComplexityEnabled:     true,
				TotalFunctions:        tt.total,
				HighComplexityCount:   tt.high,
				MediumComplexityCount: tt.medium,
			}
			if err := s.CalculateHealthScore(); err != nil {
				t.Fatalf("CalculateHealthScore() error: %v", err)
			}
			if s.ComplexityScore != tt.wantComplexity {
				t.Errorf("ComplexityScore = %d, want %d", s.ComplexityScore, tt.wantComplexity)
			}
		})
	}
}

func TestCalculateHealthScore_ArchitectureScoreUsesCompliance(t *testing.T) {
	s := &AnalyzeSummary{
		ArchEnabled:    true,
		ArchCompliance: 0.125,
	}
	if err := s.CalculateHealthScore(); err != nil {
		t.Fatalf("CalculateHealthScore() error: %v", err)
	}
	// Compliance is used directly as the score: 0.125 * 100 = 12.5 → 13
	if s.ArchitectureScore != 13 {
		t.Errorf("ArchitectureScore = %d, want 13", s.ArchitectureScore)
	}
	// Architecture is the only enabled dimension: the penalty is
	// (1-compliance)*MaxArchPenalty = 10.5 → 11 of a budget of 12, so the
	// score is 100 - round(11*100/12) = 8.
	if s.HealthScore != 8 {
		t.Errorf("HealthScore = %d, want 8", s.HealthScore)
	}
}

func TestCalculateHealthScore_MSDPenalty(t *testing.T) {
	tests := []struct {
		name    string
		msd     float64
		wantMax int // maximum expected DependencyScore
		wantMin int // minimum expected DependencyScore
	}{
		{
			name:    "zero MSD gives full score",
			msd:     0.0,
			wantMin: 100,
			wantMax: 100,
		},
		{
			name:    "moderate MSD reduces score",
			msd:     0.4,
			wantMin: 85,
			wantMax: 99,
		},
		{
			name:    "high MSD reduces score further",
			msd:     1.0,
			wantMin: 75,
			wantMax: 95,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &AnalyzeSummary{
				DepsEnabled:               true,
				DepsTotalModules:          100,
				DepsMainSequenceDeviation: tt.msd,
			}
			if err := s.CalculateHealthScore(); err != nil {
				t.Fatalf("CalculateHealthScore() error: %v", err)
			}
			if s.DependencyScore < tt.wantMin || s.DependencyScore > tt.wantMax {
				t.Errorf("DependencyScore = %d, want [%d, %d] for MSD=%.1f",
					s.DependencyScore, tt.wantMin, tt.wantMax, tt.msd)
			}
		})
	}
}
