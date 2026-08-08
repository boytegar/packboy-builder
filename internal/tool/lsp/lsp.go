package lsp

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/boytegar/packboy-builder/internal/core"
	lspsvc "github.com/boytegar/packboy-builder/internal/lsp"
	"github.com/boytegar/packboy-builder/internal/tool"
	"github.com/boytegar/packboy-builder/internal/tool/toolresult"
)

const IconLSP = "🧭"

// LSPTool exposes code intelligence backed by configured LSP servers. Kept a
// thin adapter: all protocol work lives in internal/lsp.
type LSPTool struct{}

func NewLSPTool() *LSPTool { return &LSPTool{} }

func (t *LSPTool) Name() string { return tool.ToolLSP }

func (t *LSPTool) Icon() string { return IconLSP }

func (t *LSPTool) Description() string {
	return `Code intelligence via configured language servers (LSP).

The service is lazy: servers start on first use for a matching file extension
(extensionToLanguage in plugin lspServers config, or built-in default catalog
when the server binary is on PATH).

Actions:
- "diagnostics": cached diagnostics for a file (errors/warnings).
- "definition": locate the definition of the symbol at a position.
- "references": locate all references to the symbol at a position.
- "symbols": list document symbols (functions, types, etc.) in a file.
- "call_hierarchy": incoming or outgoing calls for the symbol at a position.
- "rename": rename the symbol at a position across the workspace.
- "restart": force-restart the LSP server for a file's language.

Params: "file" (absolute path; required for all except restart), "action".
Optional: "line" (0-based), "char" (0-based) for position-based actions.
"direction" ("incoming"|"outgoing", default "incoming") for call_hierarchy.
"new_name" (required for rename).
File content is read fresh from disk before requests; the server is kept
open across calls. Dead servers auto-restart on next request.`
}

func (t *LSPTool) Schema() core.ToolSchema {
	return core.ToolSchema{
		Name:        t.Name(),
		Description: t.Description(),
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"action": map[string]any{
					"type":        "string",
					"enum":        []string{"diagnostics", "definition", "references", "symbols", "call_hierarchy", "rename", "restart"},
					"description": "The LSP operation to perform.",
				},
				"file": map[string]any{
					"type":        "string",
					"description": "Absolute path of the target file.",
				},
				"line": map[string]any{
					"type":        "integer",
					"description": "0-based line for position-based actions.",
				},
				"char": map[string]any{
					"type":        "integer",
					"description": "0-based character for position-based actions.",
				},
				"direction": map[string]any{
					"type":        "string",
					"enum":        []string{"incoming", "outgoing"},
					"description": "Call hierarchy direction (default: incoming).",
				},
				"new_name": map[string]any{
					"type":        "string",
					"description": "New name for rename action.",
				},
			},
			"required": []string{"action"},
		},
	}
}

func (t *LSPTool) Execute(ctx context.Context, params map[string]any, cwd string) toolresult.ToolResult {
	action, err := tool.RequireString(params, "action")
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), err.Error())
	}

	svc := lspsvc.Default()
	if svc == nil || svc.Manager() == nil || !svc.Manager().HasServer() {
		return toolresult.NewErrorResult(t.Name(),
			"no LSP servers configured. Add an lspServers entry to a plugin manifest "+
				"(command + extensionToLanguage) and reload.")
	}

	switch action {
	case "diagnostics":
		file := tool.GetString(params, "file")
		if file == "" {
			return toolresult.NewErrorResult(t.Name(), "file is required for diagnostics")
		}
		return t.diagnostics(svc, file)
	case "definition", "references", "symbols", "call_hierarchy", "rename":
		file, err := tool.RequireString(params, "file")
		if err != nil {
			return toolresult.NewErrorResult(t.Name(), err.Error())
		}
		line := tool.GetInt(params, "line", 0)
		ch := tool.GetInt(params, "char", 0)
		switch action {
		case "symbols":
			return t.symbols(ctx, svc, file)
		case "call_hierarchy":
			direction := tool.GetString(params, "direction")
			if direction == "" {
				direction = "incoming"
			}
			return t.callHierarchy(ctx, svc, file, line, ch, direction)
		case "rename":
			newName, err := tool.RequireString(params, "new_name")
			if err != nil {
				return toolresult.NewErrorResult(t.Name(), "new_name is required for rename")
			}
			return t.rename(ctx, svc, file, line, ch, newName)
		default:
			return t.symbolRequest(ctx, svc, action, file, line, ch)
		}
	case "restart":
		file, err := tool.RequireString(params, "file")
		if err != nil {
			return toolresult.NewErrorResult(t.Name(), "file is required for restart (to determine which server)")
		}
		return t.restart(ctx, svc, file)
	default:
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("unknown action %q", action))
	}
}

