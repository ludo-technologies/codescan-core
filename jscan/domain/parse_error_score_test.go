package domain

import "testing"

// cleanSummary is a project with nothing wrong with it, so every difference in
// the tests below comes from the file accounting alone.
func cleanSummary(totalFiles, skippedFiles int) *AnalyzeSummary {
	return &AnalyzeSummary{
		ComplexityEnabled: true,
		TotalFiles:        totalFiles,
		AnalyzedFiles:     totalFiles - skippedFiles,
		SkippedFiles:      skippedFiles,
		TotalFunctions:    100,
		FunctionsParsed:   100,
		AverageComplexity: 2,
	}
}

func TestHealthScore_UnskippedProjectIsUnaffected(t *testing.T) {
	summary := cleanSummary(50, 0)

	if err := summary.CalculateHealthScore(); err != nil {
		t.Fatalf("CalculateHealthScore: %v", err)
	}

	if summary.HealthScore != 100 || summary.Grade != "A" {
		t.Errorf("a clean project should still score 100/A, got %d/%s", summary.HealthScore, summary.Grade)
	}
}

func TestHealthScore_OneSkippedFileForfeitsAnA(t *testing.T) {
	// A single broken file in a large tree rounds to a penalty of zero, so the
	// floor is what keeps it from being free.
	summary := cleanSummary(500, 1)

	if err := summary.CalculateHealthScore(); err != nil {
		t.Fatalf("CalculateHealthScore: %v", err)
	}

	if summary.HealthScore >= GradeAThreshold {
		t.Errorf("one unanalyzable file must cost the A, got %d", summary.HealthScore)
	}
	if summary.Grade == "A" {
		t.Errorf("grade should have dropped below A, got %s", summary.Grade)
	}
}

func TestHealthScore_WhollyUnparseableTargetCannotRankAboveF(t *testing.T) {
	summary := cleanSummary(10, 10)

	if err := summary.CalculateHealthScore(); err != nil {
		t.Fatalf("CalculateHealthScore: %v", err)
	}

	if summary.HealthScore >= GradeDThreshold {
		t.Errorf("a target where nothing parses must rank F, got %d (%s)", summary.HealthScore, summary.Grade)
	}
}

func TestHealthScore_PenaltyGrowsWithTheUnanalyzedFraction(t *testing.T) {
	few := cleanSummary(100, 5)
	many := cleanSummary(100, 50)

	if err := few.CalculateHealthScore(); err != nil {
		t.Fatalf("CalculateHealthScore: %v", err)
	}
	if err := many.CalculateHealthScore(); err != nil {
		t.Fatalf("CalculateHealthScore: %v", err)
	}

	if many.HealthScore >= few.HealthScore {
		t.Errorf("half a project unanalyzed should score below a twentieth: %d vs %d",
			many.HealthScore, few.HealthScore)
	}
}

func TestValidate_RejectsImpossibleFileAccounting(t *testing.T) {
	tests := []struct {
		name       string
		totalFiles int
		skipped    int
	}{
		{"negative skipped count", 10, -1},
		{"more skipped than total", 10, 11},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			summary := &AnalyzeSummary{TotalFiles: test.totalFiles, SkippedFiles: test.skipped}

			if err := summary.Validate(); err == nil {
				t.Error("file accounting drives the parse-error penalty, so a nonsensical value must not produce a score")
			}
		})
	}
}
