package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/anhoder/foxful-cli/model"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

func TestPlaybackAcknowledgedWithoutAudioTimesOutAndCanRecover(t *testing.T) {
	SetLanguage("en")
	defer SetLanguage("en")
	c := &appleMouseClient{state: applemusic.State{Ready: true, Authorized: true, Track: applemusic.Track{ID: "new"}}}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.Update(p.dispatch(applemusic.Action{Kind: "play", ID: "new"})(), model.NewApp(model.DefaultOptions()))
	start := p.pendingSince
	if !p.pending || start.IsZero() {
		t.Fatal("queue acknowledgement invented audio readiness")
	}
	p.checkPlaybackWait(start.Add(applePlaybackWaitLimit - time.Millisecond))
	if !p.pending {
		t.Fatal("deadline fired early")
	}
	p.checkPlaybackWait(start.Add(applePlaybackWaitLimit))
	if p.pending || p.playbackError == "" || p.ready() || len(c.actions) != 1 {
		t.Fatal("failed playback stayed pending or was resubmitted")
	}
	frame := ansi.Strip(p.draw(160, 35, start.Add(applePlaybackWaitLimit), 2))
	if !strings.Contains(frame, "Playback failed") || strings.Contains(frame, "Loading…") {
		t.Fatal("footer still claims loading")
	}
	p.acceptState(applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Track: applemusic.Track{ID: "new"}}, start.Add(31*time.Second))
	if !p.ready() || p.playbackError != "" || len(c.actions) != 1 {
		t.Fatal("late confirmed playback did not recover without replay")
	}
}

func TestUnsupportedRuntimeStopsLoadingImmediately(t *testing.T) {
	p, c := boundaryTestPage(t, 0, 2)
	c.doErr = fmt.Errorf("%w: Widevine", applemusic.ErrPlaybackUnavailable)
	p.Update(p.dispatch(applemusic.Action{Kind: "play", ID: "new"})(), model.NewApp(model.DefaultOptions()))
	if p.pending || !strings.Contains(p.playbackError, "Widevine") {
		t.Fatal("unsupported runtime remained loading")
	}
	p.dispatch(applemusic.Action{Kind: "play", ID: "retry"})
	if !p.pending || p.playbackError != "" || p.pendingSince.IsZero() {
		t.Fatal("explicit retry did not reset the watchdog")
	}
}

func TestQueuedPlaybackStartsItsOwnDeadlineOnlyWhenSent(t *testing.T) {
	c := &appleMouseClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	first := p.dispatch(applemusic.Action{Kind: "play", ID: "old"})
	p.dispatch(applemusic.Action{Kind: "play", ID: "new"})
	if !p.pendingSince.IsZero() {
		t.Fatal("unsent request inherited the previous deadline")
	}
	p.checkPlaybackWait(time.Now().Add(time.Hour))
	if !p.pending {
		t.Fatal("queued request expired before it was sent")
	}
	p.Update(first(), model.NewApp(model.DefaultOptions()))
	if p.pendingSince.IsZero() {
		t.Fatal("newly dispatched request has no deadline")
	}
}

func TestLatePlaybackRecoveryRequiresTheRequestedQueueSlot(t *testing.T) {
	p, c := boundaryTestPage(t, 0, 3)
	c.state.Track.ID = "duplicate"
	p.state = c.state
	p.Update(p.dispatch(applemusic.Action{Kind: "jump", ID: "duplicate", Value: 2})(), model.NewApp(model.DefaultOptions()))
	p.checkPlaybackWait(p.pendingSince.Add(applePlaybackWaitLimit))
	if p.playbackError == "" {
		t.Fatal("wrong queue slot falsely confirmed the requested jump")
	}
	p.acceptState(c.state, time.Now())
	if p.playbackError == "" {
		t.Fatal("old queue slot cleared the playback failure")
	}
	c.state.QueueIndex = 2
	p.acceptState(c.state, time.Now())
	if p.playbackError != "" || p.pending || !p.pendingSince.IsZero() || len(c.actions) != 1 {
		t.Fatal("confirmed queue slot did not recover without replay")
	}
}

func TestPlaybackWatchdogRunsWithoutASuccessfulSnapshot(t *testing.T) {
	for _, viaFrame := range []bool{false, true} {
		p := NewAppleMusicPage(&appleMouseClient{})
		p.pending = true
		p.pendingSince = time.Now().Add(-applePlaybackWaitLimit - time.Second)
		if viaFrame {
			p.advance(time.Now())
		} else {
			p.Update(applePollMsg{}, model.NewApp(model.DefaultOptions()))
		}
		if p.pending || p.playbackError == "" {
			t.Fatal("unavailable snapshots disabled the playback watchdog")
		}
		p.Close()
	}
}
