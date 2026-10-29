# taple

An Apple Music TUI player for desktop systems, supporting Linux, Windows, and macOS. It renders a blue-and-silver Walkman cassette deck (TPS-L2 proportions) in real time with terminal block characters. The name combines **Tap**e and App**le** (Apple Music).

[简体中文](README.md) · [Releases](https://github.com/IoriAkatsuki/taple-tui/releases)

![Cassette insertion and close-up](docs/assets/taple-insert.gif)

## Features

- Catalog search and browse: search songs, albums, and artists; open artist → album → track hierarchies; paginate results; append tracks to the queue or jump within the first 50 queue entries.
- Personal library: browse and play playlists, albums, songs, recommendation groups, and the personal station. Press `P` to play a full album or playlist in order. Confirmed playback enables native Apple Music autoplay.
- 3D storage crates and deck: songs and queues use a dual-row cassette rack (single half-peek, shell style assigned on pickup); albums use a vinyl LP crate; playlists and artists use a 3D hinged dossier folder crate. The desk stage shifts across dawn, day, sunset, and night.
- Synced lyrics and metering: track changes load TTML lyrics with smooth roller scrolling and an active particle wavefront. The bottom deck shows a Nixie time counter and a dual-channel dBFS VU meter.

| Cassette rack pickup and insertion | Library crates (Vinyl LP / 3D Folder / Queue) |
| :---: | :---: |
| ![Cassette storage rack](docs/assets/apple-music-storage.gif) | ![Library crates](docs/assets/taple-library-crates.gif) |

| Listen scene (spinning reels, synced lyrics, VU meter) | Track swap, settings, and account dialogs |
| :---: | :---: |
| ![Listen scene and synced lyrics](docs/assets/taple-listen-lyrics.gif) | ![Track swap and settings](docs/assets/taple-swap-settings.gif) |

## Installation and build

Current source defaults to a windowless Chromium runtime; a full Chrome installation is optional. Initial interactive sign-in still needs a graphical session. Full playback requires an active Apple Music subscription, AAC decoding, and Widevine support in the selected runtime. **Headless DRM playback is experimental: successful startup or codec negotiation is not proof of protected playback.** Linux/Windows use Vulkan compute; macOS uses native Metal, with CPU fallback when unavailable.

The current `refactor/taple-identity` branch uses `taple` for the product, command, and new build artifacts, and `TAPLE_*` for project environment variables. This naming change has not been released. See [migration rules (Chinese)](docs/naming.md). The headless/native-Metal changes are not included in the published v1.1.0 binaries; see [runtime and validation notes (Chinese)](docs/headless-runtime.md).

### 1. Historical prebuilt releases (v1.2.0 and earlier)

Choose an existing version from [GitHub Releases](https://github.com/IoriAkatsuki/taple-tui/releases) (checksums are listed in `SHA256SUMS`). The table retains the real `taple-tui` filenames and commands for historical packages; available platforms and files depend on the selected Release. It does not announce new `taple` packages. For environment variables in historical binaries, consult the documentation at the corresponding tag.

| Platform | Arch | Historical release artifact | Install / Run historical package |
| :--- | :--- | :--- | :--- |
| Linux (Fedora / RHEL / openSUSE) | `x86_64` | `taple-tui-*.x86_64.rpm` | `sudo dnf install ./taple-tui-*.rpm` or `sudo zypper install ./taple-tui-*.rpm` |
| Linux (Debian / Ubuntu) | `x86_64` | `taple-tui_*_amd64.deb` | `sudo apt install ./taple-tui_*_amd64.deb` |
| Linux (Portable archive) | `x86_64` | `taple-tui_*_linux_amd64.tar.gz` | Extract and run `./taple-tui` |
| Windows | `x86_64` | `taple-tui_*_windows_amd64.zip` | Extract and run `./taple-tui.exe` in Windows Terminal (PowerShell) |
| Windows (Standalone EXE) | `x86_64` | `taple-tui_*_windows_amd64.exe` | Run `& .\taple-tui_<version>_windows_amd64.exe` in PowerShell; no installation needed |
| macOS (Apple Silicon) | `arm64` | `taple-tui_*_darwin_arm64.tar.gz` | Extract and run `./taple-tui`; the old v1.1.0 GPU backend needs MoltenVK |
| macOS (Intel) | `x86_64` | `taple-tui_*_darwin_amd64.tar.gz` | Extract and run `./taple-tui`; the old v1.1.0 GPU backend needs MoltenVK |

### 2. Build from current source

Requires Go 1.26 or newer. These commands build this branch's `taple`, not the historical packages listed above.

#### Linux

Install a C compiler and Vulkan headers (`gcc vulkan-headers` on Fedora; `build-essential libvulkan-dev` on Debian/Ubuntu):

```sh
make build
./bin/taple                       # Start player
./bin/taple --pure                # Use a temporary config and browser profile
CGO_ENABLED=0 make build          # Build a CPU-only binary
```

#### Windows (PowerShell)

Install Go and the MSYS2 UCRT64 packages `mingw-w64-ucrt-x86_64-gcc` and `mingw-w64-ucrt-x86_64-vulkan-headers`. Add `C:\msys64\ucrt64\bin` to `PATH`:

```powershell
$env:CC = 'gcc'
$env:CGO_ENABLED = '1'
./hack/build.ps1 build
./bin/taple.exe
```

The EXE is a portable console application, not a graphical installer. Run it from PowerShell / Windows Terminal. No Go, MSYS2, or build tools are needed at runtime. Binaries are unsigned; a compatible browser runtime and an active Apple Music subscription are still required (v1.1.0 uses Chrome).

Test and build Windows release candidates from the current branch (ZIP, standalone EXE, and SHA-256 checksums). Replace `vX.Y.Z` with a real, unpublished version:

```powershell
go test ./...
go vet ./...
./hack/release-windows.ps1 -Version vX.Y.Z
Get-FileHash ./dist/taple_vX.Y.Z_windows_amd64.exe -Algorithm SHA256
```

Pushing a `v*` tag publishes a GitHub Release only after all native platform checks succeed; branch builds produce Actions artifacts only.

#### macOS

Install Xcode Command Line Tools and Go. Current source links the system Foundation and Metal frameworks directly; Vulkan headers and MoltenVK are not needed:

```sh
xcode-select --install
brew install go
make build
./bin/taple
```

### 3. Package release archives

```sh
make package                      # Build dist/taple_<ver>_<os>_<arch>.tar.gz (or .zip) and .sha256
make rpm                          # Build dist/taple-*.rpm on Linux
```

### Import a local cookie file

Import only your own Apple Music cookies into the dedicated application profile:

```sh
chmod 600 "$HOME/apple-music-cookies.json"
./taple --import-cookies "$HOME/apple-music-cookies.json"
```

Netscape cookie jars and browser JSON arrays are supported. The player starts after import, without requesting playback. An already authorized profile is not overwritten. The source file is unchanged and cookie values are never included in import errors or logs. **Accepted cookie writes do not prove authorization, subscription access or DRM playback.** Keep credential files outside the repository and never attach them to issues. See [format, Windows ACL and session rules](https://github.com/IoriAkatsuki/taple-tui/blob/main/docs/cookie-import.md).

### Platform notes

- Window management: all platforms use Chromium `--headless=new`, not minimized windows. Only explicit sign-in opens a visible window; successful sign-in hands the same private profile back to headless playback. DRM failure never silently opens a window. Linux background startup no longer requires `DISPLAY` / `WAYLAND_DISPLAY`, but a working audio output and valid session are still required.
- Audio metering: system-mix VU metering is currently supported on Linux via `parec` (PulseAudio / PipeWire).
- GPU acceleration: press `s` to inspect the backend (Metal / Vulkan), GPU name, or CPU fallback. macOS uses a windowless Metal compute queue and MSL generated from the same SPIR-V at development time, with no runtime Vulkan translation. Native macOS compilation and GPU validation must be performed on a Mac.

## Controls

- `/` or `1`: focus search input; arrow keys or `j/k`: move selection; `Enter` or click a track: play.
- `1/2/3` or `Tab`: switch between Search, Library, and Queue drawers; `a`: append the selected song to the queue.
- Search `4/5/6`: select Songs, Albums, or Artists. Library `4/5/6/7/8`: select Playlists, Albums, Songs, Recommendations, or Personal Station.
- `Enter`: open an album, artist, or playlist; `Backspace`: return to the parent list; `P`: play the selected collection from the start; `r`: refresh; `l`: load more.
- `i`: toggle Browse and Listen scenes; `z` or click the deck: toggle close-up lens.
- `Space`: play or pause; `p/n`: previous or next track; `←/→`: seek backward or forward; `-/+`: adjust volume.
- Listen scene `↑/↓`, `j/k`, `PgUp/PgDn`, or mouse wheel: scroll lyrics manually; `f`: resume auto-follow; `y`: reload lyrics.
- `v` or click the VU meter: toggle the VU meter on or off.
- `b`: open a visible browser window to sign in, or open the "Clear local sign-in" dialog when signed in (`y` confirms, `Esc`/`n` cancels, `Shift+B` forces open).
- `s`: open the controls and settings panel (switches between English and Chinese); `Esc`: close panel, exit close-up, or return to the parent view; `q` or `Ctrl+C`: quit.

## Configuration and local paths

The client reads settings from `go-musicfox/config.toml` under the OS configuration directory, or `$TAPLE_ROOT/config.toml` (`$XDG_CONFIG_HOME` or `~/.config` on Linux, `~/Library/Application Support` on macOS, `%APPDATA%` on Windows):

```toml
[main]
motion = "full"       # full / subtle / off
blockGlyphs = "auto"  # auto / half / quadrant / sextant / octant / braille
frameRate = 60        # 1..240
```

If the file does not exist, the client uses the defaults above. The `go-musicfox` configuration directory and the `tapefox/apple-music/chrome` profile below are retained compatibility paths; this rename does not move or delete them. The current branch does not fall back to old environment variables. Rename keys without changing their values; see [migration rules (Chinese)](docs/naming.md).

- Runtime selection: explicit `TAPLE_BROWSER` executable → `runtime/chromium` beside the player executable → system Chromium → legacy Chrome installations. An invalid override fails rather than silently selecting another browser. Keep the runtime's complete resources and license notices; current release scripts do not bundle Chromium or Widevine.
- Browser profile: Chromium communicates over `--remote-debugging-pipe` and retains the existing `tapefox/apple-music/chrome` profile under the OS configuration directory (mode `0700` on Unix; current-user-only ACL on Windows).
- Audio metering: on Linux, the VU meter runs `parec` against PulseAudio or PipeWire `@DEFAULT_MONITOR@` to measure system-mix RMS dBFS.

## Development

```sh
go test ./...
go test -race ./internal/applemusic ./internal/ui
go list -deps ./cmd
make package
```

Run `make vendor` after modifying `third_party/foxful-cli` to keep `vendor/` synchronized.

## Credits and license

Derived from [tapefox](https://github.com/Egdon/tapefox). The initial TUI foundation comes from [go-musicfox](https://github.com/go-musicfox/go-musicfox) and [foxful-cli](https://github.com/anhoder/foxful-cli) by [@anhoder](https://github.com/anhoder). The Go module path remains `github.com/go-musicfox/go-musicfox`.

Built with the following open-source projects:

- [Charmbracelet](https://charm.land/) ([Bubble Tea v2](https://github.com/charmbracelet/bubbletea), [Bubbles v2](https://github.com/charmbracelet/bubbles), [Lipgloss v2](https://github.com/charmbracelet/lipgloss), [Ultraviolet](https://github.com/charmbracelet/ultraviolet), [x/ansi](https://github.com/charmbracelet/x)) for terminal cell buffers and event loops.
- [lucasb-eyer/go-colorful](https://github.com/lucasb-eyer/go-colorful) and [fogleman/ease](https://github.com/fogleman/ease) for color interpolation and easing curves.
- [pelletier/go-toml/v2](https://github.com/pelletier/go-toml), [gookit/gcli/v2](https://github.com/gookit/gcli), [mattn/go-runewidth](https://github.com/mattn/go-runewidth), and [rivo/uniseg](https://github.com/rivo/uniseg) for TOML parsing, CLI routing, and CJK glyph width calculation.
- Khronos Vulkan Compute, SPIRV-Cross, Apple Metal, Chromium, PulseAudio / PipeWire, and the [LINUX DO community](https://linux.do/).

Licensed under [GPL-3.0-only](LICENSE).
