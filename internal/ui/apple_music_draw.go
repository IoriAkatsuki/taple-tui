package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	"github.com/anhoder/foxful-cli/motion"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

var appleNeon = npPalette{
	Bg: color.RGBA{0x0e, 0x0f, 0x18, 0xff}, Surface: color.RGBA{0x1c, 0x19, 0x24, 0xff},
	Text: color.RGBA{0xf4, 0xec, 0xde, 0xff}, Hero: color.RGBA{0xff, 0x6b, 0x4a, 0xff},
	Secondary: color.RGBA{0xa6, 0x9a, 0x92, 0xff}, Rule: color.RGBA{0x40, 0x38, 0x4a, 0xff},
	Ghost: color.RGBA{0x28, 0x24, 0x32, 0xff}, Accent: color.RGBA{0x2e, 0xd8, 0xc6, 0xff},
	Accent2: color.RGBA{0xf5, 0xb9, 0x42, 0xff},
}

func appleBox(x, y, w, h int) image.Rectangle { return image.Rect(x, y, x+max(w, 0), y+max(h, 0)) }

func (p *AppleMusicPage) drawSwissSection(r image.Rectangle, num, title, meta string) {
	if r.Dx() < 4 || r.Dy() < 1 {
		return
	}
	s, pal := &p.screen, appleNeon
	x, y, w := r.Min.X, r.Min.Y, r.Dx()
	barBg := color.RGBA{0x24, 0x1e, 0x2b, 0xff}
	tagBg := color.RGBA{0x32, 0x29, 0x3b, 0xff}
	s.fillRGBA(x, y, w, 1, barBg)
	n := 0
	n += s.textRGBA(x, y, "▐", pal.Hero, barBg, true, true, false, w)
	n += s.textRGBA(x+n, y, " "+num+" ", pal.Accent2, tagBg, true, true, true, max(0, w-n))
	n += s.textRGBA(x+n, y, "▌ ", pal.Hero, barBg, true, true, false, max(0, w-n))
	n += s.textRGBA(x+n, y, title, pal.Text, barBg, true, true, true, max(0, w-n))
	metaW := 0
	if meta != "" {
		tag := " " + meta + " "
		mw := npWidth(tag) + 2
		if x+n+2+mw <= x+w {
			metaW = mw
			mx := x + w - mw
			s.textRGBA(mx, y, "▐", pal.Rule, barBg, true, true, false, 1)
			s.textRGBA(mx+1, y, tag, pal.Secondary, tagBg, true, true, false, mw-2)
			s.textRGBA(x+w-1, y, "▌", pal.Rule, barBg, true, true, false, 1)
		}
	}
	grooveStart := x + n + 1
	grooveEnd := x + w - metaW - 1
	if grooveEnd > grooveStart+1 {
		s.repeatRGBA(grooveStart, y, grooveEnd-grooveStart, "─", pal.Rule, barBg, true, true, false)
	}
}

func (p *AppleMusicPage) draw(w, h int, now time.Time, aspect float64) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	p.screen.reset(w, h)
	p.screen.pal = appleNeon
	p.cellAspect = aspect
	p.rackHover = -1
	if hit := p.hitAt(p.hoverX, p.hoverY); hit != nil && (hit.kind == appleHitTrack || hit.kind == appleHitCassette) && hit.index >= 0 && hit.index < len(p.items()) && p.items()[hit.index].ID == hit.id {
		p.rackHover = hit.index
	} else if hit != nil && hit.kind == appleHitCatalog && hit.index >= 0 && hit.index < len(p.catalogState(p.drawer).catalog.items) && p.catalogState(p.drawer).catalog.items[hit.index].ID == hit.id {
		p.rackHover = hit.index
	}
	p.hits = p.hits[:0]
	p.lyrics.area = image.Rectangle{}
	p.advance(now)
	s, pal := &p.screen, appleNeon
	p.drawHeader(w, h)
	body := appleBox(2, 3, w-4, h-9)
	compact := w < 60 || h < 20
	mix := p.scenePosition(now)
	deviceForRack := appleBox(body.Min.X, body.Min.Y, body.Dx(), max(5, body.Dy()/2))
	gap := 3
	paneW := (body.Dx() - gap) * 47 / 100
	stageW := body.Dx() - gap - paneW
	rightDeviceX := body.Min.X + paneW + gap
	deviceX := int(float64(rightDeviceX) + float64(body.Min.X-rightDeviceX)*mix)
	if w >= 100 {
		deviceForRack = appleBox(deviceX, body.Min.Y, stageW, body.Dy())
	}
	stageForRack, _ := appleDeviceStage(deviceForRack)
	p.rackView = p.deviceView(stageForRack, aspect)
	if compact {
		if p.scene == appleBrowse {
			p.drawDrawer(body, now)
		} else {
			p.drawListening(body, 1)
		}
	} else if w >= 100 {
		left := appleBox(body.Min.X, body.Min.Y, paneW, body.Dy())
		right := appleBox(body.Min.X+stageW+gap, body.Min.Y, paneW, body.Dy())
		if mix < 1 {
			drawer := left
			drawer.Max.X = drawer.Min.X + int(float64(drawer.Dx())*(1-mix))
			if drawer.Dx() >= 18 {
				p.drawBrowse(drawer, now)
			}
		}
		if mix > 0 {
			amount := mix
			if p.inserting {
				amount = min(max(float64(now.Sub(p.insertStart)-1080*time.Millisecond)/float64(200*time.Millisecond), 0), 1)
			}
			p.drawListening(right, amount)
		}
		device := appleBox(deviceX, body.Min.Y, stageW, body.Dy())
		p.drawDevice(device, now, aspect)
		if p.scene == appleBrowse && p.drawer == appleSearch && p.editing && mix > 0 {
			editorY := left.Min.Y + 1
			if body.Dy() >= 22 {
				editorY = left.Min.Y + 6
			}
			// Keep the editor above the returning device without changing its projection.
			p.drawSearchEditor(appleBox(left.Min.X+1, editorY, left.Dx()-2, 1))
		}
	} else {
		// A focused search must be visible even while the camera or scene returns.
		deviceVisible := p.scene == appleListen || p.zoomed || p.camera.active(now) || p.inserting || p.sceneMotion.active(now)
		searchEditor := p.scene == appleBrowse && p.drawer == appleSearch && p.editing
		if searchEditor || !deviceVisible || (p.inserting && now.Before(p.insertStart)) {
			p.drawBrowse(body, now)
		} else if p.scene == appleListen || p.inserting {
			if p.inserting && !p.insertSourceCaptured {
				// Capture the fully extracted source before replacing the narrow drawer.
				hitCount := len(p.hits)
				p.drawBrowse(body, now)
				if !p.rackGraphical {
					p.insertSourceCaptured = true
				}
				p.screen.fill(body.Min.X, body.Min.Y, body.Dx(), body.Dy(), pal.Bg)
				p.hits = p.hits[:hitCount]
			}
			device := appleBox(body.Min.X, body.Min.Y, body.Dx(), max(5, body.Dy()/2))
			p.drawListening(appleBox(body.Min.X, device.Max.Y+1, body.Dx(), body.Max.Y-device.Max.Y-1), 1)
			p.drawDevice(device, now, aspect)
		} else {
			p.drawDevice(body, now, aspect)
		}
	}
	p.drawFooter(now)
	if p.panel != "" {
		p.drawPanel()
	}
	s.fillBackground()
	return s.render()
}

