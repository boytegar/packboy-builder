#!/usr/bin/env bash
# Update aur/pcb-bin for a published GitHub release and optionally push to AUR.
#
# Usage:
#   ./aur/update.sh 1.22.6           # refresh local PKGBUILD + .SRCINFO only
#   ./aur/update.sh 1.22.6 --push    # also commit + push to aur.archlinux.org
#   ./aur/update.sh v1.22.6 --push
#
# Env:
#   AUR_SSH_KEY   path to SSH private key (default: ~/.ssh/id_ed25519_aur)
#   AUR_GIT_URL   override remote (default: aur@aur.archlinux.org:pcb-bin.git)
#   AUR_PKGREL    package release number (default: 1)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PKGDIR="$ROOT/aur/pcb-bin"
REPO="boytegar/packboy-builder"
ASSET="packboy-builder"
AUR_SSH_KEY="${AUR_SSH_KEY:-$HOME/.ssh/id_ed25519_aur}"
AUR_GIT_URL="${AUR_GIT_URL:-aur@aur.archlinux.org:pcb-bin.git}"
AUR_PKGREL="${AUR_PKGREL:-1}"
PUSH=0

usage() {
	echo "Usage: $0 <version> [--push]" >&2
	echo "  version   release version with or without leading v (e.g. 1.22.6)" >&2
	echo "  --push    clone AUR repo, commit PKGBUILD/.SRCINFO, and push" >&2
	exit 1
}

[[ $# -ge 1 ]] || usage
VERSION="$1"
shift
while [[ $# -gt 0 ]]; do
	case "$1" in
	--push) PUSH=1 ;;
	-h | --help) usage ;;
	*)
		echo "unknown argument: $1" >&2
		usage
		;;
	esac
	shift
done

VERSION="${VERSION#v}"
[[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-].*)?$ ]] || {
	echo "invalid version: $VERSION" >&2
	exit 1
}

download_sha256() {
	local arch="$1"
	local url="https://github.com/${REPO}/releases/download/v${VERSION}/${ASSET}_linux_${arch}.tar.gz"
	echo "→ fetching $url" >&2
	# Prefer sha256sum of streamed body so we do not keep multi-MB temps.
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url" | sha256sum | awk '{print $1}'
	elif command -v wget >/dev/null 2>&1; then
		wget -qO- "$url" | sha256sum | awk '{print $1}'
	else
		python3 - "$url" <<'PY'
import hashlib, sys, urllib.request
url = sys.argv[1]
req = urllib.request.Request(url, headers={"User-Agent": "pcb-aur-update"})
h = hashlib.sha256()
with urllib.request.urlopen(req, timeout=600) as r:
    while True:
        chunk = r.read(1024 * 1024)
        if not chunk:
            break
        h.update(chunk)
print(h.hexdigest())
PY
	fi
}

echo "→ updating pcb-bin for v${VERSION} (pkgrel=${AUR_PKGREL})"
SHA_AMD64="$(download_sha256 amd64)"
SHA_ARM64="$(download_sha256 arm64)"
echo "  amd64  $SHA_AMD64"
echo "  arm64  $SHA_ARM64"

mkdir -p "$PKGDIR"
cat >"$PKGDIR/PKGBUILD" <<EOF
# Maintainer: Boyke Tegar <works@boytegar.xyz>
pkgname=pcb-bin
pkgver=${VERSION}
pkgrel=${AUR_PKGREL}
pkgdesc="A fast, open agent harness for the terminal — single Go binary, ~12MB, zero runtime deps"
arch=('x86_64' 'aarch64')
url="https://github.com/${REPO}"
license=('Apache-2.0')
provides=('pcb')
conflicts=('pcb' 'pcb-git')
options=('!strip' '!debug')
source_x86_64=("\$url/releases/download/v\$pkgver/${ASSET}_linux_amd64.tar.gz")
source_aarch64=("\$url/releases/download/v\$pkgver/${ASSET}_linux_arm64.tar.gz")
sha256sums_x86_64=('${SHA_AMD64}')
sha256sums_aarch64=('${SHA_ARM64}')

package() {
    install -Dm755 pcb "\$pkgdir/usr/bin/pcb"
}
EOF

if command -v makepkg >/dev/null 2>&1; then
	(cd "$PKGDIR" && makepkg --printsrcinfo >.SRCINFO)
else
	cat >"$PKGDIR/.SRCINFO" <<EOF
pkgbase = pcb-bin
	pkgdesc = A fast, open agent harness for the terminal — single Go binary, ~12MB, zero runtime deps
	pkgver = ${VERSION}
	pkgrel = ${AUR_PKGREL}
	url = https://github.com/${REPO}
	arch = x86_64
	arch = aarch64
	license = Apache-2.0
	provides = pcb
	conflicts = pcb
	conflicts = pcb-git
	options = !strip
	options = !debug
	source_x86_64 = https://github.com/${REPO}/releases/download/v${VERSION}/${ASSET}_linux_amd64.tar.gz
	sha256sums_x86_64 = ${SHA_AMD64}
	source_aarch64 = https://github.com/${REPO}/releases/download/v${VERSION}/${ASSET}_linux_arm64.tar.gz
	sha256sums_aarch64 = ${SHA_ARM64}

pkgname = pcb-bin
EOF
fi

echo "→ wrote $PKGDIR/PKGBUILD"
echo "→ wrote $PKGDIR/.SRCINFO"

if [[ "$PUSH" -ne 1 ]]; then
	echo "local AUR files updated (pass --push to publish)"
	exit 0
fi

if [[ ! -f "$AUR_SSH_KEY" ]]; then
	echo "AUR SSH key not found: $AUR_SSH_KEY" >&2
	echo "Set AUR_SSH_KEY or add the key under https://aur.archlinux.org/account/" >&2
	exit 1
fi

export GIT_SSH_COMMAND="${GIT_SSH_COMMAND:-/usr/bin/ssh -i ${AUR_SSH_KEY} -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -o BatchMode=yes}"

WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/pcb-aur.XXXXXX")"
cleanup() { rm -rf "$WORKDIR"; }
trap cleanup EXIT

echo "→ cloning $AUR_GIT_URL"
git clone "$AUR_GIT_URL" "$WORKDIR/pcb-bin"
cp "$PKGDIR/PKGBUILD" "$PKGDIR/.SRCINFO" "$WORKDIR/pcb-bin/"

cd "$WORKDIR/pcb-bin"
if git diff --quiet && git diff --cached --quiet; then
	# also detect untracked
	if [[ -z "$(git status --porcelain)" ]]; then
		echo "AUR package already at v${VERSION}-${AUR_PKGREL}; nothing to push"
		exit 0
	fi
fi

git add PKGBUILD .SRCINFO
if git diff --cached --quiet; then
	echo "AUR package already at v${VERSION}-${AUR_PKGREL}; nothing to push"
	exit 0
fi

git -c commit.gpgsign=false commit -m "pcb-bin ${VERSION}-${AUR_PKGREL}"
# AUR still uses master as the default branch for package repos.
git push origin HEAD:master
echo "→ pushed pcb-bin ${VERSION}-${AUR_PKGREL} to AUR"
echo "  https://aur.archlinux.org/packages/pcb-bin"
