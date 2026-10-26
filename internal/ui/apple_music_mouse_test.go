package ui

import (
	"context"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/motion"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

type appleMouseClient struct {
	actions  []applemusic.Action
	shown    int
	searches int
	state    applemusic.State
}

func (c *appleMouseClient) Snapshot(context.Context) (applemusic.State, error) {
	return c.state, nil
}
func (c *appleMouseClient) Catalog(context.Context, applemusic.CatalogRequest) (applemusic.CatalogPage, error) {
	c.searches++
	return applemusic.CatalogPage{}, nil
}
func (c *appleMouseClient) Do(_ context.Context, a applemusic.Action) error {
	c.actions = append(c.actions, a)
	return nil
}
func (c *appleMouseClient) Show(context.Context) error { c.shown++; return nil }
func (*appleMouseClient) Lyrics(context.Context, applemusic.LyricsRequest) (applemusic.Lyrics, error) {
	return applemusic.Lyrics{Status: "unavailable"}, nil
}

func TestAppleMusicMouseUsesRenderedTargets(t *testing.T) {
	for _, size := range [][2]int{{120, 38}, {80, 24}, {50, 20}} {
		c := &appleMouseClient{}
		p := NewAppleMusicPage(c)
		defer p.Close()
		p.level = motion.Off
		p.state = applemusic.State{Authorized: true, Track: applemusic.Track{ID: "current", Duration: 100}, Volume: .5}
		p.tracks = []applemusic.Track{{ID: "song-a", Name: "第一首"}, {ID: "song-b", Name: "Second"}}
		p.draw(size[0], size[1], time.Unix(100, 0), 2)
		for _, h := range p.hits {
			if h.x < 0 || h.y < 0 || h.x+h.w > size[0] || h.y+h.h > size[1] {
				t.Fatalf("%v: offscreen hit %+v", size, h)
			}
		}
		var row *appleHit
		for i := range p.hits {
			if p.hits[i].kind == appleHitTrack && p.hits[i].index == 0 {
				row = &p.hits[i]
				break
			}
		}
		if row == nil {
			t.Fatalf("%v: no clickable track", size)
		}
		_, cmd := p.Update(tea.MouseClickMsg{X: row.x, Y: row.y, Button: tea.MouseLeft}, model.NewApp(model.DefaultOptions()))
		if cmd == nil || len(c.actions) != 0 {
			t.Fatal("mouse playback must run in a command, not UI goroutine")
		}
		_ = cmd()
		if len(c.actions) != 1 || c.actions[0].ID != "song-a" {
			t.Fatalf("wrong click action: %+v", c.actions)
		}
	}
}

func TestAppleMusicIdleAnimationIsNotFakePlayback(t *testing.T) {
	now := time.Unix(100, 0)
	for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
		p := NewAppleMusicPage(nil)
		defer p.Close()
		p.level = level
		p.state = applemusic.State{Ready: true, Authorized: true}
		a := p.draw(120, 38, now, 2)
		b := p.draw(120, 38, now.Add(1500*time.Millisecond), 2)
		if (a != b) != (level != motion.Off) {
			t.Fatalf("idle level=%v changed=%t", level, a != b)
		}
		if !strings.Contains(ansi.Strip(a), "STANDBY") || !strings.Contains(ansi.Strip(a), "Cassette Deck") {
			t.Fatal("idle/device labels missing")
		}
		if p.state.Playing || p.state.Position != 0 || p.reels.Moving() {
			t.Fatal("standby faked playback or spinning reels")
		}
	}
}

func TestAppleMusicRepeatedSearchClickKeepsFocusState(t *testing.T) {
	p := NewAppleMusicPage(&appleMouseClient{})
	defer p.Close()
	p.level = motion.Off
	p.draw(120, 38, time.Unix(100, 0), 2)
	for _, h := range p.hits {
		if h.key == "search" {
			p.input.SetValue("query")
			if p.handleClick(h.x, h.y) == nil {
				t.Fatal("search not started")
			}
			if p.handleClick(h.x, h.y) != nil {
				t.Fatal("duplicate search dispatched")
			}
			if p.editing && !p.input.Focused() {
				t.Fatal("invisible unfocused editing state")
			}
			return
		}
	}
	t.Fatal("search submit hit missing")
}

