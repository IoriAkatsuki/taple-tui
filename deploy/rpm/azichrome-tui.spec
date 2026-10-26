# Packages the binary that hack/rpm.sh builds with `make build`; the
# script passes the version in deck_version.
%global debug_package %{nil}
# Snapshots carry no %%changelog to date the build from.
%global source_date_epoch_from_changelog 0

Name:           azichrome-tui
Version:        %{deck_version}
Release:        1%{?dist}
Summary:        Apple Music in a Linux desktop terminal with a Walkman deck renderer
License:        GPL-3.0-only
URL:            https://github.com/IoriAkatsuki/azichrome-tui
Source0:        azichrome-tui
Source1:        LICENSE
Source2:        README.md
Source3:        README.en.md
Source4:        azichrome-tui.desktop
ExclusiveArch:  x86_64 aarch64

Provides:       tapefox = %{version}-%{release}
Recommends:     pulseaudio-utils
Recommends:     vulkan-loader
Suggests:       (chromium or google-chrome-stable)

%description
azichrome-tui (evolved from tapefox) is an Apple Music client for Linux
desktop terminals featuring a real-time 3D ray-cast Walkman cassette deck,
cassette storage rack, TTML particle-synced lyrics, and dBFS VU metering.
Playback and DRM use a separately installed Chrome-compatible desktop browser.

%prep
cp -p %{SOURCE1} %{SOURCE2} %{SOURCE3} .

%install
install -Dpm 0755 %{SOURCE0} %{buildroot}%{_bindir}/azichrome-tui
ln -sf azichrome-tui %{buildroot}%{_bindir}/tapefox
install -Dpm 0644 %{SOURCE4} %{buildroot}%{_datadir}/applications/azichrome-tui.desktop

%files
%license LICENSE
%doc README.md README.en.md
%{_bindir}/azichrome-tui
%{_bindir}/tapefox
%{_datadir}/applications/azichrome-tui.desktop