func (t *LSPTool) diagnostics(svc *lspsvc.Service, file string) toolresult.ToolResult {
	uri := lspsvc.FileURI(file)
	diags := svc.Manager().Diagnostics(uri)
	if len(diags) == 0 {
		return successResult(t, "No active diagnostics for "+file)
	}

	sevName := map[int]string{1: "error", 2: "warning", 3: "info", 4: "hint"}
	counts := map[int]int{}
	var sb strings.Builder
	for _, d := range diags {
		name := sevName[d.Severity]
		if name == "" {
			name = "error"
		}
		counts[d.Severity]++
		line := d.Range.Start.Line + 1
		col := d.Range.Start.Character + 1
		src := d.Source
		code := d.Code
		if src == "" {
			src = "lsp"
		}
		fmt.Fprintf(&sb, "%s: %s:%d:%d [%s]", name, file, line, col, src)
		if code != "" {
			sb.WriteString(" " + code)
		}
		sb.WriteString(": " + d.Message + "\n")
	}

	order := []int{1, 2, 3, 4}
	total := 0
	var summary []string
	for _, s := range order {
		if n := counts[s]; n > 0 {
			summary = append(summary, fmt.Sprintf("%d %s(s)", n, sevName[s]))
			total += n
		}
	}
	head := fmt.Sprintf("%d diagnostic(s) for %s (%s)\n\n", total, file, strings.Join(summary, ", "))
	return successResult(t, head+sb.String())
}

func (t *LSPTool) symbolRequest(ctx context.Context, svc *lspsvc.Service, action, file string, line, ch int) toolresult.ToolResult {
	name, ok := svc.Manager().ServerForPath(file)
	if !ok {
		return toolresult.NewErrorResult(t.Name(),
			fmt.Sprintf("no LSP server configured for extension %q", filepath.Ext(file)))
	}
	client, err := svc.Manager().Start(ctx, name)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), err.Error())
	}

	uri := lspsvc.FileURI(file)
	text, err := lspsvc.ReadFileContent(file)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("read %s: %v", file, err))
	}
	if err := client.OpenFile(ctx, uri, lspsvc.LanguageFromPath(file), text); err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("didOpen: %v", err))
	}

	pos := lspsvc.LSPPosition{Line: line, Character: ch}
	var locs []lspsvc.Location
	switch action {
	case "definition":
		res, err := client.Definition(ctx, uri, pos)
		if err != nil {
			return toolresult.NewErrorResult(t.Name(), err.Error())
		}
		locs = res.Locations
	case "references":
		res, err := client.References(ctx, uri, pos)
		if err != nil {
			return toolresult.NewErrorResult(t.Name(), err.Error())
		}
		locs = res.Locations
	}

	if len(locs) == 0 {
		return successResult(t, fmt.Sprintf("No %s found for symbol at %s:%d:%d", action, file, line+1, ch+1))
	}

	sort.Slice(locs, func(i, j int) bool {
		a, b := locs[i].URI, locs[j].URI
		if a != b {
			return a < b
		}
		pa, pb := locs[i].Range.Start, locs[j].Range.Start
		if pa.Line != pb.Line {
			return pa.Line < pb.Line
		}
		return pa.Character < pb.Character
	})

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d %s location(s):\n\n", len(locs), action)
	for _, loc := range locs {
		path := uriToPath(loc.URI)
		fmt.Fprintf(&sb, "%s:%d:%d\n", path, loc.Range.Start.Line+1, loc.Range.Start.Character+1)
	}
	return successResult(t, sb.String())
}

