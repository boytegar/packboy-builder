// Message commit rendering: convert pending conversation messages into ANSI
// and append the result to the chat viewport render cache (see
// model_viewport.go). Rendering runs off the UI goroutine where it can be
// slow (glamour); committing is a cheap append that the next frame's view
// picks up. The terminal's native scrollback is not used — the alt-screen
// chat viewport owns the whole conversation and scrolls in place.
package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"

	"github.com/boytegar/packboy-builder/internal/app/conv"
	"github.com/boytegar/packboy-builder/internal/core"
)

func (m *model) CommitMessages() []tea.Cmd {
	return m.renderAndCommit(true)
}

// Streaming-flush pipeline. As an assistant message streams in, each of its
// completed markdown blocks migrates from the live tail (redrawn every frame in
// the viewport) into the viewport's committed cache, one block at a time:
//
//	FlushStreamingBlocks  freezes the newly-completed, not-yet-committed slice of
//	                      the live message into a flushSnapshot.
//	renderSnapshotCmd     renders that snapshot with glamour on a background Cmd,
//	                      so the parse + syntax highlight can't stall repaint.
//	flushResultMsg        carries the rendered ANSI back to the UI goroutine.
//	handleFlushResult     appends it to the committed cache (appendBlock) and
//	                      advances the message's commit offsets, so the live
//	                      tail stops redrawing the prefix now committed and
//	                      flushes the next block.
//
// One render is in flight at a time (flush.rendering) so appends stay ordered;
// the still-streaming trailing block stays live until it too completes.

// flushSnapshot is the immutable snapshot the background render goroutine works
// from, so it never touches live model state.
type flushSnapshot struct {
	msgID            string
	index            int
	thinkingSlice    string
	contentSlice     string
	thinkingEnd      int // commit offsets, advanced once this render lands
	contentEnd       int
	showThinkingIcon bool
	showBullet       bool
	width            int
	md               *conv.MDRenderer
}

// flushState is the streaming-block flush subsystem: it renders each completed
// conversation block (thinking, content) off the UI goroutine and commits the
// result to the viewport render cache. See FlushStreamingBlocks and
// model_viewport.go.
type flushState struct {
	rendering bool             // one render in flight at a time, so appends stay ordered
	renderer  *conv.MDRenderer // background renderer, off the live-view MDRenderer's mutex
	width     int              // width the renderer was built for; rebuild when it changes
}

// flushResultMsg is the result of rendering a flushSnapshot off-thread, carrying
// the rendered blocks back for handleFlushResult to commit to scrollback.
type flushResultMsg struct {
	msgID                string
	index                int
	thinkingCommittedLen int
	contentCommittedLen  int
	thinkingEmitted      bool
	bulletEmitted        bool
	printed              string // "" when the blocks rendered empty (blank-only)
}

// FlushStreamingBlocks starts one turn of the streaming-flush pipeline above:
// it freezes the last message's newly-completed blocks into a flushSnapshot and
// kicks off their background render. Returns nil when a render is already in
// flight or nothing new is ready to flush.
func (m *model) FlushStreamingBlocks() []tea.Cmd {
	if m.flush.rendering {
		return nil // a block is already rendering off-thread; wait for it to land
	}
	idx := len(m.conv.Messages) - 1
	if idx < 0 {
		return nil
	}
	msg := &m.conv.Messages[idx]
	if msg.Role != core.RoleAssistant {
		return nil
	}

	// Once content starts, flush thinking's trailing paragraph too (it has no
	// terminating blank line, but reasoning is done).
	thinkingEnd := conv.CompletedBlockBoundary(msg.Thinking)
	if len(msg.Content) > 0 {
		thinkingEnd = len(msg.Thinking)
	}
	contentEnd := conv.CompletedBlockBoundary(msg.Content)

	var thinkingSlice, contentSlice string
	if thinkingEnd > msg.ThinkingCommittedLen {
		thinkingSlice = msg.Thinking[msg.ThinkingCommittedLen:thinkingEnd]
	}
	if contentEnd > msg.ContentCommittedLen {
		contentSlice = msg.Content[msg.ContentCommittedLen:contentEnd]
	}
	if strings.TrimSpace(thinkingSlice) == "" && strings.TrimSpace(contentSlice) == "" {
		return nil // no completed block yet (or blank-only — nothing to render)
	}

	m.flush.rendering = true
	return []tea.Cmd{renderSnapshotCmd(flushSnapshot{
		msgID:            msg.ID,
		index:            idx,
		thinkingSlice:    thinkingSlice,
		contentSlice:     contentSlice,
		thinkingEnd:      thinkingEnd,
		contentEnd:       contentEnd,
		showThinkingIcon: !msg.ThinkingEmitted,
		showBullet:       !msg.BulletEmitted,
		width:            m.env.Width,
		md:               m.flush.mdRenderer(m.env.Width),
	})}
}

