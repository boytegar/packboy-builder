package lsp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/boytegar/packboy-builder/internal/tool/toolresult"
)

// AppendDiagnostics augments a tool result with cached LSP diagnostics for
// the given file path, if any server is configured and has published
// diagnostics. It is non-blocking: it only reads the in-memory cache and
// never waits for a server to publish. A background didOpen/didChange is
// fired so that future reads will have fresh diagnostics.
//
// This is the integration seam between file-touching tools (Read/Edit/Write)
// and the LSP layer, keeping the tool packages thin.
func AppendDiagnostics(result *toolresult.ToolResult, filePath string) {
	if result == nil || !result.Success {
		return
	}
	svc := Default()
	if svc == nil || svc.Manager() == nil || !svc.Manager().HasServer() {
		return
	}
	name, ok := svc.Manager().ServerForPath(filePath)
	if !ok {
		return
	}

	// Fire-and-forget: ensure the server starts and the file is opened so
	// diagnostics arrive for subsequent reads. Errors are silently ignored
	// — diagnostics are best-effort enrichment, not a primary result.
	go backgroundOpen(filePath, name)

	uri := FileURI(filePath)
	diags := svc.Manager().Diagnostics(uri)
	if len(diags) == 0 {
		return
	}

	sevName := map[int]string{1: "error", 2: "warning", 3: "info", 4: "hint"}
	var sb strings.Builder
	sb.WriteString("\n\n--- LSP diagnostics ---\n")
	for _, d := range diags {
		name := sevName[d.Severity]
		if name == "" {
			name = "error"
		}
		line := d.Range.Start.Line + 1
		col := d.Range.Start.Character + 1
		src := d.Source
		if src == "" {
			src = "lsp"
		}
		fmt.Fprintf(&sb, "%s: %s:%d:%d [%s]", name, filePath, line, col, src)
		if d.Code != "" {
			sb.WriteString(" " + d.Code)
		}
		sb.WriteString(": " + d.Message + "\n")
	}
	result.Output += sb.String()
}

// backgroundOpen starts the matching server (if not running) and sends
// didOpen so the server begins analyzing the file. Non-fatal on any error.
func backgroundOpen(filePath, serverName string) {
	ctx, cancel := context.WithTimeout(context.Background(), defaultOpenTimeout)
	defer cancel()

	mgr := Default().Manager()
	client, err := mgr.Start(ctx, serverName)
	if err != nil || client == nil {
		return
	}
	text, err := ReadFileContent(filePath)
	if err != nil {
		return
	}
	_ = client.OpenFile(ctx, FileURI(filePath), LanguageFromPath(filePath), text)
}

const defaultOpenTimeout = 15 * time.Second
