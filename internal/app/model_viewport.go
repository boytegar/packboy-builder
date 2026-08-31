// Viewport chat rendering state: the conversation render cache and the
// follow/scroll machinery that drive the alt-screen chat viewport.
//
// The terminal now runs in full-window (alt-screen) mode (see view.go).
// Every message — committed and live — renders into one in-app viewport
// (charm.land/bubbles/v2/viewport) that fills the window above the input.
// This file owns the render cache (renderedBlocks), the follow state,
// the user-scroll offset, and the small message protocol (scrollMsg /
// followMsg) that routes wheel/PageUp/PageDown into the model's single
// Update loop, avoiding cross-thread state mutation.
//
// The previous scrollback architecture (tea.Println + insertAbove +
// pendingPrints FIFO) is gone; the text that formerly moved into native
// scrollback now appends to renderedBlocks and the viewport re-slices on
// every frame. Nothing in this file talks to the terminal.
package app

import (
	"strings"

	"charm.land/bubbles/v2/viewport"

	"github.com/boytegar/packboy-builder/internal/app/kit"
)

// scrollStep is the number of viewport rows one wheel notch or PgUp/PgDn
// scrolls. A throwback to terminal paging: roughly a third of a typical
// 40-row screen.
const scrollStep = 12

// chatMaxLinesDefault caps the rendered conversation scrollback kept in the
// chat viewport, in physical terminal lines (at the current width). Once the
// committed blocks plus the live tail exceed it, the oldest lines are dropped
// from the viewport so a long session doesn't grow it without bound — keeping
// the scrollbar thumb usable and scroll length short. Dropped lines are not
// recoverable in the viewport; the full transcript still lives in session
// persistence.
const chatMaxLinesDefault = 500

// scrollMsg is sent by the OnMouse wheel handler (which runs on the event
// loop) into the regular Update loop. The handler never mutates model
// state; it only packages the wheel delta. Update applies the delta to
// the model-owned scroll offset and unfollows the chat.
type scrollMsg struct{ delta int }

// followMsg forces the chat back into follow mode (scroll to bottom).
type followMsg struct{}

// scrollbarAction is the logical scroll a mouse press/drag on the thin right
// scrollbar gutter requests. The OnMouse handler packages one per press/drag
// event; Update resolves it into a scrollY jump and (dis)engages drag state.
// Enumerating logical actions (rather than raw columns) keeps the event loop
// free of viewport math — it never reads scroll state off the event thread.
type scrollbarAction int

const (
	scrollbarTop        scrollbarAction = iota // click above the thumb (page up)
	scrollbarBottom                            // click below the thumb (page down)
	scrollbarThumbStart                        // press on the thumb: begin thumb-drag
	scrollbarThumbDrag                         // thumb-drag: jump to the drag's Y ratio
	scrollbarThumbEnd                          // release the thumb: end the drag
)

// scrollbarJumpMsg is sent by the OnMouse press/drag handler for a pointer
// event on the scrollbar gutter; it carries a logical action plus (for a drag)
// the pointer row so Update can map it to a scroll position.
type scrollbarJumpMsg struct {
	action scrollbarAction
	row    int // viewport-relative pointer row for thumb drags
}

// chatView is the model-owned render cache + scroll controller behind the
// chat viewport. It is a pointer on the model (like flushState) because
// the model is copied by value on every Update; the pointer is the only
// thing that survives.
type chatView struct {
	buf viewport.Model
	// renderedBlocks caches one rendered block (ANSI + trailing newline)
	// per conversation row index, so rendering is incremental: only the
	// live tail re-renders per frame, and the viewport content is one
	// join over cached strings — committed rows are never re-parsed.
	renderedBlocks []string
	// dirty marks renderedBlocks appended since the last SetContent.
	dirty bool
	// sizeDirty is set when the terminal size changed and the viewport
	// content must be re-flowed at the new width.
	sizeDirty bool

	// follow pins the view to the bottom. True by default. Any upward
	// user scroll (wheel or PgUp) clears it and the “▼ Scroll to bottom
	// (End)” banner appears; End or followMsg re-pins it.
	follow bool
	// scrollY is the viewport Y offset while !follow, owned by the model
	// so the wheel handler can nudge it without touching viewport state
	// from the event loop, and so Update can react to content growth
	// without fighting the viewport.
	scrollY int
	// lastLive is the live-tail string rendered into the viewport last frame.
	// Following, the live tail changes every frame (spinners, streaming,
	// tracker) without setting dirty, so this fingerprint tells ensureSynced a
	// content change occurred without a full SetContent re-parse when nothing
	// visible changed. The offset must re-pin to the true bottom whenever the
	// tail grows — otherwise a wheel-up after arriving at the bottom captures a
	// stale yOffset and starts scrolling mid-screen.
	lastLive string
	// height is the chat pane height in rows; set on resize and used by
	// the banner logic (whether it has room to overlap content).
	height int
	// maxLines caps the rendered scrollback in physical lines (see
	// chatMaxLinesDefault). 0 disables the cap.
	maxLines int
	// dragRow is the viewport-relative pointer row captured when a thumb-drag
	// starts, so each drag event maps pointer row → scroll position against a
	// stable thumb anchor. Set only while a drag is in progress.
	dragRow int
}

