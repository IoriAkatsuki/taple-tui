package ui

import (
	"errors"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/motion"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

func collectionReplayPage(t *testing.T) (*AppleMusicPage, *appleBoundaryClient) {
	t.Helper()
	s := applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Storefront: "us",
		Position: 42, QueueIndex: 0, QueueTotal: 3, Track: applemusic.Track{ID: "first", Duration: 240}}
	c := &appleBoundaryClient{appleMouseClient: appleMouseClient{state: s}}
	c.change = func(s *applemusic.State, _ applemusic.Action) { s.Position = 0 }
	p := NewAppleMusicPage(c)
	t.Cleanup(func() { p.Close() })
	p.level, p.scene, p.drawer = motion.Off, appleBrowse, appleSearch
	p.catalog.request = applemusic.CatalogRequest{Kind: applemusic.CatalogAlbumTracks, ID: "collection"}
	// A partial catalog cannot be used to guess the actual first queue item.
	p.tracks = []applemusic.Track{{ID: "unrelated", Playable: true}}
	p.acceptState(s, time.Now())
	return p, c
}

func TestAppleMusicCollectionReplayConfirmsUnchangedFirstTrack(t *testing.T) {
	for _, resource := range []string{"albums", "playlists", "library-albums", "library-playlists"} {
		t.Run(resource, func(t *testing.T) {
			p, c := collectionReplayPage(t)
			action := applemusic.Action{Kind: "play-collection", ID: "collection", ResourceType: resource}
			cmd := p.dispatch(action)
			start := p.pendingSince
			msg := cmd().(appleActionMsg)
			if msg.err != nil || msg.notSent || msg.after == nil || msg.before == nil ||
				msg.before.Position != 42 || msg.after.Position != 0 {
				t.Fatalf("invalid replay/read-back: %+v", msg)
			}
			p.Update(msg, model.NewApp(model.DefaultOptions()))
			p.checkPlaybackWait(start.Add(applePlaybackWaitLimit + time.Second))
			if p.pending || !p.ready() || p.playbackError != "" || !p.pendingSince.IsZero() {
				t.Fatalf("successful same-track collection replay did not settle: pending=%v ready=%v error=%q", p.pending, p.ready(), p.playbackError)
			}
			if len(c.actions) != 1 || c.actions[0] != action || p.pendingID != "" {
				t.Fatal("replay was resubmitted or its first track was guessed from the catalog")
			}
		})
	}
}

func TestAppleMusicCollectionReplayNeedsFreshFullReadback(t *testing.T) {
	p, c := collectionReplayPage(t)
	cmd := p.handleKey(tea.KeyPressMsg{Code: 'P'})
	oldPoll := p.poll()()
	c.failRead = c.reads + 2 // Pre-control snapshot succeeds; immediate read-back fails.
	msg := cmd().(appleActionMsg)
	if msg.err != nil || msg.notSent || msg.after != nil {
		t.Fatalf("invalid successful control with failed read-back: %+v", msg)
	}
	app := model.NewApp(model.DefaultOptions())
	p.Update(msg, app)
	p.Update(oldPoll, app)
	if !p.pending || p.ready() {
		t.Fatal("successful control or pre-control poll invented readiness")
	}
	c.state.Full = false
	p.Update(p.poll()(), app)
	if !p.pending || p.ready() {
		t.Fatal("partial playback confirmed a collection replay")
	}
	p.checkPlaybackWait(p.pendingSince.Add(applePlaybackWaitLimit))
	if p.playbackError == "" {
		t.Fatal("missing full playback disabled the watchdog")
	}
	c.state.Full, c.state.Playing, c.state.Paused = true, false, true
	p.Update(p.poll()(), app)
	if p.pending || !p.ready() || p.playbackError != "" || len(c.actions) != 1 {
		t.Fatal("fresh full paused read-back did not recover without resubmission")
	}
}

func TestAppleMusicCollectionReplayRequiresCurrentSuccessfulControl(t *testing.T) {
	for _, failure := range []string{"failed", "not-sent", "superseded"} {
		t.Run(failure, func(t *testing.T) {
			p, c := collectionReplayPage(t)
			if failure == "failed" {
				c.doErr = errors.New("unknown playback result")
			} else if failure == "not-sent" {
				c.failRead = 1
			}
			action := applemusic.Action{Kind: "play-collection", ID: "collection", ResourceType: "albums"}
			cmd := p.dispatch(action)
			app := model.NewApp(model.DefaultOptions())
			if failure == "superseded" {
				p.dispatch(action)
				_, next := p.Update(cmd(), app)
				if next == nil {
					t.Fatal("new collection request was lost")
				}
				c.doErr = errors.New("latest playback result unknown")
				p.Update(next(), app)
			} else {
				p.Update(cmd(), app)
			}
			p.Update(p.poll()(), app)
			if failure == "not-sent" {
				if len(c.actions) != 0 || p.settledPlayGeneration == p.playGeneration {
					t.Fatal("unsent collection operation was acknowledged")
				}
			} else if !p.pending || p.ready() {
				t.Fatal("failed or superseded acknowledgement confirmed an unchanged collection")
			}
		})
	}
}

func TestAppleMusicCollectionReplayDoesNotWeakenJumpOccurrence(t *testing.T) {
	p, c := collectionReplayPage(t)
	app := model.NewApp(model.DefaultOptions())
	p.Update(p.handleKey(tea.KeyPressMsg{Code: 'P'})(), app)
	p.Update(p.dispatch(applemusic.Action{Kind: "jump", ID: "first", Value: 2})(), app)
	if !p.pending || p.ready() {
		t.Fatal("successful collection acknowledgement accepted the wrong duplicate occurrence")
	}
	c.state.QueueIndex = 2
	p.Update(p.poll()(), app)
	if p.pending || !p.ready() || len(c.actions) != 2 {
		t.Fatal("requested occurrence could not confirm playback without resubmission")
	}
}