func (p *AppleMusicPage) drawHeader(w, h int) {
	s, pal := &p.screen, appleNeon
	fasciaBg := color.RGBA{0x26, 0x20, 0x2c, 0xff}
	podBg := color.RGBA{0x17, 0x13, 0x1e, 0xff}
	acctBg := color.RGBA{0x2f, 0x27, 0x38, 0xff}
	s.fillRGBA(0, 0, w, 1, fasciaBg)
	x := 1
	if w >= 72 {
		x += s.textRGBA(x, 0, "◉ ", pal.Secondary, fasciaBg, true, true, false, w-x)
		x += s.textRGBA(x, 0, "▐", pal.Accent2, fasciaBg, true, true, false, w-x)
		x += s.textRGBA(x, 0, " T A P E F O X ", pal.Text, podBg, true, true, true, max(0, w-x-26))
		x += s.textRGBA(x, 0, "▌ ", pal.Accent2, fasciaBg, true, true, false, max(0, w-x-26))
		s.textRGBA(x, 0, "Apple Music · Cassette Deck", pal.Secondary, fasciaBg, true, true, false, max(0, w-x-26))
	} else {
		s.textRGBA(1, 0, "Apple Music · Cassette Deck", pal.Text, fasciaBg, true, true, false, max(0, w-28))
	}
	if w >= 98 {
		playLit := p.ready() && p.state.Playing
		vuLit := p.meter != nil && p.meter.Frame().Ready
		lrcLit := len(p.lyrics.data.Lines) > 0
		lx := w - 42
		jewel := false
		if w >= 108 {
			lx = w - 53
			jewel = true
		}
		for _, lamp := range []struct {
			label string
			lit   bool
			col   color.RGBA
		}{{"PLAY", playLit, pal.Hero}, {"VU", vuLit, pal.Accent}, {"LRC", lrcLit, pal.Accent2}} {
			fg := pal.Ghost
			if lamp.lit {
				fg = lamp.col
			}
			text := lamp.label
			bg := fasciaBg
			if jewel {
				dot := "○ "
				if lamp.lit {
					dot = "● "
				}
				text = " " + dot + lamp.label + " "
				bg = podBg
			}
			lx += s.textRGBA(lx, 0, text, fg, bg, true, true, true, max(0, w-lx)) + 1
		}
	}
	account := T("account.sign_in")
	if p.state.Authorized {
		account = T("account.signed_in")
	}
	if !p.state.Ready {
		account = T("account.connecting")
	}
	if p.loggingOut {
		account = T("account.clearing")
	}
	ax := max(0, w-23)
	aw := min(w, 21)
	s.textRGBA(ax, 0, " "+account+" ", pal.Accent, acctBg, true, true, true, aw)
	if w >= 80 && w-1 >= ax+aw {
		s.textRGBA(w-2, 0, "◉", pal.Secondary, fasciaBg, true, true, false, 1)
	}
	p.addHit(appleHit{kind: appleHitKey, x: ax, y: 0, w: aw, h: 1, key: "b"})
	if h >= 10 {
		p.drawNavigation(w)
		p.drawNavigationContext(w)
	}
}

func (p *AppleMusicPage) drawNavigation(w int) {
	railBg := color.RGBA{0x1b, 0x18, 0x24, 0xff}
	switchBg := color.RGBA{0x33, 0x29, 0x3b, 0xff}
	p.screen.fillRGBA(0, 1, w, 1, railBg)
	scene, target := T("scene.browse"), T("nav.listen")
	if p.scene == appleListen {
		scene, target = T("scene.listen"), T("nav.browse")
	}
	x := 2
	x += p.screen.textRGBA(x, 1, "▐", appleNeon.Hero, railBg, true, true, false, max(0, w-x-2))
	x += p.screen.textRGBA(x, 1, " ▸ "+scene+" ", appleNeon.Text, switchBg, true, true, true, max(0, w-x-2))
	x += p.screen.textRGBA(x, 1, "▌ ", appleNeon.Hero, railBg, true, true, false, max(0, w-x-2))
	p.drawKeys(x, 1, w-x-2, []struct{ key, label string }{{"i", target}})
	x += npWidth(target) + 2
	x += p.screen.textRGBA(x, 1, "┃ ", appleNeon.Rule, railBg, true, true, false, max(0, w-x-2))
	p.drawKeys(x, 1, w-x-2, []struct{ key, label string }{
		{"1", T("nav.search")}, {"2", T("nav.library")}, {"3", T("nav.queue")}, {"s", T("nav.settings")},
	})
	p.screen.tintBg(0, 1, w, 1, railBg)
}

