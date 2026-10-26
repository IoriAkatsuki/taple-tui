package ui

import (
	"image/color"
	"math"
	"time"

	"github.com/anhoder/foxful-cli/motion"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

type appleDeskStage struct {
	w, h, bucket int
	valid        bool
	stop         diurnalStop
	frame        deck.Frame
}

type diurnalStop struct {
	hour                     float64
	skyTop, skyMid, skyBot   color.RGBA
	wood, blotter            color.RGBA
	lampCore, lampHalo       color.RGBA
	lampInt, stars, sunSlats float64
}

var martinaiseStops = [4]diurnalStop{
	{6.5, color.RGBA{18, 30, 46, 255}, color.RGBA{46, 70, 92, 255}, color.RGBA{138, 104, 76, 255},
		color.RGBA{28, 24, 34, 255}, color.RGBA{22, 28, 36, 255}, color.RGBA{228, 178, 96, 255}, color.RGBA{86, 60, 32, 255}, .55, 0, .42},
	{13.0, color.RGBA{32, 50, 68, 255}, color.RGBA{62, 86, 108, 255}, color.RGBA{128, 146, 162, 255},
		color.RGBA{38, 28, 24, 255}, color.RGBA{26, 34, 38, 255}, color.RGBA{236, 176, 84, 255}, color.RGBA{96, 58, 26, 255}, .54, 0, .52},
	{18.0, color.RGBA{38, 16, 52, 255}, color.RGBA{128, 42, 44, 255}, color.RGBA{212, 86, 34, 255},
		color.RGBA{42, 20, 22, 255}, color.RGBA{30, 18, 26, 255}, color.RGBA{252, 174, 32, 255}, color.RGBA{134, 58, 18, 255}, .84, .15, .78},
	{23.0, color.RGBA{5, 7, 14, 255}, color.RGBA{11, 20, 36, 255}, color.RGBA{22, 36, 56, 255},
		color.RGBA{16, 11, 18, 255}, color.RGBA{14, 12, 20, 255}, color.RGBA{255, 198, 72, 255}, color.RGBA{148, 72, 14, 255}, 1.0, 1.0, 0},
}

var deskQuadrantGlyphs = [16]string{" ", "▘", "▝", "▀", "▖", "▌", "▞", "▛", "▗", "▚", "▐", "▜", "▄", "▙", "▟", "█"}

func appleStageHour(now time.Time) float64 {
	if now.Year() < 2000 {
		return 18.0 // synthetic unit-test timestamps default to Martinaise sunset
	}
	local := now.Local()
	return float64(local.Hour()) + float64(local.Minute())/60.0
}

func sampleDiurnalStop(hour float64) diurnalStop {
	h := math.Mod(math.Mod(hour, 24)+24, 24)
	ring := [6]diurnalStop{
		martinaiseStops[3], martinaiseStops[0], martinaiseStops[1],
		martinaiseStops[2], martinaiseStops[3], martinaiseStops[0],
	}
	ring[0].hour, ring[5].hour = -1.0, 30.5
	a, b := ring[0], ring[1]
	for i := range 5 {
		if h >= ring[i].hour && h <= ring[i+1].hour {
			a, b = ring[i], ring[i+1]
			break
		}
	}
	t := (h - a.hour) / max(b.hour-a.hour, 1e-3)
	s := t * t * (3 - 2*t)
	return diurnalStop{
		hour:     h,
		skyTop:   deskMixRGBA(a.skyTop, b.skyTop, s),
		skyMid:   deskMixRGBA(a.skyMid, b.skyMid, s),
		skyBot:   deskMixRGBA(a.skyBot, b.skyBot, s),
		wood:     deskMixRGBA(a.wood, b.wood, s),
		blotter:  deskMixRGBA(a.blotter, b.blotter, s),
		lampCore: deskMixRGBA(a.lampCore, b.lampCore, s),
		lampHalo: deskMixRGBA(a.lampHalo, b.lampHalo, s),
		lampInt:  a.lampInt + (b.lampInt-a.lampInt)*s,
		stars:    a.stars + (b.stars-a.stars)*s,
		sunSlats: a.sunSlats + (b.sunSlats-a.sunSlats)*s,
	}
}

func deskMixRGBA(a, b color.RGBA, t float64) color.RGBA {
	t = min(max(t, 0), 1)
	return color.RGBA{
		R: uint8(math.Round(float64(a.R)*(1-t) + float64(b.R)*t)),
		G: uint8(math.Round(float64(a.G)*(1-t) + float64(b.G)*t)),
		B: uint8(math.Round(float64(a.B)*(1-t) + float64(b.B)*t)),
		A: 0xff,
	}
}

// drawMartinaiseStage renders the Disco Elysium inspired diurnal window, brass
// desk lamp cone, worn leather blotter, and Walkman contact shadow.
func (p *AppleMusicPage) drawMartinaiseStage(x, y, w, h int, now time.Time) {
	if w <= 0 || h <= 0 {
		return
	}
	hour := appleStageHour(now)
	bucket := int(hour * 60)
	horizonSubY := max(2, int(float64(h*2)*.36))
	if !p.desk.valid || p.desk.w != w || p.desk.h != h || p.desk.bucket != bucket {
		p.desk.valid, p.desk.w, p.desk.h, p.desk.bucket = true, w, h, bucket
		p.desk.stop = sampleDiurnalStop(hour)
		n := w * h
		if cap(p.desk.frame.Cells) < n {
			p.desk.frame.Cells = make([]deck.Cell, n)
		}
		p.desk.frame.W, p.desk.frame.H, p.desk.frame.Cells = w, h, p.desk.frame.Cells[:n]
		for cy := range h {
			for cx := range w {
				var px [4]color.RGBA
				for q := range 4 {
					px[q] = sampleMartinaiseSubpixel(float64(cx*2+q%2), float64(cy*2+q/2), float64(w), float64(h), float64(horizonSubY), &p.desk.stop)
				}
				ch, fg, bg := fitDeskQuadrant(px)
				p.desk.frame.Cells[cy*w+cx] = deck.Cell{Ch: ch, Fg: fg, Bg: bg}
			}
		}
	}
	p.screen.drawDeck(&p.desk.frame, x, y)
	p.drawMartinaiseDetails(x, y, w, h, horizonSubY/2, &p.desk.stop, now)
}

func sampleMartinaiseSubpixel(u, v, sw, sh, horizonSubY float64, th *diurnalStop) color.RGBA {
	var col color.RGBA
	if v < horizonSubY {
		skyT := v / max(1, horizonSubY-1)
		if skyT < .55 {
			col = deskMixRGBA(th.skyTop, th.skyMid, skyT/.55)
		} else {
			col = deskMixRGBA(th.skyMid, th.skyBot, (skyT-.55)/.45)
		}
		wave := math.Sin(u*.14)*1.2 + math.Cos(u*.31)*.8
		if v > horizonSubY-2.2+wave && (u < sw*.68 || u > sw*1.45) {
			col = deskMixRGBA(col, color.RGBA{10, 12, 20, 255}, .65)
		}
		if int(v)%3 == 0 {
			col = deskMixRGBA(col, color.RGBA{12, 10, 18, 255}, .42)
		}
		if math.Abs(u-sw*1.32) <= 1 {
			col = color.RGBA{20, 16, 26, 255}
		}
	} else {
		deskT := (v - horizonSubY) / max(1, sh*2-horizonSubY)
		grain := .5 + .5*math.Sin(v*1.7+math.Sin(u*.18)*2)
		col = deskMixRGBA(th.wood, color.RGBA{62, 38, 28, 255}, grain*.22)
		if deskT > .14 && deskT < .94 && u > sw*.12 && u < sw*1.88 {
			col = deskMixRGBA(th.blotter, color.RGBA{12, 10, 18, 255}, (1-deskT)*.2)
		}
		if th.sunSlats > .05 {
			if slat := math.Sin(u*.35 - v*.65); slat > .35 && u > sw*.38 {
				col = deskMixRGBA(col, th.skyBot, th.sunSlats*.22*(slat-.35))
			}
		}
	}
	col = applyBrassLampAndShadow(col, u, v, sw, sh, horizonSubY, th)
	return col
}

func applyBrassLampAndShadow(col color.RGBA, u, v, sw, sh, horizonSubY float64, th *diurnalStop) color.RGBA {
	dx, dy := u-sw*.40, v-sh*.28
	along := dx*.52 + dy*.85
	perp := -dx*.85 + dy*.52
	if along > 0 {
		spread := 4.8 + along*.70
		radial := math.Abs(perp) / spread
		axial := max(0, 1-along/(sh*2.3))
		if radial < 1.35 {
			beam := math.Pow(max(0, 1-radial/1.35), 1.8) * axial * th.lampInt
			tint := th.lampHalo
			if radial < .45 {
				tint = deskMixRGBA(th.lampHalo, th.lampCore, (.45-radial)/.45)
			}
			weight := .56
			if v < horizonSubY {
				weight = .26
			}
			col = deskMixRGBA(col, tint, beam*weight)
		}
	}
	// Vintage green-glass and brass banker's lamp hood in the upper-left corner.
	if u >= 4 && u <= 22 && v >= 2 && v <= 7 && math.Abs(u-13)-(v-1)*1.4 <= 3 {
		if v >= 6.5 {
			col = th.lampCore
		} else {
			col = deskMixRGBA(color.RGBA{42, 74, 56, 255}, color.RGBA{204, 156, 68, 255}, (v/7)*.5)
		}
	}
	// Grounding contact ambient-occlusion shadow directly beneath the Walkman housing.
	wdx := (u - sw*1.04) / max(1, sw*.54)
	wdy := (v - sh*1.12) / max(1, sh*.32)
	if dist := wdx*wdx + wdy*wdy; dist < 1.0 && v >= horizonSubY-2 {
		col = deskMixRGBA(col, color.RGBA{4, 2, 7, 255}, math.Pow(1-dist, 1.5)*.82)
	}
	return col
}

func (p *AppleMusicPage) drawMartinaiseDetails(x, y, w, h, sillRow int, th *diurnalStop, now time.Time) {
	if th.stars > .15 && sillRow > 1 {
		stars := [4]struct {
			fx, fy float64
			glyph  string
		}{{.48, .10, "✦"}, {.64, .20, "·"}, {.78, .12, "✧"}, {.88, .24, "⋆"}}
		for i, st := range stars {
			sx, sy := x+int(float64(w)*st.fx), y+int(float64(h)*st.fy)
			if sx <= x || sx >= x+w-1 || sy <= y || sy >= y+sillRow {
				continue
			}
			twinkle := .75
			if p.level != motion.Off {
				twinkle = .55 + .45*math.Sin(float64(now.UnixMilli())/1800+float64(i)*1.7)
			}
			bg := appleNeon.Bg
			if c := p.screen.buf.CellAt(sx, sy); c != nil && c.Style.Bg != nil {
				r, g, b, _ := c.Style.Bg.RGBA()
				bg = color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 255}
			}
			fg := deskMixRGBA(bg, color.RGBA{196, 228, 255, 255}, th.stars*twinkle)
			p.screen.set(sx, sy, st.glyph, 1, uv.Style{Fg: fg, Bg: bg})
		}
	}
	if p.level == motion.Off || th.lampInt <= .3 || w < 20 || h < 8 {
		return
	}
	// Ultra-slow drifting dust motes inside the warm brass lamp beam (never audio-driven).
	for i, pt := range [2][2]float64{{.22, .38}, {.31, .52}} {
		mx := x + int(float64(w)*pt[0]+math.Sin(float64(now.UnixMilli())/6000+float64(i)*2.1)*1.4)
		my := y + int(float64(h)*pt[1]+math.Cos(float64(now.UnixMilli())/7500+float64(i)*1.5)*.8)
		if mx > x+1 && mx < x+w-2 && my > y+1 && my < y+h-2 {
			if c := p.screen.buf.CellAt(mx, my); c != nil && c.Style.Bg != nil {
				p.screen.set(mx, my, "·", 1, uv.Style{Fg: npLerp(c.Style.Bg, th.lampCore, .62*th.lampInt), Bg: c.Style.Bg})
			}
		}
	}
}

