// Provider selector: Vision tab — designates a model that pre-analyzes pasted
// images before the main agent sees the turn. Persisted to settings.json
// (visionModel). Mirrors the Subagents tab's 2-phase pick (phase 0 = the
// vision-model slot; phase 1 = model catalog), but without per-agent rows.
package input

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/boytegar/packboy-builder/internal/app/kit"
	"github.com/boytegar/packboy-builder/internal/llm"
	"github.com/boytegar/packboy-builder/internal/setting"
)

// visionModelSavedMsg is sent when the vision model override is persisted.
type visionModelSavedMsg struct {
	Model string // "inherit" clears it
}

// visionTabActive reports whether the Vision tab should be rendered. The tab
// is always available (there is no registry gating like the Subagents tab) so
// the user can pre-configure a vision model even before any image is pasted.
func (s *ProviderSelector) visionTabActive() bool {
	return true
}

// rebuildVisionTab builds the visible-items list for the Vision tab.
// Phase 0 lists a single "Vision model" row; phase 1 lists the model catalog
// (reusing the same allModels/filteredModels the Models tab uses) so the user
// picks from fetched models, not free text.
func (s *ProviderSelector) rebuildVisionTab() {
	if s.visionAgentPhase() == 1 {
		s.updateFilter()
		s.rebuildVisionModelList()
		return
	}
	// phase 0: single vision-model slot
	s.visibleItems = append(s.visibleItems, providerListItem{
		Kind: providerItemVisionDefault,
	})
}

// rebuildVisionModelList lists models for phase 1, filtering to
// vision-capable models (llm.SupportsImages). It reuses the Models tab's
// filteredModels so the search box narrows the same pool.
func (s *ProviderSelector) rebuildVisionModelList() {
	for i := range s.filteredModels {
		m := &s.filteredModels[i]
		// Skip models the provider reports as non-vision. SupportsImages defaults
		// to true when a provider doesn't implement ImageSupportProvider, so only
		// explicit opt-outs (e.g. DeepSeek) are filtered out here.
		p, err := llm.NewProviderPool(s.store).Resolve(context.Background(), llm.Name(m.ProviderName))
		if err != nil || !llm.SupportsImages(p, m.ID) {
			continue
		}
		s.visibleItems = append(s.visibleItems, providerListItem{
			Kind:        providerItemModel,
			Model:       m,
			ProviderIdx: -1,
		})
	}
}

// visionAgentPhase returns the phase (0 = slot, 1 = model list), reusing the
// same subAgentPhase field the Subagents tab uses so switchTab's reset logic
// (which zeroes subAgentPhase) applies uniformly.
func (s *ProviderSelector) visionAgentPhase() int {
	return s.subAgentPhase
}

// visionModelRef formats the model ref to persist: a cross-vendor model is
// stored as "vendor/model"; a same-provider model is stored as the bare id.
// Mirrors subagentModelRef.
func (s *ProviderSelector) visionModelRef(m *providerModelItem) string {
	vendor := llm.Name(m.ProviderName)
	if parent := s.parentProviderNameForSubagents(); parent != "" && vendor != parent {
		return string(vendor) + "/" + m.ID
	}
	return m.ID
}

// visionCurrentRef reads the live settings snapshot for the current vision model.
func (s *ProviderSelector) visionCurrentRef() string {
	if s.settings == nil {
		return ""
	}
	return strings.TrimSpace(s.settings.Snapshot().VisionModel)
}

// selectVisionSlot enters phase 1 (model picker) for the vision slot.
func (s *ProviderSelector) selectVisionSlot() tea.Cmd {
	s.subAgentPhase = 1
	s.resetNavigation()
	s.resetModelSearch()
	s.rebuildVisibleItems()
	return nil
}

// handleVisionSelect routes Enter on the Vision tab: phase 0 opens the model
// picker; phase 1 persists the picked model and closes.
func (s *ProviderSelector) handleVisionSelect() tea.Cmd {
	if s.selectedIdx < 0 || s.selectedIdx >= len(s.visibleItems) {
		return nil
	}
	item := s.visibleItems[s.selectedIdx]
	if s.visionAgentPhase() == 0 {
		if item.Kind != providerItemVisionDefault {
			return nil
		}
		return s.selectVisionSlot()
	}
	// phase 1: persist the vision model
	if item.Kind != providerItemModel || item.Model == nil {
		return nil
	}
	ref := s.visionModelRef(item.Model)
	s.active = false
	s.subAgentPhase = 0
	s.subSelected = ""
	return func() tea.Msg {
		_ = setting.UpdateVisionModel(ref, true)
		return visionModelSavedMsg{Model: ref}
	}
}

// handleVisionBack routes Esc/left on the Vision tab: phase 1 returns to the
// slot; phase 0 hands off to the normal tab/back behavior.
func (s *ProviderSelector) handleVisionBack() bool {
	if s.activeTab != providerTabVision {
		return false
	}
	if s.visionAgentPhase() == 1 {
		s.subAgentPhase = 0
		s.resetNavigation()
		s.resetModelSearch()
		s.rebuildVisibleItems()
		return true
	}
	return false
}

// handleVisionInheritKey routes the "i" key on phase 1 to clear the vision model.
func (s *ProviderSelector) handleVisionInheritKey() tea.Cmd {
	if s.activeTab != providerTabVision || s.visionAgentPhase() != 1 {
		return nil
	}
	s.active = false
	s.subAgentPhase = 0
	s.subSelected = ""
	return func() tea.Msg {
		_ = setting.UpdateVisionModel("inherit", true)
		return visionModelSavedMsg{Model: "inherit"}
	}
}

// visionPhaseLabel is the small header shown above the model list in phase 1.
func (s *ProviderSelector) visionPhaseLabel() string {
	return kit.DimStyle().PaddingLeft(2).Render(
		"Pick vision model — Enter saves · i clear · ← back")
}

// renderVisionRow renders the vision-model slot row (phase 0).
func (s *ProviderSelector) renderVisionRow(item providerListItem, isSelected bool) string {
	current := s.visionCurrentRef()
	effective := "inherit (main model handles images)"
	if current != "" {
		effective = current
	}
	line := fmt.Sprintf("  Vision model  %s", kit.DimStyle().Render(effective))
	desc := "pre-analyzes pasted images; lets a text-only main model act on them"
	const prefixAndGap = 4
	budget := s.panel().ContentWidth() - lipglossWidth(line) - prefixAndGap
	if budget >= 8 {
		line += "  " + kit.DimStyle().Render(kit.TruncateText(desc, budget))
	}
	return kit.RenderSelectableRow(line, isSelected)
}

// visionSavedNotice renders the post-save notice line appended to the
// conversation by the overlay handler.
func visionSavedNotice(msg visionModelSavedMsg) string {
	if msg.Model == "inherit" {
		return "Vision model cleared (images go to the main model directly)."
	}
	return fmt.Sprintf("Vision model saved: %s", msg.Model)
}
