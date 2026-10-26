package ui

// Presentation tests deliberately inspect private scene state and real cells/hits;
// exporting that state would expand the production API just for rendering tests.
import (
	"fmt"
	"image"
	"strings"
	"testing"
	"time"

	"github.com/anhoder/foxful-cli/motion"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

func presentationPage(t *testing.T) (*AppleMusicPage, time.Time) {
	t.Helper()
	p := NewAppleMusicPage(nil)
	t.Cleanup(func() { p.Close() })
	now := time.Unix(100, 0)
	p.level, p.scene = motion.Off, appleListen
	p.sceneMotion = appleTween{from: 1, to: 1}
	p.state = applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true,
		Storefront: "us", Position: 1.5, Track: applemusic.Track{ID: "fixture", Name: "Presentation fixture", Duration: 60}}
	p.stateAt = now
	p.lyrics = appleLyricsState{status: "available", data: applemusic.Lyrics{
		TrackID: "fixture", Storefront: "us", Synced: true,
		Lines: []applemusic.LyricLine{{Text: "Synthetic presentation line", Start: time.Second, End: 3 * time.Second}},
	}}
	p.status = "SYNTHETIC FRAMEBUFFER TEST / NOT LIVE PLAYBACK"
	return p, now
}

func presentationRow(p *AppleMusicPage, y int) string {
	var line strings.Builder
	for x := range p.screen.w {
		if cell := p.screen.buf.CellAt(x, y); cell != nil && cell.Width > 0 {
			line.WriteString(cell.Content)
		}
	}
	return line.String()
}

func presentationHasKey(p *AppleMusicPage, key string) bool {
	for _, hit := range p.hits {
		if hit.kind == appleHitKey && hit.key == key {
			return true
		}
	}
	return false
}

func TestAppleMusicSearchEditorVisibleDuringReturn(t *testing.T) {
	for _, width := range []int{80, 99, 100} {
		for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
			for _, elapsed := range []time.Duration{0, 100 * time.Millisecond} {
				t.Run(fmt.Sprintf("%d/%v/%v", width, level, elapsed), func(t *testing.T) {
					p, now := presentationPage(t)
					p.level, p.scene, p.drawer, p.editing = level, appleBrowse, appleSearch, true
					p.sceneMotion = appleTween{from: 1, to: 1}
					p.camera = appleTween{from: 1, to: 1}
					p.sceneMotion.start(0, now, 640*time.Millisecond, level)
					p.camera.start(0, now, 350*time.Millisecond, level)
					p.input.SetValue("visible-query")
					p.tracks = []applemusic.Track{p.state.Track}
					p.input.Focus()
					out := ansi.Strip(p.draw(width, 30, now.Add(elapsed), 2))
					if !strings.Contains(out, "/ visible-query▏") || !presentationHasKey(p, "search") {
						t.Fatalf("focused editor is invisible during return: %s", out)
					}
					assertPresentationEditorHits(t, p)
					exportPresentation(t, p, fmt.Sprintf("search-%d-%v-%d", width, level, elapsed.Milliseconds()))
				})
			}
		}
	}
}

func assertPresentationEditorHits(t *testing.T, p *AppleMusicPage) {
	t.Helper()
	searchHits := 0
	for _, hit := range p.hits {
		if hit.key == "search" {
			searchHits++
		}
	}
	if searchHits != 1 {
		t.Fatalf("expected one visible search button, got %d hits", searchHits)
	}
	if p.screen.w < 100 {
		return
	}
	editor := appleBox(3, 4, (p.screen.w-7)*47/100-2, 1)
	for _, hit := range p.hits {
		bounds := image.Rect(hit.x, hit.y, hit.x+hit.w, hit.y+hit.h)
		if (hit.kind == appleHitTrack || hit.kind == appleHitCassette) && bounds.Overlaps(p.stageRect) {
			t.Fatalf("hidden drawer item remains clickable under the device: %+v", hit)
		}
		if bounds.Overlaps(editor) && (hit.kind != appleHitKey || (hit.key != "/" && hit.key != "search")) {
			t.Fatalf("search editor retains an occluded hit: %+v", hit)
		}
	}
}

