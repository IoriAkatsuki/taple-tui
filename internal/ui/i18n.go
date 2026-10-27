package ui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/go-musicfox/go-musicfox/internal/configs"
)

type i18nEntry struct {
	en string
	zh string
}

var i18nCatalog = map[string]i18nEntry{
	// Navigation / Tabs / Headers
	"scene.browse":      {"Browse", "浏览"},
	"scene.listen":      {"Listen", "聆听"},
	"nav.browse":        {"i Browse", "i 浏览"},
	"nav.listen":        {"i Listen", "i 聆听"},
	"location.search":   {"Search", "搜索"},
	"location.library":  {"Library", "资料库"},
	"location.queue":    {"Queue", "队列"},
	"location.track":    {"Track & Lyrics", "曲目与歌词"},
	"location.parent":   {"parent folder", "上层目录"},
	"focus.search":      {"Input: search", "输入：搜索"},
	"lens.close_up":     {"Lens: close-up", "镜头：近景"},
	"lens.moving":       {"Lens: moving", "镜头：移动中"},
	"back.editing":      {"Esc Leave input", "Esc 退出编辑"},
	"back.zoom":         {"Esc Zoom out", "Esc 拉远镜头"},
	"back.browse":       {"Esc Back to Browse", "Esc 返回浏览"},
	"back.loading":      {"Esc Cancel loading", "Esc 取消目录等待"},
	"back.parent":       {"Esc Back to %s", "Esc 返回 %s"},
	"nav.search":        {"1 Search", "1 搜索"},
	"nav.library":       {"2 Library", "2 我的资料库"},
	"nav.queue":         {"3 Queue", "3 队列"},
	"nav.browse_listen": {"i Browse/Listen", "i 浏览/聆听"},
	"nav.settings":      {"s Settings", "s 设置"},
	"drawer.search":     {"SEARCH / CASSETTE RACK", "搜索 / 磁带抽屉"},
	"drawer.library":    {"MY LIBRARY", "我的资料库"},
	"drawer.queue":      {"QUEUE · %d TRACKS", "队列 · %d 首"},
	"bezel.now_playing": {"NOW PLAYING · i DETAIL", "NOW PLAYING · i 详情"},
	"bezel.listening":   {"LISTENING · DETAIL", "LISTENING · DETAIL"},

	// Catalog Categories
	"cat.songs":           {"4 Songs", "4 歌曲"},
	"cat.albums":          {"5 Albums", "5 专辑"},
	"cat.artists":         {"6 Artists", "6 歌手"},
	"cat.playlists":       {"4 Playlists", "4 歌单"},
	"cat.lib_albums":      {"5 Albums", "5 专辑"},
	"cat.lib_songs":       {"6 Songs", "6 歌曲"},
	"cat.made_for_you":    {"7 Made For You", "7 推荐"},
	"cat.stations":        {"8 Stations", "8 电台"},
	"cat.back":            {"‹ Back", "‹ 返回"},
	"cat.refresh":         {"r Refresh", "r 刷新"},
	"cat.play_collection": {"P Play Collection", "P 播放歌单/专辑"},
	"cat.load_more":       {"l Load more", "l 加载更多"},

	// Library Titles
	"lib.playlists":    {"My Playlists", "我的歌单"},
	"lib.albums":       {"Library Albums", "资料库专辑"},
	"lib.songs":        {"Library Songs", "资料库歌曲"},
	"lib.made_for_you": {"Made For You", "为你推荐"},
	"lib.station":      {"Personal Station", "个人电台"},
	"type.album":       {"Album", "专辑"},
	"type.artist":      {"Artist", "歌手"},
	"type.playlist":    {"Playlist", "歌单"},
	"type.station":     {"Station", "电台"},
	"type.recommend":   {"Recommendation", "推荐"},

	// Transport & Account
	"account.sign_in":        {"b Sign In", "b 登录"},
	"account.signed_in":      {"Signed In · b Account", "已登录 · b 账户"},
	"account.connecting":     {"Connecting · b Sign In", "连接中 · b 登录"},
	"account.clear_title":    {"CLEAR LOCAL LOGIN?", "清空本机登录？"},
	"account.clear_body":     {"Stops playback and clears tapefox's Chrome login.\nYou must sign in again.\nOther browsers and cloud library stay unchanged.", "停止播放，清空 tapefox 专用浏览器的本机登录数据。\n之后必须重新登录。\n不影响其他浏览器、云端资料库或播放器设置。"},
	"account.clear_key":      {"B Clear login", "B 清空登录"},
	"account.cancel":         {"Esc Cancel", "Esc 取消"},
	"account.confirm":        {"y Clear & sign out", "y 清空并退出"},
	"account.clearing":       {"Clearing local login…", "正在清空本机登录…"},
	"account.clearing_body":  {"Stopping this player's browser and clearing its local session.\nPlease wait. No new playback will be submitted.", "正在停止本播放器的浏览器并清空本机会话。\n请稍候，期间不会提交新的播放请求。"},
	"account.cleared":        {"Local login cleared · b Sign in again", "本机登录已清空 · 按 b 重新登录"},
	"account.resize":         {"Resize to 40x12 to confirm; Esc cancels", "请放大到 40列×12行后确认；Esc 取消"},
	"status.restoring":       {"Restoring session · b Sign in", "正在恢复会话 · b 登录"},
	"status.signed_in":       {"Signed in; playing in background", "已登录；浏览器在后台播放"},
	"status.unauthorized":    {"Session unauthorized · b Sign in", "会话未授权 · b 登录"},
	"status.playing":         {"Playing", "正在播放"},
	"status.paused":          {"Paused", "已暂停"},
	"status.idle":            {"Not Playing", "尚未播放"},
	"status.track_unchanged": {"Current track retained", "保留当前曲目"},
	"status.loading":         {"Loading…", "正在加载…"},
	"status.play_failed":     {"Playback failed · select a track to retry", "播放失败 · 重新选曲重试"},
	"status.play_timeout":    {"Audio did not start within 30s; check Widevine, subscription and audio output. Select a track to retry.", "音频等待超过 30 秒；请检查 Widevine、订阅及音频输出，重新选曲重试。"},
	"status.connecting":      {"Connecting to Apple Music…", "正在连接 Apple Music…"},
	"status.sign_in_hint":    {"Press b to sign in and view library", "请按 b 登录后查看资料库"},
	"status.wait_backend":    {"Connecting to Apple Music…", "正在连接 Apple Music…"},
	"status.wait_region":     {"Getting region info…", "正在获取地区信息…"},
	"status.input_empty":     {"Please enter search query", "请输入搜索词"},
	"status.cat_loading":     {"Loading catalog…", "正在加载曲库…"},
	"status.cat_canceled":    {"Cancelled", "已取消"},
	"status.cat_returned":    {"Returned to %s", "已返回 %s"},
	"status.cat_loaded_n":    {"Loaded %d items", "已载入 %d 项"},
	"status.cat_more":        {" · l Load more", " · l 加载更多"},
	"status.cat_retry":       {"%s; r refresh / l retry", "%s；r 刷新 / l 重试"},
	"status.search_ready":    {"Search ready", "搜索已就绪"},
	"status.item_expired":    {"Selection expired, please choose again", "条目已失效，请重新选择"},
	"status.unplayable":      {"Item contains no playable audio", "该内容暂不可播放"},
	"status.select_first":    {"Select an album or playlist first", "未选择歌单或专辑"},
	"status.search_first":    {"Search and select a track first", "未选择播放曲目"},

	// Meter
	"meter.off":        {"VU meter disabled", "已关闭 VU 电平表"},
	"meter.enabling":   {"Enabling VU meter…", "正在开启 VU 电平表…"},
	"meter.on":         {"VU meter enabled", "已开启 VU 电平表"},
	"meter.restored":   {"VU meter restored", "VU 电平表已恢复"},
	"meter.error":      {"VU unavailable: %s; press v to retry", "VU 不可用：%s；v 重试"},
	"meter.need_pulse": {"Requires parec and PulseAudio/PipeWire", "需要 parec 和可连接的 PulseAudio/PipeWire"},
	"meter.platform":   {"System-output VU currently requires Linux; playback is unaffected", "系统输出 VU 暂仅支持 Linux；不影响播放"},
	"meter.lbl_off":    {"VU OFF · v Turn On", "VU OFF · v 开启"},
	"meter.lbl_dbfs":   {"VU METER · dBFS", "VU METER · dBFS"},
	"meter.lbl_ready":  {"VU METER · READY", "VU METER · READY"},
	"meter.lbl_init":   {"VU · INITIALIZING", "VU · 正在开启"},
	"meter.lbl_retry":  {"VU ERR · v Retry", "VU ERR · v 重试"},

	// Lyrics
	"lyrics.title":             {"LYRICS", "歌词"},
	"lyrics.focus":             {"Synced to playback", "逐行同步 · 真实进度"},
	"lyrics.plain":             {"Plain text · no timestamps", "纯文本 · 无时间戳"},
	"lyrics.paused":            {"Paused at real position", "已暂停 · 真实位置"},
	"lyrics.manual":            {"Browsing lyrics · follow off", "手动浏览 · 已停止跟随"},
	"lyrics.preview":           {"Preview · sync unavailable", "试听 · 暂不同步"},
	"lyrics.unconfirmed":       {"Playback unconfirmed · sync unavailable", "播放未确认 · 暂不同步"},
	"lyrics.clock_unavailable": {"Clock unavailable · sync suspended", "时钟过期或不可用 · 暂不同步"},
	"lyrics.btn_sync":          {"f Sync Line", "f 回到当前句"},
	"lyrics.btn_top":           {"f Top", "f 回到开头"},
	"lyrics.btn_retry":         {"y Retry Lyrics", "y 重试歌词"},
	"lyrics.auto_load":         {"Lyrics load automatically on playback", "播放歌曲后自动加载歌词"},
	"lyrics.loading":           {"Loading lyrics…", "正在读取歌词…"},
	"lyrics.sign_in":           {"Press b to sign in and read lyrics", "请按 b 登录后读取歌词"},
	"lyrics.wait_region":       {"Waiting for Apple Music region info", "等待 Apple Music 地区信息"},
	"lyrics.wait_conn":         {"Waiting for Apple Music connection", "等待 Apple Music 后台连接"},
	"lyrics.none":              {"No lyrics available for this track", "暂无歌词"},
	"lyrics.no_mapping":        {"No lyrics found for this track", "未找到对应歌词"},
	"lyrics.unsupported":       {"Lyrics not supported for this media", "当前音频不支持歌词"},
	"lyrics.auth_expired":      {"Lyrics authorization expired; check sign-in", "歌词授权已失效；请检查登录"},
	"lyrics.denied":            {"Lyrics access denied; verify subscription", "歌词访问被拒绝；请检查订阅和地区"},
	"lyrics.rate_limit":        {"Lyrics rate limited; press y later", "歌词请求受限；请稍后按 y 重试"},
	"lyrics.unavailable":       {"Lyrics unavailable", "歌词暂不可用"},
	"lyrics.timeout":           {"Lyrics request timed out; press y to retry", "歌词读取超时；按 y 重试"},
	"lyrics.stale":             {"Track changed · press y to retry", "曲目已变更 · 按 y 刷新"},
	"lyrics.req_error":         {"Failed to load lyrics; check network and press y", "歌词读取失败；检查网络后按 y 重试"},
	"lyrics.parse_error":       {"Unsupported lyrics format", "歌词格式不支持"},
	"lyrics.parse_detail":      {"Lyrics parse error: %s", "歌词解析失败：%s"},

	// Empty States
	"empty.queue":        {"Queue is empty", "队列为空"},
	"empty.loading":      {"Loading catalog…", "正在加载曲库…"},
	"empty.failed":       {"Loading failed, please retry", "加载失败，请重试"},
	"empty.no_results":   {"No results found", "没有搜索结果"},
	"empty.search_guide": {"Press / to search, Enter to play", "按 / 开始搜索，Enter 播放"},
	"empty.sign_in_lib":  {"Press b to sign in and view library", "请按 b 登录后查看资料库"},
	"empty.not_loaded":   {"Not loaded yet, press r to refresh", "尚未加载，按 r 刷新"},
	"empty.none":         {"No results", "没有结果"},

	// Help / Controls Panel
	"panel.title":                {"CONTROLS & SETTINGS", "操作与设置"},
	"panel.esc_close":            {"Esc Close", "Esc 关闭"},
	"panel.shortcuts":            {"Shortcuts: 1 Search  2 Library  3 Queue  / Find", "快捷键：1 搜索  2 资料库  3 队列  / 输入搜索"},
	"panel.categories":           {"Catalog: 4 Songs  5 Albums/Radio  6 Artists  l More", "分类：4 歌曲  5 专辑/电台  6 歌手  l 加载更多"},
	"panel.playback":             {"Playback: SPC Play/Pause  n/p Track  ←/→ Seek  a Append", "播放：SPC 播放/暂停  n/p 切歌  ←/→ 快进退  a 末尾追加"},
	"panel.views":                {"Views: i Browse/Listen  z Deck Zoom  f Sync Lyric  y Retry Lyric", "视图：i 浏览/聆听切换  z 设备近景  f 回到当前歌词  y 重试歌词"},
	"panel.motion_fmt":           {"Motion: %v · Glyphs: %v (config.toml)", "动效：%v · 字形：%v（config.toml）"},
	"panel.vu_desc":              {"VU Meter: Enabled by default, press v to toggle", "VU 电平表：默认开启，按 v 随时开关"},
	"panel.account_desc":         {"Account: b Sign in/account · B Clear local login", "账户：b 登录/账户 · B 清空本机登录"},
	"panel.render_fmt":           {"Render: %s", "渲染：%s"},
	"panel.autoplay_on":          {"Autoplay: ON · Apple Music continues the queue", "自动续播：已开启 · Apple Music 接续队列"},
	"panel.autoplay_pending":     {"Autoplay: enabled on next play", "自动续播：选曲播放时开启"},
	"panel.autoplay_unavailable": {"Autoplay: unavailable · album/playlist order still applies", "自动续播：SDK 不支持 · 专辑/歌单仍按顺序播放"},
	"panel.autoplay_wait":        {"Autoplay: waiting for Apple Music", "自动续播：等待 Apple Music 后台"},

	// Bottom bar hints
	"bar.search":   {"/ Search", "/ 搜索"},
	"bar.meter":    {"v Meter", "v 电平"},
	"bar.append":   {"a Append", "a 末尾追加"},
	"bar.zoom":     {"z Deck Zoom", "z 设备近景"},
	"bar.mode":     {"i Mode", "i 场景"},
	"bar.controls": {"o Controls", "o 操作"},
	"bar.back":     {"Esc Back", "Esc 返回"},
	"bar.quit":     {"q Quit", "q 退出"},

	// Input placeholder
	"input.search":     {"Search songs / albums / artists", "搜索歌曲 / 专辑 / 歌手"},
	"input.prompt":     {"/ Search songs / albums / artists", "/ 搜索歌曲 / 专辑 / 歌手"},
	"input.searching":  {"Searching…", "正在搜索…"},
	"input.btn_search": {"Search", "搜索"},
	"rack.unplayable":  {"[UNPLAYABLE]", "[不可播放]"},
}

var (
	testLangMu sync.RWMutex
	testLang   string
)

// SetLanguage sets the UI language dynamically ("en" or "zh").
func SetLanguage(lang string) {
	testLangMu.Lock()
	defer testLangMu.Unlock()
	switch strings.ToLower(lang) {
	case "zh":
		testLang = "zh"
	case "en":
		testLang = "en"
	default:
		testLang = "en"
	}
	if configs.AppConfig != nil {
		configs.AppConfig.Main.Language = testLang
	}
}

// currentLang returns the active language code: "en" (default) or "zh".
func currentLang() string {
	testLangMu.RLock()
	tl := testLang
	testLangMu.RUnlock()
	if tl != "" {
		return tl
	}
	if configs.AppConfig != nil && strings.ToLower(configs.AppConfig.Main.Language) == "zh" {
		return "zh"
	}
	return "en"
}

// T retrieves the localized string for a key, formatting if arguments are provided.
func T(key string, args ...any) string {
	entry, ok := i18nCatalog[key]
	var text string
	if !ok {
		text = key
	} else if currentLang() == "zh" {
		text = entry.zh
	} else {
		text = entry.en
	}
	if len(args) > 0 {
		return fmt.Sprintf(text, args...)
	}
	return text
}
