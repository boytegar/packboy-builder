package specmode

import (
	"context"
	"fmt"
	"strings"

	"github.com/boytegar/packboy-builder/internal/core"
	"github.com/boytegar/packboy-builder/internal/tool"
	"github.com/boytegar/packboy-builder/internal/tool/perm"
	"github.com/boytegar/packboy-builder/internal/tool/toolresult"
)

// ExitSpecModeTool is an interactive tool that presents a concrete
// implementation plan to the user and blocks until the user approves
// or rejects it. Once approved, the agent exits spec/research mode and
// begins implementation. This creates an explicit approval gate between
// research/planning and write/execute.
type ExitSpecModeTool struct {
	requestCounter int
}

func NewExitSpecModeTool() *ExitSpecModeTool {
	return &ExitSpecModeTool{}
}

func (t *ExitSpecModeTool) Name() string {
	return "ExitSpecMode"
}

func (t *ExitSpecModeTool) Description() string {
	return `Present an implementation plan to the user for approval before starting coding.

Use this tool ONLY when spec mode is active (research/planning phase before any code is written). Present the plan once you have finished gathering context, exploring the codebase, and forming a concrete approach — not before.

Do NOT use this tool for:
- Pure research or investigation tasks with no implementation.
- Tasks where the user has already approved the approach.
- Conversational/informational responses.

The plan should be concise: a numbered list of implementation steps, with minimal key code snippets where they clarify the approach. Avoid unresolved alternatives — if multiple approaches are equally strong, ask the user to choose via AskUserQuestion first, then present the chosen approach here.`
}

func (t *ExitSpecModeTool) Icon() string {
	return "📋"
}

func (t *ExitSpecModeTool) Schema() core.ToolSchema {
	return core.ToolSchema{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"plan": map[string]any{
					"type":        "string",
					"description": "The concrete implementation plan in markdown. Numbered steps, file paths, approach, and minimal key code snippets where needed.",
				},
				"title": map[string]any{
					"type":        "string",
					"description": "Optional short title for the spec/plan.",
				},
			},
			"required": []string{"plan"},
		},
	}
}

func (t *ExitSpecModeTool) RequiresPermission() bool {
	return false
}

func (t *ExitSpecModeTool) RequiresInteraction() bool {
	return true
}

func (t *ExitSpecModeTool) PreparePermission(ctx context.Context, params map[string]any, cwd string) (*perm.PermissionRequest, error) {
	return nil, fmt.Errorf("ExitSpecMode does not use the permission flow")
}

func (t *ExitSpecModeTool) PrepareInteraction(ctx context.Context, params map[string]any, cwd string) (any, error) {
	plan := tool.GetString(params, "plan")
	if strings.TrimSpace(plan) == "" {
		return nil, fmt.Errorf("plan is required and must not be empty")
	}

	title := tool.GetString(params, "title")
	if strings.TrimSpace(title) == "" {
		title = "Implementation Plan"
	}

	t.requestCounter++
	requestID := fmt.Sprintf("spec-%d", t.requestCounter)

	return &tool.QuestionRequest{
		ID: requestID,
		Questions: []tool.Question{
			{
				Question: "Do you approve this plan? Approve to start implementation, or reject to continue refining.",
				Header:   "Spec Approval",
				Options: []tool.QuestionOption{
					{Label: "Approve", Description: "Exit spec mode and begin implementation"},
					{Label: "Reject", Description: "Stay in spec mode; the agent will revise the plan"},
				},
			},
		},
	}, nil
}

func (t *ExitSpecModeTool) ExecuteWithResponse(ctx context.Context, params map[string]any, response any, cwd string) toolresult.ToolResult {
	resp, ok := response.(*tool.QuestionResponse)
	if !ok {
		return toolresult.NewErrorResult(t.Name(), "invalid response type")
	}

	plan := tool.GetString(params, "plan")
	title := tool.GetString(params, "title")
	if strings.TrimSpace(title) == "" {
		title = "Implementation Plan"
	}

	if resp.Cancelled {
		return toolresult.ToolResult{
			Success: false,
			Output:  "User cancelled the spec approval prompt without answering.",
			Metadata: toolresult.ResultMetadata{
				Title:    t.Name(),
				Icon:     t.Icon(),
				Subtitle: "Cancelled",
			},
		}
	}

	for _, answers := range resp.Answers {
		for _, answer := range answers {
			switch strings.ToLower(strings.TrimSpace(answer)) {
			case "approve":
				return toolresult.ToolResult{
					Success: true,
					Output:  "Plan approved. Exiting spec mode — begin implementation now.",
					HookResponse: map[string]any{
						"specApproved": true,
						"plan":         plan,
						"title":        title,
						"exitSpecMode": true,
					},
					Metadata: toolresult.ResultMetadata{
						Title:    t.Name(),
						Icon:     t.Icon(),
						Subtitle: "Approved — implementation can begin",
					},
				}
			case "reject":
				return toolresult.ToolResult{
					Success: false,
					Output:  "Plan rejected. Stay in spec mode and revise the plan based on the user's feedback.",
					HookResponse: map[string]any{
						"specApproved": false,
						"plan":         plan,
						"title":        title,
						"exitSpecMode": false,
					},
					Metadata: toolresult.ResultMetadata{
						Title:    t.Name(),
						Icon:     t.Icon(),
						Subtitle: "Rejected — revise the plan",
					},
				}
			}
		}
	}

	return toolresult.NewErrorResult(t.Name(), "no valid answer received")
}

func (t *ExitSpecModeTool) Execute(ctx context.Context, params map[string]any, cwd string) toolresult.ToolResult {
	return toolresult.NewErrorResult(t.Name(), "this tool requires user interaction — use PrepareInteraction and ExecuteWithResponse")
}

func init() {
	tool.Register(NewExitSpecModeTool())
}