func TestAppleMusicPresentationNavigationLayers(t *testing.T) {
	defer SetLanguage("en")
	for _, lang := range []string{"en", "zh"} {
		SetLanguage(lang)
		p, now := presentationPage(t)
		p.scene, p.drawer = appleBrowse, appleLibrary
		p.sceneMotion = appleTween{}
		p.library.catalog = appleCatalogView{title: "Fixture collection", request: applemusic.CatalogRequest{Kind: applemusic.LibraryPlaylistTracks}}
		p.library.catalogHistory = []appleCatalogParent{{view: appleCatalogView{title: "Fixture root"}}}
		p.draw(140, 45, now, 2)
		browse, listen := "Browse", "Listen"
		if lang == "zh" {
			browse, listen = "浏览", "聆听"
		}
		if row := presentationRow(p, 1); !strings.Contains(row, browse) || !strings.Contains(row, listen) {
			t.Fatalf("%s: scene navigation missing from row 1: %q", lang, row)
		}
		if row := presentationRow(p, 2); !strings.Contains(row, "Fixture root / Fixture collection") {
			t.Fatalf("%s: real catalog path missing from row 2: %q", lang, row)
		}
		if !strings.Contains(presentationRow(p, 0), "Apple Music") {
			t.Fatal("brand/account row lost")
		}
		exportPresentation(t, p, "navigation-"+lang)
	}
}

func TestAppleMusicPresentationBackPriority(t *testing.T) {
	defer SetLanguage("en")
	cases := []struct{ name, en, zh string }{
		{"panel", "Esc Close", "Esc 关闭"}, {"editing", "Esc Leave input", "Esc 退出编辑"},
		{"zoomed", "Esc Zoom out", "Esc 拉远镜头"}, {"listen", "Esc Back to Browse", "Esc 返回浏览"},
		{"inserting", "Esc Back to Browse", "Esc 返回浏览"}, {"loading", "Esc Cancel loading", "Esc 取消目录等待"},
		{"history", "Esc Back to Parent", "Esc 返回 Parent"}, {"root", "", ""}, {"queue", "", ""},
	}
	for _, lang := range []string{"en", "zh"} {
		SetLanguage(lang)
		for _, tc := range cases {
			t.Run(lang+"/"+tc.name, func(t *testing.T) {
				p, now := presentationPage(t)
				p.scene, p.sceneMotion = appleBrowse, appleTween{}
				setPresentationBackState(p, tc.name, now)
				p.draw(140, 45, now, 2)
				want := tc.en
				if lang == "zh" {
					want = tc.zh
				}
				row := presentationRow(p, 2)
				if want != "" && !strings.Contains(row, want) {
					t.Fatalf("expected next Esc target %q, got %q", want, row)
				}
				if (want != "") != presentationHasKey(p, "esc") {
					t.Fatalf("root must not expose a no-op back hit: %q", row)
				}
				if tc.name != "panel" && want != "" && !strings.Contains(presentationRow(p, 43), want) {
					t.Fatalf("top/bottom Esc targets disagree: %q", presentationRow(p, 43))
				}
			})
		}
	}
}

func setPresentationBackState(p *AppleMusicPage, name string, now time.Time) {
	p.catalogHistory = []appleCatalogParent{{view: appleCatalogView{title: "Parent"}}}
	p.searching = name != "history"
	p.zoomed = name == "zoomed" || name == "editing" || name == "panel"
	p.editing = name == "editing" || name == "panel"
	if name == "panel" {
		p.panel = T("panel.title")
	}
	if name == "listen" {
		p.scene, p.sceneMotion = appleListen, appleTween{from: 1, to: 1}
	}
	if name == "inserting" {
		p.inserting, p.insertStart = true, now
	}
	if name == "root" {
		p.searching, p.catalogHistory = false, nil
	}
	if name == "queue" {
		p.drawer = appleQueue
	}
}

