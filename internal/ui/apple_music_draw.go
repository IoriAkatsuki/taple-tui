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
	Bg: color.RGBA{0x0a, 0x06, 0x12, 0xff}, Surface: color.RGBA{0x14, 0x0b, 0x22, 0xff},
	Text: color.RGBA{0xd9, 0xd2, 0xec, 0xff}, Hero: color.RGBA{0xff, 0x2b, 0xd6, 0xff},
	Secondary: color.RGBA{0x8e, 0x83, 0xb0, 0xff}, Rule: color.RGBA{0x4b, 0x35, 0x70, 0xff},
	Ghost: color.RGBA{0x24, 0x19, 0x38, 0xff}, Accent: color.RGBA{0x00, 0xe5, 0xff, 0xff},
	Accent2: color.RGBA{0xff, 0xb0, 0x00, 0xff},
}

func appleBox(x, y, w, h int) image.Rectangle { return image.Rect(x, y, x+max(w, 0), y+max(h, 0)) }

func (p *AppleMusicPage) drawSwissSection(r image.Rectangle, num, title, meta string) {
	if r.Dx() < 4 || r.Dy() < 1 {
		return
	}
	s, pal := &p.screen, appleNeon
	x, y, w := r.Min.X, r.Min.Y, r.Dx()
	n := s.text(x, y, num, uv.Style{Fg: pal.Accent, Attrs: uv.AttrBold}, w)
	n += s.text(x+n, y, " ── ", uv.Style{Fg: pal.Rule}, max(0, w-n))
	n += s.text(x+n, y, title, uv.Style{Fg: pal.Text, Attrs: uv.AttrBold}, max(0, w-n))
	if meta != "" {
		mw := npWidth(meta)
		if x+n+2+mw <= x+w {
			s.text(x+w-mw, y, meta, uv.Style{Fg: pal.Secondary}, mw)
		}
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
			// Keep the editor above the returning device without changing its projection.
			p.drawSearchEditor(appleBox(left.Min.X+1, left.Min.Y+1, left.Dx()-2, 1))
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
	s.fill(0, 0, w, 1, pal.Surface)
	x := 1
	if w >= 64 {
		x += s.text(x, 0, "■", uv.Style{Fg: pal.Accent, Bg: pal.Surface}, w-x)
		x += s.text(x, 0, " T A P L E  ", uv.Style{Fg: pal.Text, Bg: pal.Surface, Attrs: uv.AttrBold}, max(0, w-x-26))
		s.text(x, 0, "Apple Music / Cassette Deck", uv.Style{Fg: pal.Secondary, Bg: pal.Surface}, max(0, w-x-26))
	} else {
		s.text(1, 0, "Apple Music / Cassette Deck", uv.Style{Fg: pal.Text, Bg: pal.Surface}, max(0, w-28))
	}
	if w >= 98 {
		lx := w - 42
		playLit := p.ready() && p.state.Playing
		vuLit := p.meter != nil && p.meter.Frame().Ready
		lrcLit := len(p.lyrics.data.Lines) > 0
		for _, lamp := range []struct {
			label string
			lit   bool
			col   color.Color
		}{{"PLAY", playLit, pal.Hero}, {"VU", vuLit, pal.Accent}, {"LRC", lrcLit, pal.Accent2}} {
			fg := color.Color(pal.Ghost)
			if lamp.lit {
				fg = lamp.col
			}
			lx += s.text(lx, 0, lamp.label, uv.Style{Fg: fg, Bg: pal.Surface, Attrs: uv.AttrBold}, max(0, w-lx)) + 2
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
	s.text(max(0, w-23), 0, account, uv.Style{Fg: pal.Accent, Bg: pal.Surface}, min(w, 22))
	p.addHit(appleHit{kind: appleHitKey, x: max(0, w-23), y: 0, w: min(w, 22), h: 1, key: "b"})
	if h >= 10 {
		p.drawNavigation(w)
		p.drawNavigationContext(w)
	}
}

func (p *AppleMusicPage) drawNavigation(w int) {
	scene, target := T("scene.browse"), T("nav.listen")
	if p.scene == appleListen {
		scene, target = T("scene.listen"), T("nav.browse")
	}
	x := 2 + p.screen.text(2, 1, scene, uv.Style{Fg: appleNeon.Hero, Attrs: uv.AttrBold}, max(0, w-4)) + 2
	p.drawKeys(x, 1, w-x-2, []struct{ key, label string }{{"i", target}})
	x += npWidth(target) + 2
	x += p.screen.text(x, 1, "│ ", uv.Style{Fg: appleNeon.Rule}, max(0, w-x-2))
	p.drawKeys(x, 1, w-x-2, []struct{ key, label string }{
		{"1", T("nav.search")}, {"2", T("nav.library")}, {"3", T("nav.queue")}, {"s", T("nav.settings")},
	})
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
	available := max(0, w-4)
	if backW > 0 {
		available = max(0, available-backW-2)
	}
	p.screen.text(2, 2, label, uv.Style{Fg: appleNeon.Secondary}, available)
	if backW > 0 {
		x := w - 2 - backW
		p.screen.text(x, 2, back, uv.Style{Fg: appleNeon.Accent}, backW)
		p.addHit(appleHit{kind: appleHitKey, key: "esc", x: x, y: 2, w: backW, h: 1})
	}
}

func (p *AppleMusicPage) drawBrowse(r image.Rectangle, now time.Time) {
	if p.screen.w >= 100 || r.Dy() < 8 {
		p.drawDrawer(r, now)
		return
	}
	s := &p.screen
	p.drawSwissSection(appleBox(r.Min.X+1, r.Min.Y, r.Dx()-2, 1), "00", "N O W   P L A Y I N G", "i")
	name := p.state.Track.Name
	if name == "" {
		name = T("status.idle")
	}
	flag := "‖ "
	if p.ready() && p.state.Playing {
		flag = "▶ "
	}
	s.text(r.Min.X+1, r.Min.Y+1, flag+name, uv.Style{Fg: s.pal.Hero, Attrs: uv.AttrBold}, r.Dx()-2)
	s.text(r.Min.X+1, r.Min.Y+2, p.state.Track.Artist, uv.Style{Fg: s.pal.Secondary}, r.Dx()-2)
	s.repeat(r.Min.X+1, r.Min.Y+3, max(0, r.Dx()-2), "─", uv.Style{Fg: s.pal.Ghost})
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
			message := T("status.sign_in_hint")
			if !p.state.Ready {
				message = T("status.connecting")
			}
			p.screen.text(x+1, y, message, uv.Style{Fg: appleNeon.Accent2}, w-2)
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
	fg := npLerp(pal.Bg, pal.Hero, stage(0, .55))
	name := p.state.Track.Name
	if name == "" {
		name = T("status.idle")
	}
	s.text(x, y, name, uv.Style{Fg: fg, Attrs: uv.AttrBold}, w)
	if u := stage(.15, .55); u > 0 && y+1 < r.Max.Y-1 {
		s.text(x, y+1, p.state.Track.Artist, uv.Style{Fg: npLerp(pal.Bg, pal.Secondary, u)}, w)
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
		p.screen.text(stage.Min.X+1, y, "TIME COUNTER", uv.Style{Fg: appleNeon.Secondary}, 22)
		counterTime, counterState := now, &p.counter
		if p.level == motion.Off {
			counterTime, counterState = time.Unix(0, 0), nil
		}
		DrawNixieCounter(&p.screen, stage.Min.X+1, y+1, appleDuration(p.state.Position), p.ready(), counterTime, counterState)
		p.drawMeter(stage.Min.X+25, y, stage.Dx()-26, p.meter.Frame())
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
	if h < 3 {
		if h > 1 {
			p.drawKeys(0, h-1, w, []struct{ key, label string }{{"space", "▶/‖"}, {"b", "b 登录"}, {"q", "q"}})
		}
		return
	}
	clock := "--:--"
	if p.ready() {
		clock = npClock(appleDuration(p.state.Position))
	}
	if h >= 7 {
		s.text(2, h-5, clock+" / "+npClock(appleDuration(p.state.Track.Duration)), uv.Style{Fg: appleNeon.Accent2}, max(0, w-4))
		if p.pending {
			s.text(24, h-5, T("status.loading"), uv.Style{Fg: appleNeon.Accent2}, w-26)
		} else if p.playbackError != "" {
			s.text(24, h-5, T("status.play_failed"), uv.Style{Fg: appleNeon.Accent2}, w-26)
		}
		x, y, width := 2, h-4, w-4
		s.repeat(x, y, width, "─", uv.Style{Fg: appleNeon.Rule})
		if p.ready() {
			head := int(p.progress() * float64(max(0, width-1)))
			s.repeat(x, y, head, "━", uv.Style{Fg: appleNeon.Accent})
			if width > 0 {
				s.set(x+min(head, width-1), y, "●", 1, uv.Style{Fg: appleNeon.Hero})
			}
			p.addHit(appleHit{kind: appleHitSeek, x: x, y: y, w: width, h: 1})
		}
	}
	p.drawTransport(1, h-3, w-2)
	if h >= 4 {
		back := p.navigationBackLabel()
		backW := min(npWidth(back), max(0, w-2)/2)
		keysW := w - 2
		if backW > 0 {
			keysW = max(0, keysW-backW-2)
			p.screen.text(w-1-backW, h-2, back, uv.Style{Fg: appleNeon.Accent}, backW)
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
	}
	status := p.status
	if p.playbackError != "" {
		status = p.playbackError
	}
	s.text(1, h-1, status, uv.Style{Fg: appleNeon.Secondary}, w-2)
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
			p.screen.repeat(x+8, y, width, "─", uv.Style{Fg: appleNeon.Rule})
			pos := int(min(max(p.state.Volume, 0), 1) * float64(width-1))
			p.screen.set(x+8+pos, y, "●", 1, uv.Style{Fg: appleNeon.Accent})
			p.addHit(appleHit{kind: appleHitVolume, x: x + 8, y: y, w: width, h: 1})
		}
	}
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
	occupied := func(x, y int) bool { return frame.Cells[(y-offsetY)*frame.W+x-offsetX].Ch != "" }
	width := stage.Dx()
	glow := make([]float64, width*stage.Dy())
	for y := stage.Min.Y; y < stage.Max.Y; y++ {
		for x := stage.Min.X; x < stage.Max.X; x++ {
			if !occupied(x, y) {
				continue
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -4; dx <= 4; dx++ {
					xx, yy := x+dx, y+dy
					distance := float64(dx*dx) + float64(dy*dy)*aspect*aspect
					if distance > 16 || xx < stage.Min.X || xx >= stage.Max.X || yy < stage.Min.Y || yy >= stage.Max.Y {
						continue
					}
					index := (yy-stage.Min.Y)*width + xx - stage.Min.X
					glow[index] = max(glow[index], math.Exp(-distance/6))
				}
			}
		}
	}
	for y := stage.Min.Y; y < stage.Max.Y; y++ {
		for x := stage.Min.X; x < stage.Max.X; x++ {
			amount := glow[(y-stage.Min.Y)*width+x-stage.Min.X] * strength
			if amount == 0 || occupied(x, y) {
				continue
			}
			cell := uv.Cell{Content: " ", Width: 1}
			if existing := p.screen.buf.CellAt(x, y); existing != nil {
				cell = *existing
			}
			baseBg := color.Color(appleNeon.Bg)
			if cell.Style.Bg != nil {
				baseBg = cell.Style.Bg
			}
			cell.Style.Bg = npLerp(baseBg, appleNeon.Accent, amount)
			if cell.Style.Fg != nil {
				cell.Style.Fg = npLerp(cell.Style.Fg, appleNeon.Accent, amount)
			}
			p.screen.buf.SetCell(x, y, &cell)
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
	inputW := w
	if w >= 16 {
		inputW = w - 7
	}
	value := T("input.prompt")
	if p.input.Value() != "" {
		value = "/ " + p.input.Value()
	}
	st := uv.Style{Fg: s.pal.Secondary, Bg: s.pal.Surface}
	if p.editing {
		st.Fg = s.pal.Accent
		runes := []rune(p.input.Value())
		pos := min(p.input.Position(), len(runes))
		left := string(runes[:pos])
		for npWidth(left) > max(0, inputW-4) && len(left) > 0 {
			_, _, n := npNextCluster(left)
			left = left[n:]
		}
		value = "/ " + left + "▏" + string(runes[pos:])
	}
	if p.searching {
		value = T("input.searching")
	}
	s.fill(x, y, w, 1, s.pal.Surface)
	s.text(x, y, value, st, inputW)
	p.addHit(appleHit{kind: appleHitKey, x: x, y: y, w: inputW, h: 1, key: "/"})
	if w >= 16 {
		s.text(x+w-6, y, " "+T("input.btn_search")+" ", uv.Style{Fg: s.pal.Accent, Bg: s.pal.Surface}, 6)
		p.addHit(appleHit{kind: appleHitKey, x: x + w - 6, y: y, w: 6, h: 1, key: "search"})
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
	drawLEDMeter(&p.screen, x, y, w, label, frame)
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
