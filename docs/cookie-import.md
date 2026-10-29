# 本地 Cookie 导入

以下命令针对当前 `refactor/taple-identity` 分支：使用 `taple` 与 `TAPLE_*`，本轮命名变更尚未发布。v1.2.0 及以前的历史包仍使用原名称；环境变量迁移见 [命名说明](naming.md)。

`--import-cookies FILE` 读取用户明确提供的本地文件，向本程序专用 Chromium profile 写入 Cookie，然后继续启动 TUI。它不读取个人浏览器数据库、不提供网页上传服务、不导出 Cookie，也不请求播放歌曲。

Cookie 是登录凭证。只导入自己的 Apple Music 会话，不要将文件提交到仓库、上传到聊天或附在 issue 中。程序不把输入文件名、Cookie 名称或值写入导入错误和日志；浏览器正常访问 Apple 时会按 Cookie 的作用域携带它们。

## Linux / macOS

把文件保存在仓库之外，权限限定为当前用户读写：

```sh
chmod 600 "$HOME/apple-music-cookies.json"
TAPLE_BROWSER=/absolute/path/to/chromium ./taple --import-cookies "$HOME/apple-music-cookies.json"
# 也可显式使用唯一播放器命令：
./taple apple-music --import-cookies "$HOME/apple-music-cookies.json"
```

普通文件必须由当前用户所有，不接受符号链接、目录、FIFO 或公开可读文件。程序不会修改或删除输入文件。无需再次导入时正常运行 `./taple`。

## Windows

使用当前用户专用的、禁止继承的文件 ACL。以下 PowerShell 示例只修改明确指定的导入文件权限，不修改目录或浏览器资料：

```powershell
$file = Join-Path $HOME 'apple-music-cookies.json'
$me = [System.Security.Principal.WindowsIdentity]::GetCurrent().User
$acl = [System.Security.AccessControl.FileSecurity]::new()
$acl.SetOwner($me)
$acl.SetAccessRuleProtection($true, $false)
$rule = [System.Security.AccessControl.FileSystemAccessRule]::new($me, 'FullControl', 'Allow')
$acl.AddAccessRule($rule)
Set-Acl -LiteralPath $file -AclObject $acl
.\taple.exe --import-cookies $file
```

导入通过打开的文件句柄检查属主、ACL 和 reparse 属性，不接受共享读取权限、目录、UNC 网络路径、设备路径或最终路径的 reparse point。

## 支持的格式与范围

- Netscape Cookie Jar：七个制表符分隔字段，支持 `#HttpOnly_`。到期时间 `0` 保留为会话 Cookie。
- JSON：浏览器导出的 Cookie 对象数组，支持 `name`、`value`、`domain`、`path`、`secure`、`httpOnly`、`hostOnly`、`sameSite`、`session`、`expires` 或 `expirationDate`。`expires`/`expirationDate` 使用 Unix 秒数；JSON 的 `-1` 表示会话，`0` 是已到期时间，不猜测毫秒单位。
- 只接受 `music.apple.com` 的 host-only/domain Cookie，以及该域继承的 `.apple.com` domain Cookie；不会自动扩大 host-only 范围，也不接收 iCloud、其他站点或分区 Cookie。
- 保留 Secure、HttpOnly、SameSite、路径及到期属性。`SameSite=None` 必须同时设置 Secure。
- 最多 1 MiB、256 项、每项名称与值合计 4096 字节；拒绝异常控制字符、冲突的到期属性和重复名称/域/路径。整份文件通过校验后才打开浏览器，过期条目会跳过；全部过期则拒绝。
- 不支持原始 `Cookie: ...` 请求头、命令行中的凭证字符串、其他站点的整库导出或浏览器 profile 目录。

## 会话保护与结果含义

- 已授权的程序专用 profile 拒绝再次导入，以免静默混入另一账号。需要更换时，先在账户面板明确选择“清空本机登录”，然后重新导入。
- 未授权 profile 中相同名称/域/路径的 Cookie 会由浏览器覆盖；其余资料不会被清空。输入文件必须经过用户筛选，不能把多个账号的 Cookie 混在一起。
- CDP 写入不保证事务原子性。写入超时/失败时，部分条目可能已生效；程序会明确报告，不自动重试或声称已回滚。成功写入但网页刷新失败也会单独说明。
- 会话 Cookie 不会被改成长效 Cookie。导入后继续运行 TUI，浏览器关闭时会话 Cookie 可能失效；`--pure` 使用一次性 profile，退出后不保留导入内容。
- **“已导入”只表示浏览器接受了写入，不表示已登录、订阅有效、取得 DRM 许可证或实际出声。** TUI 继续读取官方 MusicKit 授权和播放状态。Cookie 可能不足以恢复账号所需的全部本地状态；不成功时仍需官方交互登录。
- 独立 Chromium 运行时仍需 AAC 与合法可用的 Widevine CDM。系统 FFmpeg 不能替代 CDM；纯 SSH 也不会自动把服务器声音传回本机。

## 无账号回归测试

```sh
go test ./internal/applemusic ./cmd -run 'TestCookie' -count=1
TAPLE_COOKIE_SMOKE=1 go test ./internal/applemusic -run '^TestCookieImportSyntheticBrowserSmoke$' -count=1 -v
```

真实浏览器测试使用新的临时 profile 和固定合成 Cookie，验证作用域、会话/持久化及安全属性；不使用用户账号，不证明实际订阅播放。
