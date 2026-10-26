package ui

import (
	"context"
	"errors"
	"fmt"
	"html"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/motion"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

func TestAppleMusicFramesAreBoundedAndDoNotFakeVU(t *testing.T) {
	for _, size := range [][2]int{{160, 48}, {120, 38}, {100, 30}, {120, 24}, {100, 18}, {99, 30}, {80, 24}, {60, 20}, {30, 8}, {12, 2}, {1, 1}} {
		for _, level := range []motion.Level{motion.Full, motion.Off} {
			p := NewAppleMusicPage(nil)
			p.state = applemusic.State{Ready: true, Authorized: true, Playing: true, Track: applemusic.Track{ID: "i.opaque", Name: "夜行磁带 ❤️ 超长歌名与\x1b[31m终端控制字符", Artist: "艺术家", Duration: 245}, Position: 72}
			p.tracks = []applemusic.Track{p.state.Track}
			p.status = "浏览器已连接"
			p.level = level
			out := p.draw(size[0], size[1], time.Unix(100, 0), 2)
			lines := strings.Split(out, "\n")
			if len(lines) != size[1] {
				t.Fatalf("%v: %d rows", size, len(lines))
			}
			for _, line := range lines {
				if ansi.WcWidth.StringWidth(line) != size[0] || ansi.GraphemeWidth.StringWidth(line) != size[0] {
					t.Fatalf("%v: bad width %q", size, line)
				}
			}
			if size[0] >= 100 && size[1] >= 30 && !strings.Contains(ansi.Strip(out), "VU OFF") {
				t.Fatal("missing honest VU status")
			}
			_ = p.Close()
		}
	}
}

func TestAppleMusicUsesHardwareFaceplate(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	frame := ansi.Strip(p.draw(120, 38, time.Unix(100, 0), 2))
	for _, brand := range []string{"sony", "walkman"} {
		if strings.Contains(strings.ToLower(frame), brand) {
			t.Errorf("device branding remains visible: %q", brand)
		}
	}
	for _, label := range []string{"Cassette Deck", "TIME COUNTER", "dBFS", "SPC"} {
		if !strings.Contains(frame, label) {
			t.Errorf("missing hardware section %q", label)
		}
	}
}

func TestAppleMusicLandscapeKeepsGapBeforeClock(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Off
	p.state.Track = applemusic.Track{Name: "Song", Artist: strings.Repeat("A", 100)}
	p.draw(120, 24, time.Unix(100, 0), 2)
	for x := 44; x < 47; x++ {
		c := p.screen.buf.CellAt(x, 19)
		if c != nil && c.Content != " " && c.Content != "" {
			t.Fatalf("artist overflow at x=%d: %q", x, c.Content)
		}
	}
}

type appleTestClient struct{ searches int }

func (*appleTestClient) Snapshot(context.Context) (applemusic.State, error) {
	return applemusic.State{}, nil
}
func (c *appleTestClient) Catalog(context.Context, applemusic.CatalogRequest) (applemusic.CatalogPage, error) {
	c.searches++
	return applemusic.CatalogPage{}, nil
}
func (*appleTestClient) Do(context.Context, applemusic.Action) error { return nil }
func (*appleTestClient) Show(context.Context) error                  { return nil }
func (*appleTestClient) Lyrics(context.Context, applemusic.LyricsRequest) (applemusic.Lyrics, error) {
	return applemusic.Lyrics{Status: "unavailable"}, nil
}

func TestAppleMusicSearchDoesNotRunOnUIThread(t *testing.T) {
	c := &appleTestClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.editing = true
	p.input.SetValue("A Song")
	cmd := p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || c.searches != 0 {
		t.Fatal("search should return a background command")
	}
	_ = cmd()
	if c.searches != 1 {
		t.Fatal("search command did not execute")
	}
}

func TestAppleMusicReducedMotionHasNoGlitch(t *testing.T) {
	for _, level := range []motion.Level{motion.Subtle, motion.Off} {
		p := NewAppleMusicPage(nil)
		p.level = level
		p.state.Track.ID = "first"
		p.acceptState(applemusic.State{Track: applemusic.Track{ID: "second"}}, time.Unix(1, 0))
		if !p.glitchAt.IsZero() {
			t.Fatal("reduced motion triggered glitch")
		}
		p.Close()
	}
}

func TestAppleMusicGlitchEndsAfterTrackTransition(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	p.state.Track.ID = "first"
	now := time.Unix(100, 0)
	p.acceptState(applemusic.State{Track: applemusic.Track{ID: "second"}}, now)
	if p.glitchAt != now {
		t.Fatal("track transition did not start glitch")
	}
	for _, elapsed := range []time.Duration{80 * time.Millisecond, 200 * time.Millisecond} {
		p.screen.reset(30, 10)
		p.screen.pal = appleNeon
		p.screen.set(4, 4, "█", 1, uv.Style{Fg: appleNeon.Hero})
		before := p.screen.render()
		p.glitch(0, 0, 20, 10, now.Add(elapsed))
		changed := before != p.screen.render()
		if changed != (elapsed < 160*time.Millisecond) {
			t.Fatalf("glitch active at %v: %t", elapsed, changed)
		}
	}
}

func TestAppleMusicGlitchSeparatesBothChannelsInsideStage(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	p.glitchAt = time.Unix(100, 0)
	p.screen.reset(30, 12)
	p.screen.pal = appleNeon
	p.screen.set(4, 4, "█", 1, uv.Style{Fg: appleNeon.Text})
	p.screen.set(24, 4, "X", 1, uv.Style{Fg: appleNeon.Text})
	p.screen.set(4, 11, "Y", 1, uv.Style{Fg: appleNeon.Text})
	p.glitch(0, 0, 20, 10, p.glitchAt.Add(80*time.Millisecond))
	for x, want := range map[int]color.Color{3: appleNeon.Accent, 5: appleNeon.Hero} {
		c := p.screen.buf.CellAt(x, 4)
		if c == nil || !npSameColor(c.Style.Fg, want) {
			t.Errorf("missing chromatic fringe at x=%d", x)
		}
	}
	if p.screen.buf.CellAt(24, 4).Content != "X" || p.screen.buf.CellAt(4, 11).Content != "Y" {
		t.Fatal("glitch escaped into controls")
	}
}

func TestAppleMusicLEDStepsUseMeasuredLevelsAndLeaveGaps(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.screen.reset(40, 8)
	p.screen.pal = appleNeon
	p.drawMeter(0, 0, 30, applemusic.MeterFrame{Ready: true, Levels: [2]float64{1, 0}})
	if c := p.screen.buf.CellAt(2, 2); c.Content != "█" || !npSameColor(c.Style.Fg, appleNeon.Accent) {
		t.Fatal("left channel did not light")
	}
	if c := p.screen.buf.CellAt(2, 3); npSameColor(c.Style.Fg, appleNeon.Accent) {
		t.Fatal("silent right channel was fabricated")
	}
	if c := p.screen.buf.CellAt(3, 2); c != nil && c.Content != " " && c.Content != "" {
		t.Fatal("LED steps must be separated")
	}
	p.screen.reset(40, 8)
	p.drawMeter(0, 0, 30, applemusic.MeterFrame{Levels: [2]float64{1, 1}})
	if c := p.screen.buf.CellAt(2, 2); npSameColor(c.Style.Fg, appleNeon.Accent) {
		t.Fatal("unavailable sample was displayed as signal")
	}
}

func TestAppleMusicCompactKeepsErrorsAndInvalidatesConnection(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Off
	p.state = applemusic.State{Ready: true, Authorized: true, Playing: true}
	p.Update(appleStateMsg{err: errors.New("浏览器已断开")}, model.NewApp(model.DefaultOptions()))
	frame := ansi.Strip(p.draw(50, 20, time.Unix(100, 0), 2))
	if p.state.Authorized || p.state.Playing || !strings.Contains(frame, "浏览器已断开") {
		t.Fatalf("stale or silent error: %s", frame)
	}
}

func TestAppleMusicCompactPauseSettlesAnimations(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	p.state.Playing = true
	p.reels.Run()
	now := time.Unix(100, 0)
	p.draw(120, 38, now, 2)
	p.state.Playing = false
	for range 200 {
		now = now.Add(50 * time.Millisecond)
		p.draw(50, 20, now, 2)
	}
	if p.reels.Moving() {
		t.Fatal("hidden cassette never settled after pause")
	}
}

// Export the real cell buffer for visual QA, not a separate mockup renderer.
func TestAppleMusicPreview(t *testing.T) {
	dir := os.Getenv("TAPEFOX_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set TAPEFOX_PREVIEW_DIR to export visual fixtures")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{120, 38}, {80, 24}} {
		p := NewAppleMusicPage(nil)
		p.level = motion.Full
		p.state = applemusic.State{Ready: true, Authorized: true, Playing: true, Volume: .65, Position: 72,
			Track: applemusic.Track{ID: "fixture", Name: "Night Drive / 夜行磁带", Artist: "渲染测试数据 · Synthwave Session", Duration: 245}}
		for i, name := range []string{"Night Drive / 夜行磁带", "磁带 B 面", "Electric Dreams", "霓虹街角", "After Midnight", "Low Fidelity", "深夜电台", "Blue Hour"} {
			p.tracks = append(p.tracks, applemusic.Track{ID: fmt.Sprint(i), Name: name, Artist: "视觉测试曲目"})
		}
		p.selected = 1
		p.input.SetValue("Synthwave")
		p.status = "渲染测试画面 · 非真实登录或播放 · VU 未采样"
		p.draw(size[0], size[1], time.Unix(100, 0), 2)
		path := filepath.Join(dir, fmt.Sprintf("apple-music-%dx%d.html", size[0], size[1]))
		writeAppleFrame(t, p, path)
		p.Close()
	}
}