func TestAppleMusicClickControlsAndResize(t *testing.T) {
	c := &appleMouseClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Off
	p.state = applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Track: applemusic.Track{ID: "current", Duration: 100}, Volume: .5, Queue: []applemusic.Track{{ID: "queued-a", Name: "Queued"}}}
	c.state = p.state
	p.tracks = []applemusic.Track{{ID: "song-a", Name: "Song"}}
	p.draw(120, 38, time.Unix(100, 0), 2)
	click := func(kind appleHitKind, key string, right bool) {
		t.Helper()
		var found *appleHit
		for i := range p.hits {
			h := &p.hits[i]
			if h.kind == kind && (key == "" || h.key == key) {
				found = h
				break
			}
		}
		if found == nil {
			t.Fatalf("missing control %v/%s", kind, key)
		}
		x := found.x
		if right {
			x += found.w - 1
		}
		p.busy = false
		if cmd := p.handleClick(x, found.y); cmd != nil {
			_ = cmd()
		}
	}
	click(appleHitKey, "space", false)
	click(appleHitSeek, "", true)
	click(appleHitVolume, "", true)
	if len(c.actions) != 3 || c.actions[0].Kind != "toggle" || c.actions[1].Value != 100 || c.actions[2].Value != 1 {
		t.Fatalf("controls: %+v", c.actions)
	}
	p.input.SetValue("query")
	click(appleHitKey, "search", false)
	if c.searches != 1 {
		t.Fatal("search button played a track instead of submitting query")
	}
	p.drawer = appleQueue
	p.draw(120, 38, time.Unix(101, 0), 2)
	click(appleHitTrack, "", false)
	last := c.actions[len(c.actions)-1]
	if last.Kind != "jump" || last.ID != "queued-a" {
		t.Fatalf("queue click: %+v", last)
	}
	p.Update(tea.WindowSizeMsg{Width: 50, Height: 20}, model.NewApp(model.DefaultOptions()))
	if len(p.hits) != 0 {
		t.Fatal("resize retained stale controls")
	}
}

func TestAppleMusicStaleTrackHitDoesNotPlayAnotherSong(t *testing.T) {
	p := NewAppleMusicPage(&appleMouseClient{})
	defer p.Close()
	p.level = motion.Off
	p.tracks = []applemusic.Track{{ID: "before", Name: "Before"}}
	p.draw(120, 38, time.Unix(100, 0), 2)
	for _, h := range p.hits {
		if h.kind == appleHitTrack {
			p.tracks[0].ID = "after"
			if cmd := p.handleClick(h.x, h.y); cmd != nil {
				t.Fatal("stale hit dispatched replacement track")
			}
			return
		}
	}
	t.Fatal("missing track hit")
}

func TestAppleMusicWalkmanZoomAndInsertion(t *testing.T) {
	c := &appleMouseClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Full
	p.state = applemusic.State{Authorized: true, Track: applemusic.Track{ID: "track-1", Duration: 180}, Playing: true}
	p.tracks = []applemusic.Track{{ID: "track-1", Name: "First Song", Artist: "Artist One"}}

	// 1. Initial draw: zoomed is false
	p.draw(120, 38, time.Unix(100, 0), 2)
	if p.zoomed {
		t.Fatal("expected zoomed to be false initially")
	}

	// 2. Click stage -> toggles zoomed to true
	var stageHit *appleHit
	for i := range p.hits {
		if p.hits[i].kind == appleHitStage {
			stageHit = &p.hits[i]
			break
		}
	}
	if stageHit == nil {
		t.Fatal("stage hit missing")
	}
	p.handleClick(stageHit.x, stageHit.y)
	if !p.zoomed {
		t.Fatal("clicking stage did not toggle zoomed to true")
	}

	// Click stage again -> toggles zoomed to false
	p.handleClick(stageHit.x, stageHit.y)
	if p.zoomed {
		t.Fatal("clicking stage again did not toggle zoomed back to false")
	}

	// 3. Select track -> triggers insertion animation
	var trackHit *appleHit
	for i := range p.hits {
		if p.hits[i].kind == appleHitTrack {
			trackHit = &p.hits[i]
			break
		}
	}
	if trackHit == nil {
		t.Fatal("track hit missing")
	}
	cmd := p.handleClick(trackHit.x, trackHit.y)
	if cmd == nil {
		t.Fatal("clicking track did not return play command")
	}
	if !p.inserting {
		t.Fatal("clicking track did not initiate insertion animation")
	}

	// 4. Verify insertion frame progresses Walkman Door & Cassette
	f := p.cassette(60, 20, p.insertStart.Add(350*time.Millisecond), 2)
	if f == nil || len(f.Cells) == 0 {
		t.Fatal("cassette frame during insertion was nil or empty")
	}
}

func TestAppleMusicStageHoverTiltsWalkman(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	now := time.Unix(100, 0)
	p.draw(140, 45, now, 2)
	if p.stageRect.Empty() {
		t.Fatal("stageRect was not recorded during draw")
	}
	base := p.walkmanPose(now, 0)
	p.hoverX, p.hoverY = p.stageRect.Min.X+2, p.stageRect.Min.Y+2
	tiltedLeft := p.walkmanPose(now, 0)
	p.hoverX, p.hoverY = p.stageRect.Max.X-2, p.stageRect.Max.Y-2
	tiltedRight := p.walkmanPose(now, 0)
	if tiltedLeft.Yaw <= base.Yaw || tiltedRight.Yaw >= base.Yaw {
		t.Fatalf("Walkman did not tilt toward stage pointer: left=%v base=%v right=%v", tiltedLeft.Yaw, base.Yaw, tiltedRight.Yaw)
	}
	if tiltedLeft.Pitch >= base.Pitch || tiltedRight.Pitch <= base.Pitch {
		t.Fatalf("Walkman pitch did not respond to vertical pointer: top=%v base=%v bottom=%v", tiltedLeft.Pitch, base.Pitch, tiltedRight.Pitch)
	}
}
