package agent

import (
	"fmt"
	"strings"
	"time"

	"github.com/boytegar/packboy-builder/internal/tool"
	"github.com/boytegar/packboy-builder/internal/tool/toolresult"
)

// maxResultActivityLines caps the tool activity echoed into the parent's
// context. The full trail stays visible in the TUI activity stream; the parent
// LLM only needs enough of the tail to sanity-check what the agent did.
//
// Raised from 30 to 50: a terse activity trail is what lets the parent trust
// the summary without re-scanning, and 50 lines is still a small fraction of
// a turn's context budget. The subagent's final summary (result.Content) is
// always included in full below the trail, so the cap only affects the trace.
const maxResultActivityLines = 50

// formatForegroundAgentResult renders a finished subagent's result for the
// parent's tool result: a short header, a capped tail of the tool trace, then
// the subagent's final message.
func formatForegroundAgentResult(agentName string, result *tool.AgentExecResult, duration time.Duration) string {
	displayName := result.AgentName
	if displayName == "" {
		displayName = agentName
	}
	agentDuration := result.Duration
	if agentDuration == 0 {
		agentDuration = duration
	}

	var outputBuilder strings.Builder
	fmt.Fprintf(&outputBuilder, "Agent: %s\nModel: %s\nSteps: %d\nToolUses: %d\nTokens: in=%d out=%d\nDuration: %s\n",
		displayName, result.Model, result.StepCount, result.ToolUses, result.TotalInputTokens, result.TotalOutputTokens, toolresult.FormatDuration(agentDuration))
	if result.AgentID != "" {
		fmt.Fprintf(&outputBuilder, "AgentID: %s\n", result.AgentID)
	}
	outputBuilder.WriteString("\n")

	activity := result.Activity
	if len(activity) > maxResultActivityLines {
		fmt.Fprintf(&outputBuilder, "(%d earlier tool calls omitted)\n", len(activity)-maxResultActivityLines)
		activity = activity[len(activity)-maxResultActivityLines:]
	}
	for _, line := range activity {
		outputBuilder.WriteString(line)
		outputBuilder.WriteString("\n")
	}
	if result.Content != "" {
		// Label the subagent's final message as a summary so the parent can
		// distinguish the report (what was found) from the activity trail
		// (what was done) at a glance. This is the part the parent should
		// rely on for the next turn.
		outputBuilder.WriteString("\n## Summary\n")
		outputBuilder.WriteString(result.Content)
	}
	return outputBuilder.String()
}
