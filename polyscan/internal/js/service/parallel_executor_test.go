package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/config"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

// mockTask implements domain.ExecutableTask for testing
type mockTask struct {
	name     string
	enabled  bool
	execFunc func(ctx context.Context) (interface{}, error)
}

func (t *mockTask) Name() string {
	return t.name
}

func (t *mockTask) Execute(ctx context.Context) (interface{}, error) {
	if t.execFunc != nil {
		return t.execFunc(ctx)
	}
	return nil, nil
}

func (t *mockTask) IsEnabled() bool {
	return t.enabled
}

// newMockTask creates a simple mock task
func newMockTask(name string, enabled bool) *mockTask {
	return &mockTask{
		name:    name,
		enabled: enabled,
	}
}

// newMockTaskWithExec creates a mock task with custom execution function
func newMockTaskWithExec(name string, enabled bool, execFunc func(ctx context.Context) (interface{}, error)) *mockTask {
	return &mockTask{
		name:     name,
		enabled:  enabled,
		execFunc: execFunc,
	}
}

func TestNewParallelExecutor(t *testing.T) {
	executor := NewParallelExecutor()

	if executor == nil {
		t.Fatal("NewParallelExecutor returned nil")
	}
	if executor.maxConcurrency <= 0 {
		t.Errorf("maxConcurrency should be > 0, got %d", executor.maxConcurrency)
	}
	if executor.timeout != DefaultTimeout {
		t.Errorf("timeout should be %v, got %v", DefaultTimeout, executor.timeout)
	}
}

func TestNewParallelExecutorFromConfig(t *testing.T) {
	cfg := &config.PerformanceConfig{
		MaxGoroutines:  8,
		TimeoutSeconds: 120,
	}

	executor := NewParallelExecutorFromConfig(cfg)

	if executor.maxConcurrency != 8 {
		t.Errorf("maxConcurrency should be 8, got %d", executor.maxConcurrency)
	}
	if executor.timeout != 120*time.Second {
		t.Errorf("timeout should be 120s, got %v", executor.timeout)
	}
}

func TestNewParallelExecutorFromConfig_Defaults(t *testing.T) {
	cfg := &config.PerformanceConfig{
		MaxGoroutines:  0, // Invalid, should use default
		TimeoutSeconds: 0, // Invalid, should use default
	}

	executor := NewParallelExecutorFromConfig(cfg)

	if executor.maxConcurrency != DefaultMaxConcurrency {
		t.Errorf("maxConcurrency should be %d, got %d", DefaultMaxConcurrency, executor.maxConcurrency)
	}
	if executor.timeout != DefaultTimeout {
		t.Errorf("timeout should be %v, got %v", DefaultTimeout, executor.timeout)
	}
}

func TestNewParallelExecutorWithProgress(t *testing.T) {
	cfg := &config.PerformanceConfig{
		MaxGoroutines:  4,
		TimeoutSeconds: 60,
	}
	pm := &NoOpProgressManager{}

	executor := NewParallelExecutorWithProgress(cfg, pm)

	if executor.progress != pm {
		t.Error("progress manager should be set")
	}
}

func TestParallelExecutor_EmptyTaskList(t *testing.T) {
	executor := NewParallelExecutor()
	ctx := context.Background()

	err := executor.Execute(ctx, []domain.ExecutableTask{})

	if err != nil {
		t.Errorf("empty task list should return nil, got %v", err)
	}
}

func TestParallelExecutor_AllTasksSucceed(t *testing.T) {
	executor := NewParallelExecutor()
	ctx := context.Background()

	var executedCount atomic.Int32
	tasks := []domain.ExecutableTask{
		newMockTaskWithExec("task1", true, func(ctx context.Context) (interface{}, error) {
			executedCount.Add(1)
			return "result1", nil
		}),
		newMockTaskWithExec("task2", true, func(ctx context.Context) (interface{}, error) {
			executedCount.Add(1)
			return "result2", nil
		}),
		newMockTaskWithExec("task3", true, func(ctx context.Context) (interface{}, error) {
			executedCount.Add(1)
			return "result3", nil
		}),
	}

	err := executor.Execute(ctx, tasks)

	if err != nil {
		t.Errorf("all tasks succeeded should return nil, got %v", err)
	}
	if executedCount.Load() != 3 {
		t.Errorf("all 3 tasks should have executed, got %d", executedCount.Load())
	}
}

