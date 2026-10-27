# taple-tui

### Windows portable releases

Windows releases provide a ZIP and a standalone `taple-tui_<version>_windows_amd64.exe`, plus SHA-256 checksums. Run the EXE from PowerShell / Windows Terminal; it is a console application, not an installer. No Go or MSYS2 installation is needed at runtime. Chrome and an Apple Music subscription are still required. Binaries are unsigned.

From a configured Windows build environment, run `go test ./...`, `go vet ./...`, then `./hack/release-windows.ps1 -Version v1.1.0`. Pushing a `v*` tag publishes a GitHub Release only after all native platform checks succeed; branch builds produce Actions artifacts only.

An Apple Music TUI player for desktop systems, supporting Linux, Windows, and macOS. It renders a blue-and-silver Walkman cassette deck (TPS-L2 proportions) in real time with terminal block characters. The name combines **Tap**e, App**le** (Apple Music), and **TUI**.

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

Runtime requires a graphical desktop, Chrome (or Chromium with Widevine DRM), and an active Apple Music subscription. The client enables Vulkan compute acceleration by default and falls back to CPU rendering when no compatible GPU driver is available.

### 1. Prebuilt releases

Download the archive or package for your platform from [GitHub Releases](https://github.com/IoriAkatsuki/taple-tui/releases) (checksums are listed in `SHA256SUMS`):

| Platform | Arch | Release artifact | Install / Run |
| :--- | :--- | :--- | :--- |
| Linux (Fedora / RHEL / openSUSE) | `x86_64` | `taple-tui-*.x86_64.rpm` | `sudo dnf install ./taple-tui-*.rpm` or `sudo zypper install ./taple-tui-*.rpm` |
| Linux (Debian / Ubuntu) | `x86_64` | `taple-tui_*_amd64.deb` | `sudo apt install ./taple-tui_*_amd64.deb` |
| Linux (Portable archive) | `x86_64` | `taple-tui_*_linux_amd64.tar.gz` | Extract and run `./taple-tui` |
| Windows | `x86_64` | `taple-tui_*_windows_amd64.zip` | Extract and run `./taple-tui.exe` in Windows Terminal (PowerShell) |
| macOS (Apple Silicon) | `arm64` | `taple-tui_*_darwin_arm64.tar.gz` | Optional `brew install molten-vk` for GPU; extract and run `./taple-tui` |
| macOS (Intel) | `x86_64` | `taple-tui_*_darwin_amd64.tar.gz` | Optional `brew install molten-vk` for GPU; extract and run `./taple-tui` |

### 2. Build from source

Requires Go 1.26 or newer.

#### Linux

Install a C compiler and Vulkan headers (`gcc vulkan-headers` on Fedora; `build-essential libvulkan-dev` on Debian/Ubuntu):

```sh
make build
./bin/taple-tui                   # Start player (also creates ./bin/tapefox symlink)
./bin/taple-tui --pure            # Use a temporary config and browser profile
CGO_ENABLED=0 make build          # Build a CPU-only binary
```

#### Windows (PowerShell)

Install Go and the MSYS2 UCRT64 packages `mingw-w64-ucrt-x86_64-gcc` and `mingw-w64-ucrt-x86_64-vulkan-headers`. Add `C:\msys64\ucrt64\bin` to `PATH`:

```powershell
$env:CC = 'gcc'
$env:CGO_ENABLED = '1'
./hack/build.ps1 build
./bin/taple-tui.exe
```

#### macOS

Install Xcode Command Line Tools, then install Vulkan headers and MoltenVK (Vulkan-to-Metal translation) via Homebrew:

```sh
brew install go vulkan-headers molten-vk
export CGO_CFLAGS="-I$(brew --prefix vulkan-headers)/include"
make build
./bin/taple-tui
```

### 3. Package release archives

```sh
make package                      # Build dist/taple-tui_<ver>_<os>_<arch>.tar.gz (or .zip) and .sha256
make rpm                          # Build dist/taple-tui-*.rpm on Linux
```

### Platform notes

- Window management: Linux runs Chrome in an Ozone headless virtual window. Windows and macOS launch a dedicated Chrome window (requested minimized) to preserve standard DRM playback.
- Audio metering: system-mix VU metering is currently supported on Linux via `parec` (PulseAudio / PipeWire).
- GPU acceleration: press `s` to inspect the active GPU name or CPU fallback status. On macOS, the client uses `VK_EXT_layer_settings` to disable Metal Argument Buffers for its own instance.

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

The client reads settings from `go-musicfox/config.toml` under the OS configuration directory, or `$MUSICFOX_ROOT/config.toml` (`$XDG_CONFIG_HOME` or `~/.config` on Linux, `~/Library/Application Support` on macOS, `%APPDATA%` on Windows):

```toml
[main]
motion = "full"       # full / subtle / off
blockGlyphs = "auto"  # auto / half / quadrant / sextant / octant / braille
frameRate = 60        # 1..240
```

If the file does not exist, the client uses the defaults above.

- Browser profile: Chrome communicates over `--remote-debugging-pipe` and stores its dedicated profile in `tapefox/apple-music/chrome` under the OS configuration directory (mode `0700` on Unix; current-user-only ACL on Windows).
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
- Khronos Vulkan Compute, MoltenVK, Chromium, PulseAudio / PipeWire, and the [LINUX DO community](https://linux.do/).

Licensed under [GPL-3.0-only](LICENSE).
