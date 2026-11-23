package ui

import (
	"image/color"
	"math"
	"time"

	"github.com/anhoder/foxful-cli/motion"
)

type appleEmptyMood uint8

const (
	appleEmptySleepy appleEmptyMood = iota
	appleEmptyAuth
	appleEmptyLoading
	appleEmptyPuzzled
)

type appleEmptyPill struct {
	key   string
	label string
}

type appleEmptySpec struct {
	mood     appleEmptyMood
	headline string
	subtitle string
	pills    [2]appleEmptyPill
	pillN    int
	reveal   float64
}

type appleEmptyPillLayout struct {
	pill appleEmptyPill
	x, w int
}

func (p *AppleMusicPage) drawLibraryGateEmpty(x, y, w, rows int, now time.Time) {
	spec := appleEmptySpec{
		mood:     appleEmptyAuth,
		headline: T("status.sign_in_hint"),
		subtitle: T("empty.sub_auth"),
		pills: [2]appleEmptyPill{
			{key: "b", label: T("empty.btn_signin")},
			{key: "1", label: T("empty.btn_browse")},
		},
		pillN:  2,
		reveal: 1,
	}
	if !p.state.Ready {
		spec.mood = appleEmptyLoading
		spec.headline = T("status.connecting")
		spec.pills[0] = appleEmptyPill{key: "1", label: T("empty.btn_browse")}
		spec.pillN = 1
	}
	if !p.drawCozyEmptyCard(x, y, w, rows, now, spec) && rows > 0 && w > 2 {
		p.screen.textRGBA(x+1, y, spec.headline, appleNeon.Accent2, appleNeon.Bg, true, false, false, max(0, w-2))
	}
}

func (p *AppleMusicPage) drawDrawerEmpty(x, y, w, rows int, now time.Time) bool {
	c := p.catalogState(p.drawer)
	headline := p.rackEmptyLabel()
	if p.catalogAlbumCrate() {
		headline = p.catalogEmptyLabel()
	}
	spec := appleEmptySpec{
		mood:     appleEmptySleepy,
		headline: headline,
		subtitle: T("empty.sub_search"),
		reveal:   1,
	}
	switch {
	case p.drawer == appleQueue:
		spec.subtitle = T("empty.sub_queue")
		spec.pills = [2]appleEmptyPill{
			{key: "1", label: T("empty.btn_browse")},
			{key: "2", label: T("empty.btn_library")},
		}
		spec.pillN = 2
	case c.searching:
		spec.mood = appleEmptyLoading
		if p.drawer == appleLibrary {
			spec.subtitle = T("empty.sub_library")
		}
	case c.catalogError != "":
		spec.mood = appleEmptyPuzzled
		spec.subtitle = T("empty.sub_library")
		spec.pills = [2]appleEmptyPill{
			{key: "r", label: T("empty.btn_refresh")},
			{key: "/", label: T("empty.btn_search")},
		}
		spec.pillN = 2
	case p.drawer == appleLibrary:
		if c.catalog.title != "" {
			spec.mood = appleEmptyPuzzled
		}
		spec.subtitle = T("empty.sub_library")
		spec.pills = [2]appleEmptyPill{
			{key: "r", label: T("empty.btn_refresh")},
			{key: "1", label: T("empty.btn_browse")},
		}
		spec.pillN = 2
	case c.catalog.title != "":
		spec.mood = appleEmptyPuzzled
		spec.pills = [2]appleEmptyPill{
			{key: "/", label: T("empty.btn_search")},
			{key: "2", label: T("empty.btn_library")},
		}
		spec.pillN = 2
	default:
		spec.pills = [2]appleEmptyPill{
			{key: "/", label: T("empty.btn_search")},
			{key: "2", label: T("empty.btn_library")},
		}
		if p.state.Ready && !p.state.Authorized {
			spec.pills[1] = appleEmptyPill{key: "b", label: T("empty.btn_signin")}
		}
		spec.pillN = 2
	}
	return p.drawCozyEmptyCard(x, y, w, rows, now, spec)
}

