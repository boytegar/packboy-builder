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

// Regression: the reported bug — the UI sits flush at the bottom, but the first
// wheel-up starts the scroll mid-screen. Root cause: while following, the live
// tail grows every frame (spinners / streaming / tracker) without setting dirty,
// so the viewport never re-pinned to the true bottom. yOffset stayed frozen at a
// half-grown value; the wheel-up that exits follow then captured that stale
// offset as the scroll start. ensureSynced now detects the content change by
// fingerprint while following and re-pins to the real bottom first.
func TestChatViewportScrollFromBottomAfterLiveGrowth(t *testing.T) {
	c := newTestChatView(40, 5)
	for i := 0; i < 20; i++ {
		c.appendBlock(blockLine(i))
	}
	c.view("live-head\n")
	if !c.follow {
		t.Fatalf("expected follow mode after initial sync")
	}

	// Streaming: live tail grows frame-over-frame without any dirty commit
	// (nothing appended to the block cache). The old code left yOffset frozen.
	grown := ""
	for i := 0; i < 60; i++ {
		grown += "stream-line\n"
	}
	c.view(grown)

	// While following, yOffset must have tracked the grown content's bottom, so
	// the wheel-up that exits follow anchors to the true bottom, not mid-pane.
	trueBottom := c.buf.YOffset()
	if c.follow {
		if trueBottom == 0 {
			t.Fatalf("setup: grown live did not bind to bottom")
		}
	} else {
		t.Fatalf("growth while following must not leave follow mode")
	}
	if !c.onScroll(scrollStep) {
		t.Fatalf("first wheel-up did not clear follow mode")
	}
	if got, want := c.scrollY, trueBottom; got != want {
		t.Fatalf("scrollY after exiting follow = %d, want the true bottom %d - "+
			"a stale yOffset makes the scroll start mid-screen", got, want)
	}
	// One more notch must reveal older content (offset decreases).
	before := c.buf.YOffset()
	if !c.onScroll(scrollStep) {
		t.Fatalf("second wheel-up did not move the view")
	}
	if after := c.buf.YOffset(); after >= before {
		t.Fatalf("wheel-up should reveal older content, got %d -> %d", before, after)
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
