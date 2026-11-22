package ui

import (
	"image/color"
	"math"
	"time"

	"github.com/anhoder/foxful-cli/motion"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

type appleDeskStage struct {
	w, h, bucket int
	tier         deck.Tier
	valid        bool
	stop         diurnalStop
	frame        deck.Frame
}

type diurnalStop struct {
	hour                            float64
	skyTop, skyMid, skyBot          color.RGBA
	wood, blotter                   color.RGBA
	lampCore, lampHalo              color.RGBA
	lampInt, stars, sunSlats, bokeh float64
}

// 4-phase Tokyo late-night jazz kissa listening window stops.
var martinaiseStops = [4]diurnalStop{
	{6.5, color.RGBA{22, 30, 50, 255}, color.RGBA{64, 82, 112, 255}, color.RGBA{186, 138, 108, 255},
		color.RGBA{38, 27, 23, 255}, color.RGBA{25, 24, 34, 255}, color.RGBA{240, 188, 104, 255}, color.RGBA{98, 66, 36, 255}, .56, 0, .42, .12},
	{13.0, color.RGBA{32, 66, 94, 255}, color.RGBA{74, 122, 146, 255}, color.RGBA{188, 192, 182, 255},
		color.RGBA{52, 34, 24, 255}, color.RGBA{30, 32, 42, 255}, color.RGBA{244, 190, 94, 255}, color.RGBA{112, 68, 30, 255}, .55, 0, .58, .08},
	{18.0, color.RGBA{38, 18, 52, 255}, color.RGBA{158, 56, 52, 255}, color.RGBA{242, 118, 66, 255},
		color.RGBA{46, 26, 22, 255}, color.RGBA{32, 21, 30, 255}, color.RGBA{245, 185, 66, 255}, color.RGBA{148, 66, 26, 255}, .88, .22, .85, .42},
	{23.0, color.RGBA{8, 11, 22, 255}, color.RGBA{18, 20, 44, 255}, color.RGBA{38, 30, 62, 255},
		color.RGBA{24, 16, 19, 255}, color.RGBA{17, 16, 26, 255}, color.RGBA{245, 185, 66, 255}, color.RGBA{158, 86, 26, 255}, 1.0, 1.0, 0, 1.0},
}

func appleStageHour(now time.Time) float64 {
	if now.Year() < 2000 {
		return 18.0 // synthetic unit-test timestamps default to golden hour
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
		bokeh:    a.bokeh + (b.bokeh-a.bokeh)*s,
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

// drawMartinaiseStage renders the all-tier 1982 Tokyo listening window, walnut
// bar counter, brass desk lamp cone, Shibuya skyline + neon signage, and dynamic
// Walkman contact shadow.
func (p *AppleMusicPage) drawMartinaiseStage(x, y, w, h int, now time.Time) {
	if w <= 0 || h <= 0 {
		return
	}
	tier := p.tier
	if tier == 0 || tier == deck.Braille {
		tier = deck.Quadrant
	}
	gw, gh := tier.Grid()
	hour := appleStageHour(now)
	bucket := int(hour * 60)
	horizonSubY := max(2, int(float64(h*2)*.40))
	if !p.desk.valid || p.desk.w != w || p.desk.h != h || p.desk.bucket != bucket || p.desk.tier != tier {
		p.desk.valid, p.desk.w, p.desk.h, p.desk.bucket, p.desk.tier = true, w, h, bucket, tier
		p.desk.stop = sampleDiurnalStop(hour)
		n := w * h
		if cap(p.desk.frame.Cells) < n {
			p.desk.frame.Cells = make([]deck.Cell, n)
		}
		p.desk.frame.W, p.desk.frame.H, p.desk.frame.Cells = w, h, p.desk.frame.Cells[:n]
		var px [8]color.RGBA
		for cy := range h {
			for cx := range w {
				for q := range gw * gh {
					u := (float64(cx) + (float64(q%gw)+.5)/float64(gw)) * 2.0
					v := (float64(cy) + (float64(q/gw)+.5)/float64(gh)) * 2.0
					px[q] = sampleMartinaiseSubpixel(u, v, float64(w), float64(h), float64(horizonSubY), &p.desk.stop)
				}
				p.desk.frame.Cells[cy*w+cx] = deck.FitCellRGBA(px[:gw*gh], tier)
			}
		}
	}
	p.screen.drawDeck(&p.desk.frame, x, y)
	p.applyStageContactShadow(x, y, w, h, horizonSubY/2, now)
	p.drawMartinaiseDetails(x, y, w, h, horizonSubY/2, &p.desk.stop, now)
}

func shibuyaRoofHeight(nx float64) (roofFrac float64, shade float64) {
	switch {
	case nx < .14:
		return .44, .18
	case nx < .26:
		return .28, .28
	case nx < .36:
		return .52, .12
	case nx < .50:
		return .34, .24
	case nx < .62:
		return .64, .10
	case nx < .72:
		return .30, .26
	case nx < .84:
		return .46, .16
	default:
		return .32, .22
	}
}

func sampleMartinaiseSubpixel(u, v, sw, sh, horizonSubY float64, th *diurnalStop) color.RGBA {
	fullW := max(1.0, sw*2.0)
	nx := min(max(u/fullW, 0), 1)
	var col color.RGBA

	switch {
	case v < 1.1 || u < 1.4 || u > fullW-1.4:
		// Dark walnut cafe window outer perimeter frame.
		col = color.RGBA{26, 20, 24, 255}
		if v >= 0.8 && v < 1.1 {
			col = color.RGBA{56, 44, 38, 255}
		}
	case math.Abs(u-sw*0.66) <= 1.0 || math.Abs(u-sw*1.36) <= 1.0:
		// Crisp architectural vertical mullions with edge highlight.
		col = color.RGBA{22, 17, 22, 255}
		if math.Abs(u-sw*0.66) > .65 || math.Abs(u-sw*1.36) > .65 {
			col = color.RGBA{64, 50, 44, 255}
		}
	case v < horizonSubY-1.0:
		skyT := v / max(1, horizonSubY-1.0)
		if skyT < .55 {
			col = deskMixRGBA(th.skyTop, th.skyMid, skyT/.55)
		} else {
			col = deskMixRGBA(th.skyMid, th.skyBot, (skyT-.55)/.45)
		}
		// Distant radio tower spire & red aviation beacon in right-center pane.
		if math.Abs(nx-.77)*fullW < 0.7 && v > horizonSubY*.18 && v < horizonSubY-1.0 {
			col = deskMixRGBA(col, color.RGBA{22, 20, 32, 255}, .82)
			if v < horizonSubY*.26 {
				col = color.RGBA{255, 107, 74, 255}
			}
		}
		if th.bokeh > .05 {
			bokehs := [4]struct {
				fx, fy, rad float64
				col         color.RGBA
			}{
				{.42, .45, 3.0, color.RGBA{46, 216, 198, 255}},
				{.56, .38, 3.8, color.RGBA{255, 107, 74, 255}},
				{.88, .48, 3.2, color.RGBA{245, 185, 66, 255}},
				{.20, .54, 2.6, color.RGBA{255, 107, 74, 255}},
			}
			for _, bk := range bokehs {
				du, dv := u-sw*2*bk.fx, v-horizonSubY*bk.fy
				if d2 := du*du + dv*dv*2.0; d2 < bk.rad*bk.rad {
					w := math.Pow(1-d2/(bk.rad*bk.rad), 1.4) * th.bokeh * .45
					col = deskMixRGBA(col, bk.col, w)
				}
			}
		}
		// Multi-building Shibuya skyline with lit interior grid windows and neon signboards.
		roofFrac, facadeTint := shibuyaRoofHeight(nx)
		roofY := horizonSubY * (1.0 - roofFrac)
		if v >= roofY {
			bldgDark := deskMixRGBA(color.RGBA{16, 16, 28, 255}, th.skyMid, .18+facadeTint*.5)
			if th.sunSlats > .2 && math.Mod(u, 11) < 2.2 {
				bldgDark = deskMixRGBA(bldgDark, th.skyBot, .22*th.sunSlats)
			}
			col = bldgDark
			// Window floor grid inside buildings.
			if int(v*1.4)%2 == 0 && int(u*0.8)%2 == 0 {
				lit := 0.25 + 0.65*th.bokeh
				h := int(u*3.1 + v*7.3)
				if h%5 == 0 {
					col = deskMixRGBA(col, color.RGBA{245, 185, 66, 255}, .55*lit)
				} else if h%7 == 0 {
					col = deskMixRGBA(col, color.RGBA{46, 216, 198, 255}, .45*lit)
				} else {
					col = deskMixRGBA(col, color.RGBA{124, 136, 164, 255}, .20)
				}
			}
			// Vertical neon signboards along Shibuya streetscape (matte painted sign in day, neon at night).
			signBoost := 0.35 + 0.65*th.bokeh
			switch {
			case nx >= .16 && nx <= .185 && v > roofY+1.0 && v < horizonSubY-2.0:
				col = deskMixRGBA(col, color.RGBA{255, 107, 74, 255}, .85*signBoost)
			case nx >= .67 && nx <= .695 && v > roofY+1.0 && v < horizonSubY-1.8:
				col = deskMixRGBA(col, color.RGBA{46, 216, 198, 255}, .85*signBoost)
			case nx >= .88 && nx <= .905 && v > roofY+1.2 && v < horizonSubY-2.0:
				col = deskMixRGBA(col, color.RGBA{245, 185, 66, 255}, .85*signBoost)
			}
		}
		// Subtle horizontal frosted glass blind louvers across upper pane.
		if int(v)%3 == 0 && v < horizonSubY*0.42 {
			col = deskMixRGBA(col, color.RGBA{16, 16, 26, 255}, .28)
		}
	case v < horizonSubY+0.9:
		// Wooden cafe window sill ledge + miniature LP record sleeve stack on far-right sill.
		col = deskMixRGBA(th.wood, color.RGBA{96, 68, 48, 255}, .42)
		if v > horizonSubY {
			col = color.RGBA{24, 18, 22, 255}
		}
	default:
		deskT := (v - horizonSubY) / max(1, sh*2-horizonSubY)
		grain := .5 + .5*math.Sin(v*1.7+math.Sin(u*.18)*2)
		col = deskMixRGBA(th.wood, color.RGBA{76, 48, 34, 255}, grain*.30)
		// Stitched dark listening blotter mat covering center counter.
		if deskT > .12 && deskT < .92 && nx > .09 && nx < .91 {
			col = deskMixRGBA(th.blotter, color.RGBA{14, 13, 22, 255}, (1-deskT)*.22)
			if deskT < .16 || nx < .105 || nx > .895 {
				col = deskMixRGBA(col, color.RGBA{98, 82, 70, 255}, .35)
			}
		}
		if th.sunSlats > .05 {
			if slat := math.Sin(u*.35 - v*.65); slat > .30 && u > sw*.32 {
				col = deskMixRGBA(col, th.skyBot, th.sunSlats*.24*(slat-.30))
			}
		}
	}
	// Back-right sill stacked LP crate detail resting against window trim.
	if nx >= .82 && nx <= .93 && v >= horizonSubY-3.2 && v <= horizonSubY+0.6 {
		slot := int((nx - .82) * 90)
		switch slot % 3 {
		case 0:
			col = deskMixRGBA(color.RGBA{38, 92, 168, 255}, th.blotter, .35)
		case 1:
			col = deskMixRGBA(color.RGBA{222, 68, 56, 255}, th.blotter, .35)
		default:
			col = deskMixRGBA(color.RGBA{235, 156, 46, 255}, th.blotter, .40)
		}
	}
	return applyBrassLamp(col, u, v, sw, sh, horizonSubY, th)
}

func applyBrassLamp(col color.RGBA, u, v, sw, sh, horizonSubY float64, th *diurnalStop) color.RGBA {
	dx, dy := u-sw*.40, v-sh*.28
	along := dx*.52 + dy*.85
	perp := -dx*.85 + dy*.52
	if along > 0 {
		spread := 5.2 + along*.72
		radial := math.Abs(perp) / spread
		axial := max(0, 1-along/(sh*2.4))
		if radial < 1.35 {
			beam := math.Pow(max(0, 1-radial/1.35), 1.7) * axial * th.lampInt
			tint := th.lampHalo
			if radial < .48 {
				tint = deskMixRGBA(th.lampHalo, th.lampCore, (.48-radial)/.48)
			}
			weight := .62
			if v < horizonSubY {
				weight = .24
			}
			col = deskMixRGBA(col, tint, beam*weight)
		}
	}
	// Vintage green-glass and brass banker's lamp hood in the upper-left corner.
	if u >= 4 && u <= 22 && v >= 2 && v <= 7 && math.Abs(u-13)-(v-1)*1.4 <= 3 {
		if v >= 6.5 {
			col = th.lampCore
		} else {
			col = deskMixRGBA(color.RGBA{34, 72, 54, 255}, color.RGBA{228, 174, 74, 255}, (v/7)*.55)
		}
	}
	return col
}

// applyStageContactShadow tracks the Walkman pose and zoom in real time over the
// static desk stage without busting the sky/counter glyph cache.
func (p *AppleMusicPage) applyStageContactShadow(x, y, w, h, sillRow int, now time.Time) {
	if w < 10 || h < 6 {
		return
	}
	z := p.camera.value(now)
	pose := p.walkmanPose(now, z)
	cx := float64(x) + float64(w)*.52 + pose.X*.06 - pose.Yaw*4.0
	cy := float64(y) + float64(h)*.58 + pose.Y*.10 + z*float64(h)*.08
	rx := max(6.0, float64(w)*(.27-.04*z))
	ry := max(2.2, float64(h)*(.15-.02*z))
	yMin := max(y+max(0, sillRow-1), int(math.Floor(cy-ry)))
	yMax := min(y+h, int(math.Ceil(cy+ry))+1)
	xMin := max(x, int(math.Floor(cx-rx)))
	xMax := min(x+w, int(math.Ceil(cx+rx))+1)
	for py := yMin; py < yMax; py++ {
		dy := (float64(py) - cy) / ry
		dy2 := dy * dy
		if dy2 >= 1 {
			continue
		}
		for px := xMin; px < xMax; px++ {
			dx := (float64(px) - cx) / rx
			dist := dx*dx + dy2
			if dist < 1 {
				k := math.Pow(1-dist, 1.5) * .68
				p.screen.shade(px, py, k)
			}
		}
	}
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
			if c, ok := p.screen.cellAt(sx, sy); ok && c.HasBg {
				bg = c.Bg
			}
			fg := deskMixRGBA(bg, color.RGBA{244, 232, 206, 255}, th.stars*twinkle)
			p.screen.setRGBA(sx, sy, st.glyph, 1, fg, bg, true, true, false)
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
			if c, ok := p.screen.cellAt(mx, my); ok && c.HasBg {
				p.screen.setRGBA(mx, my, "·", 1, npLerpRGBA(c.Bg, th.lampCore, .62*th.lampInt), c.Bg, true, true, false)
			}
		}
	}
}
