package ui

import (
	"context"
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
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

type appleBoundaryClient struct {
	appleMouseClient
	change   func(*applemusic.State, applemusic.Action)
	doErr    error
	reads    int
	failRead int
}

func (c *appleBoundaryClient) Snapshot(context.Context) (applemusic.State, error) {
	c.reads++
	if c.reads == c.failRead {
		return applemusic.State{}, errors.New("snapshot temporarily unavailable")
	}
	return c.state, nil
}

func (c *appleBoundaryClient) Do(_ context.Context, action applemusic.Action) error {
	c.actions = append(c.actions, action)
	if c.change != nil {
		c.change(&c.state, action)
	}
	return c.doErr
}

func boundaryTestPage(t *testing.T, index, total int) (*AppleMusicPage, *appleBoundaryClient) {
	t.Helper()
	p, _ := swapTestPage(t)
	p.zoomed, p.camera.to = true, 1
	p.state.QueueIndex, p.state.QueueTotal = index, total
	p.bayNumber = index + 1
	c := &appleBoundaryClient{appleMouseClient: appleMouseClient{state: p.state}}
	p.client = c
	return p, c
}

func TestAppleMusicQueueBoundaryNeverEjectsAnUnchangedTape(t *testing.T) {
	for _, tc := range []struct {
		name         string
		key          rune
		index, total int
		stopped      bool
	}{
		{"first previous", 'p', 0, 3, false},
		{"last next", 'n', 2, 3, false},
		{"single previous", 'p', 0, 1, false},
		{"single next", 'n', 0, 1, false},
		{"restart current", 'p', 1, 3, false},
		{"completed last", 'n', 2, 3, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, c := boundaryTestPage(t, tc.index, tc.total)
			c.change = func(s *applemusic.State, a applemusic.Action) {
				if a.Kind == "previous" {
					s.Position = 0
				}
				if tc.stopped {
					s.Playing, s.Paused = false, false
				}
			}
			cmd := p.handleKey(tea.KeyPressMsg{Code: tc.key})
			waiting := time.Now().Add(3 * time.Second)
			p.advance(waiting)
			if st := p.mechanics(waiting); !p.swapStart.IsZero() || st.Cassette != 0 || st.Door != 0 || p.camera.value(waiting) != 1 {
				t.Fatal("unconfirmed skip opened the bay or moved the camera")
			}
			p.Update(cmd(), model.NewApp(model.DefaultOptions()))
			p.advance(waiting.Add(time.Second))
			if p.pending || !p.swapStart.IsZero() || p.bayID != "old" || p.mechanics(waiting).Cassette != 0 || p.camera.value(waiting) != 1 {
				t.Fatal("acknowledged no-op left LOADING or ejected the current tape")
			}
			if len(c.actions) != 1 || p.ready() == tc.stopped {
				t.Fatal("no-op invented playback or suppressed the native control")
			}
		})
	}
}

func TestAppleMusicRealSkipWaitsForIdentityBeforeAnimating(t *testing.T) {
	for _, index := range []int{0, 59} {
		p, c := boundaryTestPage(t, index, index+1)
		c.change = func(s *applemusic.State, _ applemusic.Action) {
			s.Track.ID, s.QueueIndex, s.QueueTotal = "new", index+1, index+2
		}
		old := p.mechanics(time.Now())
		cmd := p.handleKey(tea.KeyPressMsg{Code: 'n'})
		p.advance(time.Now().Add(3 * time.Second))
		if !p.swapStart.IsZero() {
			t.Fatal("skip predicted a target before backend confirmation")
		}
		p.Update(cmd(), model.NewApp(model.DefaultOptions()))
		if p.pending || p.swapStart.IsZero() || p.swap.nextID != "new" {
			t.Fatal("real replacement or dynamic radio continuation did not start swapping")
		}
		if st := p.mechanics(p.swapStart.Add(200 * time.Millisecond)); st.Style != old.Style || st.Radii != old.Radii {
			t.Fatal("confirmed replacement did not eject the captured old tape")
		}
	}
}