func (p *AppleMusicPage) navigationBackLabel() string {
	switch {
	case p.panel != "":
		return T("panel.esc_close")
	case p.editing:
		return T("back.editing")
	case p.zoomed:
		return T("back.zoom")
	case p.scene == appleListen || p.inserting:
		return T("back.browse")
	case p.drawer == appleQueue:
		return ""
	}
	c := p.catalogState(p.drawer)
	if c.searching {
		return T("back.loading")
	}
	if n := len(c.catalogHistory); n > 0 {
		title := c.catalogHistory[n-1].view.title
		if title == "" {
			title = T("location.parent")
		}
		return T("back.parent", title)
	}
	return ""
}

func (p *AppleMusicPage) navigationPath() string {
	if p.scene == appleListen {
		return T("scene.listen") + " / " + T("location.track")
	}
	path := []string{T("scene.browse"), T([]string{"location.search", "location.library", "location.queue"}[p.drawer])}
	if p.drawer != appleQueue {
		c := p.catalogState(p.drawer)
		for _, parent := range c.catalogHistory {
			if parent.view.title != "" {
				path = append(path, parent.view.title)
			}
		}
		if c.catalog.title != "" {
			path = append(path, c.catalog.title)
		}
	}
	return strings.Join(path, " / ")
}

func (p *AppleMusicPage) drawNavigationContext(w int) {
	vfdStrip := color.RGBA{0x14, 0x13, 0x1d, 0xff}
	escBg := color.RGBA{0x22, 0x1e, 0x2d, 0xff}
	p.screen.fillRGBA(0, 2, w, 1, vfdStrip)
	label := p.navigationPath()
	if p.editing {
		label = T("focus.search") + " · " + label
	} else if p.zoomed {
		label = T("lens.close_up") + " · " + label
	} else if p.camera.active(p.lastFrame) {
		label = T("lens.moving") + " · " + label
	}
	back := p.navigationBackLabel()
	backW := min(npWidth(back), max(0, w-4)/2)
	labelX := 2
	available := max(0, w-4)
	if w >= 72 {
		p.screen.textRGBA(2, 2, "▸ ", appleNeon.Accent2, vfdStrip, true, true, false, max(0, w-4))
		labelX = 4
		available = max(0, w-6)
	}
	if backW > 0 {
		available = max(0, available-backW-2)
	}
	p.screen.textRGBA(labelX, 2, label, appleNeon.Secondary, vfdStrip, true, true, false, available)
	if backW > 0 {
		x := w - 2 - backW
		p.screen.textRGBA(x, 2, back, appleNeon.Accent, escBg, true, true, false, backW)
		p.addHit(appleHit{kind: appleHitKey, key: "esc", x: x, y: 2, w: backW, h: 1})
	}
}

func (p *AppleMusicPage) drawBrowse(r image.Rectangle, now time.Time) {
	if r.Dy() < 18 || (p.screen.w >= 100 && p.editing && r.Dy() < 22) {
		p.drawDrawer(r, now)
		return
	}
	s := &p.screen
	cw := max(0, r.Dx()-2)
	cx := r.Min.X + 1
	cardBg := color.RGBA{0x23, 0x1d, 0x2a, 0xff}
	p.drawSwissSection(appleBox(cx, r.Min.Y, cw, 1), "00", "N O W   P L A Y I N G · J-CARD", "i")
	s.fillRGBA(cx, r.Min.Y+1, cw, 2, cardBg)
	name := p.state.Track.Name
	if name == "" {
		name = T("status.idle")
	}
	flag := "‖ "
	if p.ready() && p.state.Playing {
		flag = "▶ "
	}
	// Dual-stripe TDK/Maxell J-card spine along left card edge.
	s.textRGBA(cx, r.Min.Y+1, "▐", appleNeon.Hero, cardBg, true, true, false, 1)
	s.textRGBA(cx, r.Min.Y+2, "▐", appleNeon.Accent2, cardBg, true, true, false, 1)
	s.textRGBA(cx+2, r.Min.Y+1, flag+name, appleNeon.Hero, cardBg, true, true, true, max(0, cw-3))
	sub := p.state.Track.Artist
	if p.state.Track.Album != "" {
		if sub != "" {
			sub += " · " + p.state.Track.Album
		} else {
			sub = p.state.Track.Album
		}
	}
	metaW := max(0, cw-3)
	if cw >= 28 {
		side := "SIDE A"
		sw := npWidth(side)
		if sw+8 <= cw {
			s.textRGBA(cx+cw-sw-1, r.Min.Y+2, side, appleNeon.Rule, cardBg, true, true, false, sw)
			metaW = max(0, cw-sw-5)
		}
	}
	s.textRGBA(cx+2, r.Min.Y+2, sub, appleNeon.Secondary, cardBg, true, true, false, metaW)
	s.repeatRGBA(cx, r.Min.Y+3, cw, "▀", cardBg, appleNeon.Bg, true, true, false)
	p.addHit(appleHit{kind: appleHitKey, x: r.Min.X, y: r.Min.Y, w: r.Dx(), h: 4, key: "i"})
	r.Min.Y += 5
	p.drawDrawer(r, now)
}

