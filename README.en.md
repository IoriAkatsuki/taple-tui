# taple-tui

An Apple Music TUI player for desktop systems, with Linux support and experimental Windows/macOS ports. It renders a blue-and-silver Walkman cassette deck (TPS-L2 proportions) in real time with terminal block characters. The name combines **Tap**e, App**le** (Apple Music), and **TUI**.

[简体中文](README.md)

The public repository contains release source snapshots, required build files and demo assets, not development history, engineering records or local sign-in data.

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

## Build and run

Requires a graphical desktop, Go 1.26 or newer, and Chrome (or Chromium with the required DRM support). Full playback requires an Apple Music account and subscription. The default build enables Vulkan compute with explicit CPU fallback when a physical GPU or driver is unavailable.

Linux builds require a C compiler and Vulkan headers (`gcc vulkan-headers` on Fedora; `build-essential libvulkan-dev` on Debian/Ubuntu). Runtime requires the Vulkan loader and a GPU driver.

```sh
make build
./bin/taple-tui                   # Start player (also creates ./bin/tapefox symlink)
./bin/taple-tui --pure            # Use a temporary config and browser profile
./bin/taple-tui --help
```

Run `CGO_ENABLED=0 make build` to build a CPU-only binary. Press `s` to view the active GPU name or CPU fallback status.

### Windows / PowerShell (experimental)

