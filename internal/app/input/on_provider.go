// Provider selector: data model, overlay state, messages, and top-level
// message routing for the unified model & provider selection overlay.
//
// Navigation and selection live in on_provider_nav.go; credential entry and
// the connect/refresh flow in on_provider_auth.go; data loading in
// on_provider_data.go; rendering in on_provider_view.go.
package input

import (
	"context"
	"fmt"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"go.uber.org/zap"

	"github.com/boytegar/packboy-builder/internal/app/kit"
	"github.com/boytegar/packboy-builder/internal/core"
	"github.com/boytegar/packboy-builder/internal/llm"
	"github.com/boytegar/packboy-builder/internal/log"
	coresetting "github.com/boytegar/packboy-builder/internal/setting"
	"github.com/boytegar/packboy-builder/internal/tool"
)

// providerTab represents which tab is active in the provider selector.
type providerTab int

const (
	providerTabModels    providerTab = iota // model selection tab
	providerTabProviders                    // provider management tab
	providerTabVision                       // designated vision-model override
	providerTabSubagents                    // per-subagent model override
)

// providerItemKind represents a row type in the visible-items list.
type providerItemKind int

const (
	providerItemProviderHeader       providerItemKind = iota // non-selectable provider group header (Models tab)
	providerItemModel                                        // selectable model row (Models tab)
	providerItemProvider                                     // provider row (Providers tab)
	providerItemAuthMethod                                   // expanded auth-method sub-row (Providers tab)
	providerItemSubagent                                     // selectable subagent row (Subagents tab phase 1)
	providerItemSubagentDefault                              // selectable default-model row (Subagents tab phase 1)
	providerItemSubagentDefaultWrite                         // selectable default-model-for-write row (Subagents tab phase 1)
	providerItemVisionDefault                                // selectable vision-model row (Vision tab phase 0)
)

// providerListItem is a single row in the flattened visible-items list.
type providerListItem struct {
	Kind        providerItemKind
	Model       *providerModelItem
	Provider    *providerProviderItem
	AuthMethod  *providerAuthMethodItem
	Subagent    *tool.AgentConfigInfo
	ProviderIdx int // index into allProviders
}

// providerProviderItem represents a provider with its auth methods.
type providerProviderItem struct {
	Provider    llm.Name
	DisplayName string
	AuthMethods []providerAuthMethodItem
	Connected   bool // whether this provider has at least one connected auth method
}

// providerAuthMethodItem represents an auth method in the second level.
type providerAuthMethodItem struct {
	Provider    llm.Name
	AuthMethod  llm.AuthMethod
	DisplayName string
	Status      llm.Status
	EnvVars     []string
}

// providerModelItem represents a model in the provider selector.
type providerModelItem struct {
	ID               string
	Name             string
	DisplayName      string
	Description      string
	ProviderName     string
	AuthMethod       llm.AuthMethod
	IsCurrent        bool
	InputTokenLimit  int
	OutputTokenLimit int
}

func newProviderModelItem(mdl llm.ModelInfo, providerName string, authMethod llm.AuthMethod, current *llm.CurrentModelInfo) providerModelItem {
	return providerModelItem{
		ID:               mdl.ID,
		Name:             mdl.Name,
		DisplayName:      mdl.DisplayName,
		Description:      mdl.Description,
		ProviderName:     providerName,
		AuthMethod:       authMethod,
		IsCurrent:        current != nil && current.ModelID == mdl.ID && string(current.Provider) == providerName && current.AuthMethod == authMethod,
		InputTokenLimit:  mdl.InputTokenLimit,
		OutputTokenLimit: mdl.OutputTokenLimit,
	}
}

