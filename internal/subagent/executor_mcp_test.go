package subagent

import (
	"context"
	"slices"
	"testing"

	"github.com/boytegar/packboy-builder/internal/core"
	"github.com/boytegar/packboy-builder/internal/mcp"
	"github.com/boytegar/packboy-builder/internal/tool"
)

func TestModeAllowsSchemaMCPTools(t *testing.T) {
	cases := []struct {
		name string
		mode PermissionMode
		want bool
	}{
		{"explore mcp read", PermissionExplore, true},
		{"default mcp read", PermissionDefault, true},
		{"acceptEdits mcp read", PermissionAcceptEdits, true},
		{"dontAsk mcp read", PermissionDontAsk, true},
		{"auto mcp read", PermissionAuto, true},
		{"bypass mcp read", PermissionBypass, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := modeAllowsSchema(tc.mode, "mcp__server__read_file"); got != tc.want {
				t.Fatalf("modeAllowsSchema(%s, mcp read) = %v, want %v", tc.mode, got, tc.want)
			}
		})
	}
}

func TestFilterSchemasForPermissionIncludesMCP(t *testing.T) {
	schemas := []core.ToolSchema{
		{Name: "Read"},
		{Name: "Write"},
		{Name: "mcp__server__read_file"},
		{Name: "mcp__server__write_file"},
	}

	got := filterSchemasForPermission(schemas, PermissionExplore, nil)
	names := make([]string, 0, len(got))
	for _, s := range got {
		names = append(names, s.Name)
	}
	for _, want := range []string{"Read", "mcp__server__read_file"} {
		if !slices.Contains(names, want) {
			t.Fatalf("explore filtered %v missing %s", names, want)
		}
	}
	if slices.Contains(names, "Write") {
		t.Fatalf("explore filtered %v must not contain Write", names)
	}
}

func TestSubagentPermissionFuncMCPReadOnlyAllowedEveryMode(t *testing.T) {
	cases := []struct {
		name string
		mode PermissionMode
	}{
		{"explore", PermissionExplore},
		{"default", PermissionDefault},
		{"acceptEdits", PermissionAcceptEdits},
		{"dontAsk", PermissionDontAsk},
		{"auto", PermissionAuto},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			check := subagentPermissionFunc(tc.mode, nil, nil)
			allow, reason := check(context.Background(), "mcp__server__read_file", map[string]any{"path": "/tmp"})
			if !allow {
				t.Fatalf("%s mode: read-only MCP tool blocked: %s", tc.mode, reason)
			}
		})
	}
}

func TestSubagentPermissionFuncMCPWriteRequiresApproval(t *testing.T) {
	cases := []struct {
		name string
		mode PermissionMode
		want bool
	}{
		{"explore denied", PermissionExplore, false},
		{"default denied", PermissionDefault, false},
		{"dontAsk denied", PermissionDontAsk, false},
		{"bypass allowed", PermissionBypass, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			check := subagentPermissionFunc(tc.mode, nil, nil)
			allow, _ := check(context.Background(), "mcp__server__write_file", map[string]any{"content": "x"})
			if allow != tc.want {
				t.Fatalf("%s mode: MCP write allow=%v, want %v", tc.mode, allow, tc.want)
			}
		})
	}
}

func TestSubagentPermissionFuncMCPAllowTools(t *testing.T) {
	allow := ToolList{{Name: "mcp__server__read_file"}}
	check := subagentPermissionFunc(PermissionDefault, allow, nil)
	allowOK, reason := check(context.Background(), "mcp__server__read_file", map[string]any{})
	if !allowOK {
		t.Fatalf("allow_tools MCP read blocked: %s", reason)
	}
	deny, _ := check(context.Background(), "mcp__server__other", map[string]any{})
	if deny {
		t.Fatal("unlisted MCP tool allowed under allow_tools whitelist")
	}
}

func TestMCPToolNameClassification(t *testing.T) {
	isMCP := func(name string) bool {
		return mcp.IsMCPTool(name)
	}
	if !isMCP("mcp__server__read_file") {
		t.Fatal("mcp__server__read_file should classify as MCP")
	}
	if isMCP("Read") || isMCP("mcp__server") || isMCP("mcp__") {
		t.Fatal("non-MCP names misclassified")
	}
}

var _ = tool.ToolBash // keep tool import used if consts shift
