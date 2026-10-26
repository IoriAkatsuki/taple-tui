package ui

import (
	"time"

	"github.com/anhoder/foxful-cli/motion"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

type appleSwapState struct {
	old        deck.WalkmanState
	nextID     string
	nextIndex  int
	next       deck.State
	insertAt   time.Time
	queued     bool
	awaitState bool
	armed      bool
	armedID    string
	armedIndex int
}

// Skips may restart the same track or reach a boundary; do not eject speculatively.
func (p *AppleMusicPage) armSwap(now time.Time) {
	if p.swap.armed {
		return
	}
	id, index := p.state.Track.ID, p.state.QueueIndex
	if p.inserting {
		id, index = p.insertID, -1
	} else if p.swapStart.IsZero() {
		p.swap.old = p.mechanics(now)
		if p.pending || id == "" {
			id, index = p.bayID, -1
		}
	}
	if id == p.insertID && p.pendingCatalogID != "" {
		id = p.pendingCatalogID
	}
	p.swap.armed, p.swap.armedID, p.swap.armedIndex = true, id, index
}

func (p *AppleMusicPage) confirmArmedSwap(now time.Time) {
	if !p.swap.armed || p.pending || !p.state.Ready || !p.state.Authorized || p.state.Track.ID == "" {
		return
	}
	if p.state.Track.ID == p.swap.armedID && (p.swap.armedIndex < 0 || p.state.QueueIndex == p.swap.armedIndex) {
		p.swap.armed = false
	} else if p.hasFullTrackState() {
		p.beginSwap(now)
		p.swap.armed = false
	}
}

func (p *AppleMusicPage) bindSwapTarget(now time.Time) {
	if p.swapStart.IsZero() || p.swap.awaitState || !p.hasFullTrackState() {
		return
	}
	if p.swap.nextID != "" {
		if p.state.Track.ID == p.swap.nextID && p.state.QueueIndex == p.swap.nextIndex {
			p.swap.queued = false
			return
		}
		if !now.Before(p.swap.insertAt) {
			p.swap.queued = true
			return
		}
	}
	p.swap.nextID, p.swap.nextIndex = p.state.Track.ID, p.state.QueueIndex
	number := 0
	if p.state.QueueTotal > 0 && p.state.QueueIndex >= 0 && p.state.QueueIndex < p.state.QueueTotal {
		number = p.state.QueueIndex + 1
	}
	p.swap.next = deck.State{Style: p.styles[p.state.Track.ID], TrackNum: number, Radii: deck.PackRadii(p.progress(), false)}
	p.swap.insertAt = p.swapStart.Add(400 * time.Millisecond)
	if now.After(p.swap.insertAt) {
		p.swap.insertAt = now
	}
	p.reels = deck.Reels{}
}

func (p *AppleMusicPage) advanceSwap(now time.Time) {
	p.bindSwapTarget(now)
	if p.swapStart.IsZero() || p.swap.awaitState || p.swap.insertAt.IsZero() || now.Before(p.swap.insertAt.Add(500*time.Millisecond)) {
		return
	}
	// A late recovery must not backdate the next camera or mechanical transition.
	end := now
	armed, id, index := p.swap.armed, p.swap.nextID, p.swap.nextIndex
	seated := p.swapMechanics(now)
	p.bayID, p.bayStyle, p.bayNumber = p.swap.nextID, p.swap.next.Style, p.swap.next.TrackNum
	if p.swap.queued {
		seated.Keys = 0
		p.swap = appleSwapState{old: seated, armed: armed, armedID: id, armedIndex: index}
		p.swapStart = end
		p.bindSwapTarget(now)
		return
	}
	p.swapStart, p.swap = time.Time{}, appleSwapState{}
	if armed {
		p.swap = appleSwapState{old: seated, armed: true, armedID: id, armedIndex: index}
	}
	if p.restoreZoom {
		p.setZoom(true, end)
	}
}

func (p *AppleMusicPage) swapMechanics(now time.Time) deck.WalkmanState {
	if p.swap.insertAt.IsZero() || now.Before(p.swap.insertAt) {
		st := p.swap.old
		st.Keys, st.Blur = 0, 0
		ms := max(0, float64(now.Sub(p.swapStart))/float64(time.Millisecond))
		st.Door += (1 - st.Door) * motion.OutCubic(min(ms/80, 1))
		if st.Cassette >= 0 {
			st.Cassette += (1 - st.Cassette) * motion.InOutCubic(min(max((ms-80)/140, 0), 1))
		}
		if ms >= 400 {
			st.Cassette = -1
		}
		return st
	}
	st := deck.WalkmanState{State: p.swap.next, Door: 1, Cassette: 1}
	ms := float64(now.Sub(p.swap.insertAt)) / float64(time.Millisecond)
	if ms >= 180 {
		st.Cassette = 1 - motion.InQuad(min((ms-180)/160, 1))
	}
	if ms >= 340 {
		st.Door = 1 - motion.OutCubic(min((ms-340)/160, 1))
		if p.ready() && p.state.Track.ID == p.swap.nextID {
			st.Radii, st.Angles, st.Blur = deck.PackRadii(p.progress(), false), p.reels.Angles, p.reels.Blur()
			if p.state.Playing {
				st.Keys = deck.KeyPlay
			}
		}
	}
	return st
}
