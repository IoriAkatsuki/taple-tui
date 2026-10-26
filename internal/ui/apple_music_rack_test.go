package ui

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anhoder/foxful-cli/motion"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

func TestAppleMusicStandbyGlowChangesTheStageNotPlayback(t *testing.T) {
	for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
		p := NewAppleMusicPage(nil)
		p.level = level
		p.state = applemusic.State{Ready: true, Authorized: true}
		now := time.Unix(100, 0)
		r := appleBox(5, 3, 50, 14)
		var frames [2][]color.Color
		var glyphs [2][]string
		var hitStage [2][]bool
		for i, at := range []time.Time{now, now.Add(1500 * time.Millisecond)} {
			p.hits = p.hits[:0]
			p.screen.reset(70, 25)
			p.screen.pal = appleNeon
			p.drawDevice(r, at, 2)
			p.screen.fillBackground()
			for y := r.Min.Y + 1; y < r.Max.Y-2; y++ {
				for x := r.Min.X + 1; x < r.Max.X-1; x++ {
					cell := p.screen.buf.CellAt(x, y)
					frames[i] = append(frames[i], cell.Style.Bg)
					glyphs[i] = append(glyphs[i], fmt.Sprintf("%s/%v/%v", cell.Content, cell.Style.Fg, cell.Style.Bg))
					hitStage[i] = append(hitStage[i], p.hitAt(x, y) != nil)
				}
			}
		}
		glowChanged, poseChanged := 0, 0
		for i := range frames[0] {
			if !hitStage[0][i] && !hitStage[1][i] {
				if !npSameColor(frames[0][i], frames[1][i]) {
					glowChanged++
					x := r.Min.X + 1 + i%(r.Dx()-2)
					y := r.Min.Y + 1 + i/(r.Dx()-2)
					if p.hitAt(x, y) != nil {
						t.Fatal("decorative glow became a clickable device area")
					}
				}
			} else if glyphs[0][i] != glyphs[1][i] {
				poseChanged++
			}
		}
		if (glowChanged > 0) != (level != motion.Off) {
			t.Fatalf("level %v: the actual device stage has %d glow changes", level, glowChanged)
		}
		if (poseChanged > 0) != (level != motion.Off) {
			t.Fatalf("level %v: expected idle Walkman angle drift=%v, got %d body changes", level, level != motion.Off, poseChanged)
		}
		if p.state.Playing || p.state.Position != 0 || p.reels.Moving() || p.meter.Frame().Ready {
			t.Fatal("idle glow fabricated playback")
		}
		p.Close()
	}
}

func TestAppleMusicStandbyGlowStopsDuringPlaybackOrLoading(t *testing.T) {
	for _, kind := range []string{"playing", "pending", "inserting", "swapping"} {
		p := NewAppleMusicPage(nil)
		p.level = motion.Full
		p.screen.reset(70, 25)
		p.screen.pal = appleNeon
		now := time.Unix(100, 0)
		switch kind {
		case "playing":
			p.state.Playing = true
		case "pending":
			p.pending = true
		case "inserting":
			p.inserting = true
		case "swapping":
			p.swapStart = now
		}
		frame := p.cassette(48, 12, now, 2)
		before := p.screen.render()
		p.drawStandbyGlow(appleBox(6, 4, 48, 12), frame, 6, 4, now.Add(2*time.Second))
		if p.screen.render() != before {
			t.Fatalf("%s still emitted idle glow", kind)
		}
		p.Close()
	}
}

func TestAppleMusicRackUsesCassetteGeometryAndRecordedFlightOrigin(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	p.tracks = []applemusic.Track{{ID: "a", Name: "磁带 A"}, {ID: "b", Name: "磁带 B"}, {ID: "c", Name: "磁带 C"}}
	p.selected, p.rackFocused = 1, true
	now := time.Unix(100, 0)
	p.draw(140, 45, now, 2)
	p.draw(140, 45, now.Add(200*time.Millisecond), 2)
	p.beginInsert(p.tracks[p.selected], now.Add(200*time.Millisecond))
	p.draw(140, 45, p.insertStart.Add(-time.Nanosecond), 2)
	var selected appleHit
	for _, hit := range p.hits {
		if hit.kind == appleHitTrack && hit.index == p.selected {
			selected = hit
		}
	}
	if selected.cassette.W <= 0 {
		t.Fatal("pickup placement missing")
	}
	state := deck.WalkmanState{State: deck.State{Style: p.insertStyle, TrackNum: selected.index + 1, Radii: deck.PackRadii(0, false)},
		Cassette: 1, OnlyCassette: true, FlightFrom: selected.cassette, FlightPose: selected.pose}
	state.FlightFrom.Y -= 3
	var renderer deck.WalkmanRenderer
	want := renderer.RenderWalkman(p.rackView, deckPaletteOf(appleNeon), deck.RestPose(), state)
	check := func(stage string) {
		t.Helper()
		matched := 0
		for i, cell := range want.Cells {
			if cell.Ch == "" {
				continue
			}
			actual := p.screen.buf.CellAt(i%want.W, 3+i/want.W)
			if actual.Content != cell.Ch || !npSameColor(actual.Style.Bg, cell.Bg) {
				t.Fatalf("%s changed the same cassette at cell %d", stage, i)
			}
			matched++
		}
		if matched < 20 {
			t.Fatal("picked cassette is not visible")
		}
	}
	check("fully extracted")
	p.draw(140, 45, p.insertStart, 2)
	check("first flight frame")
	if p.mechanics(p.insertStart).TrackNum != selected.index+1 {
		t.Fatal("cassette label changed on extraction")
	}
	p.state.Track.ID = selected.id
	p.advance(p.insertStart.Add(1300 * time.Millisecond))
	if p.mechanics(p.insertStart.Add(1300*time.Millisecond)).Style != state.Style {
		t.Fatal("cassette colour changed after insertion")
	}
}

func TestAppleMusicRackPartialSlotsAreNotClickable(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.tracks = []applemusic.Track{{ID: "a"}, {ID: "b"}}
	p.screen.reset(140, 45)
	p.screen.pal = appleNeon
	p.drawList(2, 2, 60, 5, time.Unix(100, 0))
	count := 0
	for _, h := range p.hits {
		if h.kind == appleHitTrack {
			count++
		}
	}
	if count != 2 || p.hitAt(4, 6) != nil {
		t.Fatal("incomplete cassette slot accepts clicks")
	}
}

func TestAppleMusicRackAndGlowPreview(t *testing.T) {
	dir := os.Getenv("TAPEFOX_RACK_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set TAPEFOX_RACK_PREVIEW_DIR for physical cassettes and standby glow")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	p.state = applemusic.State{Ready: true, Authorized: true}
	for i, name := range []string{"Night Drive / 夜行磁带", "磁带 B 面", "Electric Dreams", "霓虹街角", "After Midnight", "Low Fidelity"} {
		p.tracks = append(p.tracks, applemusic.Track{ID: fmt.Sprint(i), Name: name, Artist: "视觉测试曲目", Duration: 245})
	}
	p.selected = 1
	p.status = "FRAMEBUFFER TEST · 待机光晕，不是播放或电平"
	for _, size := range [][2]int{{140, 45}, {120, 38}, {120, 24}, {80, 30}, {50, 19}} {
		for _, ms := range []int{0, 1000, 2000, 3000} {
			p.draw(size[0], size[1], time.Unix(100, 0).Add(time.Duration(ms)*time.Millisecond), 2)
			writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("rack-%dx%d-%04d.html", size[0], size[1], ms)))
		}
	}
}
