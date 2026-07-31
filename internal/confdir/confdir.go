// Package confdir resolves the per-user and per-project configuration directory.
//
// confdir is a zero-dependency infrastructure leaf so every layer (including
// internal/log and internal/core) can import it without a layering violation.
package confdir

import "path/filepath"

// Name is the configuration directory name.
const Name = ".pcb"

// Dir returns the configuration directory under root (root/.pcb).
func Dir(root string) string {
	return filepath.Join(root, Name)
}
