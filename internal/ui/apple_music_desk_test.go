package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/motion"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

func TestAppleMusicMartinaiseDeskDiurnalPhases(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	p.state = applemusic.State{Ready: true, Authorized: true}

	r := appleBox(5, 3, 52, 18)
	dawn := time.Date(2026, 10, 24, 6, 30, 0, 0, time.Local)
	day := time.Date(2026, 10, 24, 13, 0, 0, 0, time.Local)
	sunset := time.Date(2026, 10, 24, 18, 0, 0, 0, time.Local)
	night := time.Date(2026, 10, 24, 23, 30, 0, 0, time.Local)

	renderStage := func(at time.Time) string {
		p.screen.reset(70, 26)
		p.screen.pal = appleNeon
		p.drawMartinaiseStage(r.Min.X, r.Min.Y, r.Dx(), r.Dy(), at)
		return p.screen.render()
	}

	dawnOut := renderStage(dawn)
	dayOut := renderStage(day)
	sunsetOut := renderStage(sunset)
	nightOut := renderStage(night)

	if dawnOut == dayOut || dayOut == sunsetOut || sunsetOut == nightOut {
		t.Fatal("diurnal phases did not transform the Martinaise desk stage")
	}
	if !strings.ContainsAny(ansi.Strip(nightOut), "✦✧⋆") {
		t.Fatal("night phase did not render window stars above the horizon")
	}
	if p.state.Playing || p.reels.Moving() || p.meter.Frame().Ready {
		t.Fatal("diurnal desk stage fabricated playback or audio signal")
	}
}

func TestAppleMusicDossierFolderCrateForArtistsAndPlaylists(t *testing.T) {
	c := &appleCatalogClient{page: applemusic.CatalogPage{Items: []applemusic.CatalogItem{
		{Type: "artists", ID: "art-1", Name: "Sea Power"},
		{Type: "artists", ID: "art-2", Name: "Daft Punk"},
		{Type: "artists", ID: "art-3", Name: "Boards of Canada"},
	}}}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Full
	p.input.SetValue("Sea")
	a := model.NewApp(model.DefaultOptions())
	p.Update(p.handleKey(tea.KeyPressMsg{Code: '6'})(), a)
	p.rackFocused = true

	now := time.Unix(100, 0)
	p.draw(140, 45, now, 2)
	out := ansi.Strip(p.draw(140, 45, now.Add(240*time.Millisecond), 2))
	if !strings.Contains(out, "FOLDER 01") || !strings.Contains(out, "Sea Power") {
		t.Fatalf("artist search did not render 3D dossier folder crate: %s", out)
	}
	if len(p.styles) != 0 {
		t.Fatal("3D dossier folder crate consumed random cassette styles")
	}
	if p.rackPickup.pop.value(now.Add(240*time.Millisecond)) != .5 {
		t.Fatal("focused dossier folder did not settle at half-open V-parting")
	}
}

func TestAppleMusicStageGlowMatchesCompanionRhythmAndHover(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	p.state = applemusic.State{Ready: true, Authorized: true}

	now := time.Date(2026, 10, 24, 23, 30, 0, 0, time.Local)
	baseFrame := p.draw(140, 45, now, 2)
	quarterCycle := p.draw(140, 45, now.Add(1050*time.Millisecond), 2)
	if baseFrame == quarterCycle {
		t.Fatal("stage glow did not breathe along the 4.2s companion cycle")
	}
	if p.state.Playing || p.reels.Moving() || p.meter.Frame().Ready {
		t.Fatal("synchronized stage glow fabricated playback or audio signal")
	}

	// Steady-state subpixel glow compositing on the right stage must remain allocation-free.
	r := p.stageRect
	if r.Empty() {
		t.Fatal("expected non-empty stageRect after draw")
	}
	p.drawMartinaiseStage(r.Min.X, r.Min.Y, r.Dx(), r.Dy(), now)
	allocs := testing.AllocsPerRun(20, func() {
		p.drawMartinaiseStage(r.Min.X, r.Min.Y, r.Dx(), r.Dy(), now.Add(250*time.Millisecond))
	})
	if allocs > 0 {
		t.Fatalf("drawMartinaiseStage allocated %.0f objects per frame, want 0", allocs)
	}

	// Hovering inside the stage flares the lamp & amber glow just like the left companion.
	p.hoverX, p.hoverY = r.Min.X+2, r.Min.Y+2
	hoverFrame := p.draw(140, 45, now, 2)
	if hoverFrame == baseFrame {
		t.Fatal("hovering stage did not trigger companion-style glow boost")
	}

	// motion.Off freezes all desk glow & firefly motion across time.
	p.hoverX, p.hoverY = -1, -1
	p.level = motion.Off
	offA := p.draw(140, 45, now, 2)
	offB := p.draw(140, 45, now.Add(1050*time.Millisecond), 2)
	if offA != offB {
		t.Fatal("motion.Off animated stage glow over time")
	}
}

func TestAppleMusic3DDeskStagePerspectiveAndParallax(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	p.state = applemusic.State{Ready: true, Authorized: true}

	now := time.Date(2026, 10, 24, 23, 30, 0, 0, time.Local)
	restFrame := p.draw(140, 45, now, 2)
	r := p.stageRect
	if r.Empty() {
		t.Fatal("expected non-empty stageRect after draw")
	}

	// Hovering left vs right inside the stage shifts the 3D perspective parallax camera.
	p.hoverX, p.hoverY = r.Min.X+2, r.Min.Y+r.Dy()/2
	leftOut := p.draw(140, 45, now, 2)
	p.hoverX, p.hoverY = r.Max.X-2, r.Min.Y+r.Dy()/2
	rightOut := p.draw(140, 45, now, 2)
	if leftOut == rightOut {
		t.Fatal("3D desk stage did not shift perspective parallax across horizontal hover")
	}

	// Switching pointer parallax back and forth reuses pre-allocated G-buffer capacity with 0 heap allocs.
	toggle := false
	allocs := testing.AllocsPerRun(10, func() {
		toggle = !toggle
		if toggle {
			p.hoverX = r.Min.X + 3
		} else {
			p.hoverX = r.Max.X - 3
		}
		p.drawMartinaiseStage(r.Min.X, r.Min.Y, r.Dx(), r.Dy(), now)
	})
	if allocs > 0 {
		t.Fatalf("parallax camera shift allocated %.0f objects per frame, want 0", allocs)
	}

	// Engaging Close-up camera zooms both the Walkman and the 3D desk backdrop.
	p.hoverX, p.hoverY = -1, -1
	p.setZoom(true, now)
	zoomFrame := p.draw(140, 45, now.Add(400*time.Millisecond), 2)
	if zoomFrame == restFrame {
		t.Fatal("close-up zoom did not change 3D stage perspective")
	}

	// Compact and narrow stage dimensions render safely without panics.
	p.screen.reset(40, 20)
	p.screen.pal = appleNeon
	p.drawMartinaiseStage(2, 2, 12, 5, now)
	p.drawMartinaiseStage(2, 2, 26, 9, now)
}
