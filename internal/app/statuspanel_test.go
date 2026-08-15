package app

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/boytegar/packboy-builder/internal/app/conv"
	"github.com/boytegar/packboy-builder/internal/app/kit"
	"github.com/boytegar/packboy-builder/internal/mcp"
	"github.com/boytegar/packboy-builder/internal/skill"
	"github.com/boytegar/packboy-builder/internal/subagent"
)

var ansiPat = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func strip(s string) string { return ansiPat.ReplaceAllString(s, "") }

func TestRenderStatusOverviewShowsSkillAndMCPColumns(t *testing.T) {
	skillReg := skill.NewRegistryForTest(map[string]*skill.Skill{
		"graph": {Name: "graph", State: skill.StateActive},
	}, nil, nil)
	mcpReg := mcp.NewRegistryForTest(map[string]mcp.ServerConfig{
		"github": {Name: "github"},
		"fs":     {Name: "fs"},
	})

	m := &model{
		env:            env{Width: 80},
		conv:           conv.NewModel(80),
		welcomePending: true,
		services: services{
			Skill: skillReg,
			MCP:   mcpReg,
		},
	}

	out := m.renderStatusOverview()
	if out == "" {
		t.Fatal("expected a non-empty status overview")
	}
	for _, want := range []string{"Skills", "graph", "MCPs", "github", "fs"} {
		if !strings.Contains(strip(out), want) {
			t.Errorf("overview missing %q\nrendered:\n%s", want, out)
		}
	}
}

func TestRenderStatusOverviewEmptyWhenNoItems(t *testing.T) {
	m := &model{
		env:  env{Width: 80},
		conv: conv.NewModel(80),
		services: services{
			Skill: skill.NewRegistryForTest(nil, nil, nil),
			MCP:   mcp.NewRegistryForTest(nil),
		},
	}
	if out := m.renderStatusOverview(); out != "" {
		t.Fatalf("expected empty overview, got:\n%s", out)
	}
}

func TestRenderStatusOverviewShowsAgentsColumn(t *testing.T) {
	subReg := subagent.NewRegistry()
	subReg.Register(&subagent.AgentConfig{Name: "investigator"})
	subReg.Register(&subagent.AgentConfig{Name: "test-runner"})

	m := &model{
		env:            env{Width: 100},
		conv:           conv.NewModel(100),
		welcomePending: true,
		services:       services{Subagent: subReg},
	}
	out := m.renderStatusOverview()
	if out == "" {
		t.Fatal("expected a non-empty overview with agents")
	}
	for _, want := range []string{"Agents", "investigator", "test-runner"} {
		if !strings.Contains(strip(out), want) {
			t.Errorf("overview missing %q\nrendered:\n%s", want, out)
		}
	}
}

// TestClipHorizontally trims a wide multi-line blob into a narrow viewport,
// panning across the content with an offset, and never overruns the viewport.
func TestClipHorizontallyAndContentWidth(t *testing.T) {
	blob := kit.JoinColumns([]string{
		"LSPs\n  gopls-python", "Skills\n  graph",
		"MCPs\n  github", "Agents\n  investigator",
	}, -1)

	cw := kit.ContentWidth(blob)
	minW := 20
	if cw < minW {
		t.Fatalf("content width %d below expected %d", cw, minW)
	}

	viewport := 22
	for _, off := range []int{0, 10, 1000} {
		clipped := strip(kit.ClipHorizontally(blob, viewport, off))
		for i, ln := range strings.Split(clipped, "\n") {
			if len(ln) > viewport {
				t.Fatalf("offset %d line %d exceeds viewport %d (got %d): %q", off, i, viewport, len(ln), ln)
			}
		}
		if clipped == "" {
			t.Fatalf("offset %d produced empty clip", off)
		}
	}
}

func (m *model) statusPanelScrollMsg(s string) tea.KeyMsg {
	switch s {
	case "shift+right":
		return tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModShift}
	case "shift+left":
		return tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModShift}
	default:
		return tea.KeyPressMsg{Code: 'x'}
	}
}

// TestHandleStatusPanelScrollClaimsShiftArrowsAndClamps removed because
// horizontal scroll is no longer needed with fixed 25% column widths.

func TestStatusPanelVisibleDependsOnWelcomeAndEmptyChat(t *testing.T) {
	// welcome cleared → not visible.
	m := &model{env: env{Width: 60}, conv: conv.NewModel(60)}
	if m.statusPanelVisible() {
		t.Fatal("expected panel hidden once welcome cleared")
	}
	// fresh → visible.
	m2 := &model{env: env{Width: 60}, conv: conv.NewModel(60), welcomePending: true}
	if !m2.statusPanelVisible() {
		t.Fatal("expected visible on fresh session")
	}
	// welcome still set but content committed → hidden.
	m3 := &model{env: env{Width: 60}, conv: conv.NewModel(60), welcomePending: true}
	m3.conv.CommittedCount = 1
	if m3.statusPanelVisible() {
		t.Fatal("expected hidden once first message committed")
	}
}
