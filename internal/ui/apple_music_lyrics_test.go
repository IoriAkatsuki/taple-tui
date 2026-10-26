package ui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
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

type appleLyricsClient struct {
	appleMouseClient
	requests []applemusic.LyricsRequest
}

func TestAppleMusicLyricsRejectLateResultsAndWaitForExplicitRetry(t *testing.T) {
	c := &appleLyricsClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us", Track: applemusic.Track{ID: "old"}}
	old := p.loadLyrics(false)
	p.state.Track.ID = "new"
	current := p.loadLyrics(false)
	p.acceptLyrics(old().(appleLyricsMsg))
	if p.lyrics.status != "loading" || len(p.lyrics.data.Lines) != 0 {
		t.Fatal("late old song overwrote current lyrics")
	}
	m := current().(appleLyricsMsg)
	m.err = errors.New("network failure")
	p.acceptLyrics(m)
	if p.loadLyrics(false) != nil || p.lyrics.status != "request_error" {
		t.Fatal("failure was hidden or automatically retried")
	}
	retry := p.handleKey(tea.KeyPressMsg{Code: 'y'})
	if retry == nil || p.handleKey(tea.KeyPressMsg{Code: 'y'}) != nil {
		t.Fatal("explicit retry absent or duplicated")
	}
	p.acceptLyrics(retry().(appleLyricsMsg))
	if p.lyrics.data.TrackID != "new" {
		t.Fatal("retry did not load current track")
	}
	if !c.requests[len(c.requests)-1].Refresh {
		t.Fatal("explicit retry did not bypass the browser lyric cache")
	}
	p.state.Storefront = "jp"
	late := p.loadLyrics(false)
	p.state.Authorized = false
	p.loadLyrics(false)
	p.acceptLyrics(late().(appleLyricsMsg))
	if len(p.lyrics.data.Lines) != 0 || p.lyrics.status != "login_required" {
		t.Fatal("logout retained or accepted old lyrics")
	}
}

func TestAppleMusicLyricsEmptyStatusesRemainLocalized(t *testing.T) {
	defer SetLanguage("en")
	p := NewAppleMusicPage(nil)
	defer p.Close()
	cases := []struct{ status, key, detail string }{
		{"", "lyrics.auto_load", ""},
		{"no_track", "lyrics.auto_load", ""},
		{"loading", "lyrics.loading", ""},
		{"login_required", "lyrics.sign_in", ""},
		{"storefront_loading", "lyrics.wait_region", ""},
		{"sdk_unavailable", "lyrics.wait_conn", ""},
		{"no_lyrics", "lyrics.none", ""},
		{"no_catalog_mapping", "lyrics.no_mapping", ""},
		{"unsupported_media", "lyrics.unsupported", ""},
		{"unauthorized", "lyrics.auth_expired", ""},
		{"access_denied", "lyrics.denied", ""},
		{"rate_limited", "lyrics.rate_limit", ""},
		{"unavailable", "lyrics.unavailable", ""},
		{"timeout", "lyrics.timeout", ""},
		{"stale", "lyrics.stale", ""},
		{"request_error", "lyrics.req_error", ""},
		{"parse_error", "lyrics.parse_error", ""},
		{"unknown_status", "lyrics.unavailable", ""},
		{"parse_error", "lyrics.parse_detail", "XML 8:42"},
	}
	for _, lang := range []string{"en", "zh"} {
		SetLanguage(lang)
		for _, tc := range cases {
			t.Run(lang+"/"+tc.status+"/"+tc.detail, func(t *testing.T) {
				p.screen.reset(160, 3)
				p.lyrics.status, p.lyrics.data.Detail = tc.status, tc.detail
				p.drawLyricsEmpty(0, 0, 160, 3, 1)
				want := T(tc.key)
				if tc.detail != "" {
					want = T(tc.key, tc.detail)
				}
				if want == tc.key {
					t.Fatalf("missing translation for %q", tc.key)
				}
				if got := strings.TrimSpace(ansi.Strip(p.screen.render())); got != want {
					t.Fatalf("status %q: got %q, want %q", tc.status, got, want)
				}
			})
		}
	}
}

func lyricTestPage(t *testing.T) (*AppleMusicPage, time.Time) {
	t.Helper()
	p := NewAppleMusicPage(&appleLyricsClient{})
	t.Cleanup(func() { p.Close() })
	p.styleRNG = rand.New(rand.NewPCG(1, 2))
	p.level = motion.Off
	p.scene = appleListen
	p.sceneMotion = appleTween{from: 1, to: 1}
	now := time.Unix(100, 0)
	p.acceptState(applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Storefront: "us", Track: applemusic.Track{ID: "123", Name: "SYNTHETIC LYRICS TEST", Artist: "非真实播放"}, Position: 10.5}, now)
	cmd := p.loadLyrics(false)
	m := cmd().(appleLyricsMsg)
	m.data.Lines = nil
	for i := range 40 {
		m.data.Lines = append(m.data.Lines, applemusic.LyricLine{Text: fmt.Sprintf("Fixture line %02d · 夜行测试", i), Start: time.Duration(i) * time.Second, End: time.Duration(i+1) * time.Second})
	}
	p.acceptLyrics(m)
	p.status = "FRAMEBUFFER TEST · 合成 TTML，非真实账号/播放"
	return p, now
}