func fitDeskQuadrant(px [4]color.RGBA) (string, color.RGBA, color.RGBA) {
	var mr, mg, mb float64
	for _, c := range px {
		mr += float64(c.R)
		mg += float64(c.G)
		mb += float64(c.B)
	}
	mean := color.RGBA{uint8(mr / 4), uint8(mg / 4), uint8(mb / 4), 255}
	a, b := px[0], px[0]
	for _, c := range px {
		if deskColorDist2(c, mean) > deskColorDist2(a, mean) {
			a = c
		}
	}
	for _, c := range px {
		if deskColorDist2(c, a) > deskColorDist2(b, a) {
			b = c
		}
	}
	if deskColorDist2(a, b) < 36 {
		return " ", mean, mean
	}
	mask := uint8(0)
	for i, c := range px {
		if deskColorDist2(c, b) < deskColorDist2(c, a) {
			mask |= 1 << i
		}
	}
	if mask == 0 || mask == 15 {
		return " ", mean, mean
	}
	return deskQuadrantGlyphs[mask], b, a
}

func deskColorDist2(a, b color.RGBA) float64 {
	dr := float64(a.R) - float64(b.R)
	dg := float64(a.G) - float64(b.G)
	db := float64(a.B) - float64(b.B)
	return dr*dr + dg*dg + db*db
}
