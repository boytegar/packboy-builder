// Push-status panel helpers — a compact three-column overview (LSPs, Skills,
// MCPs) rendered above the chat area while the session is still fresh (no
// committed scrollback yet). Mirrors crush's landing view at a much smaller
// vertical footprint. Style helpers are lazy so theme switches are reflected.
package kit

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// Activity status glyphs. Reuse the saved state string from MCP/lsp where
// sensible; these are the equivalent unicode markers for the header row.
const (
	DotConnected  = "\u25cf" // ● solid dot
	DotConnecting = "\u25b9" // ▹ half-filled, faint
	DotOffline    = "\u25cb" // ○ empty ring
	DotError      = "\u2717" // ✗
)

// SectionTitleStyle renders a small bold title for one status-overview column.
func SectionTitleStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(CurrentTheme.Primary).
		Bold(true).Underline(true).UnderlineSpaces(true)
}

// StatusItemStyle is the base, uncolored row for one item inside a column.
func StatusItemStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(CurrentTheme.Text)
}

// StatusDimItemStyle renders a muted ("None"/disabled) item.
func StatusDimItemStyle() lipgloss.Style {
	return SelectorStatusNone()
}

// ClipHorizontally slices a width-rich multi-line string to a viewport of
// `width` columns, starting at column `offset`. Each line is truncated on the
// right to the viewport width; ANSI sequences are preserved line-by-line so
// colors survive. Returns an empty string if the viewport is non-positive.
func ClipHorizontally(s string, width, offset int) string {
	if width <= 0 {
		return ""
	}
	if offset < 0 {
		offset = 0
	}
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		trimmed := ansi.TruncateLeft(ln, offset, "")
		// Fill once clipped so the right edge stays aligned to the viewport;
		// trailing spaces make short values occupy their full column slot and
		// the panel box stays rectangular.
		clipped := ansi.Truncate(trimmed, width, "")
		out = append(out, clipped)
	}
	return strings.Join(out, "\n")
}

// ClipNeeded reports whether a string is wider than the given viewport.
func ClipNeeded(s string, width int) bool {
	for _, ln := range strings.Split(s, "\n") {
		if ansi.StringWidth(ln) > width {
			return true
		}
	}
	return false
}

// ContentWidth returns the display-column width of the widest line in s.
func ContentWidth(s string) int {
	max := 0
	for _, l := range strings.Split(s, "\n") {
		if w := ansi.StringWidth(l); w > max {
			max = w
		}
	}
	return max
}

// JoinColumns wraps exactly 4 pre-rendered columns side-by-side with equal
// width (25% each) and fixed height. Returns plain text without lipgloss wrapping.
// JoinColumns wraps exactly 4 pre-rendered columns side-by-side with equal
// width (25% each) and fixed height. Returns plain text without lipgloss wrapping.
func JoinColumns(columns []string, width int) string {
	// Always expect exactly 4 columns (LSPs, Skills, MCPs, Agents)
	if len(columns) != 4 {
		return ""
	}
	
	// Each column gets exactly 25% of total viewport
	colWidth := width / 4
	if colWidth < 14 {
		colWidth = 14
	}
	
	// Fixed height: title + 10 rows = 11 lines total
	const fixedHeight = 11
	
	// Process all 4 columns with fixed height padding and exact width truncation
	cols := make([][]string, 4)
	for i := 0; i < 4; i++ {
		lines := strings.Split(columns[i], "\n")
		truncated := make([]string, fixedHeight)
		
		for j := 0; j < fixedHeight; j++ {
			if j < len(lines) {
				line := lines[j]
				// Truncate if too wide (consistent for all columns)
				if ansi.StringWidth(line) > colWidth {
					line = ansi.Truncate(line, colWidth, "…")
				}
				// Pad to exact width for consistent alignment
				padLen := colWidth - ansi.StringWidth(line)
				if padLen > 0 {
					line = line + strings.Repeat(" ", padLen)
				}
				truncated[j] = line
			} else {
				// Empty line padded to exact width
				truncated[j] = strings.Repeat(" ", colWidth)
			}
		}
		cols[i] = truncated
	}
	
	// Join horizontally line by line
	result := make([]string, fixedHeight)
	for row := 0; row < fixedHeight; row++ {
		result[row] = cols[0][row] + cols[1][row] + cols[2][row] + cols[3][row]
	}
	
	return strings.Join(result, "\n")
}

// divWidth returns a sensible per-column width for a row of n columns inside
// a container of width w, leaving a narrow gap between columns.
func divWidth(w, n int) int {
	maxW := 60
	per := (w - (n - 1)) / n
	if per < maxW {
		if per < 14 {
			return 14
		}
		return per
	}
	return maxW
}

// max is a local alias to avoid depending on stdlib generics in older files.
// (unused — retained for future helpers)
