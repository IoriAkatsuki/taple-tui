package ui

// These white-box fixtures export real cells without expanding the public API.
import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anhoder/foxful-cli/motion"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

func presentationPreviewDir(t *testing.T) string {
	t.Helper()
	dir := os.Getenv("TAPEFOX_PRESENTATION_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set TAPEFOX_PRESENTATION_PREVIEW_DIR to export synthetic framebuffer fixtures")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func exportPresentation(t *testing.T, p *AppleMusicPage, name string) {
	t.Helper()
	if os.Getenv("TAPEFOX_PRESENTATION_PREVIEW_DIR") == "" {
		return
	}
	writeAppleFrame(t, p, filepath.Join(presentationPreviewDir(t), name+".html"))
}

func presentationPreviewSizes() [][2]int {
	return [][2]int{{140, 45}, {120, 24}, {80, 30}, {100, 30}, {99, 30}, {60, 20}, {59, 20}, {60, 19}}
}

func presentationPreviewPage(t *testing.T) (*AppleMusicPage, time.Time) {
	t.Helper()
	p, now := presentationPage(t)
	p.styleRNG = rand.New(rand.NewPCG(1, 2))
	p.state.Track.Name, p.state.Track.Artist = "合成磁带 / Synthetic tape", "NOT LIVE PLAYBACK"
	p.state.Volume = .6
	p.styles[p.state.Track.ID] = 1
	p.bayID, p.bayStyle, p.bayNumber = p.state.Track.ID, 1, 1
	for i := range 8 {
		track := applemusic.Track{ID: fmt.Sprintf("fixture-%d", i), Name: fmt.Sprintf("合成曲目 %02d / Fixture", i+1), Artist: "Synthetic fixture", Duration: 60}
		p.tracks = append(p.tracks, track)
		p.styles[track.ID] = uint8(i%5 + 1)
	}
	p.lyrics.data.Lines = nil
	for i := range 32 {
		p.lyrics.data.Lines = append(p.lyrics.data.Lines, applemusic.LyricLine{
			Text: fmt.Sprintf("合成歌词 %02d / Synthetic line", i), Start: time.Duration(i) * time.Second, End: time.Duration(i+1) * time.Second,
		})
	}
	p.input.SetValue("Synthetic fixture")
	p.status = "NOT LIVE PLAYBACK · SYNTHETIC FIXTURE · NO AUDIO"
	return p, now
}

func assertPresentationFrame(t *testing.T, p *AppleMusicPage, frame string) {
	t.Helper()
	for _, line := range strings.Split(frame, "\n") {
		if ansi.WcWidth.StringWidth(line) != p.screen.w || ansi.GraphemeWidth.StringWidth(line) != p.screen.w {
			t.Fatal("frame exceeds its terminal width")
		}
	}
	for _, hit := range p.hits {
		if hit.x < 0 || hit.y < 0 || hit.x+hit.w > p.screen.w || hit.y+hit.h > p.screen.h {
			t.Fatalf("frame retains an offscreen hit: %+v", hit)
		}
	}
	if !strings.Contains(ansi.Strip(frame), "NOT LIVE PLAYBACK") {
		t.Fatal("synthetic fixture lost its visible disclosure")
	}
}

func TestAppleMusicPresentationPreviewScenes(t *testing.T) {
	presentationPreviewDir(t)
	defer SetLanguage("en")
	SetLanguage("en")
	for _, size := range presentationPreviewSizes() {
		for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
			for _, view := range []string{"browse", "listen", "browse-zoom", "listen-zoom", "panel"} {
				name := fmt.Sprintf("scene-%s-%dx%d-%s", view, size[0], size[1], level)
				t.Run(name, func(t *testing.T) {
					p, now := presentationPreviewPage(t)
					p.level = level
					if strings.HasPrefix(view, "browse") {
						p.scene, p.sceneMotion = appleBrowse, appleTween{}
					}
					if strings.HasSuffix(view, "zoom") {
						p.setZoom(true, now.Add(-time.Second))
					}
					if view == "panel" {
						p.openPanel(now.Add(-time.Second))
					}
					assertPresentationFrame(t, p, p.draw(size[0], size[1], now, 2))
					exportPresentation(t, p, name)
				})
			}
		}
	}
}