func (p *AppleMusicPage) drawDrawer(r image.Rectangle, now time.Time) {
	if r.Dx() < 4 || r.Dy() < 1 {
		return
	}
	label := T("drawer.search")
	if p.drawer == appleLibrary {
		label = T("drawer.library")
	}
	if p.drawer == appleQueue {
		label = T("drawer.queue", p.state.QueueTotal)
	}
	meta := fmt.Sprintf("%02d · i", p.listCount())
	p.drawSwissSection(appleBox(r.Min.X+1, r.Min.Y, r.Dx()-2, 1), "01", label, meta)
	if r.Dx() >= 16 {
		hw := min(10, r.Dx()-2)
		p.addHit(appleHit{kind: appleHitKey, x: r.Max.X - 1 - hw, y: r.Min.Y, w: hw, h: 1, key: "i"})
	}
	x, y, w := r.Min.X+1, r.Min.Y+1, r.Dx()-2
	if p.drawer == appleLibrary {
		y = p.drawCatalogHeader(x, y, w)
		if !p.state.Ready || !p.state.Authorized {
			p.resetRackPickup()
			p.drawLibraryGateEmpty(x, y, w, max(0, r.Max.Y-1-y), now)
			return
		}
		message := p.library.catalog.title
		if p.library.searching {
			message = T("status.loading")
		}
		if p.library.catalogError != "" {
			message = p.library.catalogError
		}
		if y < r.Max.Y-1 {
			p.screen.text(x+1, y, message, uv.Style{Fg: appleNeon.Secondary}, w-2)
			y++
		}
	}
	if p.drawer == appleSearch {
		p.drawInput(x, y, w)
		y = p.drawCatalogHeader(x, y+1, w)
	}
	bottom := r.Max.Y - 1
	c := p.catalogState(p.drawer)
	if p.drawer != appleQueue && c.catalog.next != "" && bottom > y {
		bottom--
		label := T("cat.load_more")
		if c.searching {
			label = T("status.loading")
		}
		p.drawKeys(x+1, bottom, w-2, []struct{ key, label string }{{"l", label}})
	}
	if p.catalogContainers() {
		p.drawCatalogList(x, y, w, max(0, bottom-y), now)
	} else {
		p.drawList(x, y, w, max(0, bottom-y), now)
	}
}

func (p *AppleMusicPage) drawListening(r image.Rectangle, reveal float64) {
	if reveal <= 0 || r.Dx() < 4 || r.Dy() < 3 {
		return
	}
	pal, s := appleNeon, &p.screen
	boxW := r.Dx()
	if reveal < 1 {
		boxW = max(4, min(r.Dx(), int(math.Round(float64(r.Dx())*(.35+.65*reveal)))))
	}
	p.drawSwissSection(appleBox(r.Min.X+1, r.Min.Y, max(0, boxW-2), 1), "02", "T R A C K  ·  LISTENING", "DETAIL")
	x, y, w := r.Min.X+1, r.Min.Y+2, max(0, boxW-2)
	if w <= 0 {
		return
	}
	stage := func(start, span float64) float64 {
		if reveal >= 1 {
			return 1
		}
		return min(max((reveal-start)/span, 0), 1)
	}
	cardBg := color.RGBA{0x23, 0x1d, 0x2a, 0xff}
	if w >= 6 && y+1 < r.Max.Y {
		s.fillRGBA(x, y, w, 2, cardBg)
		s.textRGBA(x, y, "▐", npLerpRGBA(pal.Bg, pal.Hero, stage(0, .55)), cardBg, true, true, false, 1)
		s.textRGBA(x, y+1, "▐", npLerpRGBA(pal.Bg, pal.Accent2, stage(.1, .55)), cardBg, true, true, false, 1)
	}
	fg := npLerpRGBA(pal.Bg, pal.Hero, stage(0, .55))
	name := p.state.Track.Name
	if name == "" {
		name = T("status.idle")
	}
	flag := "‖ "
	if p.ready() && p.state.Playing {
		flag = "▶ "
	}
	badge := ""
	if w >= 38 {
		model := "CrO₂"
		if st := p.bayStyle; st >= 1 && int(st) <= len(deck.CassetteStyles) {
			model = deck.CassetteStyles[st-1].Name
		}
		badge = "SIDE A · " + model
	}
	tx := x + 2
	tw := max(0, w-2)
	titleMax := tw
	if badge != "" {
		bw := npWidth(badge)
		if bw+14 <= tw {
			s.textRGBA(x+w-bw-1, y, badge, npLerpRGBA(pal.Bg, pal.Accent2, stage(.1, .5)), cardBg, true, true, false, bw)
			titleMax = max(0, tw-bw-3)
		}
	}
	s.textRGBA(tx, y, flag+name, fg, cardBg, true, true, true, titleMax)
	if u := stage(.15, .55); u > 0 && y+1 < r.Max.Y-1 {
		meta := p.state.Track.Artist
		if p.state.Track.Album != "" {
			if meta != "" {
				meta += " · " + p.state.Track.Album
			} else {
				meta = p.state.Track.Album
			}
		}
		s.textRGBA(tx, y+1, meta, npLerpRGBA(pal.Bg, pal.Secondary, u), cardBg, true, true, false, max(0, w-3))
	}
	if u := stage(.3, .55); u > 0 && y+3 < r.Max.Y-1 {
		if r.Dy() >= 10 {
			p.drawSwissSection(appleBox(x, y+2, w, 1), "03", T("lyrics.title"), p.lyricsLabel(p.lastFrame))
		}
		p.drawLyrics(appleBox(x, y+3, w, r.Max.Y-y-4), p.lastFrame, u)
	}
}

