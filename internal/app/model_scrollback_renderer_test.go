package app

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
)

// The chat viewport render cache is the replacement for the native-scrollback
// pipeline: committed blocks append to the cache, and the viewport content is
// one join over cached strings. These tests exercise the cache/follow/scroll
// machinery directly, no terminal.

func newTestChatView(width, height int) *chatView {
	c := chatViewer(width, height)
	// force a clean viewport for controllable assertions
	c.buf = viewport.New(viewport.WithWidth(width), viewport.WithHeight(height))
	c.buf.SoftWrap = false
	c.buf.MouseWheelEnabled = false
	c.follow = true
	return c
}

func TestChatViewportAppendKeepsFIFOAndPinsBottom(t *testing.T) {
	c := newTestChatView(40, 6)

	c.appendBlock("block-1\n")
	c.appendBlock("block-2\n")
	c.appendBlock("block-3\n")

	// Follow mode: content grows, view jumps to the bottom each sync.
	out := renderLines(c.view("live-tail\n"))
	if !c.follow {
		t.Fatalf("follow cleared after appends")
	}
	if !strings.Contains(out, "block-1\nblock-2\nblock-3") {
		t.Fatalf("viewport content missing appended blocks:\n%q", out)
	}
	if !strings.Contains(out, "live-tail") {
		t.Fatalf("viewport content missing live tail:\n%q", out)
	}
	// With follow, the bottom of content is visible — the last block row.
	if !strings.HasSuffix(strings.TrimRight(out, "\n"), "live-tail") {
		t.Fatalf("followed view must end at the live tail, got:\n%q", out)
	}
}

func TestChatViewportScrollUpUnfollowsAndBannerShows(t *testing.T) {
	c := newTestChatView(40, 5)
	for i := 0; i < 30; i++ {
		c.appendBlock(blockLine(i))
	}
	c.view("")

	// One wheel-up scrolls back and leaves follow mode.
	if !c.onScroll(scrollStep) {
		t.Fatalf("expected first scroll to move the view")
	}
	if c.follow {
		t.Fatalf("scroll up did not clear follow mode")
	}
	// scrolledUp() drives the footer banner: the state is the signal.
	if !c.scrolledUp() {
		t.Fatalf("scrolledUp() = false, want true after scrolling up")
	}
}

func TestAppViewportFollowMsgResumesBottom(t *testing.T) {
	c := newTestChatView(20, 5)
	for i := 0; i < 30; i++ {
		c.appendBlock(blockLine(i))
	}
	c.view("")
	c.onScroll(scrollStep)

	// followMsg handler: re-pin to bottom.
	c.follow = true
	c.buf.GotoBottom()
	if c.scrolledUp() {
		t.Fatalf("refollowed chat still reports scrolledUp()")
	}
	out := renderLines(c.view(""))
	if !strings.Contains(renderLines(out), strings.TrimRight(blockLine(29), "\n")) {
		t.Fatalf("refollowed view should show the last block, got:\n%q", out)
	}
}

func TestAppViewportRebuildReflows(t *testing.T) {
	c := newTestChatView(20, 4)
	c.appendBlock("old-width-line-AAA-long-content\nsecond-line\n")
	c.view("")

	// Reflow: replace cache with re-rendered blocks at a new width.
	c.rebuildCache([]string{"new-width-short\nsecond\n"})
	out := c.view("")
	if strings.Contains(out, "old-width") {
		t.Fatalf("reflow kept stale blocks:\n%q", out)
	}
	if !strings.Contains(out, "new-width-short") {
		t.Fatalf("reflow missing new blocks:\n%q", out)
	}
}

// Regression: scrolling must not be inverted. Wheel-up (delta > 0) reveals
// OLDER content (offset decreases toward 0); wheel-down (delta < 0) reveals
// NEWER content (offset increases toward the bottom). The original bug
// applied scrollY+delta, so a wheel-down moved the conversation up instead
// of down.
func TestChatViewportScrollDirection(t *testing.T) {
	c := newTestChatView(40, 5)
	for i := 0; i < 60; i++ {
		c.appendBlock(blockLine(i))
	}
	c.view("")
	c.buf.GotoBottom()
	bottom := c.buf.YOffset()

	// First wheel-up exits follow, pinning scrollY to the bottom; a second
	// notch then reveals older content (offset toward 0).
	if !c.onScroll(scrollStep) {
		t.Fatalf("first wheel-up did not clear follow mode")
	}
	if !c.onScroll(scrollStep) {
		t.Fatalf("second wheel-up did not move the view")
	}
	up := c.buf.YOffset()
	if up >= bottom {
		t.Fatalf("wheel-up must reveal older content: offset %d -> %d",
			bottom, up)
	}
	// Wheel-down reveals newer content again (offset grows back toward bottom).
	if !c.onScroll(-scrollStep) {
		t.Fatalf("wheel-down did not move the view")
	}
	if down := c.buf.YOffset(); down <= up {
		t.Fatalf("wheel-down must reveal newer content: offset %d -> %d", up, down)
	}
}

// renderLines strips trailing whitespace per line (the viewport pads each row to
// its width), returning the trimmed visible content for assertions.
func renderLines(out string) string {
	lines := strings.Split(out, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Join(lines, "\n")
}

func blockLine(i int) string {
	return "row-" + strings.Repeat("x", i%9+1) + "\n"
}
