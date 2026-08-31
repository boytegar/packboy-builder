package app

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/viewport"
	"github.com/charmbracelet/x/ansi"

	"github.com/boytegar/packboy-builder/internal/app/kit"
)

// TestChatViewBodyHasNoScrollbarGlyphs ensures the conversation body returned
// by chatView.view is copy-clean: the scrollbar (│/▎) lives in its own widget,
// never appended to chat text. Selecting/copying chat content must not grab
// scrollbar characters.
func TestChatViewBodyHasNoScrollbarGlyphs(t *testing.T) {
	c := newTestChatView(20, 5)
	for i := 0; i < 60; i++ {
		c.appendBlock(blockLine(i))
	}
	// Force enough content to scroll so the bar would be needed.
	c.view("")
	if _, _, needed := c.scrollbarGeometry(); !needed {
		t.Fatalf("expected the scrollbar to be needed for overflowing content")
	}

	body := c.view("")
	if strings.Contains(body, "│") || strings.Contains(body, "▎") || strings.Contains(body, "▊") {
		t.Fatalf("chat body must be copy-clean but contains scrollbar glyphs:\n%q", body)
	}
}

// TestScrollbarWidgetContainsSingleThumb verifies the standalone scrollbar
// widget renders exactly one thumb marker (FocusBar accent) positioned at the
// row reported by scrollThumbPosition, plus muted track cells for the rest.
func TestScrollbarWidgetContainsSingleThumb(t *testing.T) {
	c := newTestChatView(20, 6)
	for i := 0; i < 60; i++ {
		c.appendBlock(blockLine(i))
	}
	c.view("")

	thumbRow, trackHeight, needed := c.scrollbarGeometry()
	if !needed {
		t.Fatalf("expected scrollbar to be needed for overflowing content")
	}
	if trackHeight != c.buf.Height() {
		t.Fatalf("track height = %d, want viewport height %d", trackHeight, c.buf.Height())
	}

	bar := c.scrollbarWidget()
	if bar == "" {
		t.Fatal("scrollbarWidget returned empty for scrollable content")
	}

	// The thumb glyph is kit.FocusBar ("▎"); the track glyph is "│".
	plain := ansi.Strip(bar)
	thumb := ansi.Strip(kit.FocusBarStyle().Render(kit.FocusBar))
	if got := strings.Count(plain, thumb); got != 1 {
		t.Fatalf("thumb count = %d, want 1 (one thumb at the scroll position)", got)
	}

	barLines := strings.Split(plain, "\n")
	if len(barLines) != trackHeight {
		t.Fatalf("scrollbar widget line count = %d, want track height %d", len(barLines), trackHeight)
	}
	if barLines[thumbRow] != thumb {
		t.Fatalf("thumb at row %d = %q, want %q", thumbRow, barLines[thumbRow], thumb)
	}
	for i, ln := range barLines {
		if i == thumbRow {
			continue
		}
		// Track rows are blank (copy-clean): shift-click full-width selection
		// copies a space here, not a │ glyph.
		if ln != " " {
			t.Fatalf("track row %d = %q, want a space (no track glyph)", i, ln)
		}
	}
}

// TestScrollbarWidgetEmptyWhenContentFits verifies no scrollbar is rendered when
// the content fits within the viewport (nothing to scroll).
func TestScrollbarWidgetEmptyWhenContentFits(t *testing.T) {
	c := newTestChatView(20, 10)
	c.appendBlock("only a few lines\nfit here\n")
	c.view("")

	if _, _, needed := c.scrollbarGeometry(); needed {
		t.Fatal("scrollbar should not be needed when content fits")
	}
	if bar := c.scrollbarWidget(); bar != "" {
		t.Fatalf("scrollbarWidget must be empty when nothing to scroll, got:\n%q", bar)
	}
}

// TestScrollbarWidgetTracksScrollPosition verifies the thumb row moves as the
// viewport scrolls, so the indicator reflects the current position.
//
// Note: the first wheel-up from the bottom only exits follow mode (it captures
// the current offset without moving) — this is the established viewport
// contract (see TestChatViewportScrollFromBottomAfterLiveGrowth). A second
// wheel-up is required to actually reveal older content and move the thumb.
func TestScrollbarWidgetTracksScrollPosition(t *testing.T) {
	c := newTestChatView(20, 5)
	for i := 0; i < 60; i++ {
		c.appendBlock(blockLine(i))
	}
	c.view("")

	topThumb, _, _ := c.scrollbarGeometry()
	// First wheel-up exits follow mode without moving.
	if !c.onScroll(scrollStep) {
		t.Fatal("first wheel-up did not exit follow mode")
	}
	// Second wheel-up actually scrolls toward older content.
	if !c.onScroll(scrollStep) {
		t.Fatal("second wheel-up did not move the view")
	}
	upThumb, _, _ := c.scrollbarGeometry()
	if upThumb >= topThumb {
		t.Fatalf("after scrolling up, thumb row %d must be < initial %d", upThumb, topThumb)
	}

	// Scroll back down toward the bottom; thumb should move toward the bottom.
	for i := 0; i < 8; i++ {
		c.onScroll(-scrollStep)
	}
	bottomThumb, _, _ := c.scrollbarGeometry()
	if bottomThumb <= upThumb {
		t.Fatalf("after scrolling to bottom, thumb row %d must be > scrolled-up %d", bottomThumb, upThumb)
	}
}

// Regression: the viewport model used to embed the bar into chat text. This test
// pins the new contract by exercising the exact rendering path used at runtime
// (chatViewer with the real bubbles viewport, no manual buf swap) to make sure
// no scrollbar glyphs leak into the body even after the sync path runs.
func TestChatViewRealViewportBodyStaysCopyClean(t *testing.T) {
	c := chatViewer(20, 5)
	c.buf = viewport.New(viewport.WithWidth(20), viewport.WithHeight(5))
	c.buf.SoftWrap = false
	c.buf.MouseWheelEnabled = false
	c.follow = true

	for i := 0; i < 60; i++ {
		c.appendBlock(blockLine(i))
	}
	body := c.view("live-tail\n")
	if strings.ContainsAny(body, "│▎▊") {
		t.Fatalf("body leaked scrollbar glyphs through the real viewport path:\n%q", body)
	}
}