func (p *AppleMusicPage) drawDevice(r image.Rectangle, now time.Time, aspect float64) {
	if r.Dx() < 10 || r.Dy() < 5 {
		return
	}
	secNum := "02"
	if p.scene == appleListen {
		secNum = "01"
	}
	modelTag := "TPS-L2 · CrO₂"
	if st := p.mechanics(now).Style; st >= 1 && int(st) <= len(deck.CassetteStyles) {
		modelTag = deck.CassetteStyles[st-1].Model + " · SIDE A"
	}
	p.drawSwissSection(appleBox(r.Min.X+1, r.Min.Y, r.Dx()-2, 1), secNum, "D E C K  ·  Cassette Deck", modelTag)
	stage, meters := appleDeviceStage(r)
	p.stageRect = stage
	p.drawMartinaiseStage(stage.Min.X, stage.Min.Y, stage.Dx(), stage.Dy(), now)
	frame, offsetX, offsetY := p.deviceFrame(stage, now, aspect)
	p.drawStandbyGlow(stage, frame, offsetX, offsetY, now)
	if offsetX == stage.Min.X && offsetY == stage.Min.Y {
		p.screen.drawDeckOverStage(frame, offsetX, offsetY)
	} else {
		p.screen.drawDeck(frame, offsetX, offsetY)
	}
	// The travelling cassette occludes rack controls as well as their pixels.
	if offsetX == 0 && offsetY == 3 {
		for i, cell := range frame.Cells {
			if cell.Ch == "" {
				continue
			}
			x, y := i%frame.W, 3+i/frame.W
			if !image.Pt(x, y).In(stage) {
				p.addHit(appleHit{kind: appleHitMoving, x: x, y: y, w: 1, h: 1})
			}
		}
	}
	for y := stage.Min.Y - offsetY; y < stage.Max.Y-offsetY; y++ {
		for x := stage.Min.X - offsetX; x < stage.Max.X-offsetX; {
			if frame.Cells[y*frame.W+x].Ch == "" {
				x++
				continue
			}
			start := x
			for x < stage.Max.X-offsetX && frame.Cells[y*frame.W+x].Ch != "" {
				x++
			}
			p.addHit(appleHit{kind: appleHitStage, x: offsetX + start, y: offsetY + y, w: x - start, h: 1, key: "z"})
		}
	}
	if !p.ready() || !p.state.Playing {
		p.drawStandby(stage.Min.X, stage.Min.Y, stage.Dx(), stage.Dy(), now)
	}
	p.glitch(stage.Min.X, stage.Min.Y, stage.Dx(), stage.Dy(), now)
	if meters {
		y := stage.Max.Y + 1
		podBg := color.RGBA{0x15, 0x14, 0x20, 0xff}
		p.screen.fillRGBA(stage.Min.X, y, stage.Dx(), 5, podBg)
		p.screen.textRGBA(stage.Min.X+1, y, "TIME COUNTER", appleNeon.Secondary, podBg, true, true, false, 22)
		counterTime, counterState := now, &p.counter
		if p.level == motion.Off {
			counterTime, counterState = time.Unix(0, 0), nil
		}
		DrawNixieCounter(&p.screen, stage.Min.X+1, y+1, appleDuration(p.state.Position), p.ready(), counterTime, counterState)
		for row := range 5 {
			p.screen.setRGBA(stage.Min.X+23, y+row, "│", 1, appleNeon.Rule, podBg, true, true, false)
		}
		p.drawMeter(stage.Min.X+25, y, stage.Dx()-26, p.meter.Frame())
		p.screen.tintBg(stage.Min.X, y, stage.Dx(), 5, podBg)
		if y+5 < r.Max.Y {
			p.screen.repeatRGBA(stage.Min.X, y+5, stage.Dx(), "▀", podBg, appleNeon.Bg, true, true, false)
		}
	}
}

func appleDeviceStage(r image.Rectangle) (image.Rectangle, bool) {
	stage := appleBox(r.Min.X+1, r.Min.Y+1, r.Dx()-2, r.Dy()-2)
	meters := stage.Dy() >= 17 && stage.Dx() >= 42
	if meters {
		stage.Max.Y -= 6
	}
	return stage, meters
}

func (p *AppleMusicPage) deviceView(stage image.Rectangle, aspect float64) deck.View {
	return deck.View{W: p.screen.w, H: max(0, p.screen.h-9), Aspect: aspect, Tier: p.tier,
		Fit: deck.Rect{X: float64(stage.Min.X), Y: float64(stage.Min.Y - 3), W: float64(stage.Dx()), H: float64(stage.Dy())}}
}

func (p *AppleMusicPage) deviceFrame(stage image.Rectangle, now time.Time, aspect float64) (*deck.Frame, int, int) {
	ms := float64(now.Sub(p.insertStart)) / float64(time.Millisecond)
	basePose := p.sceneBasePose(now)
	if !p.swapStart.IsZero() {
		st := p.mechanics(now)
		out := float64(now.Sub(p.swapStart)) / float64(time.Millisecond)
		in := float64(now.Sub(p.swap.insertAt)) / float64(time.Millisecond)
		outgoing := out >= 220 && out < 400 && (p.swap.insertAt.IsZero() || now.Before(p.swap.insertAt))
		incoming := !p.swap.insertAt.IsZero() && in >= 0 && in < 180
		if outgoing || incoming {
			st.FlightFrom = deck.Rect{X: 0, Y: float64(stage.Dy() + 8), W: 26, H: 8}
			if outgoing {
				st.Flight = 1 - motion.InOutCubic((out-220)/180)
			} else {
				st.Flight = motion.InOutCubic(in / 180)
			}
			v := deck.View{W: stage.Dx(), H: stage.Dy(), Aspect: aspect, Tier: p.tier}
			return p.renderer.RenderWalkman(v, deckPaletteOf(appleNeon), basePose, st), stage.Min.X, stage.Min.Y
		}
	}
	if p.inserting && p.insertOldPresent && ms < 0 && now.Sub(p.insertEjectAt) >= 200*time.Millisecond {
		st := p.mechanics(now)
		st.FlightFrom = deck.Rect{X: 0, Y: float64(stage.Dy() + 8), W: 26, H: 8}
		st.Flight = 1 - motion.InOutCubic(min(max(float64(now.Sub(p.insertEjectAt)-200*time.Millisecond)/float64(120*time.Millisecond), 0), 1))
		v := deck.View{W: stage.Dx(), H: stage.Dy(), Aspect: aspect, Tier: p.tier}
		return p.renderer.RenderWalkman(v, deckPaletteOf(appleNeon), basePose, st), stage.Min.X, stage.Min.Y
	}
	if !p.inserting || ms < 0 || ms >= 380 {
		return p.cassette(stage.Dx(), stage.Dy(), now, aspect), stage.Min.X, stage.Min.Y
	}
	u := motion.InOutCubic(min(max((ms-80)/300, 0), 1))
	source := p.insertOrigin
	source.Y -= 3
	source.Y -= (1 - u) * min(ms/80, 1)
	v := deck.View{W: p.screen.w, H: max(0, p.screen.h-9), Aspect: aspect, Tier: p.tier,
		Fit: deck.Rect{X: float64(stage.Min.X), Y: float64(stage.Min.Y - 3), W: float64(stage.Dx()), H: float64(stage.Dy())}}
	st := p.mechanics(now)
	st.Cassette, st.FlightFrom, st.Flight = 1, source, u
	st.FlightPose = p.insertPose
	return p.renderer.RenderWalkman(v, deckPaletteOf(appleNeon), basePose, st), 0, 3
}