func (p *AppleMusicPage) drawLyricsCozyEmpty(x, y, w, rows int, label string, reveal float64) bool {
	if w < 34 || rows < 7 {
		return false
	}
	spec := appleEmptySpec{
		mood:     appleEmptyPuzzled,
		headline: label,
		subtitle: T("empty.sub_lyrics"),
		reveal:   reveal,
	}
	switch p.lyrics.status {
	case "loading", "storefront_loading", "sdk_unavailable":
		spec.mood = appleEmptyLoading
	case "login_required", "unauthorized":
		spec.mood = appleEmptyAuth
		spec.pills = [2]appleEmptyPill{
			{key: "b", label: T("empty.btn_signin")},
			{key: "1", label: T("empty.btn_browse")},
		}
		spec.pillN = 2
	case "", "no_track":
		spec.mood = appleEmptySleepy
		spec.pills = [2]appleEmptyPill{
			{key: "1", label: T("empty.btn_browse")},
			{key: "/", label: T("empty.btn_search")},
		}
		spec.pillN = 2
	default:
		if p.state.Track.ID == "" {
			spec.pills = [2]appleEmptyPill{{key: "1", label: T("empty.btn_browse")}}
			spec.pillN = 1
		}
	}
	return p.drawCozyEmptyCard(x, y, w, rows, p.lastFrame, spec)
}

func (p *AppleMusicPage) emptyMotionPhase(now time.Time) float64 {
	if p.level == motion.Off {
		return 0
	}
	sec := float64(now.UnixMilli()%60000) / 1000.0
	return sec * (2 * math.Pi / 4.2)
}

func layoutEmptyPills(cx, cw int, spec appleEmptySpec) ([2]appleEmptyPillLayout, int) {
	var out [2]appleEmptyPillLayout
	if spec.pillN <= 0 || cw < 12 {
		return out, 0
	}
	n := min(spec.pillN, 2)
	maxSpan := max(0, cw-4)
	var widths [2]int
	count := 0
	total := 0
	for i := range n {
		pw := npWidth(spec.pills[i].label) + 4
		need := pw
		if count > 0 {
			need += 2
		}
		if total+need > maxSpan {
			break
		}
		widths[count] = pw
		total += need
		count++
	}
	if count == 0 {
		return out, 0
	}
	px := cx + max(0, (cw-total)/2)
	for i := range count {
		out[i] = appleEmptyPillLayout{pill: spec.pills[i], x: px, w: widths[i]}
		px += widths[i] + 2
	}
	return out, count
}