Download the Windows ZIP from [Releases](https://github.com/IoriAkatsuki/taple-tui/releases), extract it, and run `./taple-tui.exe` in PowerShell inside Windows Terminal. Install desktop Chrome and a Vulkan-capable GPU driver. PowerShell is the launch shell, not the graphics backend.

Source builds require Go and the MSYS2 UCRT64 packages `mingw-w64-ucrt-x86_64-gcc` and `mingw-w64-ucrt-x86_64-vulkan-headers`. Add `C:\msys64\ucrt64\bin` to PATH:

```powershell
$env:CC = 'gcc'
$env:CGO_ENABLED = '1'
./hack/build.ps1 build
./bin/taple-tui.exe
```

### macOS (experimental)

Builds target Apple Silicon (arm64) and Intel (amd64). MoltenVK translates Vulkan to Metal. Downloaded binaries also require MoltenVK; GPU runtimes are not bundled. MoltenVK must support `VK_EXT_layer_settings`: the client disables Metal Argument Buffers for its own instance to avoid initialization crashes on affected drivers. Unmet requirements produce an explicit CPU fallback.

```sh
brew install go vulkan-headers molten-vk
export CGO_CFLAGS="-I$(brew --prefix vulkan-headers)/include"
make build
./bin/taple-tui
```

Building requires Xcode Command Line Tools. Packages are not Apple-notarized or Windows code-signed. Verify SHA256 checksums; do not globally disable OS security protections.

### Experimental platform limits

- Windows/macOS retain a dedicated Chrome profile and private pipes, never a CDP TCP port. They use normal/requested-minimized Chrome windows to retain normal browser/DRM behavior; fully hidden playback is not guaranteed.
- System-output VU metering is currently Linux-only. Other platforms report it as unavailable without microphone capture or synthetic levels; playback controls remain available.
- Native CI checks compilation, unit tests and the Chrome pipe against a disposable blank page. It does not validate real-account DRM playback or physical GPU rendering. Successful CPU fallback is not a successful GPU test.

### Build and install RPM package

```sh
make rpm
sudo dnf install ./dist/taple-tui-*.rpm         # Fedora / RHEL
sudo zypper install ./dist/taple-tui-*.rpm      # openSUSE
```

The RPM package installs `/usr/bin/taple-tui`, the `/usr/bin/tapefox` symlink, and `/usr/share/applications/taple-tui.desktop`.

## Controls

- `/` or `1`: focus search input; arrow keys or `j/k`: move selection; `Enter` or click a track: play.
- `1/2/3` or `Tab`: switch between Search, Library, and Queue drawers; `a`: append the selected song to the queue.
- Search `4/5/6`: select Songs, Albums, or Artists. Library `4/5/6/7/8`: select Playlists, Albums, Songs, Recommendations, or Personal Station.
- `Enter`: open an album, artist, or playlist; `Backspace`: return to the parent list; `P`: play the selected collection from the start; `r`: refresh; `l`: load more.
- `i`: toggle Browse and Listen scenes; `z` or click the deck: toggle close-up lens.
- `Space`: play or pause; `p/n`: previous or next track; `←/→`: seek backward or forward; `-/+`: adjust volume.
- Listen scene `↑/↓`, `j/k`, `PgUp/PgDn`, or mouse wheel: scroll lyrics manually; `f`: resume auto-follow; `y`: reload lyrics.
- `v` or click the VU meter: toggle the VU meter on or off.
- `b`: open a visible browser window to sign in, then return to the playback session (Chrome may remain visible on Windows/macOS), or open the "Clear local sign-in" dialog when signed in (`y` confirms, `Esc`/`n` cancels, `Shift+B` forces open).
- `s`: open the controls and settings panel (switches between English and Chinese); `Esc`: close panel, exit close-up, or return to the parent view; `q` or `Ctrl+C`: quit.

## Configuration and local paths

The client reads settings from `go-musicfox/config.toml` under the OS configuration directory, or `$MUSICFOX_ROOT/config.toml`. Linux uses `$XDG_CONFIG_HOME` (default `~/.config`), macOS uses `~/Library/Application Support`, and Windows uses `%APPDATA%`:

```toml
[main]
motion = "full"       # full / subtle / off
blockGlyphs = "auto"  # auto / half / quadrant / sextant / octant / braille
frameRate = 60        # 1..240
```

If the file does not exist, the client uses the defaults above.

- Browser profile: Chrome communicates over `--remote-debugging-pipe` and stores its dedicated profile in `tapefox/apple-music/chrome` under the OS configuration directory. Unix uses mode `0700`; Windows uses a protected ACL granting access only to the current user. Personal browser profiles are not read.
- Audio metering: on Linux the VU meter runs `parec` against PulseAudio or PipeWire `@DEFAULT_MONITOR@` to measure system-mix RMS dBFS.

## Development

```sh
go test ./...
go test -race ./internal/applemusic ./internal/ui
go list -deps ./cmd
make rpm
```

Run `make vendor` after modifying `third_party/foxful-cli` to keep `vendor/` synchronized.

## Credits and license

Derived from [tapefox](https://github.com/Egdon/tapefox). The initial TUI foundation comes from [go-musicfox](https://github.com/go-musicfox/go-musicfox) and [foxful-cli](https://github.com/anhoder/foxful-cli) by [@anhoder](https://github.com/anhoder). The Go module path remains `github.com/go-musicfox/go-musicfox`.

Built with the following open-source projects:

- [Charmbracelet](https://charm.land/) ([Bubble Tea v2](https://github.com/charmbracelet/bubbletea), [Bubbles v2](https://github.com/charmbracelet/bubbles), [Lipgloss v2](https://github.com/charmbracelet/lipgloss), [Ultraviolet](https://github.com/charmbracelet/ultraviolet), [x/ansi](https://github.com/charmbracelet/x)) for terminal cell buffers and event loops.
- [lucasb-eyer/go-colorful](https://github.com/lucasb-eyer/go-colorful) and [fogleman/ease](https://github.com/fogleman/ease) for color interpolation and easing curves.
- [pelletier/go-toml/v2](https://github.com/pelletier/go-toml), [gookit/gcli/v2](https://github.com/gookit/gcli), [mattn/go-runewidth](https://github.com/mattn/go-runewidth), and [rivo/uniseg](https://github.com/rivo/uniseg) for TOML parsing, CLI routing, and CJK glyph width calculation.
- Khronos Vulkan Compute, Chromium, PulseAudio / PipeWire, and the [LINUX DO community](https://linux.do/).

Licensed under [GPL-3.0-only](LICENSE).
