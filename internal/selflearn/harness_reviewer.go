package selflearn

// This file extends the selflearn reviewer with KindHarness support.
// The continual harness refinement loop runs alongside the existing
// memory + skill review after turn completion.

import (
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/boytegar/packboy-builder/internal/core"
	"github.com/boytegar/packboy-builder/internal/log"
)

// KindHarness triggers continual harness refinement: reviewing the trajectory
// for evidence-backed updates to supplemental prompts, memories, skills, and
// subagent specs.
const KindHarness ReviewKind = 1 << 2

// HarnessRefineFunc is called when harness refinement is triggered.
// The snapshot contains the conversation messages from the just-completed turn.
type HarnessRefineFunc func(snapshot []core.Message, instructions string)

// HarnessReviewer extends the base Reviewer with harness refinement support.
// It is composed alongside the existing reviewer; the base Reviewer.Observe
// continues to handle memory + skill review.
type HarnessReviewer struct {
	harnessEnabled  bool
	harnessRefine   HarnessRefineFunc
	harnessMu       sync.Mutex
	harnessInFlight bool
}

// NewHarnessReviewer creates a harness reviewer that wraps the existing
// reviewer's harness capability.
func NewHarnessReviewer(cfg Config, refine HarnessRefineFunc) *HarnessReviewer {
	return &HarnessReviewer{
		harnessEnabled: cfg.HarnessEnabled,
		harnessRefine:  refine,
	}
}

// ObserveHarness triggers harness refinement after a turn completes.
// It is called alongside the base reviewer's Observe. The evolveRequested
// flag gates the review — harness refinement only runs when the user or agent
// has explicitly requested it (via /refine or the harness_refine tool).
func (h *HarnessReviewer) ObserveHarness(result core.Result, instructions string) {
	if !h.harnessEnabled || h.harnessRefine == nil {
		return
	}

	if result.StopReason != core.StopEndTurn {
		return
	}

	h.harnessMu.Lock()
	if h.harnessInFlight {
		h.harnessMu.Unlock()
		log.Logger().Warn("selflearn: skipping harness refine, a prior refine is still running")
		return
	}
	h.harnessInFlight = true
	h.harnessMu.Unlock()

	snapshot := make([]core.Message, len(result.Messages))
	copy(snapshot, result.Messages)

	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				log.Logger().Warn("selflearn: harness refine panicked (recovered)",
					zap.Any("panic", rec),
					zap.Stack("stack"),
				)
			}
			h.harnessMu.Lock()
			h.harnessInFlight = false
			h.harnessMu.Unlock()
		}()
		h.harnessRefine(snapshot, instructions)
	}()
}

// HarnessEnabled reports whether harness refinement is active.
func (h *HarnessReviewer) HarnessEnabled() bool {
	return h.harnessEnabled
}

// Ensure KindHarness is included in the ReviewKind String() method by
// providing an extended string representation.
func HarnessKindString(k ReviewKind) string {
	var parts []string
	if k.Has(KindMemory) {
		parts = append(parts, "memory")
	}
	if k.Has(KindSkills) {
		parts = append(parts, "skill")
	}
	if k.Has(KindHarness) {
		parts = append(parts, "harness")
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, "+")
}