func TestAppleMusicLyricsPresentation(t *testing.T) {
	defer SetLanguage("en")
	cases := []struct {
		name, en, zh string
		follow       bool
	}{
		{"no_track", "Lyrics load automatically", "播放歌曲后自动加载歌词", false},
		{"loading", "Loading lyrics", "正在读取歌词", false},
		{"login_required", "Press b to sign in", "请按 b 登录", false},
		{"sdk_unavailable", "Waiting for Apple Music connection", "等待 Apple Music 后台连接", false},
		{"storefront_loading", "Waiting for Apple Music region", "等待 Apple Music 地区", false},
		{"no_lyrics", "No lyrics available", "暂无歌词", false},
		{"plain", "Plain text · no timestamps", "纯文本 · 无时间戳", true},
		{"manual", "Browsing lyrics · follow off", "手动浏览 · 已停止跟随", true},
		{"synced", "Synced to playback", "逐行同步 · 真实进度", false},
		{"paused", "Paused at real position", "已暂停 · 真实位置", false},
		{"stale", "Clock unavailable · sync suspended", "时钟过期或不可用 · 暂不同步", false},
		{"preview", "Preview · sync unavailable", "试听 · 暂不同步", false},
		{"unconfirmed", "Playback unconfirmed · sync unavailable", "播放未确认 · 暂不同步", false},
		{"request_error", "Failed to load lyrics", "歌词读取失败", false},
		{"parse_error", "Unsupported lyrics format", "歌词格式不支持", false},
	}
	for _, lang := range []string{"en", "zh"} {
		SetLanguage(lang)
		for _, tc := range cases {
			t.Run(lang+"/"+tc.name, func(t *testing.T) {
				p, now := presentationPage(t)
				setLyricsPresentationState(p, tc.name, now)
				out := ansi.Strip(p.draw(160, 45, now, 2))
				want := tc.en
				if lang == "zh" {
					want = tc.zh
				}
				if !strings.Contains(out, want) || strings.Contains(out, "SYNCED") {
					t.Fatalf("expected truthful lyric label %q, got: %s", want, out)
				}
				if tc.follow != presentationHasKey(p, "f") {
					t.Fatalf("%s: follow action does not match visible lyric state", tc.name)
				}
				if strings.HasSuffix(tc.name, "error") && !presentationHasKey(p, "y") {
					t.Fatal("lyric failure lost explicit retry")
				}
				exportPresentation(t, p, "lyrics-"+lang+"-"+tc.name)
			})
		}
	}
}

func setLyricsPresentationState(p *AppleMusicPage, name string, now time.Time) {
	switch name {
	case "plain":
		p.lyrics.status, p.lyrics.data.Synced = "unsynced", false
	case "manual":
		p.lyrics.manual = true
	case "synced":
	case "paused":
		p.state.Playing, p.state.Paused = false, true
	case "stale":
		p.stateAt = now.Add(-3 * time.Second)
	case "preview":
		p.state.Full, p.state.Preview = false, true
	case "unconfirmed":
		p.state.Playing, p.state.Paused = false, false
	default:
		p.lyrics.status, p.lyrics.data.Lines = name, nil
	}
}

func TestAppleMusicPresentationBrowseCloseUpIsLensNotFocus(t *testing.T) {
	defer SetLanguage("en")
	SetLanguage("en")
	p, now := presentationPage(t)
	p.scene, p.sceneMotion, p.zoomed = appleBrowse, appleTween{}, true
	p.draw(80, 30, now, 2)
	row := presentationRow(p, 2)
	if !strings.Contains(row, "Lens: close-up") || strings.Contains(row, "FOCUS / DECK") {
		t.Fatalf("browse close-up must describe a lens, not an input focus: %q", row)
	}
}
