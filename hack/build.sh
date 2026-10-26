#!/usr/bin/env bash
# Build the Linux Apple Music client; Chrome owns playback and DRM.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"
GOBINARY=${GOBINARY:-go}
ACTION=${1:-build}
BUILD_GOOS=${GOOS:-$("$GOBINARY" env GOOS)}

if [[ "$BUILD_GOOS" != linux ]]; then
	echo "azichrome-tui currently targets Linux desktops only" >&2
	exit 1
fi
case "$ACTION" in
	build|install) ;;
	*) echo "Usage: $0 [build|install]" >&2; exit 1 ;;
esac

VERSION=${VERSION:-$(git -C "$ROOT" describe --tags --always --dirty=-dev 2>/dev/null || printf dev)}
TARGET=${BUILD_TARGET:-$ROOT/bin/azichrome-tui}
LDFLAGS="${LDFLAGS:-} -X github.com/go-musicfox/go-musicfox/internal/types.AppVersion=$VERSION"
mkdir -p "$(dirname "$TARGET")"
CGO_ENABLED=${CGO_ENABLED:-1} GOOS=linux "$GOBINARY" build -trimpath -ldflags "$LDFLAGS" -o "$TARGET" "$ROOT/cmd"
if [[ -z "${BUILD_TARGET:-}" ]]; then
	ln -sf azichrome-tui "$ROOT/bin/tapefox"
fi

if [[ "$ACTION" == install ]]; then
	destination=$("$GOBINARY" env GOBIN)
	if [[ -z "$destination" ]]; then
		destination=$("$GOBINARY" env GOPATH)/bin
	fi
	install -D -m 755 "$TARGET" "$destination/azichrome-tui"
	ln -sf azichrome-tui "$destination/tapefox"
fi
