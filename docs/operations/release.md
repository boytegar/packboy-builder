# Release

Release automation is currently driven by `Makefile`.

## Commands

```bash
make release
make release-push VERSION=vX.Y.Z
make aur-update VERSION=vX.Y.Z
make aur-push VERSION=vX.Y.Z
```

`release-push` expects a clean worktree and a matching `CHANGELOG.md` section.

## AUR (`pcb-bin`)

After the GitHub release assets exist (linux amd64/arm64 tarballs):

```bash
make aur-push VERSION=vX.Y.Z
```

This rewrites `aur/pcb-bin/PKGBUILD` + `.SRCINFO` from the published
checksums and pushes to https://aur.archlinux.org/packages/pcb-bin.

Requires an AUR account SSH key (`AUR_SSH_KEY`, default
`~/.ssh/id_ed25519_aur`). Details: `aur/README.md`.
