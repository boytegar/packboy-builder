package subagent

import (
	"context"
	"fmt"
	"strings"

	"github.com/boytegar/packboy-builder/internal/llm"
	"go.uber.org/zap"

	"github.com/boytegar/packboy-builder/internal/log"
)

// registerRun records an in-flight run under its broker address so
// SwapRunModel can find and hot-swap its client. Safe to call with an empty
// addr (no-op); an existing entry for the same addr is overwritten, which is
// harmless since a prior run with the same address has already unregistered.
func (e *Executor) registerRun(addr string, run *preparedRun) {
	if addr == "" || run == nil {
		return
	}
	e.runsMu.Lock()
	if e.liveRuns == nil {
		e.liveRuns = make(map[string]*preparedRun)
	}
	e.liveRuns[addr] = run
	e.runsMu.Unlock()
}

// unregisterRun removes a run from the live-run registry.
func (e *Executor) unregisterRun(addr string) {
	if addr == "" {
		return
	}
	e.runsMu.Lock()
	delete(e.liveRuns, addr)
	e.runsMu.Unlock()
}

// snapshotRuns returns a slice copy of the currently registered runs. Used by
// SwapRunModelByName so the swap iterates a stable snapshot without holding
// the registry lock across the (potentially slow) model re-resolution.
func (e *Executor) snapshotRuns() []*preparedRun {
	e.runsMu.RLock()
	defer e.runsMu.RUnlock()
	out := make([]*preparedRun, 0, len(e.liveRuns))
	for _, r := range e.liveRuns {
		out = append(out, r)
	}
	return out
}

// SwapRunModelByName hot-swaps the model of every live run whose agent config
// name matches agentName. A swap takes effect at the next inference step
// (Infer reads the model per call) — the conversation is preserved and no
// rebuild happens.
//
// modelRef is the same form accepted by the /models Subagents tab: a bare id,
// an alias (sonnet/opus/haiku), a "vendor/model" override, or "inherit" to
// follow the parent. Resolution mirrors resolveModel so a swap lands on the
// same provider/model a fresh spawn would pick.
//
// agentName selects the targeting mode:
//   - a concrete agent name → only runs of that agent
//   - "" or "Default model" → every live run (the global default applies to
//     all inherited runs)
//   - "Default model for write" → only write-enabled runs (the write default
//     only affects AllowWrite subagents)
//
// This is the entry point used by the TUI when a user changes a subagent model
// via /models → Subagents tab; it targets running runs by agent name rather
// than by task id because the TUI does not track individual run addresses.
func (e *Executor) SwapRunModelByName(ctx context.Context, agentName, modelRef string) int {
	name := strings.TrimSpace(agentName)
	isWriteDefault := name == "Default model for write"
	isGlobalDefault := name == "" || name == "Default model"
	runs := e.snapshotRuns()
	swapped := 0
	for _, run := range runs {
		if run == nil || run.client == nil || run.cfg == nil {
			continue
		}
		if isGlobalDefault || isWriteDefault {
			// Global default slots only affect runs that inherited the default,
			// i.e. runs whose own frontmatter model is empty or "inherit".
			fm := strings.TrimSpace(run.cfg.config.Model)
			if fm != "" && fm != "inherit" {
				continue
			}
			if isWriteDefault {
				if !(run.cfg.config.AllowWrite || e.isWriteEnabled(run.cfg.config.Name)) {
					continue
				}
			}
		} else if run.cfg.config.Name != name {
			continue
		}
		if err := e.swapRun(ctx, run, modelRef); err != nil {
			log.Logger().Warn("subagent model swap failed",
				zap.String("agent", run.cfg.displayName),
				zap.String("modelRef", modelRef),
				zap.Error(err))
			continue
		}
		swapped++
	}
	return swapped
}

// swapRun re-resolves modelRef for a single run and applies it to the live
// client. A same-vendor result calls SetModel; a cross-vendor result calls
// SetProvider. run.cfg is updated so subsequent compaction/activity lines
// report the new model.
func (e *Executor) swapRun(ctx context.Context, run *preparedRun, modelRef string) error {
	allowWrite := run.cfg.config.AllowWrite || e.isWriteEnabled(run.cfg.config.Name)
	provider, modelID, err := e.resolveModel(ctx, modelRef, run.cfg.config.Model, run.cfg.config.Name, allowWrite)
	if err != nil {
		return err
	}
	// If the resolved provider differs from the run's current provider, swap
	// both. Otherwise a bare SetModel keeps the existing provider connection.
	currentProvider := run.cfg.provider
	run.cfg.provider = provider
	run.cfg.modelID = modelID

	if providerSame(currentProvider, provider) {
		run.client.SetModel(modelID)
	} else {
		run.client.SetProvider(provider, modelID)
	}

	run.streamActivity(fmt.Sprintf("Model swapped: %s", modelID))
	return nil
}

// providerSame reports whether two providers talk to the same vendor, so a
// swapRun can decide between SetModel (same vendor) and SetProvider
// (cross-vendor). nil-safe.
func providerSame(a, b llm.Provider) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Name() == b.Name()
}