// renderSnapshotCmd renders the snapshot's completed blocks (glamour, off the UI
// goroutine) and returns them as a flushResultMsg.
func renderSnapshotCmd(snap flushSnapshot) tea.Cmd {
	return func() tea.Msg {
		// != "" just skips a slice absent this flush; the render helpers
		// blank-check their input and we gate on a non-empty result.
		var blocks []string
		thinkingEmitted := false
		if snap.thinkingSlice != "" {
			if b := conv.RenderCommittedThinkingBlock(snap.thinkingSlice, snap.showThinkingIcon, snap.width, snap.md); b != "" {
				blocks = append(blocks, b)
				thinkingEmitted = true
			}
		}
		bulletEmitted := false
		if snap.contentSlice != "" {
			if b := conv.RenderCommittedContentBlock(snap.contentSlice, snap.showBullet, snap.md); b != "" {
				blocks = append(blocks, b)
				bulletEmitted = true
			}
		}
		printed := ""
		if len(blocks) > 0 {
			// Match RenderMessageAt's leading newline + blank-line separation.
			printed = "\n" + strings.Join(blocks, "\n\n")
		}
		return flushResultMsg{
			msgID:                snap.msgID,
			index:                snap.index,
			thinkingCommittedLen: snap.thinkingEnd,
			contentCommittedLen:  snap.contentEnd,
			thinkingEmitted:      thinkingEmitted,
			bulletEmitted:        bulletEmitted,
			printed:              printed,
		}
	}
}

// handleFlushResult lands a background render: advance the row's offsets,
// append the rendered block to the viewport render cache, then flush the next
// completed block.
func (m *model) handleFlushResult(msg flushResultMsg) tea.Cmd {
	m.flush.rendering = false

	// Drop the render if its row was committed whole (turn-end/cancel) or
	// replaced by a retry (new ID) meanwhile — its content is already handled, so
	// appending it now would duplicate or reorder the viewport cache.
	if msg.index >= len(m.conv.Messages) ||
		msg.index < m.conv.CommittedCount ||
		m.conv.Messages[msg.index].ID != msg.msgID ||
		m.conv.Messages[msg.index].Role != core.RoleAssistant {
		return nil
	}

	row := &m.conv.Messages[msg.index]
	row.ThinkingCommittedLen = msg.thinkingCommittedLen
	row.ContentCommittedLen = msg.contentCommittedLen
	if msg.thinkingEmitted {
		row.ThinkingEmitted = true
	}
	if msg.bulletEmitted {
		row.BulletEmitted = true
	}

	var cmds []tea.Cmd
	if msg.printed != "" {
		m.chat.appendBlock(msg.printed)
	}
	// Catch a block that completed while this one rendered — Stream.Active means
	// the row is still uncommitted, so it's safe.
	if m.conv.Stream.Active {
		cmds = append(cmds, m.FlushStreamingBlocks()...)
	}
	return tea.Batch(cmds...)
}

