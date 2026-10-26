#!/usr/bin/env bash
# Builds an RPM of this checkout into dist/, for Fedora and other RPM
# distributions. Needs rpmbuild (rpm-build) and what `make build` needs:
# Go. Chrome is a runtime requirement, not a native build dependency.
set -euo pipefail
cd "$(dirname "$0")/.."

make build

# RPM versions have no hyphens: a snapshot of 5.1.0 is 5.1.0^<date>git<commit>.
base=$(git describe --tags --abbrev=0 2>/dev/null || echo v0)
version="${base#v}^$(git log -1 --date=format:%Y%m%d --format=%cd)git$(git rev-parse --short HEAD)"

top=$(mktemp -d)
trap 'rm -rf "$top"' EXIT
mkdir -p "$top/SOURCES" dist
cp bin/azichrome-tui LICENSE README.md README.en.md deploy/azichrome-tui.desktop "$top/SOURCES/"
rpmbuild -bb --quiet --define "_topdir $top" --define "deck_version $version" deploy/rpm/azichrome-tui.spec
find "$top/RPMS" -name '*.rpm' -exec cp {} dist/ \; -exec basename {} \;
