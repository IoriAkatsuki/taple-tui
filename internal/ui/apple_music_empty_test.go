package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/motion"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

func TestAppleMusicCozyEmptyStatesTruthfulAndBounded(t *testing.T) {
	origLang := currentLang()
	t.Cleanup(func() { SetLanguage(origLang) })

	sizes := [][2]int{{140, 45}, {120, 24}, {80, 30}, {60, 20}, {50, 20}}
	for _, lang := range []string{"en", "zh"} {
		SetLanguage(lang)
		cases := []struct {
			name string
			want string
			prep func(*AppleMusicPage)
		}{
			{
				name: "search-idle",
				want: T("empty.search_guide"),
				prep: func(p *AppleMusicPage) {
					p.drawer = appleSearch
				},
			},
			{
				name: "search-empty",
				want: T("empty.no_results"),
				prep: func(p *AppleMusicPage) {
					p.drawer = appleSearch
					p.catalog = appleCatalogView{title: "Empty Query"}
				},
			},
			{
				name: "search-error",
				want: T("empty.failed"),
				prep: func(p *AppleMusicPage) {
					p.drawer = appleSearch
					p.catalogError = "network offline"
				},
			},
			{
				name: "library-connecting",
				want: T("status.connecting"),
				prep: func(p *AppleMusicPage) {
					p.drawer = appleLibrary
					p.state.Ready = false
				},
			},
			{
				name: "library-auth",
				want: T("status.sign_in_hint"),
				prep: func(p *AppleMusicPage) {
					p.drawer = appleLibrary
					p.state = applemusic.State{Ready: true, Authorized: false}
				},
			},
			{
				name: "library-empty",
				want: T("empty.not_loaded"),
				prep: func(p *AppleMusicPage) {
					p.drawer = appleLibrary
					p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us"}
				},
			},
			{
				name: "queue-empty",
				want: T("empty.queue"),
				prep: func(p *AppleMusicPage) {
					p.drawer = appleQueue
				},
			},
			{
				name: "lyrics-idle",
				want: T("lyrics.auto_load"),
				prep: func(p *AppleMusicPage) {
					p.scene, p.sceneMotion = appleListen, appleTween{from: 1, to: 1}
					p.lyrics.status = "no_track"
				},
			},
			{
				name: "lyrics-error",
				want: T("lyrics.req_error"),
				prep: func(p *AppleMusicPage) {
					p.scene, p.sceneMotion = appleListen, appleTween{from: 1, to: 1}
					p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us", Track: applemusic.Track{ID: "tr.1", Name: "Song"}}
					p.lyrics.status = "request_error"
				},
			},
		}

		for _, tc := range cases {
			for _, sz := range sizes {
				t.Run(fmt.Sprintf("%s/%s/%dx%d", lang, tc.name, sz[0], sz[1]), func(t *testing.T) {
					p := NewAppleMusicPage(nil)
					defer p.Close()
					p.level = motion.Full
					tc.prep(p)
					out := p.draw(sz[0], sz[1], time.Unix(100, 0), 2)
					lines := strings.Split(out, "\n")
					if len(lines) != sz[1] {
						t.Fatalf("bad row count %d, want %d", len(lines), sz[1])
					}
					for _, line := range lines {
						if ansi.WcWidth.StringWidth(line) != sz[0] || ansi.GraphemeWidth.StringWidth(line) != sz[0] {
							t.Fatalf("width overflow: %q", line)
						}
					}
					for _, hit := range p.hits {
						if hit.x < 0 || hit.y < 0 || hit.x+hit.w > sz[0] || hit.y+hit.h > sz[1] {
							t.Fatalf("offscreen hit: %+v", hit)
						}
					}
					stripped := ansi.Strip(out)
					if strings.Contains(strings.ToLower(stripped), "sony") || strings.Contains(strings.ToLower(stripped), "walkman") {
						t.Fatal("rendered unpermitted device trademark")
					}
					if sz[1] >= 24 && !strings.Contains(stripped, tc.want) {
						t.Fatalf("missing truthful headline %q in:\n%s", tc.want, stripped)
					}
				})
			}
		}
	}

	// Verify 0 heap allocations in the subpixel SDF artboard rasterizer.
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.screen.reset(80, 30)
	allocs := testing.AllocsPerRun(20, func() {
		p.drawEmptyArtboard(2, 2, 46, 11, 1.2, appleEmptySleepy, false, 1)
	})
	if allocs > 0 {
		t.Fatalf("drawEmptyArtboard allocated %.0f objects per frame, want 0", allocs)
	}
}