func TestAppleMusicLyricsUseRealClockAndManualFollow(t *testing.T) {
	SetLanguage("zh")
	defer SetLanguage("en")
	p, now := lyricTestPage(t)
	p.draw(140, 45, now, 2)
	if position, ok := p.lyricsPosition(now); !ok || position != 10500*time.Millisecond {
		t.Fatal("real playback position missing")
	}
	if position, ok := p.lyricsPosition(now.Add(time.Second)); !ok || position != 10500*time.Millisecond {
		t.Fatal("renderer invented advancing playback time")
	}
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	top := p.lyrics.top
	s := p.state
	s.Position = 30.5
	p.acceptState(s, now.Add(time.Second))
	out := ansi.Strip(p.draw(140, 45, now.Add(time.Second), 2))
	if !p.lyrics.manual || p.lyrics.top != top || !strings.Contains(out, "回到当前句") {
		t.Fatal("manual scrolling did not pause follow")
	}
	p.handleKey(tea.KeyPressMsg{Code: 'f'})
	out = ansi.Strip(p.draw(140, 45, now.Add(time.Second), 2))
	if p.lyrics.manual || !strings.Contains(out, "› Fixture line 30") {
		t.Fatal("return-to-current did not restore real timed line")
	}
	lastTop := p.lyrics.top
	p.draw(140, 45, now.Add(4*time.Second), 2)
	if _, ok := p.lyricsPosition(now.Add(4 * time.Second)); ok || p.lyrics.top != lastTop {
		t.Fatal("stale clock advanced or reset follow")
	}
	s.Position, s.Playing, s.Paused = 5.5, false, true
	p.acceptState(s, now.Add(5*time.Second))
	out = ansi.Strip(p.draw(140, 45, now.Add(5*time.Second), 2))
	if !strings.Contains(out, "› Fixture line 05") {
		t.Fatal("seek/pause did not use confirmed position")
	}
	s.Full, s.Preview = false, true
	p.acceptState(s, now.Add(6*time.Second))
	if _, ok := p.lyricsPosition(now.Add(6 * time.Second)); ok {
		t.Fatal("preview used full-track lyric offset")
	}
}

func TestAppleMusicLyricWheelAndFramebuffer(t *testing.T) {
	for _, size := range [][2]int{{140, 45}, {120, 24}, {80, 30}, {50, 20}, {12, 8}} {
		for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
			p, now := lyricTestPage(t)
			p.level = level
			p.lyrics.data.Lines[10].Text = "宽字符 ❤️ " + strings.Repeat("歌词 ", 30)
			out := p.draw(size[0], size[1], now, 2)
			for _, line := range strings.Split(out, "\n") {
				if ansi.WcWidth.StringWidth(line) != size[0] || ansi.GraphemeWidth.StringWidth(line) != size[0] {
					t.Fatalf("%v: lyrics overflow", size)
				}
			}
			for _, h := range p.hits {
				if h.x+h.w > size[0] || h.y+h.h > size[1] {
					t.Fatal("lyrics created offscreen hit target")
				}
			}
			if !p.lyrics.area.Empty() {
				p.Update(tea.MouseWheelMsg{X: p.lyrics.area.Min.X, Y: p.lyrics.area.Min.Y, Button: tea.MouseWheelDown}, model.NewApp(model.DefaultOptions()))
				if !p.lyrics.manual {
					t.Fatal("wheel inside lyrics did not pause follow")
				}
			}
			if dir := os.Getenv("TAPEFOX_LYRICS_PREVIEW_DIR"); dir != "" && level == motion.Off {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("lyrics-%dx%d.html", size[0], size[1])))
			}
		}
	}
}

func TestAppleMusicLyricsKeepOverlapsAndDoNotHighlightUnsyncedText(t *testing.T) {
	SetLanguage("zh")
	defer SetLanguage("en")
	p, now := lyricTestPage(t)
	p.lyrics.data.Lines = []applemusic.LyricLine{
		{Text: "main voice", Start: 10 * time.Second, End: 12 * time.Second},
		{Text: "second voice", Start: 10 * time.Second, End: 11 * time.Second},
	}
	out := ansi.Strip(p.draw(140, 45, now, 2))
	if !strings.Contains(out, "› main voice") || !strings.Contains(out, "› second voice") {
		t.Fatal("overlapping real lines were collapsed")
	}
	p.lyrics.status, p.lyrics.data.Synced = "unsynced", false
	out = ansi.Strip(p.draw(140, 45, now, 2))
	if strings.Contains(out, "› main voice") || !strings.Contains(out, "纯文本") {
		t.Fatal("unsynced lyrics got fake highlighting")
	}
}

