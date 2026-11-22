package ui

import (
	"image/color"
	"math"
	"strings"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

type CounterState struct {
	digits, prevDigits [4]int
	changedAt          [4]time.Time
	initialized        bool
}

func counterDigits(d time.Duration) [4]int {
	total := min(max(int(d/time.Second), 0), 99*60+59)
	return [4]int{total / 600, total / 60 % 10, total % 60 / 10, total % 10}
}

func (c *CounterState) Update(now time.Time, d time.Duration) {
	digits := counterDigits(d)
	if !c.initialized {
		c.digits, c.prevDigits, c.initialized = digits, digits, true
		return
	}
	for i, digit := range digits {
		if digit != c.digits[i] {
			c.prevDigits[i], c.digits[i], c.changedAt[i] = c.digits[i], digit, now
		}
	}
}

// Drawing never advances state, so snapshots cannot consume an ignition frame.
func DrawNixieCounter(s *npScreen, x, y int, d time.Duration, ready bool, now time.Time, state *CounterState) {
	var px [npCounterRows * 2][npCounterWidth]color.RGBA
	pal := s.pal
	glow := npLerpRGBA(pal.Bg, pal.Accent2, .12)
	for row := range px {
		for col := range px[row] {
			px[row][col] = glow
		}
	}
	digits := counterDigits(d)
	pulse := pal.Accent2
	if !ready {
		phase := float64(now.UnixNano()%int64(500*time.Millisecond)) / float64(500*time.Millisecond)
		pulse = npLerpRGBA(pal.Ghost, pal.Accent2, .5+.5*math.Cos(2*math.Pi*phase))
	}
	ghostFade := npLerpRGBA(pal.Ghost, pal.Accent2, .45)
	ignite := npLerpRGBA(pal.Accent2, pal.Text, .55)
	for i, dx := range [4]int{0, 5, 12, 17} {
		bits, oldBits := npDigitSegments[digits[i]], uint8(0)
		lit := pulse
		if !ready {
			bits = 1 << 6
		} else if state != nil && state.initialized && state.digits[i] == digits[i] && !state.changedAt[i].IsZero() {
			elapsed := now.Sub(state.changedAt[i])
			if elapsed >= 0 && elapsed < 33*time.Millisecond {
				lit = ignite
			}
			if elapsed >= 0 && elapsed < 66*time.Millisecond {
				oldBits = npDigitSegments[state.prevDigits[i]] &^ bits
			}
		}
		// Paint in passes: shared segment endpoints must keep the brighter pixel.
		for _, layer := range [3]struct {
			bits uint8
			fg   color.RGBA
		}{{0x7f, pal.Ghost}, {oldBits, ghostFade}, {bits, lit}} {
			for seg, pixels := range npSegmentPixels {
				if layer.bits&(1<<seg) != 0 {
					for _, xy := range pixels {
						px[xy[1]][dx+xy[0]] = layer.fg
					}
				}
			}
		}
	}
	px[2][10] = pulse
	px[4][10] = pulse
	for cy := range npCounterRows {
		for cx := range npCounterWidth {
			top, bot := px[cy*2][cx], px[cy*2+1][cx]
			if top == bot {
				s.setRGBA(x+cx, y+cy, "█", 1, top, color.RGBA{}, true, false, false)
			} else {
				s.setRGBA(x+cx, y+cy, "▀", 1, top, bot, true, true, false)
			}
		}
	}
}

func meterLevel(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return min(max(v, 0), 1)
}

func drawLEDMeter(s *npScreen, x, y, w int, label string, frame applemusic.MeterFrame) {
	drawLEDMeterPeaks(s, x, y, w, label, frame, [2]float64{})
}

func drawLEDMeterPeaks(s *npScreen, x, y, w int, label string, frame applemusic.MeterFrame, peaks [2]float64) {
	if w <= 0 {
		return
	}
	title := label
	if !strings.Contains(title, "dBFS") {
		title += "  dBFS"
	}
	s.text(x, y, title, uv.Style{Fg: s.pal.Secondary}, w)
	n := max(0, min((w-2)/2, 16))
	if n >= 8 {
		for _, tick := range []struct {
			offset int
			label  string
		}{{0, "-48"}, {n - 1, "-24"}, {(n - 1) * 3 / 2, "-12"}, {(n - 1) * 2, "0"}} {
			s.text(x+2+tick.offset, y+1, tick.label, uv.Style{Fg: s.pal.Secondary}, w-2-tick.offset)
		}
	}
	unlit := npLerpRGBA(s.pal.Rule, s.pal.Bg, .6)
	for ch := range 2 {
		s.text(x, y+ch+2, []string{"L", "R"}[ch], uv.Style{Fg: s.pal.Secondary}, 1)
		lvl := meterLevel(frame.Levels[ch])
		scaled := lvl * float64(n)
		activeCols := int(math.Floor(scaled))
		frac := scaled - float64(activeCols)
		peakIdx := int(math.Floor(meterLevel(peaks[ch])*float64(n) - 1e-6))
		for i := range n {
			col := unlit
			glyph := "█"
			db := -48 + 48*float64(i+1)/float64(max(1, n))
			switch {
			case frame.Ready && i < activeCols:
				col, _ = npRGBA(meterColor(s.pal, db))
			case frame.Ready && i == activeCols && frac >= 0.25:
				col, _ = npRGBA(meterColor(s.pal, db))
				if frac >= 0.75 {
					glyph = "▊"
				} else {
					glyph = "▌"
				}
			case frame.Ready && peaks[ch] > lvl && i == peakIdx && peakIdx >= activeCols:
				col, _ = npRGBA(meterColor(s.pal, db))
				glyph = "▐"
			}
			s.setRGBA(x+2+i*2, y+ch+2, glyph, 1, col, color.RGBA{}, true, false, false)
		}
	}
}

func meterColor(pal npPalette, db float64) color.Color {
	switch {
	case db >= -3:
		return color.RGBA{0xff, 0x3b, 0x5c, 0xff}
	case db >= -6:
		return color.RGBA{0xff, 0xd2, 0x3f, 0xff}
	case db >= -12:
		return pal.Hero
	default:
		return pal.Accent
	}
}

func drawPerspectiveGrid(s *npScreen, x, y, w, h, horizon, rays int, dim float64, rows []float64) {
	depth := h - horizon
	st := uv.Style{Fg: npLerp(s.pal.Accent, s.pal.Bg, dim)}
	for row := horizon; row < h; row++ {
		t := float64(row-horizon+1) / float64(max(1, depth))
		for ray := -rays; ray <= rays; ray++ {
			col := int(float64(w)/2 + float64(ray)*float64(w)*.22*t)
			ch := "╱"
			if ray > 0 {
				ch = "╲"
			}
			if ray == 0 {
				ch = "│"
			}
			if col >= 0 && col < w {
				s.set(x+col, y+row, ch, 1, st)
			}
		}
	}
	for _, t := range rows {
		row := horizon + int(math.Round(t*float64(max(0, depth-1))))
		s.repeat(x, y+row, w, "─", st)
	}
}

type BezelStyle uint8

const (
	BezelRack BezelStyle = iota
	BezelHeavy
)

func DrawBezel(s *npScreen, x, y, w, h int, label string, style BezelStyle) {
	if w < 2 || h < 2 {
		return
	}
	upper := uv.Style{Fg: npLerp(s.pal.Rule, s.pal.Text, .2)}
	lower := uv.Style{Fg: s.pal.Rule}
	top, bottom, left, right := "─", "─", "│", "│"
	corners := [4]string{"┌", "┐", "└", "┘"}
	if style == BezelHeavy {
		top, bottom, left, right = "▀", "▄", "▌", "▐"
		corners = [4]string{"▛", "▜", "▙", "▟"}
	}
	s.repeat(x+1, y, w-2, top, upper)
	s.repeat(x+1, y+h-1, w-2, bottom, lower)
	for row := y + 1; row < y+h-1; row++ {
		s.set(x, row, left, 1, upper)
		s.set(x+w-1, row, right, 1, lower)
	}
	s.set(x, y, corners[0], 1, upper)
	s.set(x+w-1, y, corners[1], 1, upper)
	s.set(x, y+h-1, corners[2], 1, lower)
	s.set(x+w-1, y+h-1, corners[3], 1, lower)
	if label != "" {
		s.text(x+2, y, " "+label+" ", uv.Style{Fg: s.pal.Secondary, Bg: s.pal.Bg}, w-4)
	}
	if style == BezelHeavy && w >= 6 && h >= 4 {
		for _, xx := range []int{x + 1, x + w - 2} {
			for _, yy := range []int{y + 1, y + h - 2} {
				s.set(xx, yy, "◉", 1, lower)
			}
		}
		grille := min(6, max(0, w-npWidth(label)-6))
		s.repeat(x+w-2-grille, y, grille, "▤", lower)
	}
}

func drawGlitch(s *npScreen, x, y, w, h, phase int) {
	if w <= 0 {
		return
	}
	if cap(s.glitchBuf) >= w {
		s.glitchBuf = s.glitchBuf[:w]
	} else {
		s.glitchBuf = make([]npCell, w)
	}
	cells := s.glitchBuf
	for row := 2 + phase; row < h; row += 5 {
		for col := range w {
			if c, ok := s.cellAt(x+col, y+row); ok {
				cells[col] = c
			} else {
				cells[col] = npCell{}
			}
		}
		shift := 0
		if phase == 0 && row%2 != 0 {
			shift = 2
		}
		s.fillRGBA(x, y+row, w, 1, s.pal.Bg)
		// Separated color ghosts precede the displaced scanline itself.
		for _, offset := range [3]int{-1, 1, 0} {
			for col, c := range cells {
				if c.Width != 1 || c.Content == "" || (c.Content == " " && (!c.HasBg || npSameRGBA(c.Bg, s.pal.Bg))) {
					continue
				}
				dest := col + shift + offset
				if dest < 0 || dest >= w {
					continue
				}
				if offset != 0 {
					fg := s.pal.Accent
					if offset > 0 {
						fg = s.pal.Hero
					}
					c.Fg = fg
					c.HasFg = true
					if c.HasBg {
						c.Bg = npLerpRGBA(c.Bg, fg, .25)
					}
				}
				s.setCell(x+dest, y+row, c)
			}
		}
	}
}
