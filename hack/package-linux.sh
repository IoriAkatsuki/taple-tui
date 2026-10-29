#!/usr/bin/env bash
# Package the already-tested native Linux binary as DEB and RPM.
set -euo pipefail
cd "$(dirname "$0")/.."
version=${1:?Usage: hack/package-linux.sh vVERSION}
[[ "$version" =~ ^v?[0-9]+\.[0-9]+\.[0-9]+([.+-][a-zA-Z0-9.+-]+)?$ ]] || { echo 'Invalid version' >&2; exit 1; }
[[ "$(go env GOOS)" == linux && "$(go env GOARCH)" == amd64 ]] || { echo 'Linux amd64 required' >&2; exit 1; }
test -x bin/taple
package_version=${version#v}
package_version=${package_version/-/\~}
package_version=${package_version//-/.}
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
deb="$stage/deb"
install -Dm755 bin/taple "$deb/usr/bin/taple"
install -Dm644 deploy/taple.desktop "$deb/usr/share/applications/taple.desktop"
install -Dm644 LICENSE "$deb/usr/share/doc/taple/copyright"
install -m644 README.md README.en.md "$deb/usr/share/doc/taple/"
mkdir -p "$deb/DEBIAN" dist
cat > "$deb/DEBIAN/control" <<EOF
Package: taple
Version: $package_version
Architecture: amd64
Maintainer: IoriAkatsuki <124779309+IoriAkatsuki@users.noreply.github.com>
Section: sound
Priority: optional
Depends: libc6 (>= 2.34)
Recommends: pulseaudio-utils, libvulkan1
Homepage: https://github.com/IoriAkatsuki/taple-tui
Description: Apple Music terminal player with a Walkman cassette deck
 Real-time cassette deck, library browsing and synchronized lyrics.
 Playback uses a separately installed Chrome browser.
EOF
debfile="dist/taple_${version}_amd64.deb"
dpkg-deb --root-owner-group --build "$deb" "$debfile"
dpkg-deb --info "$debfile"
dpkg-deb --extract "$debfile" "$stage/deb-check"
[[ "$("$stage/deb-check/usr/bin/taple" --version)" == *"$version"* ]]
top="$stage/rpm"
mkdir -p "$top/SOURCES"
cp bin/taple LICENSE README.md README.en.md deploy/taple.desktop "$top/SOURCES/"
rpmbuild -bb --quiet --define "_topdir $top" --define "deck_version $package_version" deploy/rpm/taple.spec
mapfile -t rpms < <(find "$top/RPMS" -name '*.rpm')
test "${#rpms[@]}" -eq 1
rpmfile="dist/taple-${version#v}-1.x86_64.rpm"
cp "${rpms[0]}" "$rpmfile"
rpm -qpi "$rpmfile"
mkdir -p "$stage/rpm-check"
rpm_path="$PWD/$rpmfile"
(cd "$stage/rpm-check" && rpm2cpio "$rpm_path" | cpio -idm --quiet)
[[ "$("$stage/rpm-check/usr/bin/taple" --version)" == *"$version"* ]]
for file in "$debfile" "$rpmfile"; do
  (cd dist && sha256sum "$(basename "$file")" > "$(basename "$file").sha256")
done