func TestAppleMusicPresentationPreviewLyrics(t *testing.T) {
	presentationPreviewDir(t)
	defer SetLanguage("en")
	for _, lang := range []string{"en", "zh"} {
		SetLanguage(lang)
		for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
			for _, state := range []string{"loading", "plain", "manual", "synced", "paused", "stale", "no_lyrics", "request_error", "preview", "unconfirmed"} {
				name := fmt.Sprintf("lyrics-%s-%s-%s", lang, state, level)
				t.Run(name, func(t *testing.T) {
					p, now := presentationPreviewPage(t)
					p.level = level
					setLyricsPresentationState(p, state, now)
					if state == "manual" {
						p.lyrics.top = 8
					}
					assertPresentationFrame(t, p, p.draw(140, 45, now, 2))
					exportPresentation(t, p, name)
				})
			}
		}
	}
}

func TestAppleMusicPresentationPreviewSearch(t *testing.T) {
	presentationPreviewDir(t)
	defer SetLanguage("en")
	SetLanguage("en")
	for _, width := range []int{80, 99, 100, 140} {
		for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
			for _, source := range []appleScene{appleBrowse, appleListen} {
				name := fmt.Sprintf("search-return-%d-%s-from-%d", width, level, source)
				t.Run(name, func(t *testing.T) {
					p, now := presentationPreviewPage(t)
					p.level, p.scene = level, source
					p.sceneMotion = appleTween{from: float64(source), to: float64(source)}
					p.setZoom(true, now.Add(-time.Second))
					p.editSearch(now)
					for _, elapsed := range []time.Duration{0, 100 * time.Millisecond} {
						frame := p.draw(width, 30, now.Add(elapsed), 2)
						assertPresentationFrame(t, p, frame)
						assertPresentationEditorHits(t, p)
						if !strings.Contains(ansi.Strip(frame), "/ Synthetic fixture▏") {
							t.Fatal("search input is not visible on its first focused frame")
						}
						exportPresentation(t, p, fmt.Sprintf("%s-%04dms", name, elapsed.Milliseconds()))
					}
				})
			}
		}
	}
}

func TestAppleMusicPresentationPreviewInsertionCloseUp(t *testing.T) {
	presentationPreviewDir(t)
	defer SetLanguage("en")
	SetLanguage("en")
	for _, size := range [][2]int{{140, 45}, {120, 24}, {80, 30}} {
		for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
			name := fmt.Sprintf("insert-%dx%d-%s", size[0], size[1], level)
			t.Run(name, func(t *testing.T) {
				p, now := presentationPreviewPage(t)
				p.level, p.scene, p.sceneMotion = level, appleBrowse, appleTween{}
				p.draw(size[0], size[1], now, 2)
				p.selected = 1
				p.beginInsert(p.tracks[1], now)
				state := p.state
				state.Track = p.tracks[1]
				p.acceptState(state, now)
				p.lyrics.status, p.lyrics.data.Lines = "loading", nil
				start := p.insertStart
				for _, ms := range []int{0, 80, 380, 500, 660, 760, 1080, 1280, 1630} {
					assertPresentationFrame(t, p, p.draw(size[0], size[1], start.Add(time.Duration(ms)*time.Millisecond), 2))
					exportPresentation(t, p, fmt.Sprintf("%s-%04dms", name, ms))
				}
				if p.inserting || !p.zoomed || p.camera.value(start.Add(1630*time.Millisecond)) != 1 {
					t.Fatal("automatic close-up did not reach its real 1630ms endpoint")
				}
			})
		}
	}
}