// drawCozyEmptyCard renders an animated warm study-desk companion inside empty drawers and lyric panels.
func (p *AppleMusicPage) drawCozyEmptyCard(x, y, w, rows int, now time.Time, spec appleEmptySpec) bool {
	if w < 32 || rows < 7 || spec.reveal <= 0 {
		return false
	}
	reveal := min(max(spec.reveal, 0), 1)
	cx, cy, cw, ch := x+1, y, w-2, rows
	if cw < 32 {
		cx, cw = x, w
	}

	pillLayouts, pillCount := layoutEmptyPills(cx, cw, spec)
	showPills := pillCount > 0 && ch >= 8
	showSub := ch >= 9 || (!showPills && ch >= 8)
	showBubble := ch >= 12

	bottomReserve := 1 // headline
	if showSub {
		bottomReserve++
	}
	if showPills {
		bottomReserve++
	}
	topReserve := 0
	if showBubble {
		topReserve = 1
	}

	availArt := ch - topReserve - bottomReserve
	if availArt < 4 {
		return false
	}
	artH := min(11, availArt)
	artW := min(cw-2, 46)
	totalH := topReserve + artH + bottomReserve
	startY := cy + max(0, (ch-totalH)/2)
	artY := startY + topReserve
	artX := cx + max(0, (cw-artW)/2)

	headY := artY + artH
	subY := headY + 1
	pillY := headY + 1
	if showSub {
		pillY++
	}

	perkUp := p.hoverX >= artX && p.hoverX < artX+artW && p.hoverY >= artY && p.hoverY < artY+artH
	for i := range pillCount {
		pl := pillLayouts[i]
		if showPills && p.hoverY == pillY && p.hoverX >= pl.x && p.hoverX < pl.x+pl.w {
			perkUp = true
		}
	}

	cardBg := npLerpRGBA(appleNeon.Bg, appleCozyCard, reveal)
	p.screen.fillRGBA(cx, cy, cw, ch, cardBg)
	if cw >= artW+2 && ch >= artH {
		railCol := npLerpRGBA(appleNeon.Bg, appleNeon.Ghost, reveal)
		for ry := artY; ry < artY+artH; ry++ {
			p.screen.setRGBA(artX-1, ry, "▐", 1, railCol, cardBg, true, true, false)
			p.screen.setRGBA(artX+artW, ry, "▌", 1, railCol, cardBg, true, true, false)
		}
	}

	if showBubble {
		bubbleText := p.emptyBubbleText(spec.mood, perkUp)
		innerMax := max(0, cw-8)
		textW := min(npWidth(bubbleText), innerMax)
		bx := cx + max(0, (cw-(textW+4))/2)
		bubBg := npLerpRGBA(appleNeon.Bg, appleCozyRaised, reveal)
		edgeCol := npLerpRGBA(appleNeon.Bg, appleCozyBracket, reveal)
		if perkUp {
			edgeCol = npLerpRGBA(appleNeon.Bg, appleNeon.Accent2, reveal)
		}
		textCol := npLerpRGBA(appleNeon.Bg, appleNeon.Text, reveal)
		p.screen.textRGBA(bx, startY, "╭ ", edgeCol, cardBg, true, true, false, 2)
		actualW := p.screen.textRGBA(bx+2, startY, bubbleText, textCol, bubBg, true, true, perkUp, innerMax)
		p.screen.textRGBA(bx+2+actualW, startY, " ╮", edgeCol, cardBg, true, true, false, 2)
	}

	phase := p.emptyMotionPhase(now)
	p.drawEmptyArtboard(artX, artY, artW, artH, phase, spec.mood, perkUp, reveal)

	// Centered truthful headline & warm subtitle.
	headCol := appleNeon.Text
	if spec.mood == appleEmptyAuth {
		headCol = appleNeon.Accent2
	}
	headCol = npLerpRGBA(appleNeon.Bg, headCol, reveal)
	subCol := npLerpRGBA(appleNeon.Bg, appleNeon.Secondary, reveal)
	drawCenteredEmptyLine(&p.screen, cx, cw, headY, spec.headline, headCol, cardBg, true)
	if showSub && subY < cy+ch {
		drawCenteredEmptyLine(&p.screen, cx, cw, subY, spec.subtitle, subCol, cardBg, false)
	}

	// Sculpted action pills at bottom of card.
	if showPills && pillY < cy+ch {
		for i := range pillCount {
			pl := pillLayouts[i]
			hovered := p.hoverY == pillY && p.hoverX >= pl.x && p.hoverX < pl.x+pl.w
			pillBg := appleCozyPill
			capCol := appleNeon.Accent2
			txtCol := appleNeon.Text
			if i == 0 {
				capCol = appleNeon.Hero
			}
			if hovered {
				pillBg = appleCozyPillHover
				capCol = appleNeon.Accent
			}
			pillBg = npLerpRGBA(appleNeon.Bg, pillBg, reveal)
			capCol = npLerpRGBA(appleNeon.Bg, capCol, reveal)
			txtCol = npLerpRGBA(appleNeon.Bg, txtCol, reveal)

			p.screen.textRGBA(pl.x, pillY, "▐", capCol, cardBg, true, true, false, 1)
			p.screen.fillRGBA(pl.x+1, pillY, max(0, pl.w-2), 1, pillBg)
			p.screen.textRGBA(pl.x+2, pillY, pl.pill.label, txtCol, pillBg, true, true, true, max(0, pl.w-4))
			p.screen.textRGBA(pl.x+pl.w-1, pillY, "▌", capCol, cardBg, true, true, false, 1)
			p.addHit(appleHit{kind: appleHitKey, key: pl.pill.key, x: pl.x, y: pillY, w: pl.w, h: 1})
		}
	}
	return true
}

