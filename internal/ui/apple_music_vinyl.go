package ui

import (
	"fmt"
	"image/color"
	"math"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

type appleVinylPickup struct {
	current, wanted int
	id              string
	pop             appleTween
	vinyl           deck.VinylRenderer
}

func (p *AppleMusicPage) resetVinylPickup() {
	p.vinylPickup.current, p.vinylPickup.wanted = -1, -1
	p.vinylPickup.id = ""
	p.vinylPickup.pop = appleTween{}
}

func (p *AppleMusicPage) catalogAlbumCrate() bool {
	kind := p.catalogState(p.drawer).catalog.request.Kind
	return p.drawer != appleQueue && (kind == applemusic.CatalogAlbums || kind == applemusic.CatalogArtistAlbums || kind == applemusic.LibraryAlbums)
}

func (p *AppleMusicPage) updateVinylPickup(wanted int, id string, now time.Time) {
	v := &p.vinylPickup
	changed := v.wanted != wanted
	v.wanted = wanted
	target := .5
	if wanted < 0 {
		target = 0
	}
	if v.current == wanted && v.id == id {
		if v.pop.to != target {
			v.pop.startPop(target, now, p.level)
		}
		return
	}
	if v.current >= 0 && v.pop.value(now) > 0 {
		if v.pop.to != 0 {
			v.pop.startPop(0, now, p.level)
		}
		if v.pop.value(now) > 0 {
			return
		}
	}
	start := now
	if v.current >= 0 && !changed && v.pop.to == 0 && v.pop.from > 0 {
		start = v.pop.at.Add(v.pop.duration)
	}
	v.current, v.id = wanted, id
	v.pop = appleTween{}
	if wanted >= 0 {
		v.pop.startPop(target, start, p.level)
	}
}

func appleVinylPlacement(x, y, w, rows int, aspect, amount float64) (deck.Rect, deck.Pose) {
	if aspect <= 0 {
		aspect = 2
	}
	discW := min(float64(min(34, w-14)), 12*aspect, float64(2*max(3, rows-8))*aspect)
	discH := discW / aspect
	tilt := min(max(amount*2, 0), 1)
	return deck.Rect{X: float64(x+w-6) - discW, Y: float64(y+1) - discH*amount, W: discW, H: discH},
		deck.Pose{Yaw: -.14, Pitch: .42 - .28*tilt, Roll: -.10 * tilt, Scale: 1}
}

func (p *AppleMusicPage) drawAlbumCrate(x, y, w, rows int, now time.Time) bool {
	s, items := &p.screen, p.catalogState(p.drawer).catalog.items
	graphical := s.w >= 60 && s.h >= 20 && w >= 36 && rows >= 10
	if !graphical {
		p.resetVinylPickup()
		return false
	}
	fit, _ := appleVinylPlacement(x, y, w, rows, p.cellAspect, applePopPeak)
	padding, bottom := max(2, int(math.Ceil(fit.H*applePopPeak))), 1
	if rows < padding+bottom+2 {
		p.resetVinylPickup()
		return false
	}
	capacity := max(0, (rows-padding-bottom)/2)
	start := min(max(0, p.rackStart), max(0, len(items)-capacity))
	if p.selected < start {
		start = p.selected
	}
	if p.selected >= start+capacity {
		start = p.selected - max(0, capacity-1)
	}
	p.rackStart = max(0, start)
	visible := min(capacity, max(0, len(items)-p.rackStart))
	desired := p.selected
	if p.rackHover >= 0 && p.rackHover < len(items) {
		desired = p.rackHover
	}
	if p.vinylPickup.current < p.rackStart || p.vinylPickup.current >= p.rackStart+visible ||
		(p.vinylPickup.current >= 0 && items[p.vinylPickup.current].ID != p.vinylPickup.id) {
		p.resetVinylPickup()
	}
	wanted, wantedID := -1, ""
	if desired >= p.rackStart && desired < p.rackStart+visible {
		wanted, wantedID = desired, items[desired].ID
	}
	p.updateVinylPickup(wanted, wantedID, now)
	amount := p.vinylPickup.pop.value(now)
	s.fill(x, y, w, rows, s.pal.Bg)
	st := deck.RackState{Padding: padding, Slots: capacity, Filled: visible, Out: -1, Tab: -1, Media: deck.RackVinyl}
	if i := p.vinylPickup.current - p.rackStart; p.vinylPickup.current >= 0 && amount > 0 {
		lbl := deck.VinylLabels[deck.VinylStyleForID(p.vinylPickup.id)-1]
		st.Tab, st.TabColor, st.TabMix = i, lbl.Outer, appleReveal(amount)
		st.Flash = applePopFlash(p.vinylPickup.pop, now)
	}
	v := deck.View{W: w, H: rows, Tier: p.tier, Aspect: p.cellAspect}
	s.drawDeck(p.rackPickup.rack.RenderRack(v, deckPaletteOf(appleNeon), st), x, y)
	p.drawAlbumCrateSlots(x, y, w, rows, padding, capacity, desired, amount)
	p.drawPeekingVinyl(x, y, w, rows, padding, visible, amount)
	footer := "VINYL LP CRATE · ALBUM SLEEVES"
	if p.vinylPickup.current >= 0 && p.vinylPickup.current < len(items) && amount > 0 {
		it := items[p.vinylPickup.current]
		lbl := deck.VinylLabels[deck.VinylStyleForID(it.ID)-1]
		footer = fmt.Sprintf("LP %02d [%s] · %s", p.vinylPickup.current+1, lbl.Name, it.Name)
	}
	s.text(x+3, y+rows-1, footer, uv.Style{Fg: s.pal.Secondary, Bg: color.RGBA{22, 34, 46, 255}}, w-6)
	return true
}

func (p *AppleMusicPage) drawAlbumCrateSlots(x, y, w, rows, padding, capacity, desired int, amount float64) {
	s, items := &p.screen, p.catalogState(p.drawer).catalog.items
	for row := 0; row < capacity; row++ {
		i := p.rackStart + row
		lx, ly, lw := deck.RackLabel(w, deck.RackState{Padding: padding}, row)
		lx, ly = x+lx, y+ly
		if i >= len(items) {
			if row == 0 {
				s.text(lx, ly, p.catalogEmptyLabel(), uv.Style{Fg: s.pal.Secondary}, lw)
			}
			continue
		}
		item := items[i]
		{
			var labelBg color.Color = color.RGBA{29, 44, 58, 255}
			if c := s.buf.CellAt(lx, ly); c != nil && c.Style.Bg != nil {
				labelBg = c.Style.Bg
			}
			room := p.vinylLabelRoom(x, y, w, rows, padding, ly, lx, lw, amount)
			title := fmt.Sprintf("◉ %02d %s", i+1, item.Name)
			if item.Artist != "" {
				title += " · " + item.Artist
			}
			fg := color.Color(s.pal.Text)
			if i == desired {
				fg = s.pal.Accent
			}
			s.text(lx, ly, title, uv.Style{Fg: fg, Bg: labelBg}, room)
		}
		p.addHit(appleHit{kind: appleHitCatalog, x: x + 2, y: ly, w: w - 4, h: 2, index: i, id: item.ID, slot: appleBox(x+2, ly, w-4, 2)})
	}
}

func (p *AppleMusicPage) vinylLabelRoom(x, y, w, rows, padding, ly, lx, lw int, amount float64) int {
	i := p.vinylPickup.current
	if i < p.rackStart || amount <= 0 {
		return lw
	}
	_, py, _ := deck.RackLabel(w, deck.RackState{Padding: padding}, i-p.rackStart)
	py += y
	fit, _ := appleVinylPlacement(x, py, w, rows, p.cellAspect, max(amount, applePopPeak))
	if ly >= py || float64(ly) < fit.Y-1 {
		return lw
	}
	return min(lw, max(0, int(fit.X)-lx-1))
}

func (p *AppleMusicPage) drawPeekingVinyl(x, y, w, rows, padding, visible int, amount float64) {
	i := p.vinylPickup.current
	if i < p.rackStart || i >= p.rackStart+visible || amount <= 0 {
		return
	}
	_, ly, _ := deck.RackLabel(w, deck.RackState{Padding: padding}, i-p.rackStart)
	ly += y
	fit, pose := appleVinylPlacement(x, ly, w, rows, p.cellAspect, amount)
	fit.Y -= 3
	view := deck.View{W: p.screen.w, H: max(0, p.screen.h-9), Tier: p.tier, Aspect: p.cellAspect, Fit: fit}
	style := deck.VinylStyleForID(p.vinylPickup.id)
	frame := p.vinylPickup.vinyl.RenderVinyl(view, deckPaletteOf(appleNeon), pose, deck.VinylState{Style: style, Spin: amount * 1.1, Peek: amount})
	bounds := appleFrameBounds(frame, 0, 3)
	clip := appleBox(x+2, 3, w-4, p.screen.h-9)
	reveal := bounds
	reveal.Max.Y = min(reveal.Max.Y, ly)
	reveal = reveal.Intersect(clip)
	p.drawRackFrame(frame, reveal)
	hit := appleHit{kind: appleHitCatalog, index: i, id: p.vinylPickup.id, slot: appleBox(x+2, ly, w-4, 2)}
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
			hit.x, hit.y, hit.w, hit.h = left, yy, xx-left, 1
			p.addHit(hit)
		}
	}
}
