package input

import (
	"context"

	tea "charm.land/bubbletea/v2"

	"github.com/boytegar/packboy-builder/internal/app/conv"
	"github.com/boytegar/packboy-builder/internal/llm"
)

// OverlayDeps holds all dependencies needed by overlay selector handlers.
type OverlayDeps struct {
	State *Model
	Conv  *conv.ConversationModel
	Cwd   string

	CommitMessages    func() []tea.Cmd
	CommitAllMessages func() []tea.Cmd

	SwitchProvider          func(llm.Provider)
	SetCurrentModel         func(*llm.CurrentModelInfo)
	ReloadModelStore        func()
	ClearCachedInstructions func()
	RefreshMemoryContext    func(cwd, reason string)
	FireFileChanged         func(path, tool string)
	ReloadAfterPluginChange func() error
	LoadSession             func(string) error
	SetActivePersona        func(name string) error
	OpenPersona             func(name string) tea.Cmd
	DeletePersona           func(name string) error

	// SwapSubagentModels hot-swaps the model of live subagent runs after a
	// /models → Subagents tab save. agentName is the targeted agent name, or
	// "" / "Default model" / "Default model for write" for the global default
	// slots; modelRef is the persisted ref (a bare id, alias, "vendor/model",
	// or "inherit"). Returns the number of runs swapped. nil when no executor
	// is wired (headless / pre-init).
	SwapSubagentModels func(ctx context.Context, agentName, modelRef string) int

	// ReloadSettings reloads the live *Settings handle from disk. Used after a
	// subagent model override is persisted so the override closure reads the
	// fresh snapshot for new spawns (updateSettingsFile only clears the
	// package-level cache). nil when no settings service is wired.
	ReloadSettings func() error
}