// symbols lists document symbols for a file.
func (t *LSPTool) symbols(ctx context.Context, svc *lspsvc.Service, file string) toolresult.ToolResult {
	name, ok := svc.Manager().ServerForPath(file)
	if !ok {
		return toolresult.NewErrorResult(t.Name(),
			fmt.Sprintf("no LSP server configured for extension %q", filepath.Ext(file)))
	}
	client, err := svc.Manager().Start(ctx, name)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), err.Error())
	}

	uri := lspsvc.FileURI(file)
	text, err := lspsvc.ReadFileContent(file)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("read %s: %v", file, err))
	}
	if err := client.OpenFile(ctx, uri, lspsvc.LanguageFromPath(file), text); err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("didOpen: %v", err))
	}

	hierarchical, flat, err := client.DocumentSymbols(ctx, uri)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), err.Error())
	}

	var sb strings.Builder
	if len(hierarchical) > 0 {
		fmt.Fprintf(&sb, "%d document symbol(s):\n\n", countSymbolsHierarchical(hierarchical))
		for _, sym := range hierarchical {
			renderDocumentSymbol(&sb, sym, 0)
		}
	} else if len(flat) > 0 {
		fmt.Fprintf(&sb, "%d symbol(s):\n\n", len(flat))
		sort.Slice(flat, func(i, j int) bool {
			pi, pj := flat[i].Location.Range.Start, flat[j].Location.Range.Start
			if pi.Line != pj.Line {
				return pi.Line < pj.Line
			}
			return pi.Character < pj.Character
		})
		for _, sym := range flat {
			path := uriToPath(sym.Location.URI)
			fmt.Fprintf(&sb, "%s [%s] %s:%d:%d\n", sym.Name, symbolKindName(sym.Kind), path, sym.Location.Range.Start.Line+1, sym.Location.Range.Start.Character+1)
		}
	} else {
		return successResult(t, "No symbols found in "+file)
	}
	return successResult(t, sb.String())
}

func countSymbolsHierarchical(syms []lspsvc.DocumentSymbol) int {
	n := len(syms)
	for _, s := range syms {
		n += countSymbolsHierarchical(s.Children)
	}
	return n
}

func renderDocumentSymbol(sb *strings.Builder, sym lspsvc.DocumentSymbol, depth int) {
	indent := strings.Repeat("  ", depth)
	fmt.Fprintf(sb, "%s%s [%s] L%d:%d", indent, sym.Name, symbolKindName(sym.Kind), sym.Range.Start.Line+1, sym.Range.Start.Character+1)
	if sym.Detail != "" {
		sb.WriteString(" — " + sym.Detail)
	}
	sb.WriteByte('\n')
	for _, child := range sym.Children {
		renderDocumentSymbol(sb, child, depth+1)
	}
}

func symbolKindName(kind int) string {
	names := map[int]string{
		1: "File", 2: "Module", 3: "Namespace", 4: "Package", 5: "Class",
		6: "Method", 7: "Property", 8: "Field", 9: "Constructor", 10: "Enum",
		11: "Interface", 12: "Function", 13: "Variable", 14: "Constant",
		15: "String", 16: "Number", 17: "Boolean", 18: "Array", 19: "Object",
		20: "Key", 21: "Null", 22: "EnumMember", 23: "Struct", 24: "Event",
		25: "Operator", 26: "TypeParameter",
	}
	if name, ok := names[kind]; ok {
		return name
	}
	return fmt.Sprintf("kind(%d)", kind)
}

// callHierarchy requests incoming or outgoing calls for a symbol.
func (t *LSPTool) callHierarchy(ctx context.Context, svc *lspsvc.Service, file string, line, ch int, direction string) toolresult.ToolResult {
	name, ok := svc.Manager().ServerForPath(file)
	if !ok {
		return toolresult.NewErrorResult(t.Name(),
			fmt.Sprintf("no LSP server configured for extension %q", filepath.Ext(file)))
	}
	client, err := svc.Manager().Start(ctx, name)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), err.Error())
	}

	uri := lspsvc.FileURI(file)
	text, err := lspsvc.ReadFileContent(file)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("read %s: %v", file, err))
	}
	if err := client.OpenFile(ctx, uri, lspsvc.LanguageFromPath(file), text); err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("didOpen: %v", err))
	}

	pos := lspsvc.LSPPosition{Line: line, Character: ch}
	items, err := client.PrepareCallHierarchy(ctx, uri, pos)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), err.Error())
	}
	if len(items) == 0 {
		return successResult(t, fmt.Sprintf("No call hierarchy items at %s:%d:%d", file, line+1, ch+1))
	}

	var sb strings.Builder
	for _, item := range items {
		fmt.Fprintf(&sb, "Symbol: %s [%s] at %s:%d:%d\n", item.Name, symbolKindName(item.Kind), uriToPath(item.URI), item.Range.Start.Line+1, item.Range.Start.Character+1)
		switch direction {
		case "incoming":
			calls, err := client.IncomingCalls(ctx, item)
			if err != nil {
				fmt.Fprintf(&sb, "  incoming calls: error: %v\n", err)
				continue
			}
			fmt.Fprintf(&sb, "  %d incoming call(s):\n", len(calls))
			for _, call := range calls {
				fmt.Fprintf(&sb, "    ← %s [%s] at %s:%d:%d\n", call.From.Name, symbolKindName(call.From.Kind), uriToPath(call.From.URI), call.From.Range.Start.Line+1, call.From.Range.Start.Character+1)
			}
		case "outgoing":
			calls, err := client.OutgoingCalls(ctx, item)
			if err != nil {
				fmt.Fprintf(&sb, "  outgoing calls: error: %v\n", err)
				continue
			}
			fmt.Fprintf(&sb, "  %d outgoing call(s):\n", len(calls))
			for _, call := range calls {
				fmt.Fprintf(&sb, "    → %s [%s] at %s:%d:%d\n", call.To.Name, symbolKindName(call.To.Kind), uriToPath(call.To.URI), call.To.Range.Start.Line+1, call.To.Range.Start.Character+1)
			}
		}
		sb.WriteByte('\n')
	}
	return successResult(t, sb.String())
}

