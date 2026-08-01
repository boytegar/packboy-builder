# AUR packaging

Package base: `pcb-bin` (prebuilt binary, not the unrelated `pcb` PCB editor).

## Files

- `pcb-bin/PKGBUILD` — install `pcb` from GitHub release tarballs
- `pcb-bin/.SRCINFO` — AUR metadata (keep in sync with PKGBUILD)
- `update.sh` — refresh checksums / optionally push to AUR

## Prerequisites

1. Account at https://aur.archlinux.org/register/
2. SSH public key under Account → SSH Public Key
3. Private key available locally (default `~/.ssh/id_ed25519_aur`, override with `AUR_SSH_KEY`)
4. GitHub release for the target version already published (linux amd64 + arm64 tarballs)

## Update after a GitHub release

Local files only:

```bash
make aur-update VERSION=v1.23.0
```

Update + push to AUR:

```bash
make aur-push VERSION=v1.23.0
```

Rebuild same version with a higher pkgrel (packaging-only fix):

```bash
AUR_PKGREL=2 make aur-push VERSION=v1.23.0
```

Equivalent script form:

```bash
./aur/update.sh v1.23.0
./aur/update.sh v1.23.0 --push
```

Typical release flow:

```bash
make release-push VERSION=v1.23.0   # tag → CI publishes GitHub release assets
# wait until linux tarballs are on the release page
make aur-push VERSION=v1.23.0
```

## Local install (Arch)

```bash
yay -S pcb-bin
# or
git clone https://aur.archlinux.org/pcb-bin.git
cd pcb-bin && makepkg -si
```
