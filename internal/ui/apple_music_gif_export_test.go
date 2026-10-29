package ui

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anhoder/foxful-cli/motion"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

const gifStepMS = 30

func overlayActiveVUMeter(p *AppleMusicPage, step int) {
	if p.stageRect.Empty() || p.panel != "" {
		return
	}
	l := 0.62 + 0.28*math.Sin(float64(step)*0.38) + 0.08*math.Cos(float64(step)*0.91)
	r := 0.56 + 0.26*math.Sin(float64(step)*0.38+0.7) + 0.09*math.Sin(float64(step)*0.83)
	x, y, w := p.stageRect.Min.X+25, p.stageRect.Max.Y+1, p.stageRect.Dx()-26
	p.screen.fill(x, y, w, 4, appleNeon.Bg)
	p.drawMeter(x, y, w, applemusic.MeterFrame{Ready: true, Levels: [2]float64{l, r}})
	p.screen.fillBackground()
}

func exportInsertGIFSequence(t *testing.T, dir string) {
	t.Helper()
	p, now := presentationPreviewPage(t)
	p.level, p.scene, p.sceneMotion = motion.Full, appleBrowse, appleTween{}
	p.state.Playing = true
	p.draw(140, 45, now, 2)
	p.selected = 1
	p.beginInsert(p.tracks[1], now.Add(180*time.Millisecond))
	state := p.state
	state.Track = p.tracks[1]
	state.Playing = true
	p.acceptState(state, now.Add(180*time.Millisecond))
	p.lyrics.status = "synced"
	idx := 0
	for ms := 0; ms <= 2160; ms += gifStepMS {
		at := now.Add(time.Duration(ms) * time.Millisecond)
		p.state.Position = 1.0 + float64(ms)/1000.0
		p.draw(140, 45, at, 2)
		overlayActiveVUMeter(p, idx)
		writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("insert-%04d.html", idx)))
		idx++
	}
}

func exportStorageGIFSequence(t *testing.T, dir string) {
	t.Helper()
	p, now := presentationPreviewPage(t)
	p.level, p.scene, p.sceneMotion = motion.Full, appleBrowse, appleTween{}
	p.styles = map[string]uint8{p.state.Track.ID: 5}
	p.lastCassetteStyle = 5
	p.bayID, p.bayStyle, p.bayNumber = p.state.Track.ID, 5, 1
	p.state.Playing = true
	idx := 0
	for ms := 0; ms <= 2400; ms += gifStepMS {
		at := now.Add(time.Duration(ms) * time.Millisecond)
		switch ms {
		case 120:
			storageHover(p, 1, at)
		case 510:
			storageHover(p, 3, at)
		case 960:
			p.selected = 3
			p.beginInsert(p.tracks[3], at)
			p.pending, p.pendingID = true, p.tracks[3].ID
			p.generation, p.playGeneration, p.settledPlayGeneration = 1, 1, 1
		case 1140:
			s := p.state
			s.Track, s.Position = p.tracks[3], 0
			p.acceptState(s, at)
		}
		p.state.Position = float64(ms) / 1000.0
		p.draw(140, 45, at, 2)
		overlayActiveVUMeter(p, idx)
		writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("storage-%04d.html", idx)))
		idx++
	}
}

func setupLibraryCratesCatalog(p *AppleMusicPage, kind, resType string, names []string) {
	c := &appleLibraryClient{}
	for i, name := range names {
		c.page.Items = append(c.page.Items, applemusic.CatalogItem{
			Type: resType, ID: fmt.Sprintf("%s-%d", resType, i),
			Name: name, Artist: "Apple Music Library", Playable: true,
		})
	}
	p.client = c
	p.drawer = appleLibrary
	p.acceptCatalog(p.requestCatalog(applemusic.CatalogRequest{Kind: kind, ID: "lib"}, "Library", false)().(appleCatalogMsg))
	p.rackFocused = true
}

