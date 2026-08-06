// Bubble Tea View: composes the terminal UI from active content, input area, and status bar.
package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/boytegar/packboy-builder/internal/app/conv"
	"github.com/boytegar/packboy-builder/internal/app/input"
	"github.com/boytegar/packboy-builder/internal/app/kit"
	"github.com/boytegar/packboy-builder/internal/llm"
	"github.com/boytegar/packboy-builder/internal/setting"
	"github.com/boytegar/packboy-builder/internal/subagent"
	"github.com/boytegar/packboy-builder/internal/task"
	"github.com/boytegar/packboy-builder/internal/todo"
)

var ghostTextStyle = lipgloss.NewStyle().Foreground(kit.CurrentTheme.TextDim)

// View dispatches to one of four layouts, top-down:
//
//  1. Loading splash (env not ready yet)
//  2. Active fullscreen overlay (slash-command picker / etc.)
//  3. Active docked modal (Question / Approval) — wrapped between separators
//  4. Normal mode — chat section + status + input strip
//
// The active overlay (cases 2 & 3) comes from activeOverlay — the same
// source the key router uses — so the panel that owns the keyboard is always
// the one drawn on screen.
func (m *model) View() tea.View {
	if frame, ok := m.scrollbackFrameForPrint(); ok {
		return frame
	}
	content, cursor := m.viewString()
	v := tea.NewView(content)
	v.Cursor = cursor
	return v
}

// viewString renders the UI to a styled string; View wraps it in a tea.View.
// The second return is where the terminal's cursor belongs in that frame, or
// nil for the layouts that show no composer — overlays own the keyboard there,
// and their own editors paint a virtual cursor instead.
func (m *model) viewString() (string, *tea.Cursor) {
	if !m.env.Ready {
		return "\n  " + brandMark() + ghostTextStyle.Render("  ·  Loading…"), nil
	}

	ov, hasOverlay := m.activeOverlay()
	if hasOverlay && !isDockedModal(ov) {
		return ov.Render(), nil // fullscreen slash-command picker
	}

	separator := conv.SeparatorStyle.Render(strings.Repeat("─", m.env.Width))
	trackerView := m.renderTrackerList()

	if hasOverlay { // docked modal (Question / Approval)
		trackerPrefix := ""
		if trackerView != "" {
			trackerPrefix = "\n" + strings.TrimSuffix(trackerView, "\n") + "\n"
		}
		return trackerPrefix + separator + "\n" + ov.Render(), nil
	}
	return m.renderNormalView(separator, trackerView)
}

// isDockedModal reports whether the active overlay docks above the input area
// — rendered between separators with the task tracker still visible — rather
// than taking over the full screen like the slash-command pickers do. Only
// the Question, Approval, and secret-item modals dock.
func isDockedModal(ov overlayPanel) bool {
	switch ov.(type) {
	case *conv.QuestionPrompt, *input.ApprovalModel, *input.SecretPromptModel:
		return true
	}
	return false
}

// renderNormalView composes the standard layout: chat scrollback area,
// queue preview, textarea + suggestions, and the bottom status line.
//
// Only the active (uncommitted) tail is rendered here; finished messages are
// already in the terminal's native scrollback (committed via tea.Println, see
// model_scrollback.go). The chat section is height-limited so the View()
// output never exceeds the terminal height: when the live tail is taller than
// the space above the input area, its last lines (the latest content) are
// shown and earlier lines scroll off — the full message lands in native
// scrollback at turn end, which the terminal scrolls back through natively.
//
// The managed frame is only as tall as the live tail + footer. Padding it to
// full terminal height would push native scrollback (committed thinking /
// content) far above the input and leave a large blank band between them.
func (m *model) renderNormalView(separator, trackerView string) (string, *tea.Cursor) {
	// Render the footer first so we can measure how many lines it consumes
	// and cap the chat section to the remaining terminal height.
	footer, inputRow := m.renderFooter(separator)
	// A non-positive result means the footer already fills the screen; tailLines
	// reads that as "no room" and drops the chat section entirely.
	maxContentHeight := m.env.Height - strings.Count(footer, "\n")

	activeContent := conv.RenderActiveContent(m.messageRenderParams())
	chatSection := tailLines(m.renderChatSection(activeContent, trackerView), maxContentHeight)

	view := chatSection + footer
	// While the startup banner is still live (not yet frozen into scrollback),
	// pad the managed frame to the full terminal height. A full-height managed
	// frame keeps the entire screen owned by the inline renderer, so a
	// terminal shrink can't reflow the live banner into native history (which
	// would duplicate it). This padding is only needed pre-commit — once the
	// banner is frozen (welcomePending=false) the frame shrinks so committed
	// scrollback sits directly above the input without a blank gap.
	padding := 0
	if m.welcomePending {
		padding = m.env.Height - (strings.Count(view, "\n") + 1)
		if padding < 0 {
			padding = 0
		}
	}
	if padding > 0 {
		view = strings.Repeat("\n", padding) + view
		inputRow += padding
	}

	return view, m.inputCursor(strings.Count(chatSection, "\n") + inputRow)
}

