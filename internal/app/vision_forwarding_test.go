package app

import (
	"strings"
	"testing"

	"github.com/boytegar/packboy-builder/internal/reminder"
)

// TestVisionAnalysisForwardedToAgent verifies the exact flow:
// handleVisionAnalysis enqueues the analysis, then SubmitToAgent → sendToAgent
// → attachPendingReminders drains it and attaches it to the content sent to
// the agent.
//
// This test isolates the reminder enqueue+drain path without requiring a full
// agent session. If this test passes, the forwarding logic is correct and the
// bug lies elsewhere (e.g., agent session wiring, LLM provider).
func TestVisionAnalysisForwardedToAgent(t *testing.T) {
	svc := reminder.NewService()

	// Simulate what handleVisionAnalysis does: enqueue the analysis.
	analysis := "Image 1: a screenshot showing a red error dialog with text 'connection refused'"
	svc.Enqueue("Image analysis (from the vision model):\n" + analysis)

	// Simulate what sendToAgent → attachPendingReminders does: drain.
	pending := svc.Drain()
	if len(pending) == 0 {
		t.Fatal("Drain() returned empty — vision analysis was lost between Enqueue and Drain")
	}

	// Verify the analysis is in the drained reminders.
	found := false
	for _, p := range pending {
		if strings.Contains(p, analysis) {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("vision analysis not found in drained reminders; got %d items: %v", len(pending), pending)
	}

	// Verify AttachToContent produces a message with the analysis attached.
	content := "What's in this image?"
	attached := reminder.AttachToContent(content, pending)
	if !strings.Contains(attached, content) {
		t.Error("attached content missing original user text")
	}
	if !strings.Contains(attached, analysis) {
		t.Error("attached content missing vision analysis")
	}
	if !strings.Contains(attached, "<system-reminder>") {
		t.Error("attached content missing system-reminder wrapper")
	}
}

// TestVisionAnalysisSurvivesRequeue verifies that RequeueSystemReminders
// (called at various lifecycle points) does NOT drop the vision analysis
// one-time notice.
func TestVisionAnalysisSurvivesRequeue(t *testing.T) {
	svc := reminder.NewService()

	// Register a provider (simulating wireReminderProviders).
	svc.Register(reminder.NewProvider(reminder.ProviderSkillsDirectory, func() string {
		return "skills: bash, grep"
	}))

	// Enqueue the vision analysis (one-time notice, no providerID).
	analysis := "Image 1: a red square"
	svc.Enqueue("Image analysis (from the vision model):\n" + analysis)

	// Enqueue a skill match (also one-time notice).
	svc.Enqueue("skill: vim editor")

	// Now call RequeueSystemReminders (happens at various lifecycle points).
	svc.RequeueSystemReminders()

	// Drain and verify the vision analysis survived.
	pending := svc.Drain()
	if len(pending) == 0 {
		t.Fatal("Drain() returned empty after RequeueSystemReminders")
	}

	foundVision := false
	foundSkill := false
	for _, p := range pending {
		if strings.Contains(p, analysis) {
			foundVision = true
		}
		if strings.Contains(p, "skill: vim editor") {
			foundSkill = true
		}
	}
	if !foundVision {
		t.Error("vision analysis was dropped by RequeueSystemReminders")
	}
	if !foundSkill {
		t.Error("enqueued skill was dropped by RequeueSystemReminders")
	}
}

// TestVisionAnalysisEmptyContent verifies that when the user sends only an
// image (no text), the vision analysis is still attached as the sole content
// of the user message.
func TestVisionAnalysisEmptyContent(t *testing.T) {
	svc := reminder.NewService()

	analysis := "Image 1: a chart showing Q3 revenue"
	svc.Enqueue("Image analysis (from the vision model):\n" + analysis)

	pending := svc.Drain()
	attached := reminder.AttachToContent("", pending)

	if !strings.Contains(attached, analysis) {
		t.Errorf("attached content missing vision analysis when user text is empty; got: %q", attached)
	}
	if !strings.HasPrefix(attached, "<system-reminder>") {
		t.Errorf("expected attached content to start with <system-reminder> when user text is empty; got: %q", attached)
	}
}
