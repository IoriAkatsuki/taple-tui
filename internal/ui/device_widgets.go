package ui

import (
	"image/color"
	"math"
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
	p := newNPPixels(npCounterWidth, npCounterRows*2)
	pal := s.pal
	glow := npLerp(pal.Bg, pal.Accent2, .12)
	for i := range p.px {
		p.px[i] = glow
	}
	digits := counterDigits(d)
	pulse := color.Color(pal.Accent2)
	if !ready {
		phase := float64(now.UnixNano()%int64(500*time.Millisecond)) / float64(500*time.Millisecond)
		pulse = npLerp(pal.Ghost, pal.Accent2, .5+.5*math.Cos(2*math.Pi*phase))
	}
	for i, dx := range [4]int{0, 5, 12, 17} {
		bits, oldBits := npDigitSegments[digits[i]], uint8(0)
		lit := pulse
		if !ready {
			bits = 1 << 6
		} else if state != nil && state.initialized && state.digits[i] == digits[i] && !state.changedAt[i].IsZero() {
			elapsed := now.Sub(state.changedAt[i])
			if elapsed >= 0 && elapsed < 33*time.Millisecond {
				lit = npLerp(pal.Accent2, pal.Text, .55)
			}
			if elapsed >= 0 && elapsed < 66*time.Millisecond {
				oldBits = npDigitSegments[state.prevDigits[i]] &^ bits
			}
		}
		// Paint in passes: shared segment endpoints must keep the brighter pixel.
		for _, layer := range []struct {
			bits uint8
			fg   color.Color
		}{{0x7f, pal.Ghost}, {oldBits, npLerp(pal.Ghost, pal.Accent2, .45)}, {bits, lit}} {
			for seg, pixels := range npSegmentPixels {
				if layer.bits&(1<<seg) != 0 {
					for _, xy := range pixels {
						p.set(dx+xy[0], xy[1], layer.fg)
					}
				}
			}
		}
	}
	p.set(10, 2, pulse)
	p.set(10, 4, pulse)
	s.blit(p, x, y)
}

func meterLevel(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return min(max(v, 0), 1)
}

func drawLEDMeter(s *npScreen, x, y, w int, label string, frame applemusic.MeterFrame) {
	if w <= 0 {
		return
	}
	s.text(x, y, label+"  dBFS", uv.Style{Fg: s.pal.Secondary}, w)
	n := max(0, min((w-2)/2, 16))
	if n >= 8 {
		for _, tick := range []struct {
			offset int
			label  string
		}{{0, "-48"}, {n - 1, "-24"}, {(n - 1) * 3 / 2, "-12"}, {(n - 1) * 2, "0"}} {
			s.text(x+2+tick.offset, y+1, tick.label, uv.Style{Fg: s.pal.Secondary}, w-2-tick.offset)
		}
	}
	for ch := range 2 {
		s.text(x, y+ch+2, []string{"L", "R"}[ch], uv.Style{Fg: s.pal.Secondary}, 1)
		for i := range n {
			fg := npLerp(s.pal.Rule, s.pal.Bg, .6)
			if frame.Ready && float64(i+1) <= meterLevel(frame.Levels[ch])*float64(n) {
				fg = meterColor(s.pal, -48+48*float64(i+1)/float64(n))
			}
			s.set(x+2+i*2, y+ch+2, "█", 1, uv.Style{Fg: fg})
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
	for row := 2 + phase; row < h; row += 5 {
		cells := make([]uv.Cell, w)
		for col := range w {
			if c := s.buf.CellAt(x+col, y+row); c != nil {
				cells[col] = *c
			}
		}
		shift := 0
		if phase == 0 && row%2 != 0 {
			shift = 2
		}
		s.fill(x, y+row, w, 1, s.pal.Bg)
		// Separated color ghosts precede the displaced scanline itself.
		for _, offset := range []int{-1, 1, 0} {
			for col, c := range cells {
				if c.Width != 1 || c.Content == "" || (c.Content == " " && (c.Style.Bg == nil || npSameColor(c.Style.Bg, s.pal.Bg))) {
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
					c.Style.Fg = fg
					if c.Style.Bg != nil {
						c.Style.Bg = npLerp(c.Style.Bg, fg, .25)
					}
				}
				s.set(x+dest, y+row, c.Content, 1, c.Style)
			}
		}
	}
}