func TestAppleMusicLyricsRejectInvalidPlaybackClock(t *testing.T) {
	p, now := lyricTestPage(t)
	for _, position := range []float64{-1, math.NaN(), math.Inf(1)} {
		p.state.Position = position
		if _, valid := p.lyricsPosition(now); valid {
			t.Fatal("invalid playback clock was treated as track time zero")
		}
	}
}

func TestAppleMusicLyricsShowSafeParseReason(t *testing.T) {
	p, now := lyricTestPage(t)
	p.lyrics.status = "parse_error"
	p.lyrics.data = applemusic.Lyrics{Status: "parse_error", Detail: "行开始时间格式不支持（XML 8:42）"}
	out := ansi.Strip(p.draw(140, 45, now, 2))
	if !strings.Contains(out, "行开始时间格式不支持") || !strings.Contains(out, "XML 8:42") {
		t.Fatal("lyrics UI hid the safe parse reason")
	}
	if dir := os.Getenv("TAPEFOX_LYRICS_PREVIEW_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		writeAppleFrame(t, p, filepath.Join(dir, "lyrics-parse-detail.html"))
	}
}

func (c *appleLyricsClient) Lyrics(_ context.Context, q applemusic.LyricsRequest) (applemusic.Lyrics, error) {
	c.requests = append(c.requests, q)
	return applemusic.Lyrics{TrackID: q.ID, Storefront: q.Storefront, Source: "apple-web-undocumented", Status: "available", Synced: true,
		Lines: []applemusic.LyricLine{{Text: "first synthetic line", Start: time.Second, End: 2 * time.Second}, {Text: "second synthetic line", Start: 3 * time.Second, End: 4 * time.Second}}}, nil
}

func TestAppleMusicRealTrackChangeSchedulesLyricsAsynchronously(t *testing.T) {
	c := &appleLyricsClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	a := model.NewApp(model.DefaultOptions())
	state := applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Storefront: "us", Track: applemusic.Track{ID: "123"}, Position: 1.5}
	_, cmd := p.Update(appleStateMsg{state: state}, a)
	if len(c.requests) != 0 || cmd == nil {
		t.Fatal("lyrics request blocked the UI or was not scheduled")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok {
		t.Fatal("real track change did not schedule lyrics alongside polling")
	}
	for _, command := range batch {
		msg := command()
		if _, poll := msg.(applePollMsg); !poll {
			p.Update(msg, a)
		}
	}
	if len(c.requests) != 1 || c.requests[0].ID != "123" {
		t.Fatal("lyrics were not fetched for the real current track")
	}
	_, cmd = p.Update(appleStateMsg{state: state}, a)
	if _, batch := cmd().(tea.BatchMsg); batch {
		t.Fatal("unchanged polling kept refetching lyrics")
	}
}

func TestAppleMusicLyricsSmoothScrollAndDepthFade(t *testing.T) {
	p, now := lyricTestPage(t)
	p.level = motion.Full
	p.draw(140, 45, now, 2)
	s := p.state
	s.Position = 22.5
	p.acceptState(s, now.Add(time.Second))
	p.draw(140, 45, now.Add(time.Second), 2)
	if !p.lyrics.scroll.active(now.Add(1100 * time.Millisecond)) {
		t.Fatal("real lyric line advance did not start smooth scroll tween")
	}
	midBase, midFrac := p.updateLyricScroll(now.Add(1130 * time.Millisecond))
	if midFrac == 0 && midBase == p.lyrics.top {
		t.Fatal("smooth scroll jumped immediately without intermediate offset")
	}
	out := ansi.Strip(p.draw(140, 45, now.Add(1300*time.Millisecond), 2))
	if p.lyrics.scroll.active(now.Add(1300*time.Millisecond)) || !strings.Contains(out, "› Fixture line 22") || !strings.Contains(out, "┃") {
		t.Fatal("smooth scroll did not settle on confirmed target line with focus rail")
	}
}

func TestAppleMusicLyricsParticleWavefrontAndLightUnplayed(t *testing.T) {
	p, now := lyricTestPage(t)
	p.level = motion.Full
	// Line 10 spans 10s..11s; Position=10.5s is 50% through line 10.
	out := ansi.Strip(p.draw(140, 45, now, 2))
	if !strings.Contains(out, "› Fixture line 10") {
		t.Fatal("active lyric line missing")
	}
	foundParticle := false
	for _, g := range appleParticleRunes {
		if strings.Contains(out, g) {
			foundParticle = true
			break
		}
	}
	if !foundParticle {
		t.Fatalf("active lyric line did not render trailing progress particles: %s", out)
	}
}
