// Startup status overview — a compact, live three-column panel showing the
// active LSP servers, skills, and MCP servers at the top of the window while
// the session is still fresh (before the first committed message). Mirrors
// crush's landing view (LSPs | Skills | MCPs) at terminal-width scale. Agents
// are deliberately omitted; only LSPs, Skills, and MCPs are surfaced here. It
// is rendered from live service state every frame and disappears once content
// is committed to scrollback (see renderChatSection in view.go).
package app

import (
	"slices"
	"strings"

	"github.com/boytegar/packboy-builder/internal/app/kit"
	"github.com/boytegar/packboy-builder/internal/lsp"
	"github.com/boytegar/packboy-builder/internal/mcp"
	"github.com/charmbracelet/x/ansi"
)

// maxStatusRows limits the number of items shown per column (excluding title)
// to prevent vertical overflow when there are many LSPs/MCPs/Skills.
const maxStatusRows = 10

// renderStatusOverview assembles the LSPs · Skills · MCPs columns into one
// bordered row. Returns "" when zero configured items exist across all three
// so a fresh blank session stays clean.
func (m model) renderStatusOverview() string {
	viewport := maxInt(m.env.Width-4, 1)
	colWidth := viewport / 3
	if colWidth < 14 {
		colWidth = 14
	}

	lspCol := m.lspStatusColumn(colWidth)
	skillCol := m.skillStatusColumn(colWidth)
	mcpCol := m.mcpStatusColumn(colWidth)

	if lspCol == "" && skillCol == "" && mcpCol == "" {
		return ""
	}

	// Join columns with equal width (⅓ each of viewport).
	content := kit.JoinColumns([]string{lspCol, skillCol, mcpCol}, viewport)
	inner := strings.Join(filterBlank([]string{content}), "")
	if inner == "" {
		return ""
	}

	// No horizontal scroll needed since columns fit exactly in viewport.
	scrollX := 0
	inner = kit.ClipHorizontally(inner, viewport, scrollX)
	if strings.TrimSpace(inner) == "" {
		return ""
	}

	return kit.SelectorBorderStyle().
		Width(maxInt(viewport, 20)).
		Render(inner)
}

// lspStatusColumn renders the configured/active LSP servers. Each row carries
// a ready/connecting/offline dot plus the server name.
// Limited to maxStatusRows items to prevent vertical overflow.
// Each line is truncated to maxWidth to prevent wrapping.
func (m model) lspStatusColumn(maxWidth int) string {
	if m.services.LSP == nil {
		return ""
	}
	statuses := m.services.LSP.Manager().Status()
	if len(statuses) == 0 {
		return ""
	}

	title := "LSPs"
	rows := make([]string, 0, len(statuses))
	slices.SortFunc(statuses, func(a, b lsp.ServerStatus) int {
		return strings.Compare(a.Name, b.Name)
	})
	for i, st := range statuses {
		if i >= maxStatusRows {
			break
		}
		// Strip newlines and truncate to fit column width
		name := strings.ReplaceAll(st.Name, "\n", " ")
		name = strings.Join(strings.Fields(name), " ")
		dot := "○"
		if st.Ready {
			dot = "●"
		}
		line := dot + " " + name
		if ansi.StringWidth(line) > maxWidth {
			line = ansi.Truncate(line, maxWidth, "…")
		}
		rows = append(rows, line)
	}
	return strings.Join(append([]string{title}, rows...), "\n")
}

// skillStatusColumn renders the active skills as one item per row.
// Limited to maxStatusRows items to prevent vertical overflow.
// Each line is truncated to maxWidth to prevent wrapping.
func (m model) skillStatusColumn(maxWidth int) string {
	if m.services.Skill == nil {
		return ""
	}
	skills := m.services.Skill.GetActive()
	if len(skills) == 0 {
		return ""
	}

	title := "Skills"
	rows := make([]string, 0, len(skills))
	for i, s := range skills {
		if i >= maxStatusRows {
			break
		}
		// Strip newlines and truncate to fit column width
		name := strings.ReplaceAll(s.FullName(), "\n", " ")
		name = strings.Join(strings.Fields(name), " ")
		line := "● " + name
		if ansi.StringWidth(line) > maxWidth {
			line = ansi.Truncate(line, maxWidth, "…")
		}
		rows = append(rows, line)
	}
	return strings.Join(append([]string{title}, rows...), "\n")
}

// mcpStatusColumn renders each configured MCP server with its live status.
// Limited to maxStatusRows items to prevent vertical overflow.
// Each line is truncated to maxWidth to prevent wrapping.
func (m model) mcpStatusColumn(maxWidth int) string {
	if m.services.MCP == nil {
		return ""
	}
	servers := m.services.MCP.List()
	if len(servers) == 0 {
		return ""
	}

	title := "MCPs"
	rows := make([]string, 0, len(servers))
	for i, srv := range servers {
		if i >= maxStatusRows {
			break
		}
		// Strip newlines and truncate to fit column width
		name := strings.ReplaceAll(srv.Config.Name, "\n", " ")
		name = strings.Join(strings.Fields(name), " ")
		dot := "○"
		switch srv.Status {
		case mcp.StatusConnected:
			dot = "●"
		case mcp.StatusConnecting:
			dot = "▹"
		case mcp.StatusError:
			dot = "✗"
		}
		line := dot + " " + name
		if ansi.StringWidth(line) > maxWidth {
			line = ansi.Truncate(line, maxWidth, "…")
		}
		rows = append(rows, line)
	}
	return strings.Join(append([]string{title}, rows...), "\n")
}

// mcpGlyph maps an MCP server status to a colored dot glyph.
func mcpGlyph(s mcp.ServerStatus) string {
	switch s {
	case mcp.StatusConnected:
		return kit.SelectorStatusConnected().Render(kit.DotConnected)
	case mcp.StatusConnecting:
		return kit.SelectorStatusReady().Render(kit.DotConnecting)
	case mcp.StatusError:
		return kit.SelectorStatusError().Render(kit.DotError)
	default: // StatusDisconnected
		return kit.SelectorStatusNone().Render(kit.DotOffline)
	}
}

// filterBlank drops empty strings so joins don't leave stray splits.
func filterBlank(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if strings.TrimSpace(s) != "" {
			out = append(out, s)
		}
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