// chatViewer wires the initial viewport with auto-follow on.
func chatViewer(width, height int) *chatView {
	vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))
	vp.SoftWrap = false          // blocks are already wrapped to Width at render time
	vp.MouseWheelEnabled = false // wheel flows through scrollMsg into Update
	// The right scrollbar is drawn as an overlay on the last column by
	// renderScrollbarColumn() so the content width stays full-width while the
	// bar is visible. No left/right gutter is added here — adding one would
	// consume a column of content width forever.
	return &chatView{
		buf:       vp,
		follow:    true,
		height:    height,
		maxLines:  chatMaxLinesDefault,
		dragRow:   -1,
		sizeDirty: true, // first render must flush the live tail into the viewport
	}
}

// syncSizeIfNeeded re-sizes the viewport when the terminal size changed. The
// content re-flow (SetContent at the new width) happens on the next frame in
// ensureSynced, so the live tail (re-rendered per frame) is included too.
// Cheap when nothing changed: early return.
func (c *chatView) syncSizeIfNeeded(width, height int) {
	if c == nil {
		return
	}
	if c.buf.Width() == width && c.buf.Height() == height && !c.sizeDirty {
		return
	}
	c.height = height
	c.buf.SetWidth(width)
	c.buf.SetHeight(height)
	c.sizeDirty = true
}

// view returns the current viewport slice plus a right scrollbar column (one
// row per visible line), so the caller's chat pane owns the full terminal width
// — content fills chatBodyWidth and the bar sits in the reserved last column.
// view returns the conversation body only — the scrollbar is rendered as a
// separate sibling column by the view layer (see scrollbarWidget), so it is
// never part of the copyable chat text.
func (c *chatView) view(live string) string {
	if c == nil {
		return live
	}
	c.ensureSynced(live)
	return c.buf.View()
}

// scrollbarGeometry reports the thumb row, the track height, and whether a
// scrollbar is needed at all (content taller than the viewport).
func (c *chatView) scrollbarGeometry() (thumbRow, trackHeight int, needed bool) {
	if c == nil || c.buf.Height() <= 0 {
		return -1, 0, false
	}
	trackHeight = c.buf.Height()
	thumbRow, needed = c.scrollThumbPosition()
	return thumbRow, trackHeight, needed
}

// scrollbarWidget renders a 1-column scrollbar with a draggable thumb.
// It is a standalone widget (not appended to chat text) so selecting chat
// content never copies scrollbar glyphs.
//
// Only the thumb is drawn (the track is left blank) so that shift-click
// full-width selection copies trailing spaces instead of a `│` glyph on every
// line — the only character that can still ride along is the single thumb row.
//
// The thumb uses kit.FocusBar (the brand accent, "▎") to mark "you are here",
// consistent with the TUI selection affordance per the design guide.
func (c *chatView) scrollbarWidget() string {
	if c == nil {
		return ""
	}
	thumbRow, trackHeight, needed := c.scrollbarGeometry()
	if !needed || trackHeight <= 0 {
		return ""
	}
	// Build the widget with the same height as the chat viewport so it stays
	// vertically aligned row-for-row. Non-thumb rows are spaces (copy-clean),
	// the thumb row is the focus glyph.
	focus := kit.FocusBarStyle()
	rows := make([]string, trackHeight)
	for row := 0; row < trackHeight; row++ {
		if row == thumbRow {
			rows[row] = focus.Render(kit.FocusBar)
		} else {
			rows[row] = " "
		}
	}
	return strings.Join(rows, "\n")
}

// scrolledUp reports whether the chat is currently scrolled back from the
// bottom (not following). renderFooter uses it to draw the "▼ Scroll to
// bottom (End)" banner on its own row above the separator.
func (c *chatView) scrolledUp() bool {
	return c != nil && !c.follow
}

// syncSize pushes the current terminal size into the viewport and recomputes
// the scroll bounds. It never resets follow or the user's scroll position.
func (c *chatView) fullContent(live string) string {
	var sb strings.Builder
	sb.Grow(len(c.renderedBlocks)*24 + len(live))
	for _, block := range c.renderedBlocks {
		sb.WriteString(block)
	}
	sb.WriteString(live)
	return sb.String()
}

