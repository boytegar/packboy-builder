// Package registry imports all tool sub-packages to trigger their init() registration.
package registry

import (
	_ "github.com/boytegar/packboy-builder/internal/tool/agent"
	_ "github.com/boytegar/packboy-builder/internal/tool/cron"
	_ "github.com/boytegar/packboy-builder/internal/tool/evolve"
	_ "github.com/boytegar/packboy-builder/internal/tool/fs"
	_ "github.com/boytegar/packboy-builder/internal/tool/mode"
	_ "github.com/boytegar/packboy-builder/internal/tool/skill"
	_ "github.com/boytegar/packboy-builder/internal/tool/tasktools"
	_ "github.com/boytegar/packboy-builder/internal/tool/web"
)