func TestAppleMusicNoopNeedsAPostControlSnapshot(t *testing.T) {
	p, c := boundaryTestPage(t, 0, 2)
	c.failRead = 3
	cmd := p.handleKey(tea.KeyPressMsg{Code: 'p'})
	oldPoll := p.poll()()
	p.Update(cmd(), model.NewApp(model.DefaultOptions()))
	p.Update(oldPoll, model.NewApp(model.DefaultOptions()))
	if !p.pending || !p.swapStart.IsZero() {
		t.Fatal("pre-control poll confirmed an unchanged result or triggered ejection")
	}
	p.Update(p.poll()(), model.NewApp(model.DefaultOptions()))
	if p.pending || !p.ready() || !p.swapStart.IsZero() {
		t.Fatal("fresh no-op confirmation did not keep the tape seated")
	}
}

func TestAppleMusicFailedSkipDoesNotPretendToBeANoop(t *testing.T) {
	p, c := boundaryTestPage(t, 0, 2)
	c.doErr = errors.New("unknown playback result")
	cmd := p.handleKey(tea.KeyPressMsg{Code: 'n'})
	p.Update(cmd(), model.NewApp(model.DefaultOptions()))
	p.Update(p.poll()(), model.NewApp(model.DefaultOptions()))
	if !p.pending || !p.swapStart.IsZero() {
		t.Fatal("failed request was treated as successful no-op or ejected the last tape")
	}
	c.state.Track.ID, c.state.QueueIndex = "new", 1
	p.Update(p.poll()(), model.NewApp(model.DefaultOptions()))
	if p.pending || p.swapStart.IsZero() || p.swap.nextID != "new" {
		t.Fatal("late actual replacement was not accepted")
	}
}

func TestAppleMusicUnchangedSkipPreservesAllMotionLevels(t *testing.T) {
	for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
		p, _ := boundaryTestPage(t, 0, 1)
		p.level = level
		p.Update(p.handleKey(tea.KeyPressMsg{Code: 'p'})(), model.NewApp(model.DefaultOptions()))
		if p.pending || p.bayID != "old" || !p.swapStart.IsZero() || !p.zoomed {
			t.Fatalf("motion %v changed a boundary operation's semantics", level)
		}
	}
}

func TestAppleMusicBoundaryDuringAnInsertionDoesNotReplaceTheIncomingTape(t *testing.T) {
	p, c := boundaryTestPage(t, 0, 1)
	now := time.Now().Add(-100 * time.Millisecond)
	p.beginInsert(applemusic.Track{ID: "new"}, now)
	c.state.Track.ID = "new"
	p.acceptState(c.state, now.Add(50*time.Millisecond))
	style, start := p.insertStyle, p.insertStart
	p.Update(p.handleKey(tea.KeyPressMsg{Code: 'p'})(), model.NewApp(model.DefaultOptions()))
	if !p.inserting || p.insertStart != start || p.insertStyle != style || !p.swapStart.IsZero() {
		t.Fatal("boundary control interrupted the original insertion")
	}
	p.advance(start.Add(1280 * time.Millisecond))
	if p.bayID != "new" || p.bayStyle != style || p.swap.armed || p.pending {
		t.Fatal("boundary control left the new tape unseated or waiting")
	}
}

func TestAppleMusicBoundaryWhileNewTapeEntersDoesNotQueueAnotherSwap(t *testing.T) {
	p, c := boundaryTestPage(t, 0, 2)
	now := time.Now().Add(-time.Second)
	p.beginSwap(now)
	p.dispatch(applemusic.Action{Kind: "next"})
	confirmSwap(p, "new", now.Add(100*time.Millisecond))
	p.busy = false
	c.state = p.state
	start, insertion := p.swapStart, p.swap.insertAt
	p.Update(p.handleKey(tea.KeyPressMsg{Code: 'n'})(), model.NewApp(model.DefaultOptions()))
	if p.swapStart != start || p.swap.insertAt != insertion || p.swap.queued || p.pending {
		t.Fatal("unchanged boundary result restarted or queued a replacement")
	}
	p.advance(insertion.Add(500 * time.Millisecond))
	if !p.swapStart.IsZero() || p.bayID != "new" || p.swap.armed {
		t.Fatal("last tape was ejected after its existing insertion finished")
	}
}