// appendBlock adds one completed rendered block to the cache and marks the
// cache dirty so the next frame resyncs the viewport. Used by the commit
// pipeline (renderSnapshotResult / commit all) instead of tea.Println.
func (c *chatView) appendBlock(block string) {
	if c == nil || block == "" {
		return
	}
	c.renderedBlocks = append(c.renderedBlocks, block)
	c.dirty = true
}

// ensureSynced pushes the cached content into the viewport if the block cache
// changed since the last push. Follow mode re-pins to bottom so streaming
// commits scroll the view forward; otherwise the user's offset is preserved.
//
// The live tail (spinners, streaming chunks, tracker) grows every frame without
// setting dirty, so a pure "dirty or return" check leaves yOffset frozen at a
// half-grown bottom. While following, any live-tail change is detected by
// fingerprint (lastLive != current) and triggers just a re-pin to the true
// bottom + a SetContent — that is the fix for "at the bottom yet wheel-up
// starts from mid-screen". SetContent is skipped entirely when nothing visible
// changed (static frame). Away from follow the user's offset is preserved.
func (c *chatView) ensureSynced(live string) {
	if c == nil {
		return
	}
	if c.follow && !c.dirty && !c.sizeDirty {
		// Live tail changed without a dirty commit (spinners / streaming /
		// tracker). Re-pin to the true bottom so a wheel-up that exits follow
		// anchors to the real content height, not a stale yOffset.
		if live != c.lastLive {
			c.lastLive = live
			c.buf.SetContent(c.truncate(c.fullContent(live)))
			c.buf.GotoBottom()
			c.scrollY = c.buf.YOffset()
		}
		return
	}
	if !c.dirty && !c.sizeDirty {
		return
	}
	c.dirty = false
	c.sizeDirty = false
	c.lastLive = live
	c.buf.SetContent(c.truncate(c.fullContent(live)))
	if c.follow {
		c.buf.GotoBottom()
		c.scrollY = c.buf.YOffset()
	} else {
		c.buf.SetYOffset(c.scrollY)
		c.scrollY = c.buf.YOffset() // read back the clamp
		// Guard: the truncation may have swept the user's scroll position away.
		// Clamp back into range so the next wheel notch doesn't jump.
		if c.scrollY > c.buf.TotalLineCount()-c.buf.Height() {
			c.scrollY = c.buf.TotalLineCount() - c.buf.Height()
			c.buf.SetYOffset(c.scrollY)
		}
	}
}

// truncate drops the oldest lines of s so it stays within the maxLines budget
// (committed blocks plus the live tail), preserving the newest content. A no-
// op at the heightless cache size.
func (c *chatView) truncate(s string) string {
	if c == nil || c.maxLines <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	if len(lines) <= c.maxLines {
		return s
	}
	// Cut at a block boundary — don't slice a message mid-line.
	// Blocks end with "\n\n"; dropping oldest full blocks is safest.
	// Find the first good cut: skip past the excess lines plus any line that
	// continues a block (heuristic: cut only at a line that starts a new
	// block, i.e. following an empty line). If no clean boundary, cut raw.
	excess := len(lines) - c.maxLines
	cut := excess
	for i := excess; i < len(lines); i++ {
		if lines[i] == "" {
			// Prefer cutting right after a blank line boundary.
			cut = i + 1
			break
		}
	}
	if cut <= 0 || cut >= len(lines) {
		cut = excess
	}
	return strings.Join(lines[cut:], "\n")
}

// rebuildCache replaces the committed-block cache wholesale (used by the
// resize reflow, where every committed message re-renders at the new width)
// and marks the viewport dirty for the next frame.
func (c *chatView) rebuildCache(blocks []string) {
	if c == nil {
		return
	}
	c.renderedBlocks = blocks
	c.dirty = true
}

// resetCache drops every cached committed block and the follow position so the
// next frame renders only the live tail (used by /new: the conversation is
// cleared but the committed-block cache would otherwise keep showing stale
// content, and the old viewport frame could compare equal and be skipped by
// the renderer — leaving a blank screen).
func (c *chatView) resetCache() {
	if c == nil {
		return
	}
	c.renderedBlocks = nil
	c.dirty = true
	c.follow = true
	c.scrollY = 0
}

// onScroll applies a user scroll delta. Returns true if the view actually
// moved (so Update knows the frame is stale and must repaint).
func (c *chatView) onScroll(delta int) bool {
	if c == nil {
		return false
	}
	// First wheel-up while following: capture the true bottom as scrollY and
	// exit follow mode WITHOUT scrolling. This anchors the unfollow offset to
	// the real content bottom so a subsequent wheel-up starts from the actual
	// last line instead of a stale yOffset (fixes "at bottom yet scroll starts
	// mid-screen"). Only a second wheel-up actually moves the view.
	if delta > 0 && c.follow {
		c.follow = false
		c.scrollY = c.buf.YOffset()
		return true
	}
	// Apply delta and clamp.
	c.buf.SetYOffset(c.scrollY - delta)
	ny := c.buf.YOffset()
	if ny == c.scrollY {
		// No movement. If wheeled down to true bottom, re-enable follow.
		if delta < 0 && c.buf.AtBottom() {
			c.follow = true
			return true
		}
		return false
	}
	c.scrollY = ny
	return true
}

