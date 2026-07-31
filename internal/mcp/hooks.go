package mcp

import "github.com/boytegar/packboy-builder/internal/hook"

func fireConfigChanged(source, filePath string) {
	if h := hook.DefaultEngine(); h != nil {
		h.ExecuteAsync(hook.ConfigChange, hook.HookInput{
			Source:   source,
			FilePath: filePath,
		})
		h.ExecuteAsync(hook.FileChanged, hook.HookInput{
			Source:   source,
			FilePath: filePath,
		})
	}
}
