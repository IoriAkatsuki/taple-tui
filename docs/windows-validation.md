# Windows / PowerShell 本机验证

验证日期：2026-10-27。源码基线：`6efb3d8`，另含本次发布脚本、CI 与文档改动。

## 环境

- Windows 10.0.26300，amd64；PowerShell 7.6.7。
- Go 1.26.10（便携 SDK，官方 SHA-256 校验通过）。
- MSYS2 UCRT64 GCC 16.2.0、Vulkan headers 1.4.363。
- Intel Arc 130V GPU，驱动 32.0.101.9150。
- Chrome for Testing 156.0.8078.47（仅使用独立临时 profile，不使用用户登录数据）。

Go SDK、测试浏览器与测试日志保存在被 Git 忽略的 `.devenv/` 中；MSYS2 Vulkan headers 安装在现有 MSYS2 环境。未修改用户 PATH。

## 结果

| 检查 | 结果 |
| --- | --- |
| `go test ./...` | 通过，含 Windows 原生管道与 profile ACL 测试；条件性测试仍按自身规则跳过 |
| `go test -race ./internal/applemusic ./internal/ui` | 通过 |
| `go vet ./...` | 通过 |
| `TAPEFOX_REQUIRE_VULKAN=1` + `TestVulkanWalkmanMatchesCPU` | 通过，实际使用 Intel Arc GPU，未以“无硬件跳过”代替 |
| `TAPEFOX_CHROME_SMOKE=1` + `TestChromePrivatePipeSmoke` | 通过，验证私有管道、退出、临时 profile 清理与重启 |
| `hack/release-windows.ps1 -Version v1.1.0-rc.1` | 原生 CGO EXE 与 ZIP 打包成功 |
| ZIP 校验与含空格路径解压运行 | 通过 |
| PATH 仅含 Windows 系统目录时 `--version` / `--help` | 通过，不依赖构建工具 DLL 路径 |
| ConPTY 中通过 PowerShell 启动便携 EXE `--pure` | 通过，持续产生 TUI 帧；`s` 打开设置，输出显示 `Render: Vulkan GPU · Intel(R) Arc(TM) 130V GPU (16GB)` |
| ConPTY 键盘 `Esc`、`q` | 通过，PowerShell 捕获程序退出码 0 |
| PowerShell 脚本语法 / Workflow YAML 解析 / `git diff --check` | 通过 |

ConPTY 测试 PATH 仅含便携 Chrome 与 Windows 系统目录，没有 Go / MSYS2。测试范围为编译、自动化测试、GPU、浏览器通信与终端交互；账户登录与订阅播放由用户完成。

## 产物

`dist/` 中生成：

- `taple-tui_v1.1.0-rc.1_windows_amd64.exe`（31,595,740 bytes）
- `taple-tui_v1.1.0-rc.1_windows_amd64.zip`（10,692,518 bytes，含 EXE、双语 README、LICENSE）
- 两个文件各自的 `.sha256`

EXE SHA-256：`c8444644635de0a74c52dceb43aa9706a5648567f068658232233aad9139e4d4`

ZIP SHA-256：`67961c7cde848dce7c87a216e8ebe521dec566df336a58a00fa9d9b0635d3771`

便携 EXE 无需管理员权限安装。正式版使用 `v1.1.0` 标签，由 CI 在各平台原生构建。

## 发布流程

- 新增 `hack/release-windows.ps1`，用于 PowerShell 原生构建并生成 ZIP、独立 EXE、校验文件。
- 原生构建工作流增加 Windows PowerShell 测试与 vet。
- 推送新的 `v*` 标签时，所有原生平台构建通过后创建草稿 Release、上传校验后的包与独立 Windows EXE，再公开发布；带连字符的版本标记为 prerelease。
- 普通分支及手动构建仍仅生成 Actions artifacts。

Linux 同时提供 DEB、RPM 与 tar.gz；macOS 提供 arm64 与 amd64；Windows 提供 EXE 与 ZIP。

详细本机日志：`.devenv/logs/{test,race,vet,gpu,chrome,build-package,tui-summary}.log`。`tui-conpty.log` 是含 ANSI 控制序列的原始终端输出，不包含账户登录数据。
