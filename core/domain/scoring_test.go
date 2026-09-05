package domain

import (
	"math"
	"testing"
)

func TestLinearPenalty(t *testing.T) {
	if p := LinearPenalty(1.0, 2.0, 15.0); p != 0 {
		t.Errorf("below start: got %d, want 0", p)
	}
	if p := LinearPenalty(15.0, 2.0, 15.0); p != MaxScoreBase {
		t.Errorf("at saturation: got %d, want %d", p, MaxScoreBase)
	}
	if p := LinearPenalty(100.0, 2.0, 15.0); p != MaxScoreBase {
		t.Errorf("beyond saturation: got %d, want %d", p, MaxScoreBase)
	}
	if p := LinearPenalty(8.5, 2.0, 15.0); p != 10 {
		t.Errorf("midpoint: got %d, want 10", p)
	}
	if p := LinearPenalty(5.0, 2.0, 2.0); p != MaxScoreBase {
		t.Errorf("degenerate saturation: got %d, want %d", p, MaxScoreBase)
	}
}

func TestDuplicationPenalty(t *testing.T) {
	if p := DuplicationPenalty(0.0); p != 0 {
		t.Errorf("0%%: got %d, want 0", p)
	}
	if p := DuplicationPenalty(30.0); p != 10 {
		t.Errorf("30%%: got %d, want 10", p)
	}
	if p := DuplicationPenalty(60.0); p != 20 {
		t.Errorf("60%%: got %d, want 20", p)
	}
	if p := DuplicationPenalty(90.0); p != 20 {
		t.Errorf("90%%: got %d, want 20 (capped)", p)
	}
}

func TestParseErrorPenalty(t *testing.T) {
	if p := ParseErrorPenalty(0, 100); p != 0 {
		t.Errorf("nothing skipped: got %d, want 0", p)
	}
	if p := ParseErrorPenalty(5, 0); p != 0 {
		t.Errorf("no files at all: got %d, want 0", p)
	}
	// One file in a large tree rounds to zero, so the floor is what makes it cost the A.
	if p := ParseErrorPenalty(1, 500); p != MinParseErrorPenalty {
		t.Errorf("one skipped file: got %d, want %d", p, MinParseErrorPenalty)
	}
	if 100-ParseErrorPenalty(1, 500) >= GradeAThreshold {
		t.Error("one skipped file must forfeit an A")
	}
	if p := ParseErrorPenalty(10, 10); p != MaxParseErrorPenalty {
		t.Errorf("nothing parsed: got %d, want %d", p, MaxParseErrorPenalty)
	}
	if 100-ParseErrorPenalty(10, 10) >= GradeDThreshold {
		t.Error("a wholly unparseable target must rank F")
	}
	if half, few := ParseErrorPenalty(50, 100), ParseErrorPenalty(5, 100); half <= few {
		t.Errorf("penalty must grow with the unanalyzed fraction: %d vs %d", half, few)
	}
	if p := ParseErrorPenalty(20, 10); p != MaxParseErrorPenalty {
		t.Errorf("more skipped than total: got %d, want %d (capped)", p, MaxParseErrorPenalty)
	}
}

func TestCouplingPenalty(t *testing.T) {
	if p := CouplingPenalty(0, 0, 0); p != 0 {
		t.Errorf("no classes: got %d, want 0", p)
	}
	if p := CouplingPenalty(0, 0, 10); p != 0 {
		t.Errorf("no problematic classes: got %d, want 0", p)
	}
	// 4 high / 10 classes = 0.4 ratio = saturation -> max penalty
	if p := CouplingPenalty(4, 0, 10); p != 20 {
		t.Errorf("at saturation: got %d, want 20", p)
	}
	// 1 high + 2 medium (0.3 each) = 1.6 / 10 = 0.16 -> 0.16/0.40*20 = 8
	if p := CouplingPenalty(1, 2, 10); p != 8 {
		t.Errorf("weighted ratio: got %d, want 8", p)
	}
}

func TestDependencyPenaltyCyclesLogFloor(t *testing.T) {
	// Large codebase, few modules in cycles: proportion alone would be ~0,
	// but the log floor keeps a meaningful penalty.
	// 3 modules in cycles -> log2(4) = 2.
	p := DependencyPenalty(1000, 3, 3, 0)
	if p < 2 {
		t.Errorf("log floor should keep cycles penalty >= 2, got %d", p)
	}

	// No cycles -> no cycles penalty.
	if p := DependencyPenalty(1000, 0, 3, 0); p != 0 {
		t.Errorf("no cycles/depth/msd: got %d, want 0", p)
	}
}