func (p *AppleMusicPage) emptyBubbleText(mood appleEmptyMood, perkUp bool) string {
	switch mood {
	case appleEmptyAuth:
		if perkUp {
			return T("empty.bubble_awake")
		}
		return T("empty.bubble_auth")
	case appleEmptyLoading:
		return T("empty.bubble_loading")
	case appleEmptyPuzzled:
		return T("empty.bubble_puzzled")
	default:
		if perkUp {
			return T("empty.bubble_awake")
		}
		return T("empty.bubble_sleep")
	}
}

func drawCenteredEmptyLine(s *npScreen, cx, cw int, y int, label string, fg, bg color.RGBA, bold bool) {
	if cw <= 0 || y < 0 || y >= s.h {
		return
	}
	lw := min(npWidth(label), cw)
	tx := cx + max(0, (cw-lw)/2)
	s.textRGBA(tx, y, label, fg, bg, true, true, bold, cw)
}

func (p *AppleMusicPage) drawEmptyArtboard(ax, ay, aw, ah int, phase float64, mood appleEmptyMood, perkUp bool, reveal float64) {
	if aw <= 0 || ah <= 0 {
		return
	}
	pixW := float64(aw)
	pixH := float64(ah * 2)
	midX := pixW * 0.5
	sc := min(1.0, min(pixW/44.0, pixH/19.5))
	deskY := pixH - 2.4*sc

	bob := 0.0
	if perkUp {
		bob = -0.9 * sc
	} else if mood == appleEmptyLoading {
		bob = 0.25 * sc * math.Sin(phase*2)
	} else {
		bob = 0.42 * sc * math.Sin(phase)
	}

	fireflyX := 10.6 + 1.4*math.Cos(phase)
	fireflyY := -8.8 + 1.5*math.Sin(phase*1.3) + bob*0.4
	if perkUp {
		fireflyX = 8.4
		fireflyY = -6.0 + bob
	}
	fireflyGlow := 1.0
	if phase != 0 {
		fireflyGlow = 0.72 + 0.28*math.Sin(phase*2.1)
	}
	if perkUp {
		fireflyGlow = 1.25
	}

	lampPower := 0.58
	if phase != 0 {
		lampPower = 0.54 + 0.08*math.Sin(phase)
	}
	if perkUp {
		lampPower = 0.92
	}

	// Stack-only palette keeps subpixel rasterization allocation-free.
	colCard := appleCozyCard
	colSlate := color.RGBA{0x1c, 0x17, 0x28, 0xff}
	colFrame := appleNeon.Rule
	colDesk := appleCozyWoodHi
	colDeskHi := color.RGBA{0xd6, 0xa3, 0x74, 0xff}
	colDeskShadow := appleNeon.Bg
	colLamp := color.RGBA{0xe8, 0x92, 0x4c, 0xff}
	colLampArm := color.RGBA{0x9c, 0x7a, 0x5e, 0xff}
	colBulb := color.RGBA{0xff, 0xf2, 0xac, 0xff}
	colAmberCone := color.RGBA{0xff, 0xb8, 0x5c, 0xff}
	colBookCover := color.RGBA{0x6c, 0x8e, 0x7f, 0xff}
	colBookPage := color.RGBA{0xf3, 0xeb, 0xdd, 0xff}
	colMug := color.RGBA{0xd9, 0x7b, 0x59, 0xff}
	colMugRim := color.RGBA{0xf6, 0xdf, 0xc7, 0xff}
	colSteam := color.RGBA{0xd9, 0xbe, 0xa5, 0xff}
	colBotBody := appleNeon.Text
	colBotShade := color.RGBA{0xd9, 0xc5, 0xae, 0xff}
	colBotOutline := color.RGBA{0x63, 0x4a, 0x37, 0xff}
	colBotScreen := color.RGBA{0x22, 0x19, 0x22, 0xff}
	colCheek := color.RGBA{0xff, 0x8f, 0x7a, 0xff}
	colEyeAmber := color.RGBA{0xff, 0xd4, 0x7a, 0xff}
	colEyeCyan := appleNeon.Accent
	colEyeWhite := color.RGBA{0xff, 0xfd, 0xf5, 0xff}
	colScarf := appleNeon.Hero
	colSlotWell := color.RGBA{0x2a, 0x20, 0x2b, 0xff}
	colSlotGold := appleNeon.Accent2
	colAntIdle := color.RGBA{0xb6, 0x8f, 0x6c, 0xff}
	colAntLit := appleNeon.Accent2

	samplePixel := func(px, py int) color.RGBA {
		cx := float64(px) + 0.5
		cy := float64(py) + 0.5
		ux := (cx - midX) / sc
		uy := (cy - deskY) / sc

		// Inner rounded stage backdrop.
		stageD := emptySdRoundBox(cx-midX, cy-pixH*0.5, pixW*0.5-0.7, pixH*0.5-0.4, 2.0)
		col := colCard
		if stageD <= 0 {
			col = colSlate
			// Soft lamp light cone radiating down onto companion.
			coneDx := (ux + 3.2) / 14.5
			coneDy := (uy + 6.4) / 8.8
			coneR := coneDx*coneDx + coneDy*coneDy
			if coneR < 1.0 && ux > -13.5 && uy < 1.0 {
				w := (1.0 - coneR) * (1.0 - coneR) * 0.24 * lampPower
				col = emptyBlend(col, colAmberCone, w)
			}
			// Outer subtle warm border.
			if stageD > -0.95 {
				col = emptyBlend(col, colFrame, 0.72)
			}
		}

		// Desk shadow + desk top bar.
		if sh := emptySdCircle(ux/17.5, (uy-0.95)/1.05, 1.0); sh < 0 {
			col = emptyBlend(col, colDeskShadow, min(1, -sh*1.8)*0.65)
		}
		if dDesk := emptySdRoundBox(ux, uy-0.12, 19.2, 0.48, 0.25); dDesk < 0.7 {
			a := emptySmoothAlpha(dDesk, 0.75)
			dc := colDesk
			if uy < 0.05 {
				dc = colDeskHi
			}
			col = emptyBlend(col, dc, a)
		}

		// Open notebook (left-center on desk).
		if nb := emptySdRoundBox(ux+9.8, uy+0.78, 2.3, 0.46, 0.18); nb < 0.6 {
			c := colBookCover
			if uy < -0.78 {
				c = colBookPage
			}
			col = emptyBlend(col, c, emptySmoothAlpha(nb, 0.65))
		}

		// Articulated desk lamp (left).
		if arm1 := emptySdSeg(ux, uy, -15.2, -1.2, -17.5, -7.0, 0.55); arm1 < 0.6 {
			col = emptyBlend(col, colLampArm, emptySmoothAlpha(arm1, 0.65))
		}
		if arm2 := emptySdSeg(ux, uy, -17.5, -7.0, -13.2, -12.8, 0.55); arm2 < 0.6 {
			col = emptyBlend(col, colLampArm, emptySmoothAlpha(arm2, 0.65))
		}
		if joint := emptySdCircle(ux+17.5, uy+7.0, 0.8); joint < 0.55 {
			col = emptyBlend(col, colBotBody, emptySmoothAlpha(joint, 0.6))
		}
		if base := emptySdRoundBox(ux+15.0, uy+0.8, 2.2, 0.55, 0.45); base < 0.6 {
			col = emptyBlend(col, colLamp, emptySmoothAlpha(base, 0.65))
		}
		if bulb := emptySdCircle(ux+10.5, uy+10.5, 1.25); bulb < 0.7 {
			col = emptyBlend(col, colBulb, emptySmoothAlpha(bulb, 0.7))
		}
		if shade := emptySdSeg(ux, uy, -13.6, -13.0, -9.8, -10.6, 1.55); shade < 0.6 {
			col = emptyBlend(col, colLamp, emptySmoothAlpha(shade, 0.65))
		}

		// Steaming terracotta mug (right).
		for sIdx := range 2 {
			sxBase := 13.4 + float64(sIdx)*1.7
			sOffset := float64(sIdx) * math.Pi * 0.9
			if uy < -4.9 && uy > -10.8 && math.Abs(ux-sxBase) < 1.8 {
				driftY := uy + 5.2
				waveX := sxBase + 0.55*math.Sin(uy*1.15-phase*1.1+sOffset)
				dist := math.Abs(ux - waveX)
				fade := 1.0 - min(1.0, math.Abs(driftY)/5.8)
				if dist < 0.65 {
					col = emptyBlend(col, colSteam, (1.0-dist/0.65)*fade*0.62)
				}
			}
		}
		if mh := emptySdCircle(ux-16.8, uy+2.2, 1.45); math.Abs(mh) < 0.45 && ux > 16.5 {
			col = emptyBlend(col, colBotOutline, emptySmoothAlpha(math.Abs(mh)-0.15, 0.45))
		}
		if mug := emptySdRoundBox(ux-14.4, uy+2.25, 2.25, 1.9, 0.55); mug < 0.6 {
			mc := colMug
			if uy < -3.55 {
				mc = colMugRim
			}
			col = emptyBlend(col, mc, emptySmoothAlpha(mug, 0.65))
		}

		// Central Sleepy Companion Bot.
		if feet := emptySdRoundBox(ux, uy+1.15-bob*0.4, 4.3, 0.9, 0.8); feet < 0.6 {
			col = emptyBlend(col, colBotShade, emptySmoothAlpha(feet, 0.65))
		}
		// Ear pads.
		if earL := emptySdRoundBox(ux+6.2, uy+6.4-bob, 0.95, 1.6, 0.65); earL < 0.6 {
			col = emptyBlend(col, colBotShade, emptySmoothAlpha(earL, 0.65))
		}
		if earR := emptySdRoundBox(ux-6.2, uy+6.4-bob, 0.95, 1.6, 0.65); earR < 0.6 {
			col = emptyBlend(col, colBotShade, emptySmoothAlpha(earR, 0.65))
		}
		// Side golden empty cassette slot waiting for a tape.
		if slotD := emptySdRoundBox(ux-7.8, uy+4.4-bob, 1.55, 1.75, 0.5); slotD < 0.6 {
			scol := colSlotWell
			if slotD > -0.65 {
				pulse := 0.72
				if phase != 0 {
					pulse = 0.6 + 0.4*math.Sin(phase*1.6)
				}
				scol = emptyBlend(colSlotWell, colSlotGold, pulse)
			} else if (math.Abs(ux-7.8) < 0.75 && math.Abs(uy+4.4-bob) < 0.25) || (math.Abs(ux-7.8) < 0.25 && math.Abs(uy+4.4-bob) < 0.75) {
				scol = colSlotGold
			}
			col = emptyBlend(col, scol, emptySmoothAlpha(slotD, 0.6))
		}
		// Antenna.
		if stem := emptySdSeg(ux, uy, 0, -10.8+bob, 0, -13.1+bob, 0.34); stem < 0.55 {
			col = emptyBlend(col, colBotOutline, emptySmoothAlpha(stem, 0.55))
		}
		if tip := emptySdCircle(ux, uy+13.7-bob, 0.9); tip < 1.6 {
			lit := perkUp || mood == appleEmptyLoading
			tipCol := colAntIdle
			if lit {
				tipCol = colAntLit
			}
			if tip < 0.55 {
				col = emptyBlend(col, tipCol, emptySmoothAlpha(tip, 0.6))
			} else if lit {
				glowA := (1.0 - (tip-0.55)/1.05) * 0.48
				col = emptyBlend(col, colAmberCone, glowA)
			}
		}

		// Main Cream Head & Torso.
		if body := emptySdRoundBox(ux, uy+6.5-bob, 5.6, 4.3, 2.2); body < 0.7 {
			a := emptySmoothAlpha(body, 0.7)
			bc := colBotBody
			if body > -0.65 {
				bc = emptyBlend(colBotBody, colBotOutline, 0.55)
			}
			col = emptyBlend(col, bc, a)
		}
		// Cozy Coral Scarf / Collar.
		if scarf := emptySdRoundBox(ux, uy+2.25-bob, 4.1, 0.55, 0.45); scarf < 0.5 {
			col = emptyBlend(col, colScarf, emptySmoothAlpha(scarf, 0.55))
		}
		// Dark CRT Face Screen.
		if scr := emptySdRoundBox(ux, uy+6.8-bob, 4.1, 2.65, 1.05); scr < 0.6 {
			col = emptyBlend(col, colBotScreen, emptySmoothAlpha(scr, 0.6))
		}
		// Rosy Cheeks.
		if ckL := emptySdCircle((ux+2.55)/0.85, (uy+5.4-bob)/0.5, 1.0); ckL < 0.3 {
			col = emptyBlend(col, colCheek, emptySmoothAlpha(ckL, 0.5)*0.65)
		}
		if ckR := emptySdCircle((ux-2.55)/0.85, (uy+5.4-bob)/0.5, 1.0); ckR < 0.3 {
			col = emptyBlend(col, colCheek, emptySmoothAlpha(ckR, 0.5)*0.65)
		}

		// CRT Eyes & Mouth.
		switch {
		case perkUp || mood == appleEmptyAuth:
			if eL := emptySdCircle(ux+1.7, uy+7.1-bob, 0.72); eL < 0.5 {
				col = emptyBlend(col, colEyeAmber, emptySmoothAlpha(eL, 0.5))
			}
			if hL := emptySdCircle(ux+1.45, uy+7.35-bob, 0.28); hL < 0.35 {
				col = emptyBlend(col, colEyeWhite, emptySmoothAlpha(hL, 0.35))
			}
			if eR := emptySdCircle(ux-1.7, uy+7.1-bob, 0.72); eR < 0.5 {
				col = emptyBlend(col, colEyeAmber, emptySmoothAlpha(eR, 0.5))
			}
			if hR := emptySdCircle(ux-1.95, uy+7.35-bob, 0.28); hR < 0.35 {
				col = emptyBlend(col, colEyeWhite, emptySmoothAlpha(hR, 0.35))
			}
			if m := emptySdSeg(ux, uy, -0.7, -5.5+bob, 0.7, -5.5+bob, 0.3); m < 0.45 {
				col = emptyBlend(col, colEyeAmber, emptySmoothAlpha(m, 0.45))
			}
		case mood == appleEmptyLoading:
			pulseC := colEyeCyan
			if phase != 0 && math.Sin(phase*2) < 0 {
				pulseC = emptyBlend(colEyeCyan, colEyeAmber, 0.4)
			}
			if eL := emptySdRoundBox(ux+1.7, uy+7.0-bob, 0.75, 0.45, 0.3); eL < 0.45 {
				col = emptyBlend(col, pulseC, emptySmoothAlpha(eL, 0.45))
			}
			if eR := emptySdRoundBox(ux-1.7, uy+7.0-bob, 0.75, 0.45, 0.3); eR < 0.45 {
				col = emptyBlend(col, pulseC, emptySmoothAlpha(eR, 0.45))
			}
		case mood == appleEmptyPuzzled:
			if eL := emptySdCircle(ux+1.7, uy+7.2-bob, 0.72); eL < 0.45 {
				col = emptyBlend(col, colEyeAmber, emptySmoothAlpha(eL, 0.45))
			}
			if eR := emptySdSeg(ux, uy, 1.0, -6.9+bob, 2.4, -7.3+bob, 0.35); eR < 0.45 {
				col = emptyBlend(col, colEyeAmber, emptySmoothAlpha(eR, 0.45))
			}
			if m := emptySdCircle(ux, uy+5.5-bob, 0.42); m < 0.4 {
				col = emptyBlend(col, colEyeAmber, emptySmoothAlpha(m, 0.4))
			}
		default:
			// Peaceful sleeping curved eyes.
			if math.Abs(ux+1.7) <= 1.15 {
				dx := ux + 1.7
				targetY := -6.8 + bob - 0.25*dx*dx
				if d := math.Abs(uy - targetY); d < 0.55 {
					col = emptyBlend(col, colEyeAmber, 1.0-d/0.55)
				}
			}
			if math.Abs(ux-1.7) <= 1.15 {
				dx := ux - 1.7
				targetY := -6.8 + bob - 0.25*dx*dx
				if d := math.Abs(uy - targetY); d < 0.55 {
					col = emptyBlend(col, colEyeAmber, 1.0-d/0.55)
				}
			}
			if m := emptySdCircle(ux, uy+5.4-bob, 0.35); m < 0.4 {
				col = emptyBlend(col, colEyeAmber, emptySmoothAlpha(m, 0.4)*0.85)
			}
		}

		// Orbiting Model-Spark Firefly near the golden slot.
		fDist := emptySdCircle(ux-fireflyX, uy-fireflyY, 0.65)
		if fDist < 2.6 {
			if fDist < 0.45 {
				col = emptyBlend(col, colBulb, emptySmoothAlpha(fDist, 0.45))
			} else {
				halo := 1.0 - (fDist-0.45)/2.15
				col = emptyBlend(col, colAmberCone, halo*halo*0.58*fireflyGlow)
			}
		}

		if reveal < 1 {
			col = npLerpRGBA(appleNeon.Bg, col, reveal)
		}
		return col
	}

	for iy := range ah {
		pyTop := iy * 2
		pyBot := pyTop + 1
		for ix := range aw {
			topCol := samplePixel(ix, pyTop)
			botCol := samplePixel(ix, pyBot)
			p.screen.setRGBA(ax+ix, ay+iy, "▀", 1, topCol, botCol, true, true, false)
		}
	}

	// Vector-sharp typography accents overlaid onto cell backgrounds.
	if aw >= 28 && ah >= 6 {
		cxMid := ax + aw/2
		switch {
		case perkUp:
			emptyStampGlyph(&p.screen, cxMid-1, ay, "✦", appleNeon.Accent2, reveal)
		case mood == appleEmptySleepy:
			drift := 0
			if phase != 0 && math.Sin(phase) > 0 {
				drift = 1
			}
			emptyStampGlyph(&p.screen, cxMid+4+drift, ay+max(0, ah/4-drift), "z", appleNeon.Secondary, reveal)
			emptyStampGlyph(&p.screen, cxMid+6+drift, ay+max(0, ah/4-1), "Z", appleNeon.Accent2, reveal)
		case mood == appleEmptyLoading:
			emptyStampGlyph(&p.screen, cxMid+4, ay+max(0, ah/5), "♪", appleNeon.Accent, reveal)
		case mood == appleEmptyPuzzled:
			emptyStampGlyph(&p.screen, cxMid+4, ay+max(0, ah/5), "?", appleNeon.Hero, reveal)
		}
	}
}