// mdRenderer returns the background goroutine's own markdown renderer, kept off
// m.conv.MDRenderer so a slow render can't block the live View on its mutex.
// Rebuilt on width change; needs no lock since flush.rendering means one render
// uses it at a time.
func (f *flushState) mdRenderer(width int) *conv.MDRenderer {
	if f.renderer == nil || f.width != width {
		f.renderer = conv.NewMDRenderer(width)
		f.width = width
	}
	return f.renderer
}

func (m *model) commitAllMessages() []tea.Cmd {
	return m.renderAndCommit(false)
}

func (m *model) renderAndCommit(checkReady bool) []tea.Cmd {
	var parts []string
	lastIdx := len(m.conv.Messages) - 1
	params := m.messageRenderParams()

	for i := m.conv.CommittedCount; i < len(m.conv.Messages); i++ {
		msg := m.conv.Messages[i]

		if checkReady {
			if i == lastIdx && msg.Role == core.RoleAssistant && m.conv.Stream.Active {
				break
			}
		}

		if rendered := conv.RenderSingleMessage(params, i); rendered != "" {
			parts = append(parts, rendered)
		}
		// Fully committed to the viewport cache now (any progressively-flushed
		// prefix plus this remainder). Clear the commit offsets so a later full
		// rebuild (resize reflow, compact reprint) renders the message whole,
		// not just its tail.
		m.conv.Messages[i].ResetStreamCommit()
		m.conv.CommittedCount = i + 1
	}

	if len(parts) == 0 {
		return nil
	}
	if banner := m.takeWelcomeBanner(); banner != "" {
		parts = append([]string{banner}, parts...)
	}
	// Committed messages land in the viewport render cache, not native
	// scrollback: the alt-screen chat viewport owns the whole conversation.
	m.chat.appendBlock(strings.Join(parts, "\n"))
	return nil
}

// scrollbackPhysicalLines decomposes content exactly as Bubble Tea's
// insertAbove accounts for it: ANSI escapes are zero-width, grapheme clusters
// retain their terminal width, soft wraps consume rows, and a trailing newline
// creates a final blank row.
func scrollbackPhysicalLines(content string, width int) []uv.Line {
	if content == "" {
		return nil
	}
	height := len(strings.Split(content, "\n"))
	bufferWidth := width
	if bufferWidth <= 0 {
		bufferWidth = 1
	}
	for line := range strings.SplitSeq(content, "\n") {
		lineWidth := ansi.StringWidth(line)
		if width > 0 && lineWidth > width {
			height += lineWidth / width
		}
		if width <= 0 {
			bufferWidth = max(bufferWidth, lineWidth)
		}
	}

	buffer := uv.NewScreenBuffer(bufferWidth, height)
	buffer.Method = ansi.GraphemeWidth
	styled := uv.NewStyledString(content)
	styled.Wrap = true
	styled.Draw(buffer, buffer.Bounds())
	return buffer.Lines
}

func renderScrollbackLines(lines []uv.Line) string {
	if len(lines) == 0 {
		return ""
	}
	if rendered := uv.Lines(lines).Render(); rendered != "" {
		return rendered
	}
	// insertAbove ignores an empty string. A reset sequence represents one
	// physical blank row without adding visible content.
	return ansi.ResetStyle
}

// takeWelcomeBanner freezes the startup splash into scrollback once, on the
// first commit, then clears the pending flag so the live view (liveWelcome)
// stops drawing it. Freezing it here rather than before the TUI starts lets the
// banner capture the model the user selected after launch instead of freezing
// "no model selected" into scrollback.
func (m *model) takeWelcomeBanner() string {
	if !m.welcomePending {
		return ""
	}
	m.welcomePending = false
	return m.welcomeBannerText()
}

// welcomeBannerText renders the startup splash for the current model and cwd.
// It backs both the live banner (liveWelcome) and the scrollback freeze
// (takeWelcomeBanner) so the two always read identically.
func (m model) welcomeBannerText() string {
	return welcomeBanner(welcomeInfo{
		Model: m.env.GetModelDisplayName(),
		CWD:   m.env.CWD,
	})
}
