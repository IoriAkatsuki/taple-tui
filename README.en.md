# azichrome-tui

An **Apple Music-only TUI for Linux desktops**, featuring a classic blue-and-silver Walkman-inspired 3D cassette deck rendered in real time with terminal block characters.

> **Name Origin (`azichrome-tui`)**: **Azi**muth (precision tape-head azimuth alignment) + **Chrome** (Type II $\text{CrO}_2$ chromium-dioxide tape & Chromium native debugging pipe) + **TUI** (Terminal User Interface).
>
> Evolved directly from **[tapefox](https://github.com/Egdon/tapefox)** and built on the foundational TUI architecture of **[go-musicfox](https://github.com/go-musicfox/go-musicfox)**.

[简体中文](README.md)

This repository publishes only the current source snapshot, tests and demo assets, without development history, intermediate engineering records or local sign-in data. Future releases must use independent snapshots rather than push development branches or tags.

---

## Screenshots & Demos

*All screenshots and animated GIFs below are exported directly from the real `internal/ui` terminal cell framebuffer without external post-processing.*

### 1. Cassette Insertion & Automatic Close-up Timeline (`1280ms` Insertion + Camera Zoom)

![Cassette Insertion & Close-up Demo](docs/assets/azichrome-insert.gif)

### 2. Dual Main Scenes & Close-up Views

| Browse Scene (Cassette Storage Rack + 3D Deck) | Listen Scene (3D Deck + TTML Particle-Synced Lyrics) |
| :---: | :---: |
| ![Browse Scene](docs/assets/azichrome-browse.png) | ![Listen Scene](docs/assets/azichrome-listen.png) |

| Device Close-up (Cassette Window & Reels) | Cassette Storage Rack (Single Half-Peek & Random Shell Reveal) |
| :---: | :---: |
| ![Close-up View](docs/assets/azichrome-closeup.png) | ![Storage Rack Animation](docs/assets/apple-music-storage.gif) |

---

## Status & Scope

- **Catalog & Search**: Connected official catalog song/album/artist search, artist → albums → tracks browsing with pagination, transport controls, queue append and jumps within the first 50 entries, background Chrome session and visible login handover.
- **Personal Library**: Personal playlists, albums, songs, recommendation groups and personal station with pagination, back/refresh and typed `playParams` playback. Library IDs are never conflated with catalog IDs.
- **Dual Scenes & Mechanical UI**: Browse and Listen scenes, a uniform slate-blue cassette storage rack with one half-ejected pickup at a time, randomized shell materials revealed only upon confirmed pickup, continuous old-tape ejection and new-tape insertion, standby breathing glow, device-local close-up and persistent transport controls. Visual timelines never delay playback requests or fabricate backend success.
- **Synced TTML Lyrics**: Track changes prefetch TTML lyrics in the browser session, rendering smooth roller scrolling, dimmed unplayed lines, and an active particle wavefront (`·✧◇*`).
- **Continuous Playback & Autoplay**: Selecting a track inside an album or playlist preserves collection context and continues through the set; `P` plays from the beginning. Native Apple Music autoplay is enabled after confirmed playback and reported in the `s` settings panel.

---

## Build, RPM Packaging & Run

### Requirements

- **OS & Browser**: Linux graphical desktop with Chrome, or Chromium with the required DRM capability. Initial sign-in requires a visible browser window. Headless SSH, macOS and Windows are outside the supported scope. The client never extracts raw audio URLs or bypasses Apple Music account/subscription rules.
- **Build Toolchain**: Go 1.26. The default Vulkan-enabled build requires a C compiler and Vulkan headers (`gcc vulkan-headers` on Fedora; `build-essential libvulkan-dev` on Debian/Ubuntu). Runtime requires the Vulkan loader and a physical GPU driver, with automatic CPU fallback.

### Build from Source

```sh
make build
./bin/azichrome-tui               # Metering enabled by default; lyrics load on track changes (./bin/tapefox symlink included)
./bin/azichrome-tui --pure        # Temporary config and Chrome profile
./bin/azichrome-tui --help
```

### Build & Install RPM Package (Fedora / openSUSE / RHEL)

The repository includes `deploy/rpm/azichrome-tui.spec` and `hack/rpm.sh`, packaging `/usr/bin/azichrome-tui`, the compatibility symlink `/usr/bin/tapefox`, the desktop entry `/usr/share/applications/azichrome-tui.desktop`, and weak dependencies (`pulseaudio-utils`, `vulkan-loader`):

```sh
# Build dist/azichrome-tui-<version>.x86_64.rpm locally
make rpm

# Install on Fedora / RHEL
sudo dnf install ./dist/azichrome-tui-*.rpm

# Install on openSUSE Tumbleweed
sudo zypper install ./dist/azichrome-tui-*.rpm
```

### Runtime Notes

- `apple-music` is the only explicit subcommand and shares the default startup path. `--debug` enables debug logging.
- Default startup asynchronously initializes Vulkan compute on a physical discrete or integrated GPU for ray-casting the Walkman body and moving cassettes; output remains terminal characters. Press `s` to inspect the active GPU name or CPU fallback reason. Software Vulkan (`lavapipe`) is not treated as hardware acceleration. `CGO_ENABLED=0 make build` produces a CPU-only binary.
- Chrome is controlled over `--remote-debugging-pipe` with no listening TCP debug port, using `--ozone-platform=headless` in the background and switching to a visible window only during user-initiated sign-in. Only the application's own browser process is closed.

---

## Controls

- `/`, `1`, or search bar click: focus search; arrows or `j/k`: select; Enter or valid track click: play. Hover never starts playback.
- `4/5/6` or category clicks: songs/albums/artists. Enter/click opens an artist or album; Backspace or breadcrumb returns to the parent list; `l` loads more.
- `1/2/3` or `Tab`: switch search, library, and queue drawers; `a`: append the selected playable song to the playback queue (does not edit cloud playlists).
- In the library, `4/5/6/7/8` selects playlists/albums/songs/recommendations/personal station. Enter opens collections or plays songs/stations; `P` plays the selected/open collection, `r` refreshes, and `l` loads more.
- `i`: toggle Browse/Listen scenes; `z` or device click: toggle local close-up.
- In the Listen scene, arrows, `j/k`, `PgUp/PgDn` or mouse wheel scroll lyrics manually; `f` resumes follow and `y` explicitly retries loading.
- `Space`: play/pause; `p/n`: previous/next; `←/→`: seek request; `-/+`: volume.
- System-mix VU metering starts automatically. `v` or a VU-area click disables/re-enables it live. `VU OFF` means disabled; `VU ERR` means sampling failed (`v` retries).
- `b`: sign in (or open the "Clear local sign-in" confirmation dialog when already signed in; `Shift+B` or `B` in settings forces the dialog open); `s`: controls & settings panel (supports switching English/Chinese); `Esc`: close panel → exit search edit → zoom out → return to browse; `q` or `Ctrl+C`: quit while keeping sign-in.

---

## Configuration and Privacy

Only these settings are read from `$XDG_CONFIG_HOME/go-musicfox/config.toml` (default `~/.config/go-musicfox/config.toml`), or `$MUSICFOX_ROOT/config.toml`:

```toml
[main]
motion = "full"       # full / subtle / off
blockGlyphs = "auto"  # auto / half / quadrant / sextant / octant / braille
frameRate = 60        # 1..240
```

- Missing files use in-memory defaults; no user config is generated, upgraded or overwritten.
- The dedicated Chrome profile is stored at `$XDG_CONFIG_HOME/tapefox/apple-music/chrome` (mode `0700`), preserving compatibility with existing `tapefox` sessions. Cookies, tokens and passwords are never exported.
- "Clear local sign-in" wipes only the contents of the dedicated Chrome profile via `os.Root` while keeping the `0700` directory; it does not affect other browsers or cloud libraries.
- Startup samples PulseAudio/PipeWire's `@DEFAULT_MONITOR@` via `parec` (system mix, including other applications—never the microphone, never saved to disk).

---

## Development

```sh
go test ./...
go test -race ./internal/applemusic ./internal/ui
go list -deps ./cmd
make rpm
```

---

## Attribution & Acknowledgments

`azichrome-tui` is built upon the work of several open-source projects and communities:

### Lineage & Friend Links

- **[tapefox](https://github.com/Egdon/tapefox)**: The direct predecessor of `azichrome-tui`, establishing the Linux Apple Music bridge, dual-row cassette storage rack, 3D Walkman ray-caster, and Vulkan compute pipeline.
- **[go-musicfox](https://github.com/go-musicfox/go-musicfox)** (by [@anhoder](https://github.com/anhoder)): The original TUI music client foundation. The Go module path remains `github.com/go-musicfox/go-musicfox` for internal package stability (not a NetEase compatibility promise).
- **[foxful-cli](https://github.com/anhoder/foxful-cli)** (by [@anhoder](https://github.com/anhoder)): Provides the TUI application host and frame ticker architecture (maintained in-tree under `third_party/foxful-cli`).

### Open-Source Libraries & Ecosystem

- **[Charmbracelet Ecosystem](https://charm.land/)**: [Bubble Tea v2](https://github.com/charmbracelet/bubbletea), [Bubbles v2](https://github.com/charmbracelet/bubbles), [Lipgloss v2](https://github.com/charmbracelet/lipgloss), [Ultraviolet](https://github.com/charmbracelet/ultraviolet), and [x/ansi](https://github.com/charmbracelet/x) for terminal cell buffers, TrueColor styling, and event loops.
- **Graphics, Color & Easing**: [lucasb-eyer/go-colorful](https://github.com/lucasb-eyer/go-colorful), [fogleman/ease](https://github.com/fogleman/ease), and **Khronos Vulkan Compute** for 3D ray-cast shading and mechanical motion curves.
- **Configuration, CLI & Unicode Layout**: [pelletier/go-toml/v2](https://github.com/pelletier/go-toml), [gookit/gcli/v2](https://github.com/gookit/gcli), [mattn/go-runewidth](https://github.com/mattn/go-runewidth), and [rivo/uniseg](https://github.com/rivo/uniseg).
- **Desktop Runtime & Community**: **Chromium / Google Chrome** (`--remote-debugging-pipe` & Ozone Headless), **PulseAudio / PipeWire (`parec`)**, and the **[LINUX DO Community](https://linux.do/)** for feedback and support during the cassette deck TUI exploration.

### License & Disclaimer

Licensed under **[GPL-3.0-only](LICENSE)**.
This is an independent open-source project and is not affiliated with or endorsed by Apple, Sony, `tapefox`, or `go-musicfox`.