func emptyStampGlyph(s *npScreen, x, y int, glyph string, fg color.RGBA, reveal float64) {
	if c, ok := s.cellAt(x, y); ok {
		bg := c.Bg
		if !c.HasBg {
			bg = appleNeon.Bg
		}
		s.setRGBA(x, y, glyph, 1, npLerpRGBA(appleNeon.Bg, fg, reveal), bg, true, true, true)
	}
}

func emptySdCircle(x, y, r float64) float64 {
	return math.Hypot(x, y) - r
}

func emptySdRoundBox(px, py, bx, by, r float64) float64 {
	qx := math.Abs(px) - bx + r
	qy := math.Abs(py) - by + r
	return math.Hypot(max(qx, 0), max(qy, 0)) + min(max(qx, qy), 0) - r
}

func emptySdSeg(px, py, ax, ay, bx, by, r float64) float64 {
	pax, pay := px-ax, py-ay
	bax, bay := bx-ax, by-ay
	denom := bax*bax + bay*bay
	h := 0.0
	if denom > 1e-9 {
		h = min(max((pax*bax+pay*bay)/denom, 0), 1)
	}
	return math.Hypot(pax-bax*h, pay-bay*h) - r
}

func emptySmoothAlpha(d, aa float64) float64 {
	if d <= -aa {
		return 1
	}
	if d >= aa {
		return 0
	}
	t := (aa - d) / (2 * aa)
	return t * t * (3 - 2*t)
}

func emptyBlend(dst, src color.RGBA, alpha float64) color.RGBA {
	if alpha <= 0 {
		return dst
	}
	if alpha >= 1 {
		return src
	}
	return color.RGBA{
		R: uint8(float64(dst.R)*(1-alpha) + float64(src.R)*alpha + 0.5),
		G: uint8(float64(dst.G)*(1-alpha) + float64(src.G)*alpha + 0.5),
		B: uint8(float64(dst.B)*(1-alpha) + float64(src.B)*alpha + 0.5),
		A: 0xff,
	}
}
