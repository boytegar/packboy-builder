// Provider selector: Subagents tab — per-subagent model override picked from
// the live model catalog and persisted to settings.json (subagentModels).
package input

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/boytegar/packboy-builder/internal/app/kit"
	"github.com/boytegar/packboy-builder/internal/llm"
	"github.com/boytegar/packboy-builder/internal/setting"
	"github.com/boytegar/packboy-builder/internal/tool"
)

// subagentModelSavedMsg is sent when a per-subagent model override is persisted.
type subagentModelSavedMsg struct {
	AgentName string
	Model     string // "inherit" clears the override
}

// rebuildSubagentsTab builds the visible-items list for the Subagents tab.
// Phase 0 lists subagent definitions; phase 1 lists the model catalog for the
// selected agent (reusing the same allModels/filteredModels the Models tab
// uses) so the user picks from fetched models, not free text.
func (s *ProviderSelector) rebuildSubagentsTab() {
	s.subAgents = nil
	if s.agentRegistry == nil {
		return
	}

	if s.subAgentPhase == 0 {
		s.rebuildSubagentsAgentList()
		return
	}
	// phase 1: model list (reuse the Models tab's model list builder)
	s.updateFilter()
	s.rebuildSubagentsModelList()
}

func (s *ProviderSelector) rebuildSubagentsAgentList() {
	configs := s.agentRegistry.ListConfigs()
	sort.SliceStable(configs, func(a, b int) bool {
		return strings.ToLower(configs[a].Name) < strings.ToLower(configs[b].Name)
	})
	s.subAgents = configs

	override := s.currentSubagentOverrides()
	for i := range configs {
		c := &configs[i]
		if c.Name == "" {
			continue
		}
		// Apply search filter on agent name/description.
		if s.searchQuery != "" {
			q := strings.ToLower(s.searchQuery)
			if !kit.FuzzyMatch(strings.ToLower(c.Name), q) &&
				!kit.FuzzyMatch(strings.ToLower(c.Description), q) {
				continue
			}
		}
		deref := *c
		// Surface the effective model: settings override > frontmatter > inherit.
		effective := c.Model
		if effective == "" {
			effective = "inherit"
		}
		if ov, ok := override[c.Name]; ok && ov != "" {
			effective = ov + " (override)"
		}
		deref.Model = effective
		s.visibleItems = append(s.visibleItems, providerListItem{
			Kind:     providerItemSubagent,
			Subagent: &deref,
		})
	}
}

func (s *ProviderSelector) rebuildSubagentsModelList() {
	providerModels := make(map[string][]providerModelItem)
	for i := range s.filteredModels {
		m := &s.filteredModels[i]
		providerModels[m.ProviderName] = append(providerModels[m.ProviderName], *m)
	}

	for i := range s.connectedProviders {
		cp := &s.connectedProviders[i]
		models := providerModels[string(cp.Provider)]
		if len(models) == 0 && s.searchQuery != "" {
			continue
		}
		s.visibleItems = append(s.visibleItems, providerListItem{
			Kind:        providerItemProviderHeader,
			Provider:    cp,
			ProviderIdx: i,
		})
		sortProviderModelsByNameDescending(models)
		for j := range models {
			s.visibleItems = append(s.visibleItems, providerListItem{
				Kind:        providerItemModel,
				Model:       &models[j],
				ProviderIdx: i,
			})
		}
	}
}

// currentSubagentOverrides reads the live settings map (snapshot) so the agent
// list can annotate which agents already have an override.
func (s *ProviderSelector) currentSubagentOverrides() map[string]string {
	if s.settings == nil {
		return nil
	}
	return s.settings.Snapshot().SubagentModels
}

// selectSubagent enters phase 1 (model picker) for the named agent.
func (s *ProviderSelector) selectSubagent(name string) tea.Cmd {
	s.subSelected = name
	s.subAgentPhase = 1
	s.resetNavigation()
	s.resetModelSearch()
	s.rebuildVisibleItems()
	return nil
}

// subagentModelRef formats the model override string to persist: a cross-vendor
// model is stored as "vendor/model" so resolveModel routes it; a same-provider
// model is stored as the bare id (the parent provider serves it).
func (s *ProviderSelector) subagentModelRef(m *providerModelItem) string {
	vendor := llm.Name(m.ProviderName)
	if s.parentProviderNameForSubagents() != "" && vendor != s.parentProviderNameForSubagents() {
		return string(vendor) + "/" + m.ID
	}
	return m.ID
}

// parentProviderNameForSubagents returns the current session provider name so
// same-provider models stay bare (no redundant vendor prefix).
func (s *ProviderSelector) parentProviderNameForSubagents() llm.Name {
	if s.settings == nil {
		return ""
	}
	// The store's current model carries the active provider; fall back to the
	// connected providers list if no current model is set yet.
	store := s.store
	if store == nil {
		st, err := llm.NewStore()
		if err != nil {
			return ""
		}
		store = st
	}
	if cm := store.GetCurrentModel(); cm != nil {
		return cm.Provider
	}
	for i := range s.connectedProviders {
		if s.connectedProviders[i].Connected {
			return s.connectedProviders[i].Provider
		}
	}
	return ""
}

