package ui

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

type appleActionMsg struct {
	err       error
	operation appleOperation
	before    *applemusic.State
	after     *applemusic.State
	notSent   bool
}

type appleOperation struct {
	action     applemusic.Action
	generation uint64
	queuedAt   time.Time
}

func appleDirectPlayback(kind string) bool {
	return kind == "play" || kind == "jump" || kind == "play-collection" || kind == "play-station"
}

func appleTrackOperation(kind string) bool {
	return appleDirectPlayback(kind) || kind == "next" || kind == "previous"
}

func (p *AppleMusicPage) dispatch(action applemusic.Action) tea.Cmd {
	if p.loggingOut && action.Kind != "logout" {
		return nil
	}
	p.generation++
	if appleTrackOperation(action.Kind) {
		p.pending, p.pendingID = true, action.ID
		p.pendingSince, p.playbackError = time.Time{}, ""
		p.pendingCatalogID = ""
		if action.Kind == "play-collection" {
			p.pendingID = action.StartID
		} else if action.Kind == "play-station" {
			p.pendingID = ""
		}
		if (action.Kind == "play" && action.ResourceType == "library-songs") || action.StartID != "" {
			for _, item := range p.items() {
				if item.ID == p.pendingID {
					p.pendingCatalogID = item.CatalogID
					break
				}
			}
		}
		p.playGeneration = p.generation
		p.previousID, p.pendingIndex = p.state.Track.ID, p.state.QueueIndex
		p.lastAction = action
	}
	if appleDirectPlayback(action.Kind) {
		p.swap.armed = false
		// Keep the active operation and control barriers; skip superseded picks.
		for len(p.operations) > 0 && appleDirectPlayback(p.operations[len(p.operations)-1].action.Kind) {
			p.operations = p.operations[:len(p.operations)-1]
		}
	}
	p.operations = append(p.operations, appleOperation{action: action, generation: p.generation, queuedAt: time.Now()})
	return p.nextOperation()
}

// Only one command runs at a time; input queues work instead of blocking the UI.
func (p *AppleMusicPage) nextOperation() tea.Cmd {
	if p.busy || len(p.operations) == 0 {
		return nil
	}
	op := p.operations[0]
	p.operations = p.operations[1:]
	p.busy = true
	if op.generation == p.playGeneration && appleTrackOperation(op.action.Kind) {
		p.pendingSince = time.Now()
	}
	return func() tea.Msg {
		var err error
		started := time.Now()
		sent := false
		defer func() {
			slog.Info("apple_music_dispatch", "op", op.action.Kind, "generation", op.generation,
				"wait_ms", started.Sub(op.queuedAt).Milliseconds(), "elapsed_ms", time.Since(started).Milliseconds(), "sent", sent, "ok", sent && err == nil)
		}()
		var before *applemusic.State
		var after *applemusic.State
		action := op.action
		if action.Kind == "next" || action.Kind == "previous" || (action.Kind == "play-collection" && action.StartID == "") || action.Kind == "play-station" || action.Kind == "seek" || action.Kind == "seek-relative" || action.Kind == "volume-relative" {
			state, readErr := p.client.Snapshot(p.ctx)
			if readErr != nil {
				return appleActionMsg{err: readErr, operation: op, notSent: true}
			}
			before = &state
			if (action.Kind == "seek" || action.Kind == "seek-relative") && (action.ID == "" || state.Track.ID != action.ID) {
				return appleActionMsg{err: fmt.Errorf("歌曲已变化，未发送过期进度操作"), operation: op, notSent: true}
			}
			if action.Kind == "seek-relative" {
				action.Kind, action.Value = "seek", min(max(0, state.Position+action.Value), state.Track.Duration)
			} else if action.Kind == "volume-relative" {
				action.Kind, action.Value = "volume", min(max(0, state.Volume+action.Value), 1)
			}
		}
		sent = true
		if op.action.Kind == "login" {
			err = p.client.Show(p.ctx)
		} else {
			err = p.client.Do(p.ctx, action)
		}
		if err == nil && appleTrackOperation(action.Kind) {
			// Bound best-effort read-back within the serial queue; polling recovers on failure.
			ctx, cancel := context.WithTimeout(p.ctx, 250*time.Millisecond)
			defer cancel()
			if state, readErr := p.client.Snapshot(ctx); readErr == nil {
				after = &state
			}
		}
		return appleActionMsg{err: err, operation: op, before: before, after: after}
	}
}

func (p *AppleMusicPage) acceptAction(m appleActionMsg, now time.Time) tea.Cmd {
	var cmd tea.Cmd
	p.busy = false
	if m.operation.action.Kind == "logout" {
		p.loggingOut = false
		p.pollEpoch++
		p.panel, p.hits = "", nil
		p.status = T("account.cleared")
		if m.err != nil {
			p.status = m.err.Error()
		}
		return nil
	}
	if m.operation.generation == p.playGeneration && appleTrackOperation(m.operation.action.Kind) {
		// Pre-control polls cannot prove a no-op or overwrite the read-back.
		p.pollEpoch++
		if m.err == nil && !m.notSent && (m.operation.action.Kind == "next" || m.operation.action.Kind == "previous") {
			p.skipAcknowledged = m.operation.generation
		}
	}
	if m.operation.generation == p.playGeneration && m.notSent {
		p.pending, p.pendingSince = false, time.Time{}
		p.swap.armed = false
		p.swap.awaitState = !p.swapStart.IsZero()
	}
	if m.operation.generation == p.playGeneration && !m.notSent {
		p.settledPlayGeneration = m.operation.generation
		if m.before != nil {
			p.previousID, p.pendingIndex = m.before.Track.ID, m.before.QueueIndex
		}
	}
	if m.operation.generation == p.generation {
		if m.err != nil {
			p.status = m.err.Error()
		} else {
			p.status = "正在加载…"
		}
	}
	if m.operation.generation == p.playGeneration && errors.Is(m.err, applemusic.ErrPlaybackUnavailable) {
		p.failPlayback(m.err.Error())
	}
	if m.operation.generation == p.playGeneration && m.after != nil {
		p.acceptState(*m.after, now)
		cmd = tea.Batch(p.loadLyrics(false), p.ensureLibrary())
	}
	return tea.Batch(p.nextOperation(), cmd)
}

const applePlaybackWaitLimit = 30 * time.Second

// Queue acknowledgement and available lyrics are not proof of audio playback.
// Bound this separately from the RPC timeout, without resubmitting a song.
func (p *AppleMusicPage) checkPlaybackWait(now time.Time) {
	if !p.pending || p.pendingSince.IsZero() || now.Before(p.pendingSince.Add(applePlaybackWaitLimit)) {
		return
	}
	slog.Warn("apple_music_playback_timeout", "op", p.lastAction.Kind,
		"ready", p.state.Ready, "authorized", p.state.Authorized, "full", p.state.Full,
		"playing", p.state.Playing, "paused", p.state.Paused, "preview", p.state.Preview)
	p.failPlayback(T("status.play_timeout"))
}

func (p *AppleMusicPage) failPlayback(message string) {
	p.pending, p.pendingSince = false, time.Time{}
	p.playbackError, p.status = message, message
	p.swap.armed = false
	// An already animated swap may recover when a real playback state arrives.
	p.swap.awaitState = !p.swapStart.IsZero()
}