// OnScrollbar handles a press/drag/release event on the right scrollbar
// gutter, packaging logical actions into a scrollY jump. Called from Update
// so the viewport math stays off the event loop; c may be nil (no chat yet).
// Returns true when the view moved (frame stale, repaint needed).
func (c *chatView) onScrollbar(msg scrollbarJumpMsg) bool {
	if c == nil {
		return false
	}
	// Clamp the pointer row into the viewport so a release beyond the chat
	// region (drag escaping the bar) maps to a sensible scroll target.
	row := msg.row
	if h := c.buf.Height(); h > 1 {
		if row < 0 {
			row = 0
		} else if row >= h {
			row = h - 1
		}
	}
	switch msg.action {
	case scrollbarTop, scrollbarBottom:
		c.setJump(msg.action == scrollbarTop, c.buf.Height())
		return true
	case scrollbarThumbStart:
		thumb, _ := c.scrollThumbPosition()
		// Press on the thumb begins a drag; press elsewhere on the track pages
		// one viewport in that direction.
		if row == thumb {
			c.dragRow = msg.row
			return false
		}
		if row < thumb {
			c.setJump(true, c.buf.Height())
		} else {
			c.setJump(false, c.buf.Height())
		}
		return true
	case scrollbarThumbDrag:
		// Motion while the button is held: map the row's fraction of the track
		// to a fraction of the scrollable content. Keep dragRow armed so the
		// next motion event continues the drag; scrollbarThumbEnd clears it.
		if c.dragRow < 0 {
			return false
		}
		if c.buf.TotalLineCount() <= c.buf.Height() {
			return false
		}
		maxY := c.buf.TotalLineCount() - c.buf.Height()
		target := int(float64(row) / float64(max(1, c.buf.Height()-1)) * float64(maxY))
		c.setScrollY(target)
		return true
	case scrollbarThumbEnd:
		c.dragRow = -1
		return false
	}
	return false
}

// jumpTo moves the scroll to the specified absolute position, or to the top /
// bottom, engaging / disengaging follow mode accordingly. It mirrors the
// existing wheel / key handling (which uses setScrollY). This is the one place
// that resets both follow and scrollY, so the scrollbar and wheel/keyboard stay
// consistent.
func (c *chatView) setScrollY(ny int) {
	if c == nil {
		return
	}
	c.buf.SetYOffset(ny)
	ny = c.buf.YOffset()
	c.scrollY = ny
	if c.scrollY >= c.buf.TotalLineCount()-c.buf.Height() {
		c.follow = true
		c.buf.GotoBottom()
	} else {
		c.follow = false
	}
}

// setJump adjusts the current scroll position by delta rows (signed), exiting
// follow mode when scrolling up/back and snapping back to the bottom at the end.
// used by track clicks above/below the thumb.
func (c *chatView) setJump(up bool, delta int) {
	if up {
		if c.follow {
			c.follow = false
			c.scrollY = c.buf.YOffset()
		}
		c.buf.SetYOffset(c.scrollY - delta)
		c.scrollY = c.buf.YOffset()
		if c.scrollY <= 0 {
			c.follow = true
		}
	} else {
		c.buf.SetYOffset(c.scrollY + delta)
		c.scrollY = c.buf.YOffset()
		if c.scrollY >= c.buf.TotalLineCount()-c.buf.Height() {
			c.follow = true
			c.buf.GotoBottom()
		}
	}
	if c.scrollY > c.buf.TotalLineCount()-c.buf.Height() {
		c.scrollY = c.buf.TotalLineCount() - c.buf.Height()
		c.buf.SetYOffset(c.scrollY)
	}
}

// scrollThumbPosition returns the viewport-relative row where the scrollbar
// thumb should sit, and whether a thumb is needed (content taller than the
// viewport). Returns -1,false when nothing is scrollable.
func (c *chatView) scrollThumbPosition() (row int, needed bool) {
	if c == nil || c.buf.Height() <= 0 {
		return -1, false
	}
	total := c.buf.TotalLineCount()
	height := c.buf.Height()
	if total <= height {
		return -1, false
	}
	fraction := float64(c.buf.YOffset()) / float64(total-height)
	row = int(fraction * float64(height-1))
	if row < 0 {
		row = 0
	}
	if row > height-1 {
		row = height - 1
	}
	return row, true
}