// inputCursor returns where the terminal cursor belongs inside the composer.
// baseRow is the frame row the textarea's first line lands on; the textarea
// reports its cursor relative to that, and the prompt shifts it right.
func (m *model) inputCursor(baseRow int) *tea.Cursor {
	cursor := m.userInput.Textarea.Cursor()
	if cursor == nil { // textarea is blurred
		return nil
	}
	cursor.Position.X += lipgloss.Width(conv.InputPrompt)
	cursor.Position.Y += baseRow
	return cursor
}

// tailLines returns the last maxLines newline-delimited lines of s, keeping the
// latest content visible when the live tail is taller than the available
// height. s is returned unchanged when it already fits.
func tailLines(s string, maxLines int) string {
	// No room at all — the footer already fills the screen. Returning s here
	// would push the frame past the terminal height, and the renderer resolves
	// that by dropping rows off the top, which eats into the input strip.
	if maxLines <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

// renderFooter renders everything below the chat section (separators, queue
// preview, input area, suggestions, status line) into a single string so its
// line count can be measured. It also reports the row the input area starts on,
// which View needs to place the cursor: the queue preview above the composer is
// variable-height, so the offset can't be a constant.
func (m *model) renderFooter(separator string) (string, int) {
	var b strings.Builder
	// Queued messages sit above the input separator: they are already
	// submitted — on their way into the conversation — so they read as part
	// of the chat flow, and the input strip below stays reserved for what's
	// being typed now.
	if queuePreview := m.renderQueuePreview(); queuePreview != "" {
		b.WriteString("\n")
		b.WriteString(queuePreview)
	}
	b.WriteString("\n")
	b.WriteString(separator)
	b.WriteString("\n")
	// Pending-image badges sit on the reserved image/warning line, above the
	// textarea. Rendered before inputRow so the cursor math stays correct —
	// the badge line is part of the chrome below the separator, not part of
	// the input row.
	if badgeLine := m.renderImageBadgeLine(); badgeLine != "" {
		b.WriteString(badgeLine)
		b.WriteString("\n")
	}
	inputRow := strings.Count(b.String(), "\n")
	b.WriteString(m.renderInputView())
	if suggestions := m.userInput.Suggestions.Render(m.env.Width); suggestions != "" {
		b.WriteString("\n")
		b.WriteString(suggestions)
	}
	b.WriteString("\n")
	b.WriteString(separator)
	b.WriteString("\n")
	if statusLine := m.renderModeStatus(); statusLine != "" {
		b.WriteString(statusLine)
	} else {
		b.WriteString(" ")
	}
	return b.String(), inputRow
}

func (m model) renderInputView() string {
	prompt := conv.InputPromptStyle.Render(conv.InputPrompt)
	if m.userInput.PromptSuggestion.Text != "" && m.userInput.Textarea.Value() == "" &&
		!m.conv.Stream.Active && !m.userInput.Suggestions.IsVisible() {
		return prompt + ghostTextStyle.Render(m.userInput.PromptSuggestion.Text)
	}
	return prompt + hangComposerRows(m.userInput.RenderTextarea())
}

// renderImageBadgeLine renders one truncated-filename badge per pending image
// on the reserved image/warning line, left-aligned with the textarea gutter.
// Returns "" when no images are pending so no blank line is inserted. Each
// badge mirrors the inline token's label (e.g. "[iniga-.png #2]") so the
// above-text badge and the in-text token read identically.
func (m model) renderImageBadgeLine() string {
	if len(m.userInput.Images.Pending) == 0 {
		return ""
	}
	var parts []string
	for _, p := range m.userInput.Images.Pending {
		label := input.ImageBadgeLabel(p.ID, p.Data.FileName)
		parts = append(parts, conv.PendingImageStyle.Render(label))
	}
	return strings.Join(parts, " ")
}

// hangComposerRows indents every composer row after the first by the prompt's
// width, so wrapped and newline-split input hangs under the first row's text
// instead of falling back to column 0. Two reasons it has to: a committed user
// message renders the same way in scrollback (RenderUserMessage joins the
// prompt and the body side by side), and inputCursor shifts the cursor right by
// that same prompt width on *every* row — without the indent the cursor sits
// two columns past the character it should precede on every line but the first.
func hangComposerRows(view string) string {
	lines := strings.Split(view, "\n")
	if len(lines) < 2 {
		return view
	}
	gutter := strings.Repeat(" ", lipgloss.Width(conv.InputPrompt))
	for i := 1; i < len(lines); i++ {
		lines[i] = gutter + lines[i]
	}
	return strings.Join(lines, "\n")
}

// renderChatSection assembles the active chat content (uncommitted messages,
// tracker, transient spinners) into a single string. Height-limiting is
// applied by the caller (tailLines).
func (m model) renderChatSection(activeContent, trackerView string) string {
	var parts []string

	if banner := m.liveWelcome(); banner != "" {
		// Trailing blank line so the splash isn't cramped against the
		// separator/input strip below it — matching the blank line it gets
		// in scrollback, where the first message's leading newline supplies it.
		parts = append(parts, banner, "")
	}

	if activeContent != "" {
		parts = append(parts, activeContent)
	}

	if trackerView != "" {
		// Leading "\n" forces a blank line between the assistant content
		// (often flushed to scrollback via tea.Println) and the tracker
		// block that anchors the bottom of the active view.
		parts = append(parts, "\n"+strings.TrimSuffix(trackerView, "\n"))
	}

	if m.userInput.Provider.FetchingLimits {
		spinnerView := conv.ThinkingStyle.Render(m.conv.Spinner.View() + " Fetching token limits...")
		if len(parts) > 0 {
			spinnerView = "\n" + spinnerView
		}
		parts = append(parts, spinnerView)
	}

	if m.conv.AnalyzingImages {
		spinnerView := conv.ThinkingStyle.Render(m.conv.Spinner.View() + " Analyzing image...")
		if len(parts) > 0 {
			spinnerView = "\n" + spinnerView
		}
		parts = append(parts, spinnerView)
	}

	if compactView := conv.RenderCompactStatus(m.env.Width, m.conv.Spinner.View(), m.conv.Compact); compactView != "" {
		parts = append(parts, compactView)
	}

	if live := m.renderSelfLearnLive(); live != "" {
		// Surrounded by blank rows so the inline indicator reads as
		// its own block — clearly separated from active content above
		// (so it doesn't squish against an assistant turn) and the
		// prompt below (the breathing room is what makes the row feel
		// "live" rather than nailed to the input bar).
		parts = append(parts, "", live, "")
	}

	return strings.Join(parts, "\n")
}

// liveWelcome returns the startup splash for the live view while it is still
// pending — i.e. before the first scrollback commit. Drawing it here keeps the
// banner visible from launch and lets it track the model the user picks (the
// view re-renders on selection); the identical banner is frozen into scrollback
// by takeWelcomeBanner on the first commit, after which welcomePending is false
// and this returns "".
func (m model) liveWelcome() string {
	if !m.welcomePending {
		return ""
	}
	return m.welcomeBannerText()
}

// renderSelfLearnLive returns the L1 indicator as an inline live row
// during reviewing / failed phases. The done phase is suppressed here —
// the recap card is published into the conversation flow instead and
// would otherwise duplicate the "✓ <summary>" line. Idle ⇒ "".
func (m model) renderSelfLearnLive() string {
	if m.services.SelfLearn.Indicator == nil {
		return ""
	}
	snap := m.services.SelfLearn.Indicator.Snapshot()
	switch snap.Phase {
	case selflearnReviewing, selflearnFailed:
		return selflearnLiveStyle.Render(snap.Render())
	}
	return ""
}

// Pointer receiver: Executing below is a *model method value, so a value
// receiver would take the address of the copy and force the whole model onto
// the heap on every frame.
func (m *model) renderTrackerList() string {
	if !m.conv.ShowTasks {
		return ""
	}
	return conv.RenderTrackerList(conv.TrackerListParams{
		Items:        m.services.Tracker.List(),
		StreamActive: m.conv.Stream.Active,
		Width:        m.env.Width,
		Blockers:     m.services.Tracker.OpenBlockers,
		Executing:    m.executingTrackerItem,
		Blink:        m.conv.Spinner.Frame(),
		AgentColors:  m.agentColors(),
		Expanded:     m.expandedTrackerItem,
		DetailLines:  m.trackerDetailLines,
		FocusedIdx:   m.trackerFocusIdx,
	})
}

// trackerDetailLines reports the subagent detail lines shown under an expanded
// tracker row. It joins the item's tracked background task against the live
// task manager and summarises the agent/lifecycle in a few lines for the
// dropdown; plan items (no background task) report nothing.
func (m *model) trackerDetailLines(itemID string) []string {
	item, ok := m.services.Tracker.Get(itemID)
	if !ok || item == nil {
		return nil
	}
	taskID := todo.BackgroundTaskID(item)
	if taskID == "" {
		return nil
	}
	bg, ok := m.services.Task.Get(taskID)
	if !ok {
		return nil
	}
	info := bg.GetStatus()

	var lines []string
	if info.AgentName != "" {
		lines = append(lines, "agent: "+info.AgentName)
	}
	lines = append(lines, "status: "+string(info.Status))
	if info.Type != "" {
		lines = append(lines, "type: "+string(info.Type))
	}
	if info.StepCount > 0 {
		lines = append(lines, fmt.Sprintf("steps: %d", info.StepCount))
	}
	if info.TokenUsage > 0 {
		lines = append(lines, fmt.Sprintf("tokens: %d", info.TokenUsage))
	}
	out := strings.TrimSpace(info.Output)
	if out != "" {
		lines = append(lines, "output: "+out)
	}
	if clipped := strings.TrimSpace(clipToMaxLines(info, conv.MaxSubagentLines-len(lines))); clipped != "" {
		lines = append(lines, clipped)
	}
	return lines
}

// clipToMaxLines guards a detail block from outgrowing the dropdown budget even
// before RenderTrackerList re-caps it — long bash output could otherwise push a
// single row's detail past the 15-line ceiling.
func clipToMaxLines(info task.TaskInfo, budget int) string {
	if budget <= 0 {
		return ""
	}
	out := strings.TrimSpace(info.Output)
	if out == "" {
		return ""
	}
	lines := strings.Split(out, "\n")
	if len(lines) > budget {
		lines = lines[:budget]
	}
	return strings.Join(lines, "\n")
}

func (m model) renderModeStatus() string {
	modelName := m.env.GetModelDisplayName()
	thinkingEffort := m.env.EffectiveThinkingEffort()
	showThinking := true
	if m.env.CurrentModel != nil && m.env.CurrentModel.Provider == llm.OpenAI && thinkingEffort != "" {
		modelName += " (" + thinkingEffort + ")"
		showThinking = false
	}
	if status := m.services.Hook.CurrentStatusMessageSafe(); status != "" {
		modelName = status
	}
	// Surface the default write model for write-enabled subagents when it
	// diverges from the main model, so swarm/agent mode users can see the
	// effective write target at a glance alongside the read model.
	writeModelName := ""
	if m.env.OperationMode == setting.ModeSwarm && m.services.Setting != nil {
		if w := strings.TrimSpace(m.services.Setting.Snapshot().SubagentDefaultModelForWrite); w != "" && w != "inherit" {
			writeModelName = m.env.ModelDisplayName(w)
		}
	}
	reviewApprovals := 0
	if m.reviewerApprovals != nil {
		reviewApprovals = int(m.reviewerApprovals.Load())
	}
	reviewEscalations := 0
	if m.reviewerEscalations != nil {
		reviewEscalations = int(m.reviewerEscalations.Load())
	}
	return conv.RenderModeStatus(conv.OperationModeParams{
		Mode:              m.env.OperationMode,
		Bypass:            m.env.SessionPermissions.IsBypassSafe(),
		Persona:           activePersonaName(&m),
		InputTokens:       m.env.InputTokens,
		InputLimit:        kit.GetEffectiveInputLimit(m.services.LLM.Store(), m.env.CurrentModel),
		ModelName:         modelName,
		WriteModelName:    writeModelName,
		StatusMessage:     m.userInput.Provider.StatusMessage,
		ConversationCost:  m.env.ConversationCost,
		Compressions:      m.env.Compressions,
		ShowContextBar:    m.env.ShowContextBar,
		Width:             m.env.Width,
		ThinkingEffort:    thinkingEffort,
		ShowThinking:      showThinking,
		ReviewApprovals:   reviewApprovals,
		ReviewEscalations: reviewEscalations,
		AutopilotThinking: m.autopilotDeciding,
	})
}

func (m model) renderQueuePreview() string {
	rawItems := m.userInput.Queue.Items()
	if len(rawItems) == 0 {
		return ""
	}
	previews := make([]conv.QueuePreviewItem, len(rawItems))
	for i, item := range rawItems {
		previews[i] = conv.QueuePreviewItem{
			Content:   item.Content,
			HasImages: len(item.Images) > 0,
		}
	}

	return strings.TrimSuffix(conv.RenderQueuePreview(previews, m.userInput.Queue.SelectIdx, m.env.Width), "\n")
}

func (m model) messageRenderParams() conv.RenderContext {
	return conv.RenderContext{
		// Conversation state
		Messages:       m.conv.Messages,
		CommittedCount: m.conv.CommittedCount,
		InlinedResults: conv.PrecomputeInlinedResults(m.conv.Messages, m.conv.CommittedCount),

		// Streaming + tool execution
		StreamActive:  m.conv.Stream.Active,
		BuildingTool:  m.conv.Stream.BuildingTool,
		PendingCalls:  m.conv.Tool.PendingCalls,
		CurrentIdx:    m.conv.Tool.CurrentIdx,
		ToolStartedAt: m.conv.Tool.StartedAt,
		ToolProgress:  m.conv.Tool.Progress,

		// Renderer env
		Width:      m.env.Width,
		MDRenderer: m.conv.MDRenderer,

		// Per-tick UI state
		SpinnerView:  m.conv.Spinner.View(),
		Blink:        m.conv.Spinner.Frame(),
		ModelName:    m.env.GetModelDisplayName(),
		InputTokens:  m.env.InputTokens,
		OutputTokens: m.env.OutputTokens,

		// Decorations
		AgentColors:  m.agentColors(),
		TaskActivity: m.conv.TaskActivity,
		TaskOwnerMap: buildTaskOwnerMap(m.services.Tracker.List()),

		// Modal interlock
		InteractivePromptActive: m.conv.Modal.Question != nil && m.conv.Modal.Question.IsActive(),
	}
}

func (m model) agentColors() map[string]string {
	return buildAgentColors(m.services.Subagent.ListConfigs())
}

func buildAgentColors(configs []*subagent.AgentConfig) map[string]string {
	if len(configs) == 0 {
		return nil
	}
	colors := make(map[string]string, len(configs))
	for _, cfg := range configs {
		if cfg == nil || cfg.Color == "" {
			continue
		}
		colors[strings.ToLower(cfg.Name)] = cfg.Color
	}
	return colors
}

func buildTaskOwnerMap(tasks []*todo.Item) map[string]string {
	if len(tasks) == 0 {
		return nil
	}
	ownerMap := make(map[string]string, len(tasks))
	for _, t := range tasks {
		if t.Owner != "" {
			ownerMap[t.ID] = t.Owner
		}
	}
	if len(ownerMap) == 0 {
		return nil
	}
	return ownerMap
}