func (p *AppleMusicPage) drawFooter(now time.Time) {
	s, w, h := &p.screen, p.screen.w, p.screen.h
	consoleBg := color.RGBA{0x1b, 0x18, 0x25, 0xff}
	statusBg := color.RGBA{0x13, 0x11, 0x1b, 0xff}
	if h < 3 {
		if h > 1 {
			s.fillRGBA(0, h-1, w, 1, consoleBg)
			p.drawKeys(0, h-1, w, []struct{ key, label string }{{"space", "▶/‖"}, {"b", "b 登录"}, {"q", "q"}})
		}
		return
	}
	clock := "--:--"
	if p.ready() {
		clock = npClock(appleDuration(p.state.Position))
	}
	if h >= 7 {
		s.fillRGBA(0, h-5, w, 4, consoleBg)
		s.textRGBA(2, h-5, "▸ "+clock+" / "+npClock(appleDuration(p.state.Track.Duration)), appleNeon.Accent2, consoleBg, true, true, true, max(0, w-4))
		if p.pending {
			s.textRGBA(24, h-5, T("status.loading"), appleNeon.Accent2, consoleBg, true, true, false, max(0, w-40))
		} else if p.playbackError != "" {
			s.textRGBA(24, h-5, T("status.play_failed"), appleNeon.Accent2, consoleBg, true, true, false, max(0, w-40))
		}
		if w >= 86 {
			mode := "STANDBY · CrO₂"
			if p.ready() && p.state.Playing {
				mode = "PLAYING · STEREO"
			} else if p.ready() && p.state.Paused {
				mode = "PAUSED · STEREO"
			}
			mw := npWidth(mode)
			rx := max(54, w-2-mw)
			if rx+mw <= w {
				s.textRGBA(rx, h-5, mode, appleNeon.Secondary, consoleBg, true, true, false, mw)
			}
		}
		x, y, width := 2, h-4, w-4
		troughBg := color.RGBA{0x14, 0x12, 0x1d, 0xff}
		if width >= 4 {
			s.setRGBA(x, y, "▕", 1, appleNeon.Rule, consoleBg, true, true, false)
			s.setRGBA(x+width-1, y, "▏", 1, appleNeon.Rule, consoleBg, true, true, false)
			s.fillRGBA(x+1, y, width-2, 1, troughBg)
			s.repeatRGBA(x+1, y, width-2, "─", appleNeon.Rule, troughBg, true, true, false)
			if p.ready() {
				drawSubpixelSlider(s, x+1, y, width-2, p.progress(), appleNeon.Accent, appleNeon.Hero, appleNeon.Rule)
				p.addHit(appleHit{kind: appleHitSeek, x: x, y: y, w: width, h: 1})
			}
			s.tintBg(x+1, y, width-2, 1, troughBg)
		} else {
			s.repeat(x, y, width, "─", uv.Style{Fg: appleNeon.Rule})
			if p.ready() {
				drawSubpixelSlider(s, x, y, width, p.progress(), appleNeon.Accent, appleNeon.Hero, appleNeon.Rule)
				p.addHit(appleHit{kind: appleHitSeek, x: x, y: y, w: width, h: 1})
			}
			s.tintBg(x, y, width, 1, consoleBg)
		}
	} else {
		s.fillRGBA(0, h-min(h, 3), w, min(h, 3), consoleBg)
	}
	p.drawTransport(1, h-3, w-2)
	s.tintBg(0, h-3, w, 1, consoleBg)
	if h >= 4 {
		back := p.navigationBackLabel()
		backW := min(npWidth(back), max(0, w-2)/2)
		keysW := w - 2
		if backW > 0 {
			keysW = max(0, keysW-backW-2)
			p.screen.textRGBA(w-1-backW, h-2, back, appleNeon.Accent, color.RGBA{0x22, 0x1e, 0x2d, 0xff}, true, true, false, backW)
			p.addHit(appleHit{kind: appleHitKey, key: "esc", x: w - 1 - backW, y: h - 2, w: backW, h: 1})
		}
		p.drawKeys(1, h-2, keysW, []struct{ key, label string }{
			{"/", T("bar.search")},
			{"v", T("bar.meter")},
			{"a", T("bar.append")},
			{"z", T("bar.zoom")},
			{"i", T("bar.mode")},
			{"o", T("bar.controls")},
			{"q", T("bar.quit")},
		})
		s.tintBg(0, h-2, w, 1, consoleBg)
	}
	s.fillRGBA(0, h-1, w, 1, statusBg)
	s.textRGBA(1, h-1, "▸ ", appleNeon.Accent2, statusBg, true, true, false, max(0, w-2))
	status := p.status
	if p.playbackError != "" {
		status = p.playbackError
	}
	s.textRGBA(3, h-1, status, appleNeon.Secondary, statusBg, true, true, false, max(0, w-4))
}

