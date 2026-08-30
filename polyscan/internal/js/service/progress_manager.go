package service

import (
	"io"
	"os"
	"sync"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/schollz/progressbar/v3"
)

// ProgressManagerImpl implements ProgressManager with interactive progress bars
type ProgressManagerImpl struct {
	writer io.Writer
	tasks  []*progressbar.ProgressBar
	mu     sync.Mutex
}

// NewProgressManager creates a new progress manager based on environment
func NewProgressManager(enabled bool) domain.ProgressManager {
	if enabled && IsInteractiveEnvironment() {
		return &ProgressManagerImpl{
			writer: os.Stderr,
			tasks:  make([]*progressbar.ProgressBar, 0),
		}
	}
	return &NoOpProgressManager{}
}

// StartTask creates a new progress task with a description and total count
func (pm *ProgressManagerImpl) StartTask(description string, total int) domain.TaskProgress {
	bar := progressbar.NewOptions(total,
		progressbar.OptionSetWriter(pm.writer),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(false),
		progressbar.OptionSetWidth(18),
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "█",
			SaucerHead:    "█",
			SaucerPadding: "░",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionShowCount(),
		progressbar.OptionSetPredictTime(true),
	)

	pm.mu.Lock()
	pm.tasks = append(pm.tasks, bar)
	pm.mu.Unlock()

	return &TaskProgressImpl{bar: bar}
}

// IsInteractive returns true if progress bars should be shown
func (pm *ProgressManagerImpl) IsInteractive() bool {
	return true
}

// Close cleans up all tasks
func (pm *ProgressManagerImpl) Close() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, bar := range pm.tasks {
		_ = bar.Finish()
	}
	pm.tasks = nil
}

// TaskProgressImpl implements TaskProgress with a progressbar
type TaskProgressImpl struct {
	bar *progressbar.ProgressBar
}

// Increment adds n to the current progress
func (tp *TaskProgressImpl) Increment(n int) {
	_ = tp.bar.Add(n)
}

// Describe updates the current item description
func (tp *TaskProgressImpl) Describe(description string) {
	tp.bar.Describe(description)
}

// Complete marks the task as finished
func (tp *TaskProgressImpl) Complete() {
	_ = tp.bar.Finish()
}

// NoOpProgressManager implements ProgressManager with no-op methods
type NoOpProgressManager struct{}

// StartTask returns a no-op task progress
func (pm *NoOpProgressManager) StartTask(_ string, _ int) domain.TaskProgress {
	return &NoOpTaskProgress{}
}

// IsInteractive returns false for no-op manager
func (pm *NoOpProgressManager) IsInteractive() bool {
	return false
}

// Close is a no-op
func (pm *NoOpProgressManager) Close() {}

// NoOpTaskProgress implements TaskProgress with no-op methods
type NoOpTaskProgress struct{}

// Increment is a no-op
func (tp *NoOpTaskProgress) Increment(_ int) {}

// Describe is a no-op
func (tp *NoOpTaskProgress) Describe(_ string) {}

// Complete is a no-op
func (tp *NoOpTaskProgress) Complete() {}
