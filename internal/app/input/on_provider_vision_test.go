package input

import (
	"testing"
)

// TestVisionTabRebuildPhase0 verifies the Vision tab phase 0 produces a single
// selectable vision-model row.
func TestVisionTabRebuildPhase0(t *testing.T) {
	m := NewProviderSelector()
	m.active = true
	m.activeTab = providerTabVision
	m.rebuildVisibleItems()
	if len(m.visibleItems) != 1 {
		t.Fatalf("phase 0: expected 1 item, got %d", len(m.visibleItems))
	}
	if m.visibleItems[0].Kind != providerItemVisionDefault {
		t.Errorf("expected providerItemVisionDefault, got %v", m.visibleItems[0].Kind)
	}
}

// TestVisionTabActive verifies visionTabActive is always true (no registry
// gating like Subagents).
func TestVisionTabActive(t *testing.T) {
	m := NewProviderSelector()
	if !m.visionTabActive() {
		t.Error("visionTabActive should always be true")
	}
}

// TestVisionTabCountIncludesVision verifies the tab count includes Vision.
func TestVisionTabCountIncludesVision(t *testing.T) {
	m := NewProviderSelector()
	// No subagents → Models + Providers + Vision = 3
	if got := m.providerTabCount(); got != 3 {
		t.Errorf("without subagents: tabCount=%d, want 3", got)
	}
}

// TestVisionCurrentRef verifies the current vision model is read from settings.
func TestVisionCurrentRef(t *testing.T) {
	m := NewProviderSelector()
	// No settings → empty ref
	if got := m.visionCurrentRef(); got != "" {
		t.Errorf("no settings: visionCurrentRef=%q, want empty", got)
	}
}