// clearSubagentOverride persists an "inherit" (no override) for the selected agent.
func (s *ProviderSelector) clearSubagentOverride() tea.Cmd {
	name := s.subSelected
	s.subAgentPhase = 0
	s.subSelected = ""
	s.active = false
	return func() tea.Msg {
		_ = setting.UpdateSubagentModelAt(name, "inherit", true)
		return subagentModelSavedMsg{AgentName: name, Model: "inherit"}
	}
}

// renderSubagentRow renders a single agent row in phase 1.
func (s *ProviderSelector) renderSubagentRow(item providerListItem, isSelected bool) string {
	c := item.Subagent
	if c == nil {
		return ""
	}
	indicator := "  "
	name := c.Name
	if c.Model != "" && c.Model != "inherit" {
		indicator = kit.DimStyle().Render("→")
		name = fmt.Sprintf("%s %s", c.Name, kit.DimStyle().Render(c.Model))
	}
	line := fmt.Sprintf("%s %s", indicator, name)
	if desc := strings.TrimSpace(c.Description); desc != "" {
		const prefixAndGap = 4
		budget := s.panel().ContentWidth() - lipglossWidth(line) - prefixAndGap
		if budget >= 8 {
			line += "  " + kit.DimStyle().Render(kit.TruncateText(desc, budget))
		}
	}
	return kit.RenderSelectableRow(line, isSelected)
}

// lipglossWidth is a thin alias so the render helper does not need to import
// lipgloss directly (keeps the subagent tab file's import surface small).
func lipglossWidth(s string) int {
	return lipgloss.Width(s)
}

// handleSubagentSelect routes Enter on the Subagents tab: phase 0 opens the
// model picker for an agent; phase 1 persists the picked model and closes.
func (s *ProviderSelector) handleSubagentSelect() tea.Cmd {
	if s.selectedIdx < 0 || s.selectedIdx >= len(s.visibleItems) {
		return nil
	}
	item := s.visibleItems[s.selectedIdx]
	if s.subAgentPhase == 0 {
		if item.Kind != providerItemSubagent || item.Subagent == nil {
			return nil
		}
		return s.selectSubagent(item.Subagent.Name)
	}
	// phase 1: persist the model override
	if item.Kind != providerItemModel || item.Model == nil {
		return nil
	}
	name := s.subSelected
	ref := s.subagentModelRef(item.Model)
	s.active = false
	s.subAgentPhase = 0
	s.subSelected = ""
	return func() tea.Msg {
		_ = setting.UpdateSubagentModelAt(name, ref, true)
		return subagentModelSavedMsg{AgentName: name, Model: ref}
	}
}

// handleSubagentBack routes Esc/left on the Subagents tab: phase 1 returns to
// the agent list; phase 0 hands off to the normal tab/back behavior.
func (s *ProviderSelector) handleSubagentBack() bool {
	if s.activeTab != providerTabSubagents {
		return false
	}
	if s.subAgentPhase == 1 {
		s.subAgentPhase = 0
		s.subSelected = ""
		s.resetNavigation()
		s.resetModelSearch()
		s.rebuildVisibleItems()
		return true
	}
	return false
}

// subagentsTabActive reports whether the Subagents tab should be rendered
// (registry wired and at least one agent exists).
func (s *ProviderSelector) subagentsTabActive() bool {
	return s.agentRegistry != nil && len(s.agentRegistry.ListConfigs()) > 0
}

// subagentPhaseLabel is the small header shown above the model list in phase 1.
func (s *ProviderSelector) subagentPhaseLabel() string {
	return kit.DimStyle().PaddingLeft(2).Render(
		fmt.Sprintf("Pick model for %s — Enter saves · i inherit · ← back", s.subSelected))
}

// handleSubagentInheritKey routes the "i" key on phase 1 to clear the override.
func (s *ProviderSelector) handleSubagentInheritKey() tea.Cmd {
	if s.activeTab != providerTabSubagents || s.subAgentPhase != 1 {
		return nil
	}
	return s.clearSubagentOverride()
}

// subagentSavedNotice renders the post-save notice line appended to the
// conversation by the overlay handler.
func subagentSavedNotice(msg subagentModelSavedMsg) string {
	if msg.Model == "inherit" {
		return fmt.Sprintf("Subagent %q model override cleared (inherits frontmatter/parent).", msg.AgentName)
	}
	return fmt.Sprintf("Subagent %q model override saved: %s", msg.AgentName, msg.Model)
}

// Ensure the tool import is used even when the row renderer is the only
// consumer (keeps goimports stable across edits).
var _ = tool.AgentConfigInfo{}