func TestAppleMusicQueuedSkipThenCompletedBoundaryKeepsActualLastTape(t *testing.T) {
	p, c := boundaryTestPage(t, 0, 2)
	c.change = func(s *applemusic.State, _ applemusic.Action) {
		if s.Track.ID == "old" {
			s.Track.ID, s.QueueIndex = "new", 1
		} else {
			s.Playing, s.Paused = false, false
		}
	}
	first := p.handleKey(tea.KeyPressMsg{Code: 'n'})
	p.handleKey(tea.KeyPressMsg{Code: 'n'})
	_, second := p.Update(first(), model.NewApp(model.DefaultOptions()))
	p.Update(second(), model.NewApp(model.DefaultOptions()))
	if p.pending || p.state.Track.ID != "new" || p.swap.insertAt.IsZero() {
		t.Fatal("queued no-op froze the display on an earlier track instead of the real stopped last track")
	}
	p.advance(p.swap.insertAt.Add(500 * time.Millisecond))
	if p.bayID != "new" || p.ready() || p.mechanics(time.Now()).Cassette != 0 {
		t.Fatal("stopped last track was ejected or incorrectly reported as playing")
	}
}

func TestAppleMusicReturningToFirstThenPreviousOnlyReplacesOnce(t *testing.T) {
	p, c := boundaryTestPage(t, 1, 3)
	c.change = func(s *applemusic.State, _ applemusic.Action) {
		s.Track.ID, s.QueueIndex, s.Position = "new", 0, 0
	}
	first := p.handleKey(tea.KeyPressMsg{Code: 'p'})
	p.handleKey(tea.KeyPressMsg{Code: 'p'})
	_, second := p.Update(first(), model.NewApp(model.DefaultOptions()))
	p.Update(second(), model.NewApp(model.DefaultOptions()))
	if p.pending || p.swapStart.IsZero() || p.swap.nextID != "new" || p.swap.queued {
		t.Fatal("returning to first and pressing previous again queued a phantom replacement")
	}
	if st := p.mechanics(p.swapStart.Add(200 * time.Millisecond)); st.Style != 1 || st.TrackNum != 2 {
		t.Fatal("returning to first ejected the first tape instead of the previous one")
	}
	end := p.swap.insertAt.Add(500 * time.Millisecond)
	p.advance(end)
	p.advance(end.Add(time.Second))
	if !p.swapStart.IsZero() || p.bayID != "new" || p.bayNumber != 1 || p.mechanics(end).Cassette != 0 {
		t.Fatal("the first tape was ejected again after it had been seated")
	}
}

func TestAppleMusicBoundaryFramebuffersKeepTheBayClosed(t *testing.T) {
	for _, size := range [][2]int{{140, 45}, {255, 68}, {80, 30}, {50, 20}} {
		for _, key := range []rune{'p', 'n'} {
			index := 0
			if key == 'n' {
				index = 2
			}
			p, c := boundaryTestPage(t, index, 3)
			p.tier = deck.Quadrant
			p.state.Track.Name = "SYNTHETIC BOUNDARY TEST - NOT LIVE PLAYBACK"
			p.zoomed, p.camera.to = key == 'p', 0
			if p.zoomed {
				p.camera.to = 1
			}
			c.state = p.state
			c.change = func(s *applemusic.State, a applemusic.Action) {
				if a.Kind == "previous" {
					s.Position = 0
				}
			}
			now := time.Unix(100, 0)
			var cmd tea.Cmd
			for step, phase := range []string{"before", "waiting", "after"} {
				at := now.Add(time.Duration(step) * time.Second)
				if phase == "waiting" {
					cmd = p.handleKey(tea.KeyPressMsg{Code: key})
				}
				if phase == "after" {
					p.Update(cmd(), model.NewApp(model.DefaultOptions()))
				}
				p.stateAt = at
				p.status = "SYNTHETIC BOUNDARY TEST / " + phase + " / NOT LIVE PLAYBACK"
				frame := p.draw(size[0], size[1], at, 2)
				if st := p.mechanics(at); st.Door != 0 || st.Cassette != 0 || st.Style != 1 {
					t.Fatal("boundary framebuffer ejected or replaced the current tape")
				}
				for _, line := range strings.Split(frame, "\n") {
					if ansi.WcWidth.StringWidth(line) != size[0] {
						t.Fatal("boundary framebuffer overflow")
					}
				}
				if dir := os.Getenv("TAPLE_BOUNDARY_PREVIEW_DIR"); dir != "" {
					if err := os.MkdirAll(dir, 0700); err != nil {
						t.Fatal(err)
					}
					writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("boundary-%c-%dx%d-%s.html", key, size[0], size[1], phase)))
				}
			}
		}
	}
}
