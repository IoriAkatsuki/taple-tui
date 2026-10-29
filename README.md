# taple

面向桌面的 Apple Music 终端播放器，使用块字符实时渲染蓝银 Walkman 磁带卡座（TPS-L2 比例）。支持 Linux、Windows 与 macOS。名称结合 **Tap**e（磁带）与 App**le**（Apple Music）。

[English](README.en.md) · [Releases 下载](https://github.com/IoriAkatsuki/taple-tui/releases)

![装带入仓与自动近景](docs/assets/taple-insert.gif)

## 功能与界面

- 曲库搜索与浏览：支持歌曲、专辑、歌手搜索，支持歌手→专辑→歌曲层级浏览、分页加载、队列末尾追加与前 50 项跳播。
- 个人资料库：支持歌单、专辑、歌曲、推荐分组与个人电台浏览及播放，支持按 `P` 顺序播放整张专辑或歌单，点播成功后开启原生自动续播。
- 拟物容器与卡座：歌曲与队列使用双行磁带收纳架（单张半弹出、拿起显色），专辑使用黑胶唱片箱，歌单与歌手使用 3D 铰链档案夹；桌面背景随早、中、傍晚、夜间四个时段变化。
- 同步歌词与电平：曲目切换后自动读取 TTML 歌词，聆听页提供滚筒滚动与行内进度粒子；底部提供双声道 dBFS 电平表与数码管计时器。

| 磁带收纳架与取带换带 | 资料库（黑胶唱片箱 / 3D 档案夹 / 队列） |
| :---: | :---: |
| ![磁带收纳架](docs/assets/apple-music-storage.gif) | ![资料库与容器切换](docs/assets/taple-library-crates.gif) |

| 聆听页（带盘运转 / 同步歌词 / VU 电平） | 切歌换带、设置与账户面板 |
| :---: | :---: |
| ![聆听页同步歌词与电平](docs/assets/taple-listen-lyrics.gif) | ![切歌与设置面板](docs/assets/taple-swap-settings.gif) |

## 安装与运行

开发版默认使用无窗口的 Chromium 运行时，不强制安装完整 Chrome；首次交互登录仍需图形会话。完整播放需要有效的 Apple Music 订阅，以及运行时的 AAC 解码和 Widevine DRM 支持。**无头 DRM 播放仍属实验性：进程启动或 AAC 检测成功不等于能播放受保护歌曲。** Linux/Windows 默认使用 Vulkan Compute，macOS 直接使用系统 Metal，无可用 GPU 时回退 CPU。

> 当前 `refactor/taple-identity` 分支的名称、命令与新构建产物统一为 `taple`，项目环境变量统一为 `TAPLE_*`；本轮命名变更尚未发布。迁移规则见 [命名说明](docs/naming.md)。以下无头运行时与原生 Metal 说明针对当前源码；已发布的 v1.1.0 二进制不包含这些新改动，详见 [运行时与验证说明](docs/headless-runtime.md)。

### 1. 下载历史 Release 预编译包（v1.2.0 及之前）

前往 [GitHub Releases](https://github.com/IoriAkatsuki/taple-tui/releases) 选择已有版本（含 `SHA256SUMS` 校验文件）。下表保留历史包的真实 `taple-tui` 文件名与启动命令；具体平台和文件以对应 Release 为准，不表示已有 `taple` 新包。历史包的旧环境变量行为以对应 tag 文档为准：

| 平台 | 架构 | 历史 Release 软件包 | 历史包安装与启动方式 |
| :--- | :--- | :--- | :--- |
| Linux (Fedora / RHEL / openSUSE) | `x86_64` | `taple-tui-*.x86_64.rpm` | `sudo dnf install ./taple-tui-*.rpm` 或 `sudo zypper install ./taple-tui-*.rpm` |
| Linux (Debian / Ubuntu) | `x86_64` | `taple-tui_*_amd64.deb` | `sudo apt install ./taple-tui_*_amd64.deb` |
| Linux (通用二进制) | `x86_64` | `taple-tui_*_linux_amd64.tar.gz` | 解压后运行 `./taple-tui` |
| Windows | `x86_64` | `taple-tui_*_windows_amd64.zip` | 解压后在 Windows Terminal (PowerShell) 运行 `./taple-tui.exe` |
| Windows (独立 EXE) | `x86_64` | `taple-tui_*_windows_amd64.exe` | 无需安装，在 PowerShell 中用 `& .\taple-tui_<版本>_windows_amd64.exe` 启动 |
| macOS (Apple Silicon) | `arm64` | `taple-tui_*_darwin_arm64.tar.gz` | 解压后运行 `./taple-tui`；v1.1.0 的旧 GPU 后端需 MoltenVK |
| macOS (Intel) | `x86_64` | `taple-tui_*_darwin_amd64.tar.gz` | 解压后运行 `./taple-tui`；v1.1.0 的旧 GPU 后端需 MoltenVK |

### 2. 从当前源码构建

需要 Go 1.26 或更高版本。以下命令生成本分支的 `taple`，不是上表历史包的命令。

#### Linux

安装 C 编译器和 Vulkan headers（Fedora：`gcc vulkan-headers`；Debian/Ubuntu：`build-essential libvulkan-dev`）：

```sh
make build
./bin/taple                       # 启动播放器
./bin/taple --pure                # 使用临时配置与临时浏览器目录
CGO_ENABLED=0 make build          # 构建纯 CPU 版本
```

#### Windows (PowerShell)

安装 Go 以及 MSYS2 UCRT64 的 `mingw-w64-ucrt-x86_64-gcc` 与 `mingw-w64-ucrt-x86_64-vulkan-headers`，并将 `C:\msys64\ucrt64\bin` 加入 `PATH`：

```powershell
$env:CC = 'gcc'
$env:CGO_ENABLED = '1'
./hack/build.ps1 build
./bin/taple.exe
```

EXE 是命令行便携程序，不是图形安装器；运行时无需 Go、MSYS2 或编译工具。建议在 Windows Terminal 中启动，而不是双击。发布包未进行代码签名，Windows 可能显示未知发布者提示。兼容的浏览器运行时和有效 Apple Music 订阅仍需单独准备（v1.1.0 使用 Chrome）。

在 PowerShell 中测试并生成当前分支的 Windows 发布候选产物（ZIP、独立 EXE 和各自的 SHA-256 校验文件；将 `vX.Y.Z` 替换为尚未发布的实际版本号）：

```powershell
go test ./...
go vet ./...
./hack/release-windows.ps1 -Version vX.Y.Z
Get-FileHash ./dist/taple_vX.Y.Z_windows_amd64.exe -Algorithm SHA256
```

维护者推送 `v*` 版本标签后，GitHub Actions 会在所有原生平台检查通过后自动发布 Release，包含 Windows EXE / ZIP 与 `SHA256SUMS`；普通分支构建只上传 Actions artifacts，不创建 Release。

#### macOS

安装 Xcode Command Line Tools 和 Go。当前源码直接链接系统 Foundation / Metal，不需要 Vulkan headers 或 MoltenVK：

```sh
xcode-select --install
brew install go
make build
./bin/taple
```

### 3. 打包发布产物

```sh
make package                      # 生成当前平台的 dist/taple_<ver>_<os>_<arch>.tar.gz (或 .zip) 及 .sha256
make rpm                          # 在 Linux 上构建 dist/taple-*.rpm
```

### 本地 Cookie 导入

可将自己的 Apple Music Cookie 文件导入程序专用会话，无需把凭证交给第三方：

```sh
chmod 600 "$HOME/apple-music-cookies.json"
./taple --import-cookies "$HOME/apple-music-cookies.json"
```

支持 Netscape Cookie Jar 和浏览器 JSON 数组；导入后继续启动 TUI，不自动播放。已授权会话拒绝混入，输入文件不改写，Cookie 值不进入日志。**导入成功不等于账号授权或 DRM 播放成功。** Windows ACL、允许的域、过期与会话规则见 [Cookie 导入说明](https://github.com/IoriAkatsuki/taple-tui/blob/main/docs/cookie-import.md)。文件应放在仓库外，勿上传到 issue 或聊天。

### 平台差异说明

- 窗口管理：三平台后台均使用 Chromium `--headless=new`，不是最小化窗口；不会因 DRM 失败自动弹窗。仅按 `b` 登录时使用可见窗口，登录完成后切回无头进程。Linux 后台启动不再要求 `DISPLAY` / `WAYLAND_DISPLAY`，但仍需可用音频输出与有效会话。
- 音频电平：系统混音 VU 电平表目前仅支持 Linux（依赖 `parec` 与 PulseAudio / PipeWire）。
- GPU 加速：按 `s` 查看实际后端（Metal / Vulkan）、GPU 名称或 CPU 回退原因。macOS 使用无窗口 Metal compute queue；着色器由同一份 SPIR-V 在开发时生成 MSL，无运行时 Vulkan 转译。原生 macOS 编译与设备测试需在 Mac 上完成。

## 常用操作

- `/` 或 `1`：聚焦搜索框；方向键或 `j/k` 移动光标；`Enter` 或单击曲目播放。
- `1/2/3` 或 `Tab`：切换搜索、资料库、队列抽屉；`a` 将当前选中歌曲追加到播放队列末尾。
- 搜索页 `4/5/6`：切换歌曲、专辑、歌手；资料库 `4/5/6/7/8`：切换歌单、专辑、歌曲、推荐、个人电台。
- `Enter` 打开专辑、歌手或歌单；`Backspace` 返回上一层；`P` 从头播放当前集合；`r` 刷新列表；`l` 加载更多。
- `i`：切换浏览与聆听场景；`z` 或点击机身：切换局部近景。
- `Space`：播放/暂停；`p/n`：上一首/下一首；`←/→`：快退/快进；`-/+`：调节音量。
- 聆听页 `↑/↓`、`j/k`、`PgUp/PgDn` 或滚轮：手动滚动歌词；`f`：恢复歌词自动跟随；`y`：重新加载歌词。
- `v` 或点击电平表区域：开启或关闭 VU 电平表。
- `b`：未登录时打开可见浏览器窗口登录；已登录时打开“清空本机登录”确认框（按 `y` 确认，`Esc`/`n` 取消，`Shift+B` 强制打开）。
- `s`：打开操作与设置面板（可切换中/英文）；`Esc`：逐级关闭面板、退出近景或返回上层；`q` 或 `Ctrl+C`：退出。

## 配置与本地路径

程序从系统配置目录下的 `go-musicfox/config.toml` 或 `$TAPLE_ROOT/config.toml` 读取设置（Linux 为 `$XDG_CONFIG_HOME` 默认 `~/.config`，macOS 为 `~/Library/Application Support`，Windows 为 `%APPDATA%`）：

```toml
[main]
motion = "full"       # full / subtle / off
blockGlyphs = "auto"  # auto / half / quadrant / sextant / octant / braille
frameRate = 60        # 1..240
```

配置文件缺失时直接使用上述默认值。`go-musicfox` 配置目录与下述 `tapefox/apple-music/chrome` 是保留的兼容磁盘路径，不因更名而迁移或删除。当前分支不回退读取旧环境变量；迁移时只改键名，不改原有值，见 [命名说明](docs/naming.md)。

- 运行时选择：`TAPLE_BROWSER` 显式路径 → 可执行文件旁的 `runtime/chromium` → 系统 Chromium → 兼容旧安装的 Chrome。显式路径错误时不悄悄换浏览器。运行时须包含完整资源文件，不是单独复制一个可执行文件；当前发布脚本不会打包 Chromium 或 Widevine。
- 浏览器会话：Chromium 通过 `--remote-debugging-pipe` 私有管道通信，独立 profile 仍位于上述系统配置目录下的 `tapefox/apple-music/chrome`（Unix 权限 `0700`，Windows 使用当前用户独占 ACL），兼容旧会话路径。
- 音频采样：Linux 下 VU 电平表通过 `parec` 读取 PulseAudio / PipeWire 的 `@DEFAULT_MONITOR@` 系统混音输出并计算 RMS dBFS。

## 开发与测试

```sh
go test ./...
go test -race ./internal/applemusic ./internal/ui
go list -deps ./cmd
make package
```

修改 `third_party/foxful-cli` 后需运行 `make vendor` 同步 `vendor/` 目录。

## 致谢与许可

本项目派生自 [tapefox](https://github.com/Egdon/tapefox)，早期 TUI 框架源自 [go-musicfox](https://github.com/go-musicfox/go-musicfox) 与 [foxful-cli](https://github.com/anhoder/foxful-cli)（作者 [@anhoder](https://github.com/anhoder)）。Go 模块路径保留 `github.com/go-musicfox/go-musicfox`。

感谢以下开源项目与社区：

- [Charmbracelet](https://charm.land/)（[Bubble Tea v2](https://github.com/charmbracelet/bubbletea)、[Bubbles v2](https://github.com/charmbracelet/bubbles)、[Lipgloss v2](https://github.com/charmbracelet/lipgloss)、[Ultraviolet](https://github.com/charmbracelet/ultraviolet)、[x/ansi](https://github.com/charmbracelet/x)）：终端渲染缓冲与事件循环。
- [lucasb-eyer/go-colorful](https://github.com/lucasb-eyer/go-colorful) 与 [fogleman/ease](https://github.com/fogleman/ease)：色彩插值与缓动曲线。
- [pelletier/go-toml/v2](https://github.com/pelletier/go-toml)、[gookit/gcli/v2](https://github.com/gookit/gcli)、[mattn/go-runewidth](https://github.com/mattn/go-runewidth) 与 [rivo/uniseg](https://github.com/rivo/uniseg)：配置解析、命令行路由与宽字符排版。
- Khronos Vulkan Compute、SPIRV-Cross、Apple Metal、Chromium、PulseAudio / PipeWire 以及 [LINUX DO 社区](https://linux.do/)。

采用 [GPL-3.0-only](LICENSE) 许可证发布。
