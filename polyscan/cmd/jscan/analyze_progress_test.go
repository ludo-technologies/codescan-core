package main

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js"
)

type mockTaskProgress struct {
	mu                 sync.Mutex
	totalIncrements    int
	lastDescription    string
	completedCallCount int
}

func (m *mockTaskProgress) Increment(n int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.totalIncrements += n
}

func (m *mockTaskProgress) Describe(description string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastDescription = description
}

func (m *mockTaskProgress) Complete() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completedCallCount++
}

func (m *mockTaskProgress) snapshot() (int, string, int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.totalIncrements, m.lastDescription, m.completedCallCount
}

func TestCalculateProgressPercent(t *testing.T) {
	estimated := 10 * time.Second

	tests := []struct {
		name     string
		elapsed  time.Duration
		expected int
	}{
		{name: "zero elapsed", elapsed: 0, expected: 0},
		{name: "halfway", elapsed: 5 * time.Second, expected: 45},
		{name: "at estimate", elapsed: 10 * time.Second, expected: 90},
		{name: "after estimate in tail", elapsed: 30 * time.Second, expected: 94},
		{name: "near cap", elapsed: 50 * time.Second, expected: 99},
		{name: "beyond cap", elapsed: 120 * time.Second, expected: 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateProgressPercent(tt.elapsed, estimated)
			if got != tt.expected {
				t.Fatalf("calculateProgressPercent(%v, %v) = %d, want %d", tt.elapsed, estimated, got, tt.expected)
			}
		})
	}
}

func TestCalculateProgressPercent_NonPositiveEstimate(t *testing.T) {
	if got := calculateProgressPercent(5*time.Second, 0); got != 0 {
		t.Fatalf("expected 0 for non-positive estimate, got %d", got)
	}
}

func TestEstimateAnalysisDuration_UsesSlowestAnalysisAndMinimum(t *testing.T) {
	minDuration := estimateAnalysisDuration(1, js.Selection{})
	if minDuration < 3*time.Second {
		t.Fatalf("expected minimum estimate >= 3s, got %v", minDuration)
	}

	cloneEstimate := estimateAnalysisDuration(100, js.Selection{Clones: true})
	complexityEstimate := estimateAnalysisDuration(100, js.Selection{Complexity: true})
	if cloneEstimate <= complexityEstimate {
		t.Fatalf("expected clone estimate (%v) to be greater than complexity estimate (%v)", cloneEstimate, complexityEstimate)
	}
}

func TestStartTimeBasedProgressUpdater_IncrementsBarAndUpdatesDescription(t *testing.T) {
	task := &mockTaskProgress{}
	done := startTimeBasedProgressUpdater(task, 2*time.Second)

	time.Sleep(350 * time.Millisecond)
	close(done)
	time.Sleep(50 * time.Millisecond)

	totalIncrements, description, completeCalls := task.snapshot()
	if totalIncrements <= 0 {
		t.Fatalf("expected progress increments to be > 0, got %d", totalIncrements)
	}
	if !strings.Contains(description, "Analyzing...") {
		t.Fatalf("expected description to contain base text, got %q", description)
	}
	if completeCalls != 0 {
		t.Fatalf("updater should not complete task directly, got complete call count %d", completeCalls)
	}
}
