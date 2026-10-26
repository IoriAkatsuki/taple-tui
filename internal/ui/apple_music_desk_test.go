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
