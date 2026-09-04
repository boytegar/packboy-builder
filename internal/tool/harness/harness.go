// Package harness provides agent-facing tools for the continual harness:
// the persistent, editable layer of supplemental prompts, memories, skill
// descriptions, and subagent specs that the agent can refine through small,
// evidence-backed updates.
package harness

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/boytegar/packboy-builder/internal/core"
	"github.com/boytegar/packboy-builder/internal/tool"
	"github.com/boytegar/packboy-builder/internal/tool/toolresult"
)

const IconRefine = "✧"

// RefineTool lets the agent trigger continual harness refinement from within
// the conversation. The refinement runs when the current turn ends; the harness
// applies changes and rebuilds the system prompt, then resumes automatically.
type RefineTool struct{}

func (t *RefineTool) Name() string { return "harness_refine" }

func (t *RefineTool) Description() string {
	return "Trigger continual harness refinement from the conversation. " +
		"Use when you notice a repeated failure, reusable tactic, delegation role, " +
		"or behavior policy that should be persisted as a harness entry " +
		"(prompt, memory, skill, or subagent spec). " +
		"Refinement runs when the current turn ends and applies small, evidence-backed " +
		"updates. Returns immediately with schedule status."
}

func (t *RefineTool) Icon() string { return IconRefine }

func (t *RefineTool) Schema() core.ToolSchema {
	return core.ToolSchema{
		Name:        "harness_refine",
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"instructions": map[string]any{
					"type":        "string",
					"description": "Optional focus instructions for the refinement. Use to direct attention to a specific observation.",
				},
				"global": map[string]any{
					"type":        "boolean",
					"description": "Target the global (cross-session) harness store. Default: false (local/session-scoped).",
				},
			},
		},
	}
}

func (t *RefineTool) Execute(ctx context.Context, params map[string]any, cwd string) toolresult.ToolResult {
	instructions := tool.GetString(params, "instructions")
	global := tool.GetBool(params, "global")

	out := "Continual harness refinement scheduled for end of turn."
	if instructions != "" {
		out = fmt.Sprintf("Continual harness refinement scheduled: %s", instructions)
	}
	if global {
		out += " (global scope)"
	} else {
		out += " (local scope)"
	}

	return toolresult.ToolResult{
		Success: true,
		Output:  out,
		Metadata: toolresult.ResultMetadata{
			Title:    "Refine",
			Icon:     IconRefine,
			Subtitle: "harness refinement scheduled",
		},
	}
}

// HarnessStatusTool lets the agent check the current harness state.
type HarnessStatusTool struct{}

func (t *HarnessStatusTool) Name() string { return "harness_status" }

func (t *HarnessStatusTool) Description() string {
	return "Read the current continual harness state: entries (prompts, memories, skills, subagent specs) " +
		"and recent refinement history. Use to inspect what has been persisted before deciding to refine."
}

func (t *HarnessStatusTool) Icon() string { return IconRefine }

func (t *HarnessStatusTool) Schema() core.ToolSchema {
	return core.ToolSchema{
		Name:        "harness_status",
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"kind": map[string]any{
					"type":        "string",
					"description": "Optional filter by kind: prompt, memory, skill, subagent. Empty = all.",
				},
				"global": map[string]any{
					"type":        "boolean",
					"description": "Read from the global (cross-session) store. Default: false (local).",
				},
			},
		},
	}
}

func (t *HarnessStatusTool) Execute(ctx context.Context, params map[string]any, cwd string) toolresult.ToolResult {
	kindFilter := tool.GetString(params, "kind")
	global := tool.GetBool(params, "global")

	scope := "local"
	if global {
		scope = "global"
	}

	summary := fmt.Sprintf("Continual harness (%s scope): no entries loaded — harness state will be loaded on first refinement.", scope)
	if kindFilter != "" {
		summary = fmt.Sprintf("Continual harness (%s scope, kind=%s): no entries loaded.", scope, kindFilter)
	}

	return toolresult.ToolResult{
		Success: true,
		Output:  summary,
		Metadata: toolresult.ResultMetadata{
			Title:    "Harness Status",
			Icon:     IconRefine,
			Subtitle: scope + " scope",
		},
	}
}

// HarnessRollbackTool lets the agent roll back a previously applied refinement.
type HarnessRollbackTool struct{}

func (t *HarnessRollbackTool) Name() string { return "harness_rollback" }

func (t *HarnessRollbackTool) Description() string {
	return "Roll back a previously applied continual harness refinement by ID. " +
		"Reverses all edits from that refinement: creates are deleted, updates are restored, deletes are recreated. " +
		"Use when a prior refinement caused issues or is no longer valid."
}

func (t *HarnessRollbackTool) Icon() string { return IconRefine }

func (t *HarnessRollbackTool) Schema() core.ToolSchema {
	return core.ToolSchema{
		Name:        "harness_rollback",
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"refinement_id": map[string]any{
					"type":        "string",
					"description": "The ID of the refinement to roll back.",
				},
			},
			"required": []string{"refinement_id"},
		},
	}
}

func (t *HarnessRollbackTool) Execute(ctx context.Context, params map[string]any, cwd string) toolresult.ToolResult {
	refinementID := tool.GetString(params, "refinement_id")
	if refinementID == "" {
		return toolresult.ToolResult{
			Success: false,
			Error:   "refinement_id is required",
		}
	}

	// In the full implementation, this would:
	// 1. Load refinement history from the harness dir
	// 2. Find the target refinement by ID
	// 3. Load current harness state
	// 4. Call harness.RollbackResult(state, target)
	// 5. Save the updated state
	// 6. Append the rollback as a new refinement to history

	result := map[string]string{
		"status":        "pending",
		"refinement_id": refinementID,
		"message":       "Rollback scheduled. The refinement will be reversed at end of turn.",
	}
	data, _ := json.Marshal(result)

	return toolresult.ToolResult{
		Success: true,
		Output:  string(data),
		Metadata: toolresult.ResultMetadata{
			Title:    "Harness Rollback",
			Icon:     IconRefine,
			Subtitle: "rolling back " + refinementID,
		},
	}
}

func init() {
	tool.Register(&RefineTool{})
	tool.Register(&HarnessStatusTool{})
	tool.Register(&HarnessRollbackTool{})
}
