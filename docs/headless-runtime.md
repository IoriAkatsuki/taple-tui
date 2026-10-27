# 独立无头运行时与原生 Metal（开发版）

本页描述当前源码，不表示已发布的 v1.1.0 已更新，也不表示已完成三平台 DRM 播放验证。

## Chromium、FFmpeg 与 DRM 的边界

- 可以用独立 Chromium helper 承载官方 Apple Music 网页，不必要求用户安装完整 Chrome。当前采用独立进程 + 私有 CDP 管道，并不是把 CEF 动态库嵌入 Go 进程。
- FFmpeg 有开源 AAC 解码器；解码器不是必须依赖 Google Chrome 的理由。但 Chromium 构建需要启用对应 codec，不能直接把任意版本的 FFmpeg 动态库塞进去当插件。
- Chromium / FFmpeg 不包含可任意分发的 Widevine CDM。CDM 接口、获准分发的 CDM、运行平台以及服务端认可的播放环境是不同条件。开启 Chromium 的相关编译选项不等于获得 CDM。
- 无头模式保留 sandbox、音频和 GPU；不加 `--no-sandbox`、`--mute-audio`、`--disable-gpu`，不伪装浏览器身份、不导出凭据、不绕过 DRM。
- CEF/Chromium 仍然是多进程网页运行时，去掉 Chrome 的可见界面不会让它自动缩小到一个解码库。WebKitGTK 也不能仅凭更小体积就视为 Apple Music DRM 的等价替代。

## 使用独立运行时

可先指定兼容的 Chromium 可执行文件（路径包含空格时正常引用，不要附加命令行参数）：

```sh
TAPEFOX_BROWSER=TARGET ./bin/taple-tui
```

将 `TARGET` 替换为运行时可执行文件的绝对路径。也可把**完整运行时目录**放在播放器可执行文件旁边：

```text
Linux:   runtime/chromium/chromium 或 runtime/chromium/chrome
Windows: runtime/chromium/chromium.exe 或 runtime/chromium/chrome.exe
macOS:   runtime/chromium/Chromium.app/Contents/MacOS/Chromium
```

保留运行时附带的资源、helper、库、许可证与第三方声明。当前打包脚本仍仅发布播放器，不自动下载或再分发 Chromium、codec 库或 Widevine。运行时选择顺序为显式路径、随程序目录、系统 Chromium、系统 Chrome（旧安装兼容）；错误的显式路径直接报错。

当前登录交接需要能以可见模式启动的 Chromium；仅提供 `chrome-headless-shell` 的包不能承担此流程，不能视为直接替代品。

后台统一使用 `--headless=new`；只有用户明确请求登录时启动可见窗口，完成后沿用同一独立 profile 切回无头。无 Linux 图形会话时按 `b` 会解释首次登录限制，不停止已有后台进程。登录需要同一用户可访问的会话/密钥环；不能承诺复制 profile 到另一台无桌面机器就可用。音频输出仍发生在运行播放器的机器上，SSH 本身不转发声音。

## 分层验证（不要混淆）

1. **进程/IPC**：验证无头标志、私有管道、退出、profile 权限和重启；不需要账户。
2. **媒体能力**：在本地 loopback 页面检查 AAC `canPlayType` 与 Widevine EME 配置协商；不请求歌曲或许可证，不登录 Apple Music。
3. **完整播放**：用本人订阅及专用已登录 profile，验证完整歌曲、实际进度、暂停/跳转/切歌；此项会改变播放队列。

Linux/macOS 的前两层检查：

```sh
TAPEFOX_BROWSER=TARGET \
TAPEFOX_CHROME_SMOKE=1 TAPEFOX_BROWSER_CAPABILITIES=1 \
go test -v ./internal/applemusic -run '^TestChromePrivatePipeSmoke$' -count=1
```

可额外设 `TAPEFOX_REQUIRE_WIDEVINE=1`，让能力不足直接判失败。`AAC="probably"` 仅是浏览器声明，`Widevine=true` 也只表示能协商该配置，不证明获得 Apple Music 许可证或成功出声。全曲集成测试需用户先登录并明确选择执行：

```sh
TAPEFOX_APPLEMUSIC_LIVE=1 go test -v ./internal/applemusic -run '^TestAppleMusicBrowserIntegration$' -count=1
```

## macOS 原生系统 API

Darwin 选择 `Metal.framework` + `Foundation.framework`，使用 `MTLCreateSystemDefaultDevice`、compute pipeline、command queue 和 shared buffers，不创建 NSApplication、窗口或 CAMetalLayer，不依赖 MoltenVK。

开发时用 SPIRV-Cross 将已验证的 `walkman.comp.spv` 转为 `walkman.metal`；MSL 随程序嵌入，由系统 Metal 编译。SPIRV-Cross 不是运行时依赖。更新 GLSL 后运行 `go generate ./internal/ui/deck`（libshaderc + spirv-cross）；仅重新生成 MSL 可运行 `python3 internal/ui/deck/compile_metal.py`。

在 Mac 上执行：

```sh
CGO_ENABLED=1 go test ./...
CGO_ENABLED=1 TAPEFOX_REQUIRE_GPU=1 go test -v ./internal/ui/deck/... \
  -run '^(TestGPUWalkmanMatchesCPU|TestGPUCloseRejectsFurtherWork|TestMetalRoundTripAndClose)$' -count=1
```

无 Metal 设备或未启用 CGO 时明确回退 CPU；原生着色器/管线错误不应被测试误报为“缺少硬件而跳过”。Windows/Linux 继续使用 Vulkan。Vulkan 是开放标准，驱动并非都开源；Darwin 内核也不自带 Vulkan 实现。

## 本次本地验证范围

- Windows 原生 `go test ./...`、`go vet ./...`、播放器/渲染相关 `-race` 测试通过；实际 Intel Arc GPU 的 CPU/GPU 帧一致性和关闭回退测试通过。
- `CGO_ENABLED=0` 全量测试及 Linux amd64、Darwin arm64/amd64 交叉构建通过；这些是 CPU 构建，**不验证原生 Metal 的编译或运行**。
- Vulkan C loader 回归与更新后的 GitHub Actions `actionlint` 检查通过。
- Windows：无头 Chromium 私有管道、profile 清理/重启通过；测试使用临时 profile，不读取用户登录数据。
- 本机已有 Chrome for Testing 156.0.8078.47 运行时（临时干净 profile，测试禁用组件更新）：AAC 返回 `probably`，Widevine 返回 `NotSupportedError`。这不是“FFmpeg 无法解码”，也不能推广为所有 Chromium 构建都不支持 Widevine。组件可用性仍需用实际部署的运行时另测。
- 未完成 Apple Music 已授权全曲播放；未在 Linux 实机或 Mac 原生 Metal 设备上运行本次改动。macOS CI 已改为原生 SDK 构建，但本地改动尚未触发远端 CI。

详细本地日志：`.devenv/logs/headless-{capabilities,metal-test,metal-vet,metal-race,metal-gpu,metal-cpu}.log`（Git 忽略，不包含账户资料）。