func (p *AppleMusicPage) drawTransport(x, y, w int) {
	label := "SPC ▶"
	if p.state.Playing {
		label = "SPC ‖"
	}
	p.drawKeys(x, y, w, []struct{ key, label string }{{"p", "p ◀◀"}, {"space", label}, {"n", "n ▶▶"}, {"-", "−"}, {"+", "+"}})
	if w >= 45 {
		x += 33
		p.screen.text(x, y, fmt.Sprintf("VOL %02d", int(p.state.Volume*100)), uv.Style{Fg: appleNeon.Accent2}, 7)
		width := min(12, w-42)
		if width > 1 {
			drawSubpixelSlider(&p.screen, x+8, y, width, p.state.Volume, appleNeon.Accent, appleNeon.Hero, appleNeon.Rule)
			p.addHit(appleHit{kind: appleHitVolume, x: x + 8, y: y, w: width, h: 1})
		}
	}
}

func drawSubpixelSlider(s *npScreen, x, y, width int, frac float64, fillCol, capCol, trackCol color.RGBA) {
	if width <= 0 {
		return
	}
	sub := [7]string{"▏", "▎", "▍", "▌", "▋", "▊", "▉"}
	frac = min(max(frac, 0), 1)
	s.repeatRGBA(x, y, width, "─", trackCol, color.RGBA{}, true, false, false)
	scaled := frac * float64(max(0, width-1))
	full := int(math.Floor(scaled))
	rem := scaled - float64(full)
	for i := range full {
		s.setRGBA(x+i, y, "█", 1, fillCol, color.RGBA{}, true, false, false)
	}
	if rem > 0.125 && full < width-1 {
		idx := min(max(int(rem*8)-1, 0), 6)
		s.setRGBA(x+full, y, sub[idx], 1, fillCol, color.RGBA{}, true, false, false)
	}
	knobX := x + min(int(math.Round(scaled)), width-1)
	s.setRGBA(knobX, y, "●", 1, capCol, color.RGBA{}, true, false, false)
}

func (p *AppleMusicPage) drawPanel() {
	p.hits = p.hits[:0]
	if p.panel == appleLogoutPanel {
		p.drawLogoutPanel()
		return
	}
	w, fullH := min(p.screen.w-4, 66), min(p.screen.h-4, 16)
	if w < 12 || fullH < 3 {
		p.screen.fill(0, 0, p.screen.w, p.screen.h, appleNeon.Surface)
		p.screen.text(0, 0, T("panel.shortcuts")+" · "+T("panel.esc_close"), uv.Style{Fg: appleNeon.Accent2}, p.screen.w)
		return
	}
	for sy := range p.screen.h {
		for sx := range p.screen.w {
			p.screen.shade(sx, sy, .42)
		}
	}
	open := 1.0
	if p.panelTween.active(p.lastFrame) {
		open = max(.3, p.panelTween.value(p.lastFrame))
	}
	h := max(3, int(math.Round(float64(fullH)*open)))
	x, y := (p.screen.w-w)/2, (p.screen.h-h)/2
	p.screen.fill(x, y, w, h, appleNeon.Surface)
	appleBezel(&p.screen, x, y, w, h, p.panel)

	langLabel := "English [en]"
	if currentLang() == "zh" {
		langLabel = "简体中文 [zh]"
	}
	autoplay := T("panel.autoplay_wait")
	if p.state.Ready {
		autoplay = T("panel.autoplay_unavailable")
		if p.state.AutoplayAvailable {
			autoplay = T("panel.autoplay_pending")
			if p.state.Autoplay {
				autoplay = T("panel.autoplay_on")
			}
		}
	}

	lines := []string{
		fmt.Sprintf("Language / 语言: %s  (l/tab switch)", langLabel),
		T("panel.render_fmt", p.renderBackend),
		autoplay,
		T("panel.shortcuts"),
		T("panel.categories"),
		T("panel.playback"),
		T("panel.views"),
		T("panel.motion_fmt", p.level, p.tier),
		T("panel.vu_desc"),
		T("panel.account_desc"),
	}
	for i, line := range lines {
		if y+2+i >= y+h-2 {
			break
		}
		p.screen.text(x+2, y+2+i, line, uv.Style{Fg: appleNeon.Text}, w-4)
	}
	p.drawKeys(x+2, y+h-2, w-4, []struct{ key, label string }{
		{"l", "l Language / 语言"},
		{"esc", T("panel.esc_close")},
		{"B", T("account.clear_key")},
	})
}

func (p *AppleMusicPage) drawStandby(x, y, w, h int, now time.Time) {
	strength := .55
	if p.level != motion.Off {
		strength = .4 + .35*(1+math.Sin(float64(now.UnixMilli())/1600))/2
	}
	label := "● STANDBY"
	if p.pending {
		label = "○ LOADING"
	} else if p.state.Preview {
		label = "PREVIEW"
	} else if p.state.Track.ID != "" && !p.ready() {
		label = "○ STANDBY"
	} else if p.state.Paused {
		label = "‖ PAUSED"
	}
	p.screen.text(x+1, y+h-1, label, uv.Style{Fg: npLerp(appleNeon.Rule, appleNeon.Accent, strength)}, w-2)
}

