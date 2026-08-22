package kit

import (
	"fmt"

	"github.com/boytegar/packboy-builder/internal/llm"
	"github.com/boytegar/packboy-builder/internal/setting"
)

// TokenLimitSavedMsg is emitted when the /tokenlimit overlay persists a
// role-scoped budget. The app shows it as a transcript notice.
type TokenLimitSavedMsg struct {
	Target string
	Input  int
	Output int
}

// FormatTokenCount formats a token count for display.
func FormatTokenCount(count int) string {
	switch {
	case count >= 1000000:
		return fmt.Sprintf("%.1fM", float64(count)/1000000)
	case count >= 1000:
		return fmt.Sprintf("%.1fk", float64(count)/1000)
	default:
		return fmt.Sprintf("%d", count)
	}
}

// GetMaxTokens returns the effective output limit for the main agent's client.
// The settings.json mainTokenLimit.outputTokenLimit override wins first (set via
// /tokenlimit), mirroring how llm.Store.EffectiveInputLimitFor prioritizes the
// role-scoped input budget; otherwise it falls back to the provider's configured
// / cached output cap, then defaultMaxTokens.
func GetMaxTokens(store *llm.Store, currentModel *llm.CurrentModelInfo, defaultMaxTokens int) int {
	if o := setting.Default().MainTokenLimit().OutputTokenLimit; o > 0 {
		return o
	}
	if limit := getEffectiveOutputLimit(store, currentModel); limit > 0 {
		return limit
	}
	return defaultMaxTokens
}

// GetModelTokenLimits returns the cached context window for the current model.
//
// The same model ID can be cached under several provider/auth keys with
// different windows (gpt-5.5: 400k via Direct API, 272k via ChatGPT
// subscription). Scanning the cache map for the ID picks a random one each
// render — that is what made the status-bar limit flicker. So we resolve in two
// deterministic steps:
//
//  1. this model's own provider+auth cache — the correct window. Ignores the 24h
//     TTL, otherwise an expired cache would fall to step 2 and flicker again.
//  2. else the largest window for the ID across all caches — covers a model an
//     aggregator serves with no window while its native provider knows the real one.
func GetModelTokenLimits(store *llm.Store, currentModel *llm.CurrentModelInfo) (inputLimit, outputLimit int) {
	if store == nil || currentModel == nil {
		return 0, 0
	}
	authMethod := store.ResolveAuthMethod(currentModel)
	if input, output := store.CachedModelLimitsForProvider(currentModel.Provider, authMethod, currentModel.ModelID); input > 0 {
		return input, output
	}
	return store.CachedModelLimits(currentModel.ModelID)
}

// getEffectiveOutputLimit returns the output cap: a custom limit if set,
// otherwise the cached model metadata. The input side has its own resolver
// (llm.Store.EffectiveInputLimit) because it also honors an env override and
// is shared with the agent's compaction check.
func getEffectiveOutputLimit(store *llm.Store, currentModel *llm.CurrentModelInfo) int {
	if currentModel == nil {
		return 0
	}

	if store != nil {
		if _, output, ok := store.GetTokenLimit(currentModel.ModelID); ok {
			return output
		}
	}

	_, output := GetModelTokenLimits(store, currentModel)
	return output
}

// GetEffectiveInputLimit returns the context window for the status bar's
// percentage, or 0 when it is unknown (no model selected, or a model whose
// window Packboy Builder cannot size) — the bar renders that as "--" rather than a
// percentage of a guess.
//
// It delegates to llm.Store.EffectiveInputLimit, the same resolver
// llm.Client.InputLimit uses for the auto-compaction trigger, so the bar can
// never fill against a different window than the one compaction fires on
// (issue #338).
func GetEffectiveInputLimit(store *llm.Store, currentModel *llm.CurrentModelInfo) int {
	if store == nil || currentModel == nil {
		return 0
	}
	auth := store.ResolveAuthMethod(currentModel)
	return store.EffectiveInputLimit(currentModel.Provider, auth, currentModel.ModelID)
}

// GetEffectiveInputLimitFor resolves the context window for a specific role
// (main agent / subagent), honoring that role's settings.json /tokenlimit budget
// first via llm.Store.EffectiveInputLimitFor. The status bar's context
// percentage must use Main so it never disagrees with the main agent's
// auto-compaction trigger (llm.Client.InputLimit → EffectiveInputLimitFor).
func GetEffectiveInputLimitFor(role llm.TokenRole, store *llm.Store, currentModel *llm.CurrentModelInfo) int {
	if store == nil || currentModel == nil {
		return 0
	}
	auth := store.ResolveAuthMethod(currentModel)
	return store.EffectiveInputLimitFor(role, currentModel.Provider, auth, currentModel.ModelID)
}