func TestParallelExecutor_PartialFailures(t *testing.T) {
	executor := NewParallelExecutor()
	ctx := context.Background()

	errTask1 := errors.New("task1 failed")
	errTask3 := errors.New("task3 failed")

	tasks := []domain.ExecutableTask{
		newMockTaskWithExec("task1", true, func(ctx context.Context) (interface{}, error) {
			return nil, errTask1
		}),
		newMockTaskWithExec("task2", true, func(ctx context.Context) (interface{}, error) {
			return "success", nil
		}),
		newMockTaskWithExec("task3", true, func(ctx context.Context) (interface{}, error) {
			return nil, errTask3
		}),
	}

	err := executor.Execute(ctx, tasks)

	if err == nil {
		t.Fatal("expected error for partial failures")
	}

	var aggErr *AggregatedError
	if !errors.As(err, &aggErr) {
		t.Fatalf("expected AggregatedError, got %T", err)
	}

	if len(aggErr.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(aggErr.Errors))
	}

	// Verify both errors are captured
	foundTask1 := false
	foundTask3 := false
	for _, te := range aggErr.Errors {
		if te.TaskName == "task1" {
			foundTask1 = true
		}
		if te.TaskName == "task3" {
			foundTask3 = true
		}
	}
	if !foundTask1 || !foundTask3 {
		t.Error("expected both task1 and task3 errors to be captured")
	}
}

