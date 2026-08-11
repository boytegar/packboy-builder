package tool

import "path/filepath"

// elidePathLen is the max rune length of displayed path + filename tail.
const elideMaxLen = 40

// ElidePath shortens a filesystem path for display as ".../{last folder}/{name}".
// Examples:
//
//	/home/user/proj/internal/app/conv/tool_render.go
//	→ ".../conv/tool_render.go"
//
// Short paths (<= elideMaxLen runes) are returned unchanged. The filename is
// always kept whole; exactly the last two path segments are preserved. This is
// a display-only helper — callers that feed the value back to the model must
// keep the full path.
func ElidePath(p string) string {
	if p == "" {
		return ""
	}
	if len([]rune(p)) <= elideMaxLen {
		return p
	}
	name := filepath.Base(p)
	dir := filepath.Dir(p)
	lastFolder := filepath.Base(dir)
	short := lastFolder + "/" + name
	if len([]rune(short)) > elideMaxLen {
		// Even the two-segment tail is too long; keep the filename and elide it.
		short = name
	}
	return ".../" + short
}
