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
)

// scrollStep is the number of viewport rows one wheel notch or PgUp/PgDn
// scrolls. A throwback to terminal paging: roughly a third of a typical
// 40-row screen.
const scrollStep = 12

// scrollMsg is sent by the OnMouse wheel handler (which runs on the event
// loop) into the regular Update loop. The handler never mutates model
// state; it only packages the wheel delta. Update applies the delta to
// the model-owned scroll offset and unfollows the chat.
type scrollMsg struct{ delta int }

// followMsg forces the chat back into follow mode (scroll to bottom).
type followMsg struct{}

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
	// height is the chat pane height in rows; set on resize and used by
	// the banner logic (whether it has room to overlap content).
	height int
}

// chatViewer wires the initial viewport with auto-follow on.
func chatViewer(width, height int) *chatView {
	vp := viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))
	vp.SoftWrap = false          // blocks are already wrapped to Width at render time
	vp.MouseWheelEnabled = false // wheel flows through scrollMsg into Update
	return &chatView{
		buf:    vp,
		follow: true,
		height: height,
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

// view returns the current viewport slice (how many rows of chat are visible
// this frame) and keeps the viewport render-fresh for the live tail. It is
// the Render path used by renderNormalView.
func (c *chatView) view(live string) string {
	if c == nil {
		return live
	}
	c.ensureSynced(live)
	return c.buf.View()
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
func (c *chatView) ensureSynced(live string) {
	if c == nil {
		return
	}
	if !c.dirty && !c.sizeDirty {
		return
	}
	c.dirty = false
	c.sizeDirty = false
	c.buf.SetContent(c.fullContent(live))
	if c.follow {
		c.buf.GotoBottom()
	} else {
		c.buf.SetYOffset(c.scrollY)
		c.scrollY = c.buf.YOffset() // read back the clamp
	}
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

// onScroll applies a user scroll delta. Returns true if the view actually
// moved (so Update knows the frame is stale and must repaint).
func (c *chatView) onScroll(delta int) bool {
	if c == nil {
		return false
	}
	if delta > 0 && c.follow {
		// First upward scroll: exit follow mode, remembering where the bottom
		// was so the next wheel notch scrolls back from here.
		c.follow = false
		c.scrollY = c.buf.YOffset()
		return true
	}
	// Clamp to the viewport's own bounds by letting SetYOffset do the math,
	// then read the actual offset back so scrollY agrees with the viewport.
	c.buf.SetYOffset(c.scrollY + delta)
	ny := c.buf.YOffset()
	if ny == c.scrollY {
		// No movement — already at a bound. If the user wheeled down to the
		// very bottom, snap back into follow mode so new content scrolls in.
		if delta < 0 && c.buf.AtBottom() {
			c.follow = true
			return true
		}
		return false
	}
	c.scrollY = ny
	return true
}
