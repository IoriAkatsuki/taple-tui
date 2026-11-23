package ui

import (
	"image/color"
	"math"
	"strings"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

// appleLyricLight is the soft oat-cream tint for unplayed lyric text.
var appleLyricLight = color.RGBA{0xe4, 0xd8, 0xc8, 0xff}

var appleParticleRunes = [4]string{"✦", "✧", "⋆", "·"}

// drawLyricActiveRow renders an active (currently playing) lyric row with
// real-clock syllable/word span illumination when TTML <span> timing is present
// or line-progress split coloring otherwise, plus warm tungsten wavefront & particles.
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
	glowBg := npLerp(appleNeon.Bg, color.RGBA{0x36, 0x21, 0x28, 0xff}, .85*reveal)
	if w >= 12 {
		p.screen.set(railX, y, "┃", 1, uv.Style{Fg: npLerp(appleNeon.Bg, appleNeon.Accent2, reveal), Bg: glowBg, Attrs: uv.AttrBold})
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
	waveActive := rowProg > 0 && rowProg < 1
	if len(line.Spans) > 0 {
		litCols = appleSpanLitCols(line, row.colOffset, position, totalCols)
		waveActive = position > line.Start && position < line.End
	}
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
		case col <= litCols && waveActive && litCols > 0:
			fg, cellBg, attrs = appleNeon.Accent2, npLerp(glowBg, appleNeon.Hero, .22*reveal), uv.AttrBold
		}
		p.screen.set(bodyX+col, y, cl, cw, uv.Style{Fg: npLerp(appleNeon.Bg, fg, reveal), Bg: cellBg, Attrs: attrs})
		col += cw
	}
	p.drawLyricParticles(bodyX+col, y, bodyX+bodyW, row.line, lineProg, reveal, baseBg, glowBg)
}

func appleSpanLitCols(line applemusic.LyricLine, rowColOffset int, position time.Duration, totalCols int) int {
	cleanLine := npSanitize(line.Text)
	searchFrom := 0
	lineLitCols := 0
	for _, sp := range line.Spans {
		target := npSanitize(sp.Text)
		if target == "" {
			continue
		}
		idx := strings.Index(cleanLine[searchFrom:], target)
		if idx < 0 {
			continue
		}
		startCol := npWidth(cleanLine[:searchFrom+idx])
		endCol := npWidth(cleanLine[:searchFrom+idx+len(target)])
		searchFrom += idx + len(target)
		if position >= sp.End {
			lineLitCols = endCol
			continue
		}
		if position > sp.Start {
			u := float64(position-sp.Start) / float64(max(sp.End-sp.Start, time.Millisecond))
			lineLitCols = startCol + int(math.Round(float64(endCol-startCol)*u))
		}
		break
	}
	return min(max(lineLitCols-rowColOffset, 0), totalCols)
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
			fg = appleNeon.Hero
		case 2:
			fg = appleNeon.Accent
		default:
			fg = appleLyricLight
		}
		p.screen.set(px, y, glyph, 1, uv.Style{Fg: npLerp(appleNeon.Bg, fg, reveal), Bg: bg})
	}
}
