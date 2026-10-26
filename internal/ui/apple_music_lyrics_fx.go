package ui

import (
	"image/color"
	"math"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
)

// appleLyricLight is the soft lavender-white tint for unplayed lyric text.
var appleLyricLight = color.RGBA{0xb8, 0xae, 0xd6, 0xff}

var appleParticleRunes = [4]string{"✦", "✧", "⋆", "·"}

// drawLyricActiveRow renders an active (currently playing) lyric row with
// real-progress split coloring (sung Hero vs unplayed light), a glowing
// wavefront cursor, and trailing stardust particles.
func (p *AppleMusicPage) drawLyricActiveRow(x, y, w int, row appleLyricRow, position time.Duration, reveal float64) {
	line := p.lyrics.data.Lines[row.line]
	span := max(line.End-line.Start, time.Millisecond)
	lineProg := min(max(float64(position-line.Start)/float64(span), 0), 1)
	rowProg := min(max((lineProg-row.startFrac)/max(row.endFrac-row.startFrac, 1e-6), 0), 1)

	railX, textX, textW := x, x, w
	if w >= 12 {
		textX, textW = x+1, w-1
	}
	baseBg := npLerp(appleNeon.Bg, appleNeon.Surface, .62*reveal)
	glowBg := npLerp(appleNeon.Bg, color.RGBA{0x2b, 0x12, 0x44, 0xff}, .85*reveal)
	if w >= 12 {
		p.screen.set(railX, y, "┃", 1, uv.Style{Fg: npLerp(appleNeon.Bg, appleNeon.Accent, reveal), Bg: glowBg, Attrs: uv.AttrBold})
	}
	if textW <= 0 {
		return
	}
	p.screen.fill(textX, y, textW, 1, baseBg)
	p.screen.text(textX, y, "› ", uv.Style{Fg: npLerp(appleNeon.Bg, appleNeon.Hero, reveal), Bg: glowBg, Attrs: uv.AttrBold}, min(2, textW))
	if textW <= 2 {
		return
	}
	bodyX, bodyW := textX+2, textW-2
	str := npSanitize(row.text)
	totalCols := min(npWidth(str), bodyW)
	litCols := int(math.Round(float64(totalCols) * rowProg))
	for i := 0; i < min(litCols+1, bodyW); i++ {
		p.screen.set(bodyX+i, y, " ", 1, uv.Style{Bg: glowBg})
	}
	col := 0
	for len(str) > 0 {
		cl, cw, n := npNextCluster(str)
		str = str[n:]
		if cw == 0 {
			continue
		}
		if col+cw > bodyW {
			break
		}
		fg := color.Color(appleLyricLight)
		cellBg := baseBg
		attrs := uint8(0)
		switch {
		case col+cw <= litCols:
			fg, cellBg, attrs = appleNeon.Hero, glowBg, uv.AttrBold
		case col <= litCols && rowProg > 0 && rowProg < 1:
			fg, cellBg, attrs = appleNeon.Accent, npLerp(glowBg, appleNeon.Accent, .24*reveal), uv.AttrBold
		}
		p.screen.set(bodyX+col, y, cl, cw, uv.Style{Fg: npLerp(appleNeon.Bg, fg, reveal), Bg: cellBg, Attrs: attrs})
		col += cw
	}
	p.drawLyricParticles(bodyX+col, y, bodyX+bodyW, row.line, lineProg, reveal, baseBg, glowBg)
}

// drawLyricParticles scatters deterministic stardust glyphs in the trailing
// columns of an active lyric row, driven by the real playback position.
func (p *AppleMusicPage) drawLyricParticles(startX, y, maxX, lineIdx int, prog, reveal float64, baseBg, glowBg color.Color) {
	room := maxX - startX
	if room < 3 {
		return
	}
	seed := int(math.Floor(prog*24)) + lineIdx*7
	waveX := startX + 1 + min(room-2, int(math.Round(prog*float64(min(room-2, 8)))))
	for dx := range min(4, room-1) {
		px := waveX + dx - 1
		if px <= startX || px >= maxX {
			continue
		}
		glyph := appleParticleRunes[(seed+dx)%len(appleParticleRunes)]
		var fg color.Color = appleNeon.Accent
		bg := baseBg
		switch dx {
		case 0:
			fg, bg = appleNeon.Accent2, glowBg
		case 1:
			fg = appleNeon.Accent
		case 2:
			fg = appleNeon.Hero
		default:
			fg = appleLyricLight
		}
		p.screen.set(px, y, glyph, 1, uv.Style{Fg: npLerp(appleNeon.Bg, fg, reveal), Bg: bg})
	}
}