func TestAppleMusicAnimationPreview(t *testing.T) {
	dir := os.Getenv("TAPEFOX_ANIMATION_DIR")
	if dir == "" {
		t.Skip("set TAPEFOX_ANIMATION_DIR to export labeled animation fixtures")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	p.state = applemusic.State{Ready: true, Authorized: true, Playing: true, Volume: .65, Position: 72,
		Track: applemusic.Track{ID: "fixture-a", Name: "Night Drive / 夜行磁带", Artist: "渲染测试数据", Duration: 245}}
	p.reels.Run()
	for i := range 25 {
		now := time.Unix(100, 0).Add(time.Duration(i) * 40 * time.Millisecond)
		if i == 5 {
			s := p.state
			s.Track.ID = "fixture-b"
			s.Track.Name = "Side B / Signal Shift"
			s.Position = 0
			p.acceptState(s, now)
		}
		p.status = "渲染测试 · 合成电平 · 切歌动效 · 非真实播放"
		p.draw(120, 38, now, 2)
		p.drawMeter(32, 28, 34, applemusic.MeterFrame{Ready: true, Levels: [2]float64{.65 + .25*math.Sin(float64(i)*.6), .52 + .24*math.Sin(float64(i)*.6+.8)}})
		p.screen.fill(32, 28, 34, 1, appleNeon.Bg)
		p.screen.text(32, 28, "TEST SIGNAL  dBFS", uv.Style{Fg: appleNeon.Secondary}, 34)
		p.screen.fillBackground()
		writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("frame-%03d.html", i)))
	}
}

