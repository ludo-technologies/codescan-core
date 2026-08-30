package service

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/config"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"golang.org/x/sync/errgroup"
)

// Default values for parallel executor
const (
	// DefaultMaxConcurrency is used when config value is invalid.
	// NewParallelExecutor() uses runtime.NumCPU() for optimal CPU utilization,
	// while NewParallelExecutorFromConfig() falls back to this constant.
	DefaultMaxConcurrency = 4
	DefaultTimeout        = 5 * time.Minute
)

// TaskError represents a single task failure
type TaskError struct {
	TaskName string
	Err      error
}

// Error implements the error interface
func (e TaskError) Error() string {
	return fmt.Sprintf("[%s] %v", e.TaskName, e.Err)
}

// Unwrap returns the underlying error
func (e TaskError) Unwrap() error {
	return e.Err
}

// AggregatedError collects all task failures
type AggregatedError struct {
	Errors []TaskError
}

// Error implements the error interface
func (e *AggregatedError) Error() string {
	if len(e.Errors) == 0 {
		return "no errors"
	}
	if len(e.Errors) == 1 {
		return e.Errors[0].Error()
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%d tasks failed:\n", len(e.Errors)))
	for i, err := range e.Errors {
		sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, err.Error()))
	}
	return sb.String()
}

// Unwrap returns the first error for errors.Is/As compatibility
func (e *AggregatedError) Unwrap() error {
	if len(e.Errors) == 0 {
		return nil
	}
	return e.Errors[0].Err
}

// ParallelExecutorImpl implements domain.ParallelExecutor
type ParallelExecutorImpl struct {
	maxConcurrency int
	timeout        time.Duration
	progress       domain.ProgressManager
	mu             sync.RWMutex
}

// NewParallelExecutor creates a new parallel executor with defaults
// Uses runtime.NumCPU() for concurrency and 5 minute timeout
func NewParallelExecutor() *ParallelExecutorImpl {
	return &ParallelExecutorImpl{
		maxConcurrency: runtime.NumCPU(),
		timeout:        DefaultTimeout,
	}
}

// NewParallelExecutorFromConfig creates a parallel executor from configuration
func NewParallelExecutorFromConfig(cfg *config.PerformanceConfig) *ParallelExecutorImpl {
	maxConcurrency := cfg.MaxGoroutines
	if maxConcurrency <= 0 {
		maxConcurrency = DefaultMaxConcurrency
	}

	timeout := time.Duration(cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	return &ParallelExecutorImpl{
		maxConcurrency: maxConcurrency,
		timeout:        timeout,
	}
}

// NewParallelExecutorWithProgress creates a parallel executor with progress tracking
func NewParallelExecutorWithProgress(cfg *config.PerformanceConfig, pm domain.ProgressManager) *ParallelExecutorImpl {
	executor := NewParallelExecutorFromConfig(cfg)
	executor.progress = pm
	return executor
}

// Execute runs tasks in parallel with the configured concurrency and timeout
func (e *ParallelExecutorImpl) Execute(ctx context.Context, tasks []domain.ExecutableTask) error {
	// Filter enabled tasks
	enabledTasks := e.filterEnabledTasks(tasks)
	if len(enabledTasks) == 0 {
		return nil
	}

	// Get current config values (thread-safe)
	e.mu.RLock()
	maxConcurrency := e.maxConcurrency
	timeout := e.timeout
	e.mu.RUnlock()

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Set up progress tracking
	var task domain.TaskProgress = &NoOpTaskProgress{}
	if e.progress != nil {
		task = e.progress.StartTask("Executing tasks", len(enabledTasks))
	}
	defer task.Complete()

	// Create errgroup with context for cancellation propagation
	g, gCtx := errgroup.WithContext(timeoutCtx)
	g.SetLimit(maxConcurrency)

	// Collect errors from all tasks
	var errMu sync.Mutex
	var taskErrors []TaskError

	for _, t := range enabledTasks {
		g.Go(func() error {
			// Check if context is already cancelled
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			default:
			}

			// Execute the task
			_, err := t.Execute(gCtx)

			// Update progress
			task.Increment(1)

			// Collect error if any
			if err != nil {
				errMu.Lock()
				taskErrors = append(taskErrors, TaskError{
					TaskName: t.Name(),
					Err:      err,
				})
				errMu.Unlock()
			}

			// Return nil to continue processing other tasks
			// We collect errors separately to get all failures
			return nil
		})
	}

	// Wait for all tasks to complete.
	// Note: g.Wait() always returns nil here because goroutines return nil
	// to allow all tasks to complete. Errors are collected in taskErrors.
	_ = g.Wait()

	// Return aggregated error if any tasks failed
	if len(taskErrors) > 0 {
		return &AggregatedError{Errors: taskErrors}
	}

	return nil
}

// SetMaxConcurrency sets the maximum number of concurrent tasks
func (e *ParallelExecutorImpl) SetMaxConcurrency(max int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if max > 0 {
		e.maxConcurrency = max
	}
}

// SetTimeout sets the timeout for all tasks
func (e *ParallelExecutorImpl) SetTimeout(timeout time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if timeout > 0 {
		e.timeout = timeout
	}
}

// filterEnabledTasks returns only tasks where IsEnabled() returns true
func (e *ParallelExecutorImpl) filterEnabledTasks(tasks []domain.ExecutableTask) []domain.ExecutableTask {
	enabled := make([]domain.ExecutableTask, 0, len(tasks))
	for _, t := range tasks {
		if t.IsEnabled() {
			enabled = append(enabled, t)
		}
	}
	return enabled
}
