# Release

Release automation is currently driven by `Makefile`.

## Commands

```bash
make release
make release-push VERSION=vX.Y.Z
```

`release-push` expects a clean worktree and a matching `CHANGELOG.md` section.
