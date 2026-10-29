#!/usr/bin/env bash
# Build the Linux/macOS Apple Music client; Chrome owns playback and DRM.
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"
GOBINARY=${GOBINARY:-go}
ACTION=${1:-build}
BUILD_GOOS=${GOOS:-$("$GOBINARY" env GOOS)}

case "$BUILD_GOOS" in
	linux|darwin) ;;
	*) echo "Use hack/build.ps1 for Windows; unsupported GOOS: $BUILD_GOOS" >&2; exit 1 ;;
esac
case "$ACTION" in
	build|install) ;;
	*) echo "Usage: $0 [build|install]" >&2; exit 1 ;;
esac

VERSION=${VERSION:-$(git -C "$ROOT" describe --tags --always --dirty=-dev 2>/dev/null || printf dev)}
if [[ ! "$VERSION" =~ ^[a-zA-Z0-9][a-zA-Z0-9._+-]*$ ]]; then
	echo "VERSION must contain only letters, digits, dots, underscores, plus or minus" >&2
	exit 1
fi
TARGET=${BUILD_TARGET:-$ROOT/bin/taple}
LDFLAGS="${LDFLAGS:-} -X github.com/go-musicfox/go-musicfox/internal/types.AppVersion=$VERSION"
mkdir -p "$(dirname "$TARGET")"
CGO_ENABLED=${CGO_ENABLED:-1} GOOS="$BUILD_GOOS" "$GOBINARY" build -trimpath -buildvcs=false -ldflags "$LDFLAGS" -o "$TARGET" "$ROOT/cmd"

if [[ "$ACTION" == install ]]; then
	destination=$("$GOBINARY" env GOBIN)
	if [[ -z "$destination" ]]; then
		destination=$("$GOBINARY" env GOPATH)
		destination=${destination%%:*}/bin
	fi
	mkdir -p "$destination"
	install -m 755 "$TARGET" "$destination/taple"
fi
