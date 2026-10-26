package ui

import (
	"image"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

// applePopPeak is the highest travel of a half eject, at its overshoot.
const applePopPeak = .58

// appleRackPlacement places a slot's cassette: travel 0 hides it in the well
// below the lip at row y+1, .5 shows its upper half and 1 lifts it clear.
func appleRackPlacement(x, y, w, rows int, aspect, amount float64) (deck.Rect, deck.Pose) {
	if aspect <= 0 {
		aspect = 2
	}
	fitW := min(float64(min(36, w-14)), 11*deck.FitAspect*aspect, float64(2*max(3, rows-8))*deck.FitAspect*aspect)
	fitH := fitW / (deck.FitAspect * aspect)
	tilt := min(max(amount*2, 0), 1)
	return deck.Rect{X: float64(x+w-7) - fitW, Y: float64(y+1) - fitH*amount, W: fitW, H: fitH},
		deck.Pose{Yaw: -.24, Pitch: 1.3 - .95*tilt, Scale: 1}
}

// applePopFlash lights the slot lip for a moment when the cassette settles
// into its half-eject catch.
func applePopFlash(pop appleTween, now time.Time) float64 {
	if pop.curve != applePopRaise {
		return 0
	}
	settle := pop.at.Add(150 * time.Millisecond)
	u := float64(now.Sub(settle)) / float64(80*time.Millisecond)
	if u < 0 || u >= 1 {
		return 0
	}
	return 1 - u
}

// drawRackPickup draws the one cassette or hinged dossier folder that left its
// well, hidden by the slot lip below the opening, and records it as the flight origin.
func (p *AppleMusicPage) drawRackPickup(l appleRackLayout, amount float64, departed bool, now time.Time) {
	pick := &p.rackPickup
	i := pick.current.index
	if i < l.start || i >= l.start+l.visible || amount <= 0 {
		return
	}
	_, ly, _ := deck.RackLabel(l.w, deck.RackState{Padding: l.padding}, i-l.start)
	ly += l.y
	fit, pose := appleRackPlacement(l.x, ly, l.w, l.rows, p.cellAspect, amount)
	clip := appleBox(l.x+2, 3, l.w-4, p.screen.h-9)
	if p.catalogContainers() {
		frame, bounds := p.renderRackFolderPickup(&fit, pose, i, amount, clip)
		hit := p.retargetRackHit(i, fit, pose)
		reveal := bounds
		reveal.Max.Y = min(reveal.Max.Y, ly)
		reveal = reveal.Intersect(clip)
		p.drawRackFrame(frame, reveal)
		p.addCassetteHits(frame, reveal, hit)
		return
	}
	isSource := p.inserting && pick.current.id == p.insertID && i+1 == p.insertNumber
	cover := 1.0
	if pick.style > 0 {
		cover = 1 - appleReveal(amount)
	}
	state := deck.WalkmanState{State: deck.State{Style: pick.style, TrackNum: i + 1, Radii: deck.PackRadii(0, false)},
		Cassette: 1, OnlyCassette: true, FlightFrom: fit, FlightPose: pose, StorageCover: cover}
	state.FlightFrom.Y -= 3
	// A picked-up cassette can overlap the drawer header, just like its flight.
	frame, bounds := p.renderRackPickup(&state, clip, now)
	fit = state.FlightFrom
	fit.Y += 3
	hit := p.retargetRackHit(i, fit, pose)
	if isSource {
		p.insertOrigin, p.insertPose = fit, pose
		if departed {
			p.insertSourceCaptured = true
		}
	}
	if isSource && departed {
		return
	}
	// The lip hides whatever is still below the opening.
	reveal := bounds
	reveal.Max.Y = min(reveal.Max.Y, ly+1)
	reveal = reveal.Intersect(clip)
	p.drawRackFrame(frame, reveal)
	p.addCassetteHits(frame, reveal, hit)
}

func (p *AppleMusicPage) renderRackFolderPickup(fit *deck.Rect, pose deck.Pose, index int, amount float64, clip image.Rectangle) (*deck.Frame, image.Rectangle) {
	pick := &p.rackPickup
	view := p.rackView
	view.Fit = *fit
	view.Fit.Y -= 3
	st := deck.FolderState{Style: deck.FolderStyleForID(pick.current.id), TabSlot: uint8(index % 3), Open: amount}
	frame := pick.folder.RenderFolder(view, pose, st)
	bounds := appleFrameBounds(frame, 0, 3)
	for range 2 {
		dx := 0
		if bounds.Min.X < clip.Min.X {
			dx = clip.Min.X - bounds.Min.X
		}
		if bounds.Max.X > clip.Max.X {
			dx = clip.Max.X - bounds.Max.X
		}
		if dx == 0 {
			break
		}
		view.Fit.X += float64(dx)
		frame = pick.folder.RenderFolder(view, pose, st)
		bounds = appleFrameBounds(frame, 0, 3)
	}
	*fit = view.Fit
	fit.Y += 3
	return frame, bounds
}

// renderRackPickup renders the cassette and nudges it inside the clip.
func (p *AppleMusicPage) renderRackPickup(state *deck.WalkmanState, clip image.Rectangle, now time.Time) (*deck.Frame, image.Rectangle) {
	pick := &p.rackPickup
	basePose := p.sceneBasePose(now)
	frame := pick.renderer.RenderWalkman(p.rackView, deckPaletteOf(appleNeon), basePose, *state)
	bounds := appleFrameBounds(frame, 0, 3)
	for range 2 {
		dx := 0
		if bounds.Min.X < clip.Min.X {
			dx = clip.Min.X - bounds.Min.X
		}
		if bounds.Max.X > clip.Max.X {
			dx = clip.Max.X - bounds.Max.X
		}
		if dx == 0 {
			break
		}
		state.FlightFrom.X += float64(dx)
		frame = pick.renderer.RenderWalkman(p.rackView, deckPaletteOf(appleNeon), basePose, *state)
		bounds = appleFrameBounds(frame, 0, 3)
	}
	return frame, bounds
}

// retargetRackHit stores the drawn placement on the slot's hit as the
// flight origin and returns that hit.
func (p *AppleMusicPage) retargetRackHit(index int, fit deck.Rect, pose deck.Pose) appleHit {
	for j := range p.hits {
		h := &p.hits[j]
		if (h.kind == appleHitTrack || h.kind == appleHitCatalog) && h.index == index && h.id == p.rackPickup.current.id {
			h.cassette, h.pose = fit, pose
			return *h
		}
	}
	return appleHit{}
}

// addCassetteHits makes exactly the drawn cassette cells clickable.
func (p *AppleMusicPage) addCassetteHits(frame *deck.Frame, reveal image.Rectangle, hit appleHit) {
	for yy := reveal.Min.Y; yy < reveal.Max.Y; yy++ {
		for xx := reveal.Min.X; xx < reveal.Max.X; {
			if frame.Cells[(yy-3)*frame.W+xx].Ch == "" {
				xx++
				continue
			}
			left := xx
			for xx < reveal.Max.X && frame.Cells[(yy-3)*frame.W+xx].Ch != "" {
				xx++
			}
			if hit.kind == appleHitTrack {
				hit.kind = appleHitCassette
			}
			hit.x, hit.y, hit.w, hit.h = left, yy, xx-left, 1
			p.addHit(hit)
		}
	}
}

func appleFrameBounds(frame *deck.Frame, x, y int) image.Rectangle {
	left, top, right, bottom := frame.W, frame.H, 0, 0
	for i, c := range frame.Cells {
		if c.Ch == "" {
			continue
		}
		cx, cy := i%frame.W, i/frame.W
		left, top, right, bottom = min(left, cx), min(top, cy), max(right, cx+1), max(bottom, cy+1)
	}
	if right <= left || bottom <= top {
		return image.Rectangle{}
	}
	return image.Rect(x+left, y+top, x+right, y+bottom)
}
func (p *AppleMusicPage) drawRackFrame(frame *deck.Frame, clip image.Rectangle) {
	for y := clip.Min.Y; y < clip.Max.Y; y++ {
		for x := clip.Min.X; x < clip.Max.X; x++ {
			if x < 0 || x >= frame.W || y < 3 || y-3 >= frame.H {
				continue
			}
			cell := &frame.Cells[(y-3)*frame.W+x]
			if cell.Ch != "" {
				p.screen.set(x, y, cell.Ch, 1, uv.Style{Fg: &cell.Fg, Bg: &cell.Bg})
			}
		}
	}
}

func (p *AppleMusicPage) rackEmptyLabel() string {
	c := p.catalogState(p.drawer)
	switch {
	case p.drawer == appleQueue:
		return T("empty.queue")
	case p.drawer == appleLibrary:
		return p.catalogEmptyLabel()
	case c.searching:
		return T("empty.loading")
	case c.catalogError != "":
		return T("empty.failed")
	case c.catalog.title != "":
		return T("empty.no_results")
	}
	return T("empty.search_guide")
}

func appleStyleOf(style uint8) (deck.CassetteStyle, bool) {
	if style == 0 || int(style) > len(deck.CassetteStyles) {
		return deck.CassetteStyle{}, false
	}
	return deck.CassetteStyles[style-1], true
}

// appleReveal maps travel to how much of the random style shows: the storage
// cover lifts while the cassette rises to its half-eject catch.
func appleReveal(amount float64) float64 { return min(max(amount*2, 0), 1) }

func (p *AppleMusicPage) pickStyleFor(id string) uint8 {
	if p.catalogContainers() {
		return deck.FolderStyleForID(id)
	}
	style := p.nextCassetteStyle()
	if id != "" {
		p.styles[id] = style
	}
	return style
}

func (p *AppleMusicPage) prepareRackPop(now time.Time) time.Duration {
	if !p.rackGraphical {
		return 0
	}
	wanted := appleRackChoice{id: p.insertID, index: p.insertNumber - 1, drawer: p.drawer}
	p.updateRackPickup(wanted, 1, p.insertStyle, now)
	wait := max(0, p.rackPickup.pop.at.Add(p.rackPickup.pop.duration).Sub(now))
	if p.rackPickup.current != wanted {
		wait += applePopDuration(1, p.level)
	}
	return wait
}

// rackLabelRoom shortens labels the raised cassette stands in front of, so
// text ends cleanly at its edge instead of being cut through.
func (p *AppleMusicPage) rackLabelRoom(l appleRackLayout, ly, lx, lw int, amount float64) int {
	i := p.rackPickup.current.index
	if i < l.start || i >= l.start+l.visible || amount <= 0 {
		return lw
	}
	_, py, _ := deck.RackLabel(l.w, deck.RackState{Padding: l.padding}, i-l.start)
	py += l.y
	fit, _ := appleRackPlacement(l.x, py, l.w, l.rows, p.cellAspect, max(amount, applePopPeak))
	if ly > py || float64(ly) < fit.Y {
		return lw
	}
	return min(lw, max(0, int(fit.X)-lx-1))
}