func TestParallelExecutor_Timeout(t *testing.T) {
	// Use short timeout for faster, more stable CI tests
	executor := NewParallelExecutor()
	executor.SetTimeout(100 * time.Millisecond)
	ctx := context.Background()

	tasks := []domain.ExecutableTask{
		newMockTaskWithExec("slow-task", true, func(ctx context.Context) (interface{}, error) {
			select {
			case <-time.After(500 * time.Millisecond): // Task takes longer than timeout
				return "done", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}),
	}

	err := executor.Execute(ctx, tasks)

	if err == nil {
		t.Fatal("expected timeout error")
	}

	// Check if it's an aggregated error with context deadline exceeded
	var aggErr *AggregatedError
	if errors.As(err, &aggErr) {
		if len(aggErr.Errors) == 0 {
			t.Error("expected at least one error in aggregated error")
		}
	}
}

func TestParallelExecutor_ContextCancellation(t *testing.T) {
	executor := NewParallelExecutor()
	ctx, cancel := context.WithCancel(context.Background())

	started := make(chan struct{})
	tasks := []domain.ExecutableTask{
		newMockTaskWithExec("cancellable-task", true, func(ctx context.Context) (interface{}, error) {
			close(started)
			select {
			case <-time.After(10 * time.Second):
				return "done", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}),
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- executor.Execute(ctx, tasks)
	}()

	// Wait for task to start, then cancel
	<-started
	cancel()

	err := <-errChan

	if err == nil {
		t.Fatal("expected cancellation error")
	}
}

func TestParallelExecutor_DisabledTasksSkipped(t *testing.T) {
	executor := NewParallelExecutor()
	ctx := context.Background()

	var executedCount atomic.Int32
	tasks := []domain.ExecutableTask{
		newMockTaskWithExec("enabled-task", true, func(ctx context.Context) (interface{}, error) {
			executedCount.Add(1)
			return "enabled", nil
		}),
		newMockTaskWithExec("disabled-task", false, func(ctx context.Context) (interface{}, error) {
			executedCount.Add(1)
			return "disabled", nil
		}),
	}

	err := executor.Execute(ctx, tasks)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if executedCount.Load() != 1 {
		t.Errorf("only enabled task should execute, got %d executions", executedCount.Load())
	}
}

func TestParallelExecutor_AllDisabledTasks(t *testing.T) {
	executor := NewParallelExecutor()
	ctx := context.Background()

	tasks := []domain.ExecutableTask{
		newMockTask("disabled1", false),
		newMockTask("disabled2", false),
	}

	err := executor.Execute(ctx, tasks)

	if err != nil {
		t.Errorf("all disabled tasks should return nil, got %v", err)
	}
}

func TestParallelExecutor_ConcurrencyLimit(t *testing.T) {
	cfg := &config.PerformanceConfig{
		MaxGoroutines:  2,
		TimeoutSeconds: 30,
	}
	executor := NewParallelExecutorFromConfig(cfg)
	ctx := context.Background()

	var currentConcurrency atomic.Int32
	var maxConcurrency atomic.Int32
	var mu sync.Mutex

	updateMax := func(current int32) {
		mu.Lock()
		defer mu.Unlock()
		if current > maxConcurrency.Load() {
			maxConcurrency.Store(current)
		}
	}

	// Create 5 tasks that track concurrency
	var tasks []domain.ExecutableTask
	for i := 0; i < 5; i++ {
		name := "task" + string(rune('0'+i))
		tasks = append(tasks, newMockTaskWithExec(name, true, func(ctx context.Context) (interface{}, error) {
			current := currentConcurrency.Add(1)
			updateMax(current)
			time.Sleep(50 * time.Millisecond)
			currentConcurrency.Add(-1)
			return nil, nil
		}))
	}

	err := executor.Execute(ctx, tasks)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}

	if maxConcurrency.Load() > 2 {
		t.Errorf("max concurrency should not exceed 2, got %d", maxConcurrency.Load())
	}
}

func TestParallelExecutor_SetMaxConcurrency(t *testing.T) {
	executor := NewParallelExecutor()

	executor.SetMaxConcurrency(16)

	executor.mu.RLock()
	if executor.maxConcurrency != 16 {
		t.Errorf("maxConcurrency should be 16, got %d", executor.maxConcurrency)
	}
	executor.mu.RUnlock()
}

func TestParallelExecutor_SetMaxConcurrency_InvalidValue(t *testing.T) {
	executor := NewParallelExecutor()
	original := executor.maxConcurrency

	executor.SetMaxConcurrency(0)  // Invalid
	executor.SetMaxConcurrency(-1) // Invalid

	executor.mu.RLock()
	if executor.maxConcurrency != original {
		t.Errorf("maxConcurrency should remain %d for invalid values, got %d", original, executor.maxConcurrency)
	}
	executor.mu.RUnlock()
}

func TestParallelExecutor_SetTimeout(t *testing.T) {
	executor := NewParallelExecutor()

	executor.SetTimeout(10 * time.Minute)

	executor.mu.RLock()
	if executor.timeout != 10*time.Minute {
		t.Errorf("timeout should be 10 minutes, got %v", executor.timeout)
	}
	executor.mu.RUnlock()
}

func TestParallelExecutor_SetTimeout_InvalidValue(t *testing.T) {
	executor := NewParallelExecutor()
	original := executor.timeout

	executor.SetTimeout(0)            // Invalid
	executor.SetTimeout(-time.Second) // Invalid

	executor.mu.RLock()
	if executor.timeout != original {
		t.Errorf("timeout should remain %v for invalid values, got %v", original, executor.timeout)
	}
	executor.mu.RUnlock()
}

func TestParallelExecutor_ProgressIntegration(t *testing.T) {
	cfg := &config.PerformanceConfig{
		MaxGoroutines:  4,
		TimeoutSeconds: 60,
	}

	var incrementCount atomic.Int32
	var completed atomic.Bool

	// Create a mock progress manager
	mockPM := &mockProgressManager{
		startTaskFunc: func(description string, total int) domain.TaskProgress {
			return &mockTaskProgress{
				incrementFunc: func(n int) {
					incrementCount.Add(int32(n))
				},
				completeFunc: func() {
					completed.Store(true)
				},
			}
		},
	}

	executor := NewParallelExecutorWithProgress(cfg, mockPM)
	ctx := context.Background()

	tasks := []domain.ExecutableTask{
		newMockTask("task1", true),
		newMockTask("task2", true),
		newMockTask("task3", true),
	}

	err := executor.Execute(ctx, tasks)

	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}

	if incrementCount.Load() != 3 {
		t.Errorf("expected 3 increments, got %d", incrementCount.Load())
	}

	if !completed.Load() {
		t.Error("expected Complete() to be called")
	}
}

