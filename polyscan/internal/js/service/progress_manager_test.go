package service

import (
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

func TestNewProgressManager_NonInteractive(t *testing.T) {
	// When disabled, should return NoOpProgressManager
	pm := NewProgressManager(false)
	if pm.IsInteractive() {
		t.Error("expected non-interactive progress manager when disabled")
	}

	// Should implement the interface
	var _ domain.ProgressManager = pm
}

func TestNoOpProgressManager(t *testing.T) {
	pm := &NoOpProgressManager{}

	// IsInteractive should return false
	if pm.IsInteractive() {
		t.Error("expected NoOpProgressManager.IsInteractive() to return false")
	}

	// StartTask should return a no-op task
	task := pm.StartTask("test", 100)
	if task == nil {
		t.Fatal("expected non-nil task from StartTask")
	}

	// All operations should be no-ops (not panic)
	task.Increment(10)
	task.Describe("testing")
	task.Complete()

	// Close should be a no-op
	pm.Close()
}

func TestNoOpTaskProgress(t *testing.T) {
	tp := &NoOpTaskProgress{}

	// All operations should be no-ops (not panic)
	tp.Increment(10)
	tp.Describe("testing")
	tp.Complete()

	// Should implement the interface
	var _ domain.TaskProgress = tp
}

func TestProgressManagerImpl_Interface(t *testing.T) {
	// Verify ProgressManagerImpl implements the interface
	var _ domain.ProgressManager = &ProgressManagerImpl{}
	var _ domain.TaskProgress = &TaskProgressImpl{}
}