// ProviderSelector holds the state for the unified model & provider kit.
type ProviderSelector struct {
	active bool
	width  int
	height int
	store  *llm.Store

	// Tab
	activeTab providerTab

	// Data
	connectedProviders []providerProviderItem // providers with models (Models tab headers)
	allProviders       []providerProviderItem // all providers (Providers tab)
	allModels          []providerModelItem

	// Flattened visible-items list (rebuilt on state changes)
	visibleItems []providerListItem
	selectedIdx  int
	scrollOffset int
	maxVisible   int

	// Providers tab: expanded provider
	expandedProviderIdx int // index into allProviders; -1 = none

	// Inline API-key input
	apiKeyInput       textinput.Model
	apiKeyActive      bool
	apiKeyEnvVar      string
	apiKeyProviderIdx int // index into allProviders
	apiKeyAuthIdx     int // index into that provider's AuthMethods

	// Inline custom provider form (baseURL / apiKey); see on_provider_custom.go
	customFormActive bool
	customFormInputs [2]textinput.Model
	customFormFocus  int
	customFormErr    string

	// Inline Ollama form (base URL only); see on_provider_ollama.go
	ollamaFormActive bool
	ollamaURLInput   textinput.Model
	ollamaFormErr    string

	// Inline confirm-remove prompt
	confirmRemoveActive  bool
	confirmRemoveEnvVar  string
	confirmRemoveItemIdx int // index into visibleItems for the pending remove

	// Models tab: search filter and the two flags that disambiguate keys
	// whose meaning depends on what the user is doing.
	searchQuery    string              // active filter text; "" means no filter
	filteredModels []providerModelItem // allModels narrowed to searchQuery

	// searchFocused routes Space: true while the search box has focus (the user
	// is typing a query) so Space inserts a literal space; false while
	// navigating the list so Space marks the highlighted model instead.
	searchFocused bool

	// modelMarked routes Enter: true once the user has explicitly marked a
	// model with Space, so Enter confirms that mark regardless of cursor; false
	// until then, so Enter acts on the highlighted row. (The active model is
	// rendered [*] on open, but that display state is not a mark.)
	modelMarked bool

	// Provider connection result (shown inline)
	lastConnectResult  string
	lastConnectAuthIdx int // item index that triggered the connection
	lastConnectSuccess bool

	// spinnerTick advances on each providerConnectingTickMsg; used to pick a braille
	// frame while a connect/refresh is in flight.
	spinnerTick int

	// Subagents tab state.
	agentRegistry AgentRegistry         // nil = Subagents tab unavailable
	settings      *coresetting.Settings // for reading/writing subagentModels
	subAgents     []tool.AgentConfigInfo
	subAgentPhase int    // 0 = pick agent, 1 = pick model
	subSelected   string // agent name in phase 1; "" = none
}

// NewProviderSelector creates a new provider selector ProviderSelector.
func NewProviderSelector() ProviderSelector {
	return ProviderSelector{
		active:              false,
		selectedIdx:         0,
		maxVisible:          20,
		expandedProviderIdx: -1,
	}
}

// SetAgentRegistry wires the subagent registry + settings handle the Subagents
// tab needs. Without this the tab is hidden (zero agents). Called from
// input.New after the selector is constructed so existing tests that build a
// bare ProviderSelector are unaffected.
func (s *ProviderSelector) SetAgentRegistry(reg AgentRegistry, settings *coresetting.Settings) {
	s.agentRegistry = reg
	s.settings = settings
}

// IsActive returns whether the selector is active.
func (s *ProviderSelector) IsActive() bool {
	return s.active
}

// providerModelSelectedMsg is sent when a model is selected.
type providerModelSelectedMsg struct {
	ModelID      string
	ProviderName string
	AuthMethod   llm.AuthMethod
}

// providerConnectResultMsg is sent when inline connection completes.
type providerConnectResultMsg struct {
	AuthIdx    int
	Success    bool
	Message    string
	NewStatus  llm.Status
	Provider   llm.Name
	AuthMethod llm.AuthMethod
	Models     []llm.ModelInfo
}

// providerModelsLoadedMsg is sent when async model loading completes.
type providerModelsLoadedMsg struct {
	Models []providerModelItem
}

// providerStatusDisplayInfo contains display information for a provider status.
type providerStatusDisplayInfo struct {
	icon  string
	style lipgloss.Style
	desc  string
}

// providerStatusDisplayMap maps provider status to display information.
var providerStatusDisplayMap = map[llm.Status]providerStatusDisplayInfo{
	llm.StatusConnected: {"●", kit.SelectorStatusConnected(), ""},
	llm.StatusAvailable: {"○", kit.SelectorStatusReady(), "(available)"},
}

// providerGetStatusDisplay returns the icon, style, and description for a provider status.
func providerGetStatusDisplay(status llm.Status) (icon string, style lipgloss.Style, desc string) {
	if info, ok := providerStatusDisplayMap[status]; ok {
		return info.icon, info.style, info.desc
	}
	return "◌", kit.SelectorStatusNone(), ""
}

func providerBestAuthMethodStatus(methods []providerAuthMethodItem) llm.Status {
	for _, m := range methods {
		if m.Status == llm.StatusConnected {
			return llm.StatusConnected
		}
	}
	for _, m := range methods {
		if m.Status == llm.StatusAvailable {
			return llm.StatusAvailable
		}
	}
	return llm.StatusNotConfigured
}

