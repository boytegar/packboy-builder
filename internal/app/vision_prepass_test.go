package app

import (
	"testing"

	"github.com/boytegar/packboy-builder/internal/reminder"
)

// TestVisionModelConfiguredNil verifies the gate returns false with nil settings.
func TestVisionModelConfiguredNil(t *testing.T) {
	m := &model{}
	if m.visionModelConfigured() {
		t.Error("expected not configured with nil settings")
	}
}

// TestHandleVisionAnalysisNilPending verifies a nil pending message is a no-op.
func TestHandleVisionAnalysisNilPending(t *testing.T) {
	m := &model{
		services: services{Reminder: reminder.NewService()},
	}
	cmd := m.handleVisionAnalysis(visionAnalysisMsg{Analysis: "test"})
	if cmd != nil {
		t.Errorf("expected nil cmd with nil pending, got non-nil")
	}
}