func TestDependencyPenaltyDepthAndMSD(t *testing.T) {
	// 100 modules: expected depth = max(3, ceil(log2(101))+1) = 8.
	// maxDepth 20 -> excess 12 capped at MaxDepthPenalty (3).
	p := DependencyPenalty(100, 0, 20, 0)
	if p != MaxDepthPenalty {
		t.Errorf("depth excess: got %d, want %d", p, MaxDepthPenalty)
	}

	// MSD 1.0 -> full MSD penalty.
	p = DependencyPenalty(100, 0, 0, 1.0)
	if p != MaxMSDPenalty {
		t.Errorf("full MSD: got %d, want %d", p, MaxMSDPenalty)
	}

	// All maxed: 10 + 3 + 3 = 16 = MaxDependencyPenalty.
	p = DependencyPenalty(10, 10, 50, 1.0)
	if p != MaxDependencyPenalty {
		t.Errorf("all maxed: got %d, want %d", p, MaxDependencyPenalty)
	}
}

func TestArchitecturePenalty(t *testing.T) {
	if p := ArchitecturePenalty(1.0); p != 0 {
		t.Errorf("full compliance: got %d, want 0", p)
	}
	if p := ArchitecturePenalty(0.0); p != MaxArchPenalty {
		t.Errorf("zero compliance: got %d, want %d", p, MaxArchPenalty)
	}
	if p := ArchitecturePenalty(0.5); p != 6 {
		t.Errorf("half compliance: got %d, want 6", p)
	}
	if p := ArchitecturePenalty(2.0); p != 0 {
		t.Errorf("out-of-range compliance clamps: got %d, want 0", p)
	}
}

func TestPenaltyToScore(t *testing.T) {
	if s := PenaltyToScore(0, 20); s != 100 {
		t.Errorf("no penalty: got %d, want 100", s)
	}
	if s := PenaltyToScore(20, 20); s != 0 {
		t.Errorf("max penalty: got %d, want 0", s)
	}
	if s := PenaltyToScore(10, 20); s != 50 {
		t.Errorf("half penalty: got %d, want 50", s)
	}
	if s := PenaltyToScore(5, 0); s != 100 {
		t.Errorf("zero max: got %d, want 100", s)
	}
}

func TestNormalizeToScoreBase(t *testing.T) {
	if n := NormalizeToScoreBase(16, MaxDependencyPenalty); n != MaxScoreBase {
		t.Errorf("full dependency penalty: got %d, want %d", n, MaxScoreBase)
	}
	if n := NormalizeToScoreBase(8, 16); n != 10 {
		t.Errorf("half: got %d, want 10", n)
	}
	if n := NormalizeToScoreBase(5, 0); n != 0 {
		t.Errorf("zero max: got %d, want 0", n)
	}
}

func TestHealthScoreFromPenalties(t *testing.T) {
	if s := HealthScoreFromPenalties(); s != 100 {
		t.Errorf("no penalties: got %d, want 100", s)
	}
	if s := HealthScoreFromPenalties(20, 12, 6); s != 62 {
		t.Errorf("summed penalties: got %d, want 62", s)
	}
	if s := HealthScoreFromPenalties(50, 60); s != MinimumScore {
		t.Errorf("floored: got %d, want %d", s, MinimumScore)
	}
}

func TestGradeFromScore(t *testing.T) {
	tests := []struct {
		score int
		want  string
	}{
		{100, "A"}, {90, "A"}, {89, "B"}, {75, "B"},
		{74, "C"}, {60, "C"}, {59, "D"}, {45, "D"}, {44, "F"}, {0, "F"},
	}
	for _, tt := range tests {
		if got := GradeFromScore(tt.score); got != tt.want {
			t.Errorf("GradeFromScore(%d) = %s, want %s", tt.score, got, tt.want)
		}
	}
}

func TestIsHealthyScore(t *testing.T) {
	if !IsHealthyScore(70) {
		t.Error("70 should be healthy")
	}
	if IsHealthyScore(69) {
		t.Error("69 should not be healthy")
	}
}

func TestPenaltiesGuardInvalidInputs(t *testing.T) {
	nan := math.NaN()

	if p := LinearPenalty(nan, 2.0, 15.0); p != 0 {
		t.Errorf("LinearPenalty(NaN) = %d, want 0", p)
	}
	if p := DuplicationPenalty(nan); p != 0 {
		t.Errorf("DuplicationPenalty(NaN) = %d, want 0", p)
	}
	if p := ArchitecturePenalty(nan); p != 0 {
		t.Errorf("ArchitecturePenalty(NaN) = %d, want 0", p)
	}
	if p := DependencyPenalty(10, 0, 0, nan); p != 0 {
		t.Errorf("DependencyPenalty(NaN MSD) = %d, want 0", p)
	}
	if p := CouplingPenalty(1, 0, -10); p != 0 {
		t.Errorf("CouplingPenalty(negative total) = %d, want 0", p)
	}
	if p := CouplingPenalty(-3, 0, 10); p != 0 {
		t.Errorf("CouplingPenalty(negative count) = %d, want 0", p)
	}
	if s := HealthScoreFromPenalties(-5); s != 100 {
		t.Errorf("HealthScoreFromPenalties(-5) = %d, want capped 100", s)
	}
}