// rename requests a workspace-wide rename of a symbol.
func (t *LSPTool) rename(ctx context.Context, svc *lspsvc.Service, file string, line, ch int, newName string) toolresult.ToolResult {
	name, ok := svc.Manager().ServerForPath(file)
	if !ok {
		return toolresult.NewErrorResult(t.Name(),
			fmt.Sprintf("no LSP server configured for extension %q", filepath.Ext(file)))
	}
	client, err := svc.Manager().Start(ctx, name)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), err.Error())
	}

	uri := lspsvc.FileURI(file)
	text, err := lspsvc.ReadFileContent(file)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("read %s: %v", file, err))
	}
	if err := client.OpenFile(ctx, uri, lspsvc.LanguageFromPath(file), text); err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("didOpen: %v", err))
	}

	pos := lspsvc.LSPPosition{Line: line, Character: ch}
	edit, err := client.Rename(ctx, uri, pos, newName)
	if err != nil {
		return toolresult.NewErrorResult(t.Name(), err.Error())
	}

	var sb strings.Builder
	totalEdits := 0
	if len(edit.Changes) > 0 {
		for uri, ranges := range edit.Changes {
			path := uriToPath(uri)
			fmt.Fprintf(&sb, "%s: %d edit(s)\n", path, len(ranges))
			for _, r := range ranges {
				fmt.Fprintf(&sb, "  L%d:%d → L%d:%d\n", r.Start.Line+1, r.Start.Character+1, r.End.Line+1, r.End.Character+1)
				totalEdits++
			}
		}
	} else {
		return successResult(t, fmt.Sprintf("Rename to %q returned no edits (symbol may not be renamable)", newName))
	}
	fmt.Fprintf(&sb, "\n%d total edit(s) across %d file(s). Apply manually or via Edit tool.\n", totalEdits, len(edit.Changes))
	return successResult(t, sb.String())
}

// restart force-restarts the LSP server for a file's language.
func (t *LSPTool) restart(ctx context.Context, svc *lspsvc.Service, file string) toolresult.ToolResult {
	name, ok := svc.Manager().ServerForPath(file)
	if !ok {
		return toolresult.NewErrorResult(t.Name(),
			fmt.Sprintf("no LSP server configured for extension %q", filepath.Ext(file)))
	}
	if _, err := svc.Manager().RestartServer(ctx, name); err != nil {
		return toolresult.NewErrorResult(t.Name(), fmt.Sprintf("restart %q: %v", name, err))
	}
	return successResult(t, fmt.Sprintf("Restarted LSP server %q", name))
}

func uriToPath(uri string) string {
	s := strings.TrimPrefix(uri, "file://")
	if i := strings.IndexByte(s, '?'); i >= 0 {
		s = s[:i]
	}
	if decoded, err := decodeURLEscape(s); err == nil {
		return decoded
	}
	return s
}

// decodeURLEscape minimally decodes %XX escapes in file URIs.
func decodeURLEscape(s string) (string, error) {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			var b byte
			if _, err := fmt.Sscanf(s[i+1:i+3], "%02x", &b); err == nil {
				sb.WriteByte(b)
				i += 2
				continue
			}
		}
		sb.WriteByte(s[i])
	}
	return sb.String(), nil
}

func successResult(t *LSPTool, output string) toolresult.ToolResult {
	return toolresult.ToolResult{
		Success: true,
		Output:  output,
		Metadata: toolresult.ResultMetadata{
			Title: t.Name(),
			Icon:  t.Icon(),
		},
	}
}

func init() {
	tool.Register(NewLSPTool())
}