// The bloom is decorative light behind the housing, never an audio signal.
func (p *AppleMusicPage) drawStandbyGlow(stage image.Rectangle, frame *deck.Frame, offsetX, offsetY int, now time.Time) {
	if p.level == motion.Off || p.state.Playing || p.pending || p.inserting || !p.swapStart.IsZero() {
		return
	}
	phase := float64(now.UnixMilli()%4000) / 4000
	strength := .045 + .055*(.5-.5*math.Cos(2*math.Pi*phase))
	if p.level == motion.Subtle {
		strength *= .6
	}
	aspect := p.cellAspect
	if aspect <= 0 {
		aspect = 2
	}
	var kernel [5][9]float64
	for dy := -2; dy <= 2; dy++ {
		for dx := -4; dx <= 4; dx++ {
			dist := float64(dx*dx) + float64(dy*dy)*aspect*aspect
			if dist <= 16 {
				kernel[dy+2][dx+4] = math.Exp(-dist / 6)
			}
		}
	}
	width, height := stage.Dx(), stage.Dy()
	need := width * height
	if cap(p.glowBuf) < need {
		p.glowBuf = make([]float64, need)
	}
	glow := p.glowBuf[:need]
	clear(glow)
	occupied := func(x, y int) bool { return frame.Cells[(y-offsetY)*frame.W+x-offsetX].Ch != "" }
	for y := stage.Min.Y; y < stage.Max.Y; y++ {
		for x := stage.Min.X; x < stage.Max.X; x++ {
			if !occupied(x, y) {
				continue
			}
			for dy := -2; dy <= 2; dy++ {
				yy := y + dy
				if yy < stage.Min.Y || yy >= stage.Max.Y {
					continue
				}
				rowOff := (yy - stage.Min.Y) * width
				for dx := -4; dx <= 4; dx++ {
					k := kernel[dy+2][dx+4]
					if k == 0 {
						continue
					}
					xx := x + dx
					if xx < stage.Min.X || xx >= stage.Max.X {
						continue
					}
					index := rowOff + xx - stage.Min.X
					if k > glow[index] {
						glow[index] = k
					}
				}
			}
		}
	}
	halo := appleNeon.Accent2
	for y := stage.Min.Y; y < stage.Max.Y; y++ {
		rowOff := (y - stage.Min.Y) * width
		for x := stage.Min.X; x < stage.Max.X; x++ {
			amount := glow[rowOff+x-stage.Min.X] * strength
			if amount == 0 || occupied(x, y) {
				continue
			}
			cell := npEmptyCell
			if existing, ok := p.screen.cellAt(x, y); ok {
				cell = existing
			}
			baseBg := appleNeon.Bg
			if cell.HasBg {
				baseBg = cell.Bg
			}
			cell.Bg = npLerpRGBA(baseBg, halo, amount)
			cell.HasBg = true
			if cell.HasFg {
				cell.Fg = npLerpRGBA(cell.Fg, halo, amount)
			}
			p.screen.setCell(x, y, cell)
		}
	}
}

func appleBezel(s *npScreen, x, y, w, h int, label string) {
	DrawBezel(s, x, y, w, h, label, BezelRack)
}

func (p *AppleMusicPage) drawSearchEditor(r image.Rectangle) {
	visible := p.hits[:0]
	for _, hit := range p.hits {
		if !image.Rect(hit.x, hit.y, hit.x+hit.w, hit.y+hit.h).Overlaps(r) {
			visible = append(visible, hit)
		}
	}
	p.hits = visible
	p.drawInput(r.Min.X, r.Min.Y, r.Dx())
}

func (p *AppleMusicPage) drawInput(x, y, w int) {
	s := &p.screen
	btnLabel := " " + T("input.btn_search") + " "
	btnW := npWidth(btnLabel)
	hasBtn := w >= btnW+10
	inputW := w
	if hasBtn {
		inputW = w - btnW - 1
	}
	value := T("input.prompt")
	if p.input.Value() != "" {
		value = "/ " + p.input.Value()
	}
	vfdBg := npLerpRGBA(s.pal.Bg, s.pal.Surface, .72)
	fg := s.pal.Secondary
	bevel := s.pal.Rule
	if p.editing {
		fg = s.pal.Accent
		bevel = s.pal.Hero
		runes := []rune(p.input.Value())
		pos := min(p.input.Position(), len(runes))
		left := string(runes[:pos])
		for npWidth(left) > max(0, inputW-5) && len(left) > 0 {
			_, _, n := npNextCluster(left)
			left = left[n:]
		}
		value = "/ " + left + "▏" + string(runes[pos:])
	}
	if p.searching {
		value = T("input.searching")
	}
	s.fillRGBA(x, y, w, 1, vfdBg)
	if inputW >= 2 {
		s.setRGBA(x, y, "▕", 1, bevel, vfdBg, true, true, false)
		s.textRGBA(x+1, y, value, fg, vfdBg, true, true, p.editing, inputW-1)
	} else {
		s.textRGBA(x, y, value, fg, vfdBg, true, true, p.editing, inputW)
	}
	p.addHit(appleHit{kind: appleHitKey, x: x, y: y, w: inputW, h: 1, key: "/"})
	if hasBtn {
		s.textRGBA(x+w-btnW, y, btnLabel, s.pal.Accent, s.pal.Surface, true, true, false, btnW)
		p.addHit(appleHit{kind: appleHitKey, x: x + w - btnW, y: y, w: btnW, h: 1, key: "search"})
	}
}

func (p *AppleMusicPage) drawMeter(x, y, w int, frame applemusic.MeterFrame) {
	label := T("meter.lbl_off")
	if p.meter != nil || frame.Ready {
		label = T("meter.lbl_dbfs")
		if !frame.Ready {
			label = T("meter.lbl_ready")
		}
	}
	if p.meterOpening {
		label = T("meter.lbl_init")
	}
	if p.meterError != "" || frame.Error != "" {
		label = T("meter.lbl_retry")
	}
	drawLEDMeterPeaks(&p.screen, x, y, w, label, frame, p.meterPeak)
	p.addHit(appleHit{kind: appleHitKey, x: x, y: y, w: w, h: 4, key: "v"})
}

func (p *AppleMusicPage) floor(x, y, w, h int) {
	drawPerspectiveGrid(&p.screen, x, y, w, h, h*2/3, 5, .9, []float64{.38, .72, 1})
}
func (p *AppleMusicPage) glitch(x, y, w, h int, now time.Time) {
	d := now.Sub(p.glitchAt)
	if p.level != motion.Full || p.glitchAt.IsZero() || d < 0 || d >= 160*time.Millisecond {
		return
	}
	phase := int(d / (40 * time.Millisecond))
	if phase%2 == 0 {
		drawGlitch(&p.screen, x, y, w, h, phase)
	}
}
