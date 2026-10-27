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
2. **媒体能力**：在本地 loopback 页面检查 AAC `canPlayType` 与 Widevine EME 配置协商；不请求歌曲或许可证，不登录 Apple Music。生产播放前还会在实际 `music.apple.com` 页面检查相同能力。
3. **完整播放**：用本人订阅及专用已登录 profile，验证完整歌曲、实际进度、暂停/跳转/切歌；此项会改变播放队列。

Linux/macOS 的前两层检查：

```sh
TAPEFOX_BROWSER=TARGET \
TAPEFOX_CHROME_SMOKE=1 TAPEFOX_BROWSER_CAPABILITIES=1 \
go test -v ./internal/applemusic -run '^TestChromePrivatePipeSmoke$' -count=1
```

可额外设 `TAPEFOX_REQUIRE_WIDEVINE=1`，让能力不足直接判失败。媒体探测保留浏览器正常的组件注册/更新路径；仅做 IPC 测试时才禁用后台网络及组件更新。Windows/macOS 的预装 CDM 也通过组件更新器注册，禁用它会产生假阴性。测试会为异步注册短暂重试，但不无限等待组件下载。

`AAC="probably"` 仅是浏览器声明，`Widevine=true` 也只表示能协商该配置，不证明获得 Apple Music 许可证或成功出声。全曲集成测试需用户先登录并明确选择执行：

```sh
TAPEFOX_APPLEMUSIC_LIVE=1 go test -v ./internal/applemusic -run '^TestAppleMusicBrowserIntegration$' -count=1
```

## 点歌后有歌词，但一直加载

歌词、队列设置和音频播放是独立阶段。`setQueue` 返回成功、歌词已显示、账号已登录，都不能证明 DRM 初始化成功或已经出声。

当前源码增加了以下保护：

- 已授权后、发送播放请求前检查 AAC 和 Widevine；能力不足时直接显示原因，不提交新播放队列。单次 EME 协商最多等待 3 秒；成功结果缓存 60 秒，失败结果缓存 5 秒，允许稍后显式重试。
- 暂停正在播放的歌曲不需要重新协商 DRM；普通浏览、歌词和登录也不受此检测阻塞。
- 设置队列后，如果 SDK 明确报告尚未播放，会等待 `play()` 的结果。浏览器的 `NotSupportedError` 会转成明确的运行时错误，不输出网页原始异常或会话数据。
- 音频启动确认最多等待 30 秒，从操作实际派发开始计时，不计入排队等待。超时后停止“加载中”并保留失败提示；不会自动重复点歌。后续收到匹配曲目及队列位置的真实播放状态，可以恢复界面。

### Windows 实测（2026-10-27 至 2026-10-28）

- 用户关闭播放器后，对明确选定的已登录会话仅查询快照和能力，没有发送播放、切歌或清理操作：`ready=true`、`authorized=true`、AAC 可用、Widevine 协商失败。
- 所用 Chrome for Testing 为 `156.0.8078.47`，已确认下载的原始 ZIP 也没有 Widevine 文件，并非仅解压遗漏。在另建的无账号测试 profile 中，官方组件更新器注册了 Widevine；等待 90 秒并重启后，CDM 目录仍无包文件，EME 仍返回 `NotSupportedError`。其他组件有成功安装记录，不能简单归因于更新器完全没有联网；为何未下发 CDM 尚未查明。
- 此实验只在隔离目录使用开发诊断开关 `--component-updater=fast-update`；**未加入生产参数，也未在用户会话中执行组件修补**。不复制其他浏览器的 CDM，不使用非官方 DLL，不关闭 sandbox 或 CDM 校验。
- Chromium 当前源码可为 Chrome for Testing 启用 Widevine；上述结果只描述本机该运行时和测试条件，不能推断所有 CfT 或所有无头浏览器均不支持 DRM。
- **修正之前的 Edge 探测结论**：原 IPC smoke 禁用了组件更新，同时阻止了 Windows CDM 注册，其失败不是 Edge 无头不支持 DRM 的证据。现已分离 IPC 与媒体测试的启动参数，并增加回归测试。
- 修正后，Edge `156.0.4314.42` 在独立无账号 profile、`--headless=new` 下，成功注册安装目录自带的 Widevine `4.10.3050.1`。本地 loopback 两轮启退和实际 `https://music.apple.com` 页面的 AAC/Widevine 检测均通过。没有复制 CDM，也没有打开用户 Edge profile 或跨浏览器迁移原登录数据。

因此，**无限加载的界面问题已修复并经过测试，已有通过无头媒体能力检查的运行时，但实际全曲播放尚未恢复验证**。FFmpeg 的开源 AAC 解码器不能替代 CDM。部署时需要在所选无头运行时中确认 AAC/EME 能力，再用本人订阅单独验证实际播放；不能静默退回可见 Chrome，也不能把不同浏览器的登录 profile 直接混用。

如需账户无关的官方页面能力检查，可显式运行：

```powershell
$env:TAPEFOX_BROWSER = 'TARGET'
$env:TAPEFOX_MEDIA_PREFLIGHT_SMOKE = '1'
go test -v ./internal/applemusic -run '^TestAppleMusicOriginMediaCapabilities$' -count=1
```

将 `TARGET` 换成运行时可执行文件绝对路径；测试使用新的临时 profile，不打开已有登录会话。如果官方页面未到达预期 origin，会单独报告加载超时，不把它当作 AAC/DRM 检测结果。本机 CfT 干净 profile 曾在这一阶段超时，不能据此宣称媒体检测通过；上述 Edge 官方页面检测则已通过。

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
- 项目内 Chrome for Testing `156.0.8078.47` 已用修正后的参数复测（临时干净 profile，保留正常组件注册）：两轮无头启退/私有管道通过，AAC 返回 `probably`，Widevine 均返回 `NotSupportedError`；另一次官方页面检查在 45 秒内未到达预期 origin。没有改用 Edge，也未请求全曲播放。
- 已推送的原生运行时改动触发了 [CI 40469118152](https://github.com/IoriAkatsuki/taple-tui/actions/runs/40469118152)：Windows、Darwin arm64 任务通过。Darwin amd64 的 Metal 编译和缓冲区往返测试通过，但 Apple Paravirtual GPU 帧一致性未通过（颜色 MAE `0.811`，阈值 `0.8`）；没有放宽容差。
- 同次 Linux CI 的可选 EME 探测遇到 Chromium 控制管道关闭，尚未修复；不能把该轮 CI 说成三平台全绿。上述历史 CI 不覆盖本次播放错误处理改动，新提交需以对应的 CI 结果为准。
- 本次播放修复的 Windows `go test ./...`、`go vet ./...` 和 `go test -race ./internal/applemusic ./internal/ui` 已通过。仍未完成 Apple Music 已授权全曲播放，也未在 Linux 实机或实体 Mac 上完成播放验证。

详细本地日志：`.devenv/logs/headless-{capabilities,metal-test,metal-vet,metal-race,metal-gpu,metal-cpu}.log`（Git 忽略，不包含账户资料）。
播放诊断和回归日志：`.devenv/logs/playback-*.log`；隔离组件更新记录：`.devenv/cdm-probe-*/result.log`；修正后的 Edge 记录：`.devenv/edge-cdm-probe-*/result.log`、`.devenv/logs/playback-edge-corrected-capabilities.log`（均不提交）。
