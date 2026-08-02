package input

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/boytegar/packboy-builder/internal/app/kit"
)

func pendingImageStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(kit.CurrentTheme.Primary)
}

func selectedImageStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(kit.CurrentTheme.TextBright).
		Background(kit.CurrentTheme.Primary).
		Bold(true)
}

// RenderTextarea renders the textarea with styled inline image tokens.
// The textarea's internal renderer right-pads every line to its content width
// with spaces. strings.TrimRight on the whole view only cleans the final line,
// so wrapped lines (from resize or paste-induced wrapping) keep their trailing
// padding and show as extra spaces. trimTrailingSpacesPerLine cleans each line
// individually so only the user's actual text remains visible.
func (m *Model) RenderTextarea() string {
	view := trimTrailingSpacesPerLine(m.Textarea.View())
	if len(m.Images.Pending) == 0 {
		return view
	}

	selectedPendingIdx := -1
	if match, ok := m.SelectedImageMatch(); ok {
		selectedPendingIdx = match.PendingIdx
	}

	for _, match := range m.PendingImageMatches() {
		style := pendingImageStyle()
		if match.PendingIdx == selectedPendingIdx {
			style = selectedImageStyle()
		}
		view = strings.Replace(view, match.Label, style.Render(match.Label), 1)
	}

	return view
}

// trimTrailingSpacesPerLine removes trailing spaces from each line of the
// textarea view. The textarea pads every row to its content width; without
// per-line trimming, wrapped rows display those padding spaces, which become
// visible extra whitespace when the window is resized or when pasted text
// causes a line to wrap.
func trimTrailingSpacesPerLine(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.Join(lines, "\n")
}
