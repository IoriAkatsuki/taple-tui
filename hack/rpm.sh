#!/usr/bin/env bash
# Builds an RPM of this checkout into dist/, for Fedora and other RPM
# distributions. Needs rpmbuild (rpm-build) and what `make build` needs:
# Go. Chrome is a runtime requirement, not a native build dependency.
set -euo pipefail
cd "$(dirname "$0")/.."

make build

# RPM prereleases use ~; subsequent hyphens are version-component separators.
base=$(git describe --tags --abbrev=0 2>/dev/null || echo v0)
base=${base#v}
base=${base/-/\~}
base=${base//-/.}
version="${base}^$(git log -1 --date=format:%Y%m%d --format=%cd)git$(git rev-parse --short HEAD)"

top=$(mktemp -d)
trap 'rm -rf "$top"' EXIT
mkdir -p "$top/SOURCES" dist
cp bin/taple-tui LICENSE README.md README.en.md deploy/taple-tui.desktop "$top/SOURCES/"
rpmbuild -bb --quiet --define "_topdir $top" --define "deck_version $version" deploy/rpm/taple-tui.spec
find "$top/RPMS" -name '*.rpm' -exec cp {} dist/ \; -exec basename {} \;