// ProviderState holds provider UI state for the TUI model.
// Domain state (LLM, Store, CurrentModel, tokens, thinking) lives
// on the parent app model, not here.
type ProviderState struct {
	Selector       ProviderSelector
	StatusMessage  string // Temporary status shown in status bar
	statusToken    int64
}

// SetStatusMessage sets the temporary status message displayed in the status bar.
func (s *ProviderState) SetStatusMessage(msg string) int64 {
	s.statusToken++
	s.StatusMessage = msg
	return s.statusToken
}

// UpdateProvider routes provider connection and selection messages.
func UpdateProvider(deps OverlayDeps, state *ProviderState, msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case providerConnectingTickMsg:
		// Keep ticking the spinner while connect/refresh is in flight; stop once
		// the matching providerConnectResultMsg lands and IsConnecting goes false.
		if state.Selector.IsConnecting() {
			state.Selector.AdvanceSpinner()
			return providerConnectingTickCmd(), true
		}
		return nil, true
	case providerConnectResultMsg:
		cmd := state.Selector.HandleConnectResult(msg)
		if msg.Success && deps.ReloadModelStore != nil {
			deps.ReloadModelStore()
		}
		return cmd, true
	case providerModelSelectedMsg:
		return handleProviderModelSelected(deps, state, msg), true
	case subagentModelSavedMsg:
		handleSubagentModelSaved(deps, msg)
		return tea.Batch(deps.CommitMessages()...), true
	case visionModelSavedMsg:
		deps.Conv.Append(core.ChatMessage{Role: core.RoleNotice, Content: visionSavedNotice(msg)})
		return tea.Batch(deps.CommitMessages()...), true
	case providerModelsLoadedMsg:
		state.Selector.HandleModelsLoaded(msg)
		if deps.ReloadModelStore != nil {
			deps.ReloadModelStore()
		}
		return nil, true
	case kit.StatusExpiredMsg:
		if msg.Token == state.statusToken {
			state.StatusMessage = ""
		}
		return nil, true
	}
	return nil, false
}

func handleProviderModelSelected(deps OverlayDeps, state *ProviderState, msg providerModelSelectedMsg) tea.Cmd {
	_, err := state.Selector.SetModel(msg.ModelID, msg.ProviderName, msg.AuthMethod)
	if err != nil {
		deps.Conv.Append(core.ChatMessage{Role: core.RoleNotice, Content: "Error: " + err.Error()})
		return tea.Batch(deps.CommitMessages()...)
	}

	deps.SetCurrentModel(&llm.CurrentModelInfo{
		ModelID:    msg.ModelID,
		Provider:   llm.Name(msg.ProviderName),
		AuthMethod: msg.AuthMethod,
	})
	ctx := context.Background()
	providerRefreshConnection(deps, state, ctx, llm.Name(msg.ProviderName), msg.AuthMethod)
	return tea.Batch(deps.CommitMessages()...)
}

func providerRefreshConnection(deps OverlayDeps, state *ProviderState, ctx context.Context, providerName llm.Name, authMethod llm.AuthMethod) {
	p, err := llm.GetProvider(ctx, providerName, authMethod)
	if err != nil {
		log.Logger().Warn("failed to refresh provider connection",
			zap.String("provider", string(providerName)),
			zap.Error(err))
		return
	}
	deps.SwitchProvider(p)
}

// handleSubagentModelSaved reloads the live settings handle so the
// ReconfigureAgentTool override closure reads the fresh snapshot for new
// spawns (updateSettingsFile only clears the package-level cache, not the
// live *Settings.data), then hot-swaps the model of any running subagent
// runs targeted by the save — no close/reopen needed. The swap takes effect
// at the next inference step; conversation is preserved.
func handleSubagentModelSaved(deps OverlayDeps, msg subagentModelSavedMsg) {
	if deps.ReloadSettings != nil {
		_ = deps.ReloadSettings()
	}
	notice := subagentSavedNotice(msg)
	swapped := 0
	if deps.SwapSubagentModels != nil {
		swapped = deps.SwapSubagentModels(context.Background(), msg.AgentName, msg.Model)
	}
	if swapped > 0 {
		notice += fmt.Sprintf(" · %d running subagent(s) hot-swapped", swapped)
	}
	deps.Conv.Append(core.ChatMessage{Role: core.RoleNotice, Content: notice})
}