func TestAppleMusicStandbyPreview(t *testing.T) {
	dir := os.Getenv("TAPEFOX_STANDBY_DIR")
	if dir == "" {
		t.Skip("set TAPEFOX_STANDBY_DIR for idle animation fixtures")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	p.state = applemusic.State{Ready: true, Authorized: true, Volume: .5}
	p.status = "待机动画渲染测试 · 无播放、无音频采样"
	for i := range 25 {
		p.draw(120, 38, time.Unix(100, 0).Add(time.Duration(i)*160*time.Millisecond), 2)
		writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("frame-%03d.html", i)))
	}
}

func writeAppleFrame(t *testing.T, p *AppleMusicPage, path string) {
	t.Helper()
	cssColor := func(c color.Color) string {
		if c == nil {
			return "inherit"
		}
		r, g, b, _ := c.RGBA()
		return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	}
	var out strings.Builder
	w, h := p.screen.w, p.screen.h
	fmt.Fprintf(&out, `<!doctype html><meta charset="utf-8"><title>tapefox terminal render fixture</title><style>body{margin:20px;background:#0B0716}#terminal{position:relative;font:16px/20px 'DejaVu Sans Mono',monospace;font-variant-ligatures:none;width:%dpx;height:%dpx}span{position:absolute;white-space:pre;height:20px}</style><div id="terminal">`, w*10, h*20)
	for y := range h {
		for x := range w {
			c := p.screen.buf.CellAt(x, y)
			if c == nil || c.Width == 0 {
				continue
			}
			fmt.Fprintf(&out, `<span style="left:%dpx;top:%dpx;width:%dpx;color:%s;background:%s">%s</span>`, x*10, y*20, c.Width*10, cssColor(c.Style.Fg), cssColor(c.Style.Bg), html.EscapeString(c.Content))
		}
	}
	out.WriteString("</div>")
	if err := os.WriteFile(path, []byte(out.String()), 0600); err != nil {
		t.Fatal(err)
	}
	t.Log(path)
}