func TestAppleMusicCozyEmptyHoverPerksUpAndClicksDispatch(t *testing.T) {
	origLang := currentLang()
	SetLanguage("zh")
	t.Cleanup(func() { SetLanguage(origLang) })

	// 1. Hovering the empty search CTA perks up the companion without faking playback.
	p := NewAppleMusicPage(&appleMouseClient{})
	defer p.Close()
	p.level = motion.Full
	now := time.Unix(100, 0)
	idleFrame := p.draw(140, 45, now, 2)

	var searchPill appleHit
	for _, h := range p.hits {
		if h.kind == appleHitKey && h.key == "/" && h.y > 12 && h.y < p.screen.h-5 {
			searchPill = h
			break
		}
	}
	if searchPill.w == 0 {
		t.Fatal("missing empty-card primary search pill hit")
	}
	p.Update(tea.MouseMotionMsg{X: searchPill.x + 2, Y: searchPill.y}, model.NewApp(model.DefaultOptions()))
	awakeFrame := p.draw(140, 45, now, 2)
	if awakeFrame == idleFrame {
		t.Fatal("hovering empty CTA did not trigger companion wake-up micro-interaction")
	}
	if !strings.Contains(ansi.Strip(awakeFrame), T("empty.bubble_awake")) {
		t.Fatalf("missing awake speech bubble in hovered empty frame")
	}
	if p.state.Playing || p.reels.Moving() {
		t.Fatal("empty animation faked playback")
	}

	// Clicking the pill enters search edit mode cleanly.
	p.Update(tea.MouseClickMsg{X: searchPill.x + 2, Y: searchPill.y, Button: tea.MouseLeft}, model.NewApp(model.DefaultOptions()))
	if !p.editing || !p.input.Focused() {
		t.Fatal("clicking empty search pill did not focus search input")
	}

	// 2. Unauthorized library pill triggers account sign-in asynchronously.
	c := &appleMouseClient{}
	lib := NewAppleMusicPage(c)
	defer lib.Close()
	lib.level = motion.Off
	lib.drawer = appleLibrary
	lib.state = applemusic.State{Ready: true, Authorized: false}
	lib.draw(140, 45, now, 2)
	var signInPill appleHit
	for _, h := range lib.hits {
		if h.kind == appleHitKey && h.key == "b" && h.y > 4 && h.y < lib.screen.h-5 {
			signInPill = h
			break
		}
	}
	if signInPill.w == 0 {
		t.Fatal("missing library sign-in pill inside cozy empty card")
	}
	_, cmd := lib.Update(tea.MouseClickMsg{X: signInPill.x + 2, Y: signInPill.y, Button: tea.MouseLeft}, model.NewApp(model.DefaultOptions()))
	if cmd == nil {
		t.Fatal("clicking library empty sign-in pill did not start sign-in command")
	}

	// 3. motion.Off keeps empty card static over time while catalog errors show puzzled state cleanly.
	errPage := NewAppleMusicPage(&appleLibraryClient{err: errors.New("boom")})
	defer errPage.Close()
	errPage.level = motion.Off
	errPage.catalogError = "boom"
	a := errPage.draw(140, 45, now, 2)
	b := errPage.draw(140, 45, now.Add(2*time.Second), 2)
	if a != b {
		t.Fatal("motion.Off animated the empty state")
	}
}

func TestAppleMusicEmptyStatePreviewExport(t *testing.T) {
	dir := os.Getenv("TAPEFOX_EMPTY_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set TAPEFOX_EMPTY_PREVIEW_DIR to export cozy empty state HTML frames")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	origLang := currentLang()
	SetLanguage("zh")
	t.Cleanup(func() { SetLanguage(origLang) })

	now := time.Unix(100, 0)
	write := func(name string, setup func(*AppleMusicPage)) {
		p := NewAppleMusicPage(&appleMouseClient{})
		defer p.Close()
		p.level = motion.Full
		setup(p)
		p.status = "FRAMEBUFFER TEST · 暖光书桌空状态预览"
		p.draw(140, 45, now, 2)
		writeAppleFrame(t, p, filepath.Join(dir, name+".html"))
	}

	write("search-idle-zh", func(p *AppleMusicPage) {})
	write("search-hover-awake-zh", func(p *AppleMusicPage) {
		p.draw(140, 45, now, 2)
		for _, h := range p.hits {
			if h.kind == appleHitKey && h.key == "/" && h.y > 12 && h.y < p.screen.h-5 {
				p.hoverX, p.hoverY = h.x+2, h.y
				break
			}
		}
	})
	write("library-signin-zh", func(p *AppleMusicPage) {
		p.drawer = appleLibrary
		p.state = applemusic.State{Ready: true, Authorized: false}
	})
	write("queue-empty-zh", func(p *AppleMusicPage) {
		p.drawer = appleQueue
	})
	write("lyrics-empty-zh", func(p *AppleMusicPage) {
		p.scene, p.sceneMotion = appleListen, appleTween{from: 1, to: 1}
		p.state = applemusic.State{Ready: true, Authorized: true, Full: true, Storefront: "us"}
		p.lyrics.status = "no_track"
	})
}
