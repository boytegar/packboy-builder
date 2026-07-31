package skill

import (
	"embed"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

//go:embed builtin
var builtinFS embed.FS

var (
	builtinOnce sync.Once
	builtinRoot string
	builtinErr  error
)

// materializeBuiltinSkills extracts the embedded skill pack to a user cache
// directory so the existing disk loader (and Read tool) can open SKILL.md and
// references/ as normal files. Runs once per process.
func materializeBuiltinSkills() (string, error) {
	builtinOnce.Do(func() {
		cache, err := os.UserCacheDir()
		if err != nil {
			builtinErr = err
			return
		}
		root := filepath.Join(cache, "pcb", "builtin-skills")
		if err := os.MkdirAll(root, 0o755); err != nil {
			builtinErr = err
			return
		}
		err = fs.WalkDir(builtinFS, "builtin", func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel("builtin", path)
			if err != nil {
				return err
			}
			if rel == "." {
				return nil
			}
			dest := filepath.Join(root, rel)
			if d.IsDir() {
				return os.MkdirAll(dest, 0o755)
			}
			return writeEmbeddedFile(path, dest)
		})
		if err != nil {
			builtinErr = err
			return
		}
		builtinRoot = root
	})
	return builtinRoot, builtinErr
}

func writeEmbeddedFile(src, dest string) error {
	in, err := builtinFS.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
