package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anhoder/foxful-cli/motion"
)

const gifStepMS = 30

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
		writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("storage-%04d.html", idx)))
		idx++
	}
}

func TestAppleMusicHighFPSGIFExport(t *testing.T) {
	dir := os.Getenv("TAPEFOX_GIF_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set TAPEFOX_GIF_PREVIEW_DIR to export 33.33fps (30ms step) animation frames")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	defer SetLanguage("en")
	SetLanguage("en")
	exportInsertGIFSequence(t, dir)
	exportStorageGIFSequence(t, dir)
}
