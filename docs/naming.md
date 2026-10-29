# taple 命名与环境变量迁移

本说明针对当前 `refactor/taple-identity` 分支。本轮命名变更尚未发布：产品名、命令及新构建产物统一为 `taple`（Windows 为 `taple.exe`），不生成其他旧名称的命令别名。v1.2.0 及以前的历史 Release 仍使用真实的 `taple-tui` 文件名；使用历史包时，以对应 tag 的文档为准。

## 环境变量只改键名，不改值

- `MUSICFOX_ROOT` 改为 `TAPLE_ROOT`，继续指向原来的自定义配置目录。
- 所有 `TAPEFOX_` 前缀的项目变量改为 `TAPLE_`，后缀与原有值保持不变。例如 `TAPEFOX_BROWSER` → `TAPLE_BROWSER`，浏览器可执行文件的路径不变；测试开关也采用同一规则。
- 内部父子进程标记同步为 `TAPLE_INTERNAL_CHILD`，由程序自动设置，用户无需导出。
- 当前分支只读取新键，不保留旧环境变量回退。请同步修改自己的 shell 配置、服务环境与本地测试命令；不要把环境变量值中的历史目录名一并替换。
- `PATH`、`HOME`、`XDG_CONFIG_HOME`、`APPDATA`、`DISPLAY`、`WAYLAND_DISPLAY`、`CGO_ENABLED` 等系统或工具链变量不属于项目命名，不改名。

## 保留的兼容路径与来源

- 系统配置目录下的 `go-musicfox/config.toml` 和程序专用 `tapefox/apple-music/chrome` profile 路径保持不变；不因更名而迁移或删除已有数据。已有配置内容不会因本次更名被改写。`--pure` 仍使用一次性目录；用户明确确认的“清空本机登录”行为不变。
- 新日志文件名为 `taple.log`；设置 `TAPLE_ROOT` 时写入其 `log/` 子目录，否则沿用现有状态目录下的 `go-musicfox/log/`。旧 `musicfox.log` 及其轮转文件不重命名、不清理。
- GitHub 仓库仍为 `IoriAkatsuki/taple-tui`，Go module 仍为 `github.com/go-musicfox/go-musicfox`，真实外部 URL 与上游致谢保留原名。
- 资源文件名保持不变。`docs/releases/`、`docs/windows-validation.md` 以及其他文档的既往测试结果保留历史事实，不作为新命名分支已经验证的证据。