func exportLibraryCratesGIFSequence(t *testing.T, dir string) {
	t.Helper()
	p, _ := presentationPreviewPage(t)
	p.level, p.scene, p.sceneMotion = motion.Full, appleBrowse, appleTween{}
	p.state.Playing = true
	albums := []string{"Discovery · Daft Punk", "Random Access Memories", "Untrue · Burial", "Selected Ambient Works", "Music Has the Right to Children", "Cross · Justice"}
	playlists := []string{"Late Night Synthwave", "Tokyo Walkman 2000", "Lo-Fi Cassette Beats", "Analog Tape Archive", "Cyber Y2K Essentials", "Midnight Drive"}
	base := time.Date(2026, 10, 24, 6, 30, 0, 0, time.Local)
	setupLibraryCratesCatalog(p, applemusic.LibraryAlbums, "library-albums", albums)
	p.selected = 0
	idx := 0
	for ms := 0; ms <= 2520; ms += gifStepMS {
		switch ms {
		case 420:
			p.selected = 2
		case 930:
			base = time.Date(2026, 10, 24, 13, 0, 0, 0, time.Local)
			setupLibraryCratesCatalog(p, applemusic.LibraryPlaylists, "library-playlists", playlists)
			p.selected = 0
		case 1350:
			p.selected = 2
		case 1830:
			base = time.Date(2026, 10, 24, 23, 30, 0, 0, time.Local)
			p.drawer = appleQueue
			p.state.QueueTotal = len(p.tracks)
			p.state.QueueIndex = 0
			p.state.Queue = p.tracks
			p.selected, p.rackFocused = 1, true
		case 2160:
			p.selected = 3
		}
		at := base.Add(time.Duration(ms) * time.Millisecond)
		p.state.Position = 12.0 + float64(ms)/1000.0
		p.draw(140, 45, at, 2)
		overlayActiveVUMeter(p, idx)
		writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("crates-%04d.html", idx)))
		idx++
	}
}

func exportListenLyricsGIFSequence(t *testing.T, dir string) {
	t.Helper()
	p, _ := presentationPreviewPage(t)
	p.level, p.scene = motion.Full, appleListen
	p.sceneMotion = appleTween{from: 1, to: 1}
	setLyricsPresentationState(p, "synced", time.Date(2026, 10, 24, 23, 30, 0, 0, time.Local))
	p.reels.Run()
	night := time.Date(2026, 10, 24, 23, 30, 0, 0, time.Local)
	idx := 0
	for ms := 0; ms <= 2340; ms += gifStepMS {
		at := night.Add(time.Duration(ms) * time.Millisecond)
		p.state.Position = 1.15 + float64(ms)*1.85/1000.0
		p.stateAt = at
		p.draw(140, 45, at, 2)
		overlayActiveVUMeter(p, idx)
		writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("listen-%04d.html", idx)))
		idx++
	}
}

func exportSwapSettingsGIFSequence(t *testing.T, dir string) {
	t.Helper()
	p, now := swapTestPage(t)
	p.level, p.scene = motion.Full, appleListen
	p.sceneMotion = appleTween{from: 1, to: 1}
	p.zoomed, p.camera.from, p.camera.to = true, 1, 1
	p.state.Playing, p.state.Full = true, true
	p.state.Track.Name, p.state.Track.Artist = "Side A · Chrome Tape", "Synthetic Deck"
	p.lyrics.status = "synced"
	idx := 0
	for ms := 0; ms <= 2400; ms += gifStepMS {
		at := now.Add(time.Duration(ms) * time.Millisecond)
		switch ms {
		case 90:
			p.beginSwap(at)
			p.dispatch(applemusic.Action{Kind: "next"})
		case 480:
			confirmSwap(p, "track-side-b", at)
			p.state.Track.Name = "Side B · Azimuth Lock"
		case 1560:
			p.openPanel(at)
		case 1860:
			SetLanguage("zh")
		case 2070:
			SetLanguage("en")
			p.panel = appleLogoutPanel
		}
		p.state.Position = 18.0 + float64(ms)/1000.0
		p.draw(140, 45, at, 2)
		overlayActiveVUMeter(p, idx)
		writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("swap-%04d.html", idx)))
		idx++
	}
}

func TestAppleMusicHighFPSGIFExport(t *testing.T) {
	dir := os.Getenv("TAPLE_GIF_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set TAPLE_GIF_PREVIEW_DIR to export 33.33fps (30ms step) animation frames")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	defer SetLanguage("en")
	SetLanguage("en")
	exportInsertGIFSequence(t, dir)
	exportStorageGIFSequence(t, dir)
	exportLibraryCratesGIFSequence(t, dir)
	exportListenLyricsGIFSequence(t, dir)
	exportSwapSettingsGIFSequence(t, dir)
}
