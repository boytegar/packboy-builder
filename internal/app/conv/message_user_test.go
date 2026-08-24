package conv

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/boytegar/packboy-builder/internal/core"
)

func TestRenderUserMessageWrapsLongTextToWidth(t *testing.T) {
	long := strings.Repeat("word ", 200) // 1000 chars, far wider than any terminal
	rendered := RenderUserMessage(long, long, nil, nil, 80)

	lines := strings.Split(strings.TrimSuffix(rendered, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected long user text to wrap onto multiple lines, got %d line(s)", len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > 80 {
			t.Fatalf("line %d width %d exceeds terminal width 80: %q", i, w, line)
		}
	}
	// Content must survive wrapping (prompt stripped from each line).
	joined := strings.Join(lines, "")
	if !strings.Contains(joined, "word") {
		t.Fatalf("wrapped output lost content: %q", rendered)
	}
}

func TestRenderUserMessagePreservesInlineImagePosition(t *testing.T) {
	rendered := RenderUserMessage(
		"这个图片说了什么 请说一下",
		"[Image #1] 这个图片说了什么 请说一下",
		[]core.Image{{FileName: "clip.png"}},
		nil,
		80,
	)

	imageIdx := strings.Index(rendered, "[Image #1]")
	textIdx := strings.Index(rendered, "这个图片说了什么")
	if imageIdx < 0 || textIdx < 0 {
		t.Fatalf("expected inline image token and text in rendered output: %q", rendered)
	}
	if imageIdx > textIdx {
		t.Fatalf("expected image token to remain before text, got %q", rendered)
	}
}
