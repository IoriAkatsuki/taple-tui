package ui

import (
	"fmt"
	"image/color"
	"math"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

type appleRackChoice struct {
	id     string
	index  int
	drawer appleDrawer
}
type appleRackPickup struct {
	current, wanted appleRackChoice
	pop             appleTween
	style           uint8
	renderer        deck.WalkmanRenderer
	rack            deck.RackRenderer
	folder          deck.FolderRenderer
}

// appleRackLayout is the visible window of the rack in screen cells.
type appleRackLayout struct {
	x, y, w, rows    int
	padding          int
	capacity, start  int
	visible, desired int
}

// Presentation data only: catalog entries retain catalog hit targets, never song actions.
type appleRackEntry struct {
	id, name, detail     string
	kind                 appleHitKind
	playing, unavailable bool
}

func (p *AppleMusicPage) rackEntry(index int) appleRackEntry {
	if p.catalogContainers() {
		item := p.catalogState(p.drawer).catalog.items[index]
		detail := catalogItemLabel(item.Type)
		if item.Artist != "" {
			detail += " · " + item.Artist
		}
		return appleRackEntry{id: item.ID, name: item.Name, detail: detail, kind: appleHitCatalog}
	}
	item := p.items()[index]
	return appleRackEntry{id: item.ID, name: item.Name, detail: item.Artist, kind: appleHitTrack,
		playing:     item.ID == p.state.Track.ID && (p.drawer != appleQueue || index == p.state.QueueIndex),
		unavailable: p.drawer != appleQueue && item.Type == "library-songs" && !item.Playable}
}

func (p *AppleMusicPage) resetRackPickup() {
	p.rackPickup.current, p.rackPickup.wanted = appleRackChoice{index: -1}, appleRackChoice{index: -1}
	p.rackPickup.pop = appleTween{}
	p.rackPickup.style = 0
	p.resetVinylPickup()
}
func (p *AppleMusicPage) rackAnimating(now time.Time) bool {
	return p.rackPickup.pop.active(now) || p.vinylPickup.pop.active(now) || p.lyrics.scroll.active(now)
}

// There is one physical preview: retract it completely before raising another.
func (p *AppleMusicPage) updateRackPickup(wanted appleRackChoice, target float64, style uint8, now time.Time) {
	if wanted.index < 0 {
		target = 0
	}
	r := &p.rackPickup
	changed := r.wanted != wanted
	r.wanted = wanted
	if r.current == wanted {
		if style > 0 {
			r.style = style
		} else if r.style == 0 && target > 0 && wanted.index >= 0 {
			r.style = p.pickStyleFor(wanted.id)
		}
		if r.pop.to != target {
			r.pop.startPop(target, now, p.level)
		}
		return
	}
	if r.current.index >= 0 && r.pop.value(now) > 0 {
		if r.pop.to != 0 {
			r.pop.startPop(0, now, p.level)
		}
		if r.pop.value(now) > 0 {
			return
		}
	}
	start := now
	if r.current.index >= 0 && !changed && r.pop.to == 0 && r.pop.from > 0 {
		start = r.pop.at.Add(r.pop.duration)
	}
	r.current = wanted
	if style == 0 && wanted.index >= 0 && target > 0 {
		style = p.pickStyleFor(wanted.id)
	}
	r.style = style
	r.pop = appleTween{}
	if wanted.index >= 0 {
		r.pop.startPop(target, start, p.level)
	}
}

func (p *AppleMusicPage) drawList(x, y, w, rows int, now time.Time) {
	if w < 8 || rows < 2 {
		return
	}
	l := p.layoutRack(x, y, w, rows)
	departed := p.syncRackPickup(l, now)
	amount := p.rackPickup.pop.value(now)
	p.screen.fill(x, y, w, rows, p.screen.pal.Bg)
	if p.rackGraphical {
		p.drawRackCabinet(l, amount, now)
	}
	for row := 0; row < l.capacity; row++ {
		p.drawRackSlot(l, row, amount, now)
	}
	if p.rackGraphical {
		p.drawRackPickup(l, amount, departed, now)
	}
}

// layoutRack picks graphics or text spines and keeps the selection in view
// without moving the saved viewport otherwise.
func (p *AppleMusicPage) layoutRack(x, y, w, rows int) appleRackLayout {
	s, count := &p.screen, p.listCount()
	l := appleRackLayout{x: x, y: y, w: w, rows: rows}
	p.rackGraphical = s.w >= 60 && s.h >= 20 && w >= 36 && rows >= 10
	bottom := 0
	if p.rackGraphical {
		// Headroom for the exposed half and its overshoot; edge slots never move.
		fit, _ := appleRackPlacement(x, y, w, rows, p.cellAspect, applePopPeak)
		l.padding, bottom = max(2, int(math.Ceil(fit.H*applePopPeak))), 1
		if rows < l.padding+bottom+2 {
			p.rackGraphical, l.padding, bottom = false, 0, 0
		}
	}
	l.capacity = max(0, (rows-l.padding-bottom)/2)
	selection := p.selected
	if p.inserting && !p.insertSourceCaptured {
		selection = p.insertNumber - 1
	}
	start := p.rackStart
	if selection < start {
		start = selection
	}
	if selection >= start+l.capacity {
		start = selection - max(0, l.capacity-1)
	}
	l.start = min(max(0, start), max(0, count-l.capacity))
	p.rackStart = l.start
	l.visible = min(l.capacity, max(0, count-l.start))
	l.desired = -1
	if p.rackFocused && !p.editing {
		l.desired = p.selected
	}
	if p.rackHover >= 0 {
		l.desired = p.rackHover
	}
	if p.inserting {
		l.desired = p.insertNumber - 1
		if l.desired < 0 || l.desired >= count || p.rackEntry(l.desired).id != p.insertID {
			l.desired = -1
		}
	}
	return l
}

// syncRackPickup steers the single physical preview towards the cursor and
// reports whether the picked cassette has departed for the Walkman.
func (p *AppleMusicPage) syncRackPickup(l appleRackLayout, now time.Time) bool {
	current := p.rackPickup.current
	if !p.rackGraphical || current.drawer != p.drawer || current.index < l.start || current.index >= l.start+l.visible || (current.index >= 0 && p.rackEntry(current.index).id != current.id) {
		p.resetRackPickup()
	}
	wanted, target, style := appleRackChoice{index: -1}, .5, uint8(0)
	if l.desired >= l.start && l.desired < l.start+l.visible {
		wanted = appleRackChoice{id: p.rackEntry(l.desired).id, index: l.desired, drawer: p.drawer}
	}
	if p.inserting {
		target, style = 1, p.insertStyle
	}
	departed := p.inserting && !now.Before(p.insertStart)
	if departed && p.insertSourceCaptured {
		p.resetRackPickup()
	} else if p.rackGraphical {
		p.updateRackPickup(wanted, target, style, now)
		if departed && !p.insertSourceCaptured {
			p.rackPickup.current, p.rackPickup.wanted = wanted, wanted
			p.rackPickup.pop = appleTween{from: 1, to: 1}
			p.rackPickup.style = p.insertStyle
		}
	}
	return departed
}

// drawRackCabinet ray-casts the cabinet, wells, tabs and stowed spines.
func (p *AppleMusicPage) drawRackCabinet(l appleRackLayout, amount float64, now time.Time) {
	pick := &p.rackPickup
	st := deck.RackState{Padding: l.padding, Slots: l.capacity, Filled: l.visible, Out: -1, Tab: -1}
	if p.catalogContainers() {
		st.Media = deck.RackFolder
	}
	if i := pick.current.index - l.start; pick.current.index >= 0 && amount > 0 {
		st.Out, st.Tab = i, i
		if p.catalogContainers() {
			fp := deck.FolderPalettes[deck.FolderStyleForID(pick.current.id)-1]
			st.TabColor, st.TabMix = fp.Tab, appleReveal(amount)
		} else if cs, ok := appleStyleOf(pick.style); ok {
			st.TabColor, st.TabMix = cs.Ink, appleReveal(amount)
		}
		st.Flash = applePopFlash(pick.pop, now)
	}
	// A cassette that has flown to the Walkman leaves its well empty.
	if i := p.insertNumber - 1 - l.start; p.inserting && p.insertSourceCaptured && i >= 0 && i < l.visible {
		st.Out = i
	}
	v := deck.View{W: l.w, H: l.rows, Tier: p.tier, Aspect: p.cellAspect}
	p.screen.drawDeck(pick.rack.RenderRack(v, deckPaletteOf(appleNeon), st), l.x, l.y)
	label := "CASSETTE STORAGE · FILE RACK"
	if p.catalogContainers() {
		label = "DOSSIER CRATE · 3D FILE FOLDERS"
	}
	if pick.current.index >= 0 && pick.current.index < p.listCount() && amount > 0 {
		item := p.rackEntry(pick.current.index)
		label = fmt.Sprintf("%02d · %s", pick.current.index+1, item.name)
		if p.catalogContainers() {
			fp := deck.FolderPalettes[deck.FolderStyleForID(pick.current.id)-1]
			label = fmt.Sprintf("FOLDER %02d [%s] · %s", pick.current.index+1, fp.Name, item.name)
		} else if cs, ok := appleStyleOf(pick.style); ok {
			label = fmt.Sprintf("%02d [%s] · %s", pick.current.index+1, cs.Name, item.name)
		}
	}
	p.screen.text(l.x+3, l.y+l.rows-1, label, uv.Style{Fg: p.screen.pal.Secondary, Bg: color.RGBA{22, 34, 46, 255}}, l.w-6)
}

// drawRackSlot prints one slot's spine label and records its hit area.
func (p *AppleMusicPage) drawRackSlot(l appleRackLayout, row int, amount float64, now time.Time) {
	s, pick := &p.screen, &p.rackPickup
	i := l.start + row
	lx, ly, lw := deck.RackLabel(l.w, deck.RackState{Padding: l.padding}, row)
	lx, ly = l.x+lx, l.y+ly
	var labelBg color.Color = color.RGBA{29, 44, 58, 255}
	if c := s.buf.CellAt(lx, ly); p.rackGraphical && c != nil && c.Style.Bg != nil {
		labelBg = c.Style.Bg // print onto the ray-cast spine itself
	}
	if p.rackGraphical {
		lw = p.rackLabelRoom(l, ly, lx, lw, amount)
	}
	if !p.rackGraphical {
		lx, lw, labelBg = l.x+1, l.w-2, s.pal.Surface
		s.fill(lx, ly, lw, 1, labelBg)
		s.repeat(lx+1, ly+1, lw-1, "▔", uv.Style{Fg: s.pal.Rule})
	}
	if i >= p.listCount() {
		if row == 0 {
			s.text(lx, ly, p.rackEmptyLabel(), uv.Style{Fg: s.pal.Secondary}, lw)
		}
		return
	}
	item := p.rackEntry(i)
	out := p.rackGraphical && i == pick.current.index && amount > 0
	number := fmt.Sprintf("%02d", i+1)
	if item.playing {
		number = "▶ "
	}
	title := number + " " + item.name
	if item.unavailable {
		title = number + " " + T("rack.unplayable") + " " + item.name
	}
	if item.detail != "" {
		title += " · " + item.detail
	}
	st := uv.Style{Fg: s.pal.Text, Bg: labelBg}
	if out || i == l.desired {
		// Keep the title on the dark well after its physical spine lifts away.
		st.Attrs = uv.AttrBold
	}
	indent := 0
	if p.drawerTween.active(now) {
		u := min(max((p.drawerTween.value(now)-float64(row)*.08)/.72, 0), 1)
		indent = int(math.Round((1 - u) * 3))
	}
	s.text(lx+indent, ly, title, st, max(0, lw-indent))
	fit, pose := appleRackPlacement(l.x, ly, l.w, l.rows, p.cellAspect, 0)
	slot := appleBox(l.x+2, ly, l.w-4, 2)
	p.addHit(appleHit{kind: item.kind, x: slot.Min.X, y: slot.Min.Y, w: slot.Dx(), h: slot.Dy(), index: i, id: item.id, cassette: fit, pose: pose, slot: slot})
}