func TestAggregatedError_Error(t *testing.T) {
	tests := []struct {
		name     string
		errors   []TaskError
		contains string
	}{
		{
			name:     "no errors",
			errors:   []TaskError{},
			contains: "no errors",
		},
		{
			name: "single error",
			errors: []TaskError{
				{TaskName: "task1", Err: errors.New("failed")},
			},
			contains: "[task1] failed",
		},
		{
			name: "multiple errors",
			errors: []TaskError{
				{TaskName: "task1", Err: errors.New("failed1")},
				{TaskName: "task2", Err: errors.New("failed2")},
			},
			contains: "2 tasks failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aggErr := &AggregatedError{Errors: tt.errors}
			errStr := aggErr.Error()

			if len(errStr) == 0 {
				t.Error("error string should not be empty")
			}
			if !strings.Contains(errStr, tt.contains) {
				t.Errorf("error string should contain %q, got %q", tt.contains, errStr)
			}
		})
	}
}

func TestAggregatedError_Unwrap(t *testing.T) {
	originalErr := errors.New("original error")
	aggErr := &AggregatedError{
		Errors: []TaskError{
			{TaskName: "task1", Err: originalErr},
		},
	}

	unwrapped := aggErr.Unwrap()

	if !errors.Is(unwrapped, originalErr) {
		t.Error("Unwrap should return the first error's underlying error")
	}
}

func TestAggregatedError_Unwrap_Empty(t *testing.T) {
	aggErr := &AggregatedError{Errors: []TaskError{}}

	unwrapped := aggErr.Unwrap()

	if unwrapped != nil {
		t.Error("Unwrap on empty errors should return nil")
	}
}

func TestTaskError_Error(t *testing.T) {
	te := TaskError{
		TaskName: "my-task",
		Err:      errors.New("something went wrong"),
	}

	errStr := te.Error()

	if errStr != "[my-task] something went wrong" {
		t.Errorf("unexpected error string: %s", errStr)
	}
}

func TestTaskError_Unwrap(t *testing.T) {
	originalErr := errors.New("original")
	te := TaskError{
		TaskName: "task",
		Err:      originalErr,
	}

	if !errors.Is(te, originalErr) {
		t.Error("TaskError should unwrap to original error")
	}
}

// Helper types for testing

type mockProgressManager struct {
	startTaskFunc func(description string, total int) domain.TaskProgress
}

func (m *mockProgressManager) StartTask(description string, total int) domain.TaskProgress {
	if m.startTaskFunc != nil {
		return m.startTaskFunc(description, total)
	}
	return &NoOpTaskProgress{}
}

func (m *mockProgressManager) IsInteractive() bool {
	return false
}

func (m *mockProgressManager) Close() {}

type mockTaskProgress struct {
	incrementFunc func(n int)
	describeFunc  func(description string)
	completeFunc  func()
}

func (m *mockTaskProgress) Increment(n int) {
	if m.incrementFunc != nil {
		m.incrementFunc(n)
	}
}

func (m *mockTaskProgress) Describe(description string) {
	if m.describeFunc != nil {
		m.describeFunc(description)
	}
}

func (m *mockTaskProgress) Complete() {
	if m.completeFunc != nil {
		m.completeFunc()
	}
}
