package ui

import (
	"fmt"
	"image/color"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

// npPalette holds the color roles of the framebuffer. Text is near
// neutral; Accent marks what is happening now and Accent2 switches that are on.
// Ghost, for unlit segments and lamps, is derived from Bg and Rule.
type npPalette struct {
	Bg, Surface, Text, Hero, Secondary, Rule, Ghost, Accent, Accent2 color.RGBA
}

// npScreen is the page framebuffer. Cells left without a background get the
// page background at the end, so widgets only set backgrounds they own.
type npScreen struct {
	buf  *uv.Buffer
	w, h int
	pal  npPalette
}

func (s *npScreen) reset(w, h int) {
	if s.buf == nil {
		s.buf = uv.NewBuffer(w, h)
	} else {
		s.buf.Resize(w, h)
	}
	s.w, s.h = w, h
	s.buf.Clear()
}

// render serializes the frame: exactly h lines, no trailing newline.
func (s *npScreen) render() string {
	return s.buf.Render()
}

func (s *npScreen) set(x, y int, content string, width int, st uv.Style) {
	if x < 0 || y < 0 || y >= s.h || x+width > s.w {
		return
	}
	s.buf.SetCell(x, y, &uv.Cell{Content: content, Width: width, Style: st})
}

func (s *npScreen) fill(x, y, w, h int, bg color.Color) {
	for yy := y; yy < y+h; yy++ {
		for xx := x; xx < x+w; xx++ {
			s.set(xx, yy, " ", 1, uv.Style{Bg: bg})
		}
	}
}

func (s *npScreen) repeat(x, y, n int, ch string, st uv.Style) {
	for i := range max(n, 0) {
		s.set(x+i, y, ch, 1, st)
	}
}

// npNextCluster returns the first grapheme cluster of str, normalized so that
// grapheme and wcwidth measurements agree. The compositor measures graphemes
// while the renderer pads with wcwidth; a disagreement shifts the whole frame.
func npNextCluster(str string) (cluster string, width, size int) {
	cl, gw := ansi.FirstGraphemeCluster(str, ansi.GraphemeWidth)
	if len(cl) == 0 {
		_, size = utf8.DecodeRuneInString(str)
		return "", 0, size
	}
	if gw == ansi.WcWidth.StringWidth(cl) {
		return cl, gw, len(cl)
	}
	r, _ := utf8.DecodeRuneInString(cl)
	if rs := string(r); ansi.GraphemeWidth.StringWidth(rs) == ansi.WcWidth.StringWidth(rs) {
		return rs, ansi.WcWidth.StringWidth(rs), len(cl)
	}
	return "?", 1, len(cl)
}

func npSanitize(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, str)
}

func npWidth(str string) int {
	w := 0
	for len(str) > 0 {
		_, cw, n := npNextCluster(str)
		w += cw
		str = str[n:]
	}
	return w
}

// text draws str at (x, y) within maxW columns, ending with "…" when cut, and
// returns the number of columns used.
func (s *npScreen) text(x, y int, str string, st uv.Style, maxW int) int {
	if maxW <= 0 || str == "" {
		return 0
	}
	str = npSanitize(str)
	limit, cut := maxW, npWidth(str) > maxW
	if cut {
		limit = maxW - 1
	}
	col := 0
	for len(str) > 0 {
		cl, w, n := npNextCluster(str)
		str = str[n:]
		if w == 0 {
			continue
		}
		if col+w > limit {
			break
		}
		s.set(x+col, y, cl, w, st)
		col += w
	}
	if cut {
		s.set(x+col, y, "…", 1, st)
		col++
	}
	return col
}

// npPixels is a pixel canvas drawn with half blocks: every cell holds a top
// and a bottom pixel, which are roughly square in a terminal font.
type npPixels struct {
	w, h int
	px   []color.Color
}

func newNPPixels(w, h int) *npPixels {
	h += h % 2
	return &npPixels{w: w, h: h, px: make([]color.Color, w*h)}
}

func (p *npPixels) set(x, y int, c color.Color) {
	if x >= 0 && y >= 0 && x < p.w && y < p.h {
		p.px[y*p.w+x] = c
	}
}

func (p *npPixels) at(x, y int) color.Color {
	if x < 0 || y < 0 || x >= p.w || y >= p.h {
		return nil
	}
	return p.px[y*p.w+x]
}

func npSameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

// blit copies the canvas to the screen; transparent pixels keep the screen
// background.
func (s *npScreen) blit(p *npPixels, x0, y0 int) {
	for cy := 0; cy < p.h/2; cy++ {
		for cx := 0; cx < p.w; cx++ {
			top, bot := p.at(cx, cy*2), p.at(cx, cy*2+1)
			switch {
			case top == nil && bot == nil:
			case bot == nil:
				s.set(x0+cx, y0+cy, "▀", 1, uv.Style{Fg: top})
			case top == nil:
				s.set(x0+cx, y0+cy, "▄", 1, uv.Style{Fg: bot})
			case npSameColor(top, bot):
				s.set(x0+cx, y0+cy, "█", 1, uv.Style{Fg: top})
			default:
				s.set(x0+cx, y0+cy, "▀", 1, uv.Style{Fg: top, Bg: bot})
			}
		}
	}
}

// npCounterWidth and npCounterRows are the size of "mm:ss" in seven-segment
// digits of 4×7 pixels.
const (
	npCounterWidth = 21
	npCounterRows  = 4
)

// npDigitSegments holds the lit segments of each digit, bit 0 = a … bit 6 = g.
var npDigitSegments = [10]uint8{0x3F, 0x06, 0x5B, 0x4F, 0x66, 0x6D, 0x7D, 0x07, 0x7F, 0x6F}

// npSegmentPixels maps the segments a–g onto a 4×7 pixel grid.
var npSegmentPixels = [7][4][2]int{
	{{0, 0}, {1, 0}, {2, 0}, {3, 0}}, // a
	{{3, 0}, {3, 1}, {3, 2}, {3, 3}}, // b
	{{3, 3}, {3, 4}, {3, 5}, {3, 6}}, // c
	{{0, 6}, {1, 6}, {2, 6}, {3, 6}}, // d
	{{0, 3}, {0, 4}, {0, 5}, {0, 6}}, // e
	{{0, 0}, {0, 1}, {0, 2}, {0, 3}}, // f
	{{0, 3}, {1, 3}, {2, 3}, {3, 3}}, // g
}

func npLerp(from, to color.Color, t float64) color.Color {
	t = min(max(t, 0), 1)
	r1, g1, b1, _ := from.RGBA()
	r2, g2, b2, _ := to.RGBA()
	mix := func(a, b uint32) uint8 {
		return uint8((float64(a)*(1-t) + float64(b)*t) / 257)
	}
	return color.RGBA{mix(r1, r2), mix(g1, g2), mix(b1, b2), 0xFF}
}

// fillBackground gives every cell without a background the page background.
func (s *npScreen) fillBackground() {
	for y := 0; y < s.h; y++ {
		line := s.buf.Line(y)
		for x := range line {
			if line[x].Width == 0 && line[x].Content == "" {
				continue // wide-cell placeholder
			}
			if line[x].Style.Bg == nil {
				line[x].Style.Bg = s.pal.Bg
			}
		}
	}
}

// drawDeck copies the cells a cassette frame draws onto the screen at (x, y).
// Where only its shadow falls on something already drawn, that is darkened
// instead, so a cassette passing over text does not cut a hole in it.
func (s *npScreen) drawDeck(f *deck.Frame, x, y int) {
	if f == nil {
		return
	}
	for i := range f.Cells {
		c := &f.Cells[i]
		if c.Ch == "" {
			continue
		}
		cx, cy := x+i%f.W, y+i/f.W
		if c.Shadow > 0 && s.shade(cx, cy, c.Shadow) {
			continue
		}
		// The frame outlives serialization; borrowing avoids per-cell color boxes.
		st := uv.Style{Bg: &c.Bg}
		if c.Ch != " " {
			st.Fg = &c.Fg
		}
		s.set(cx, cy, c.Ch, 1, st)
	}
}

// drawDeckOverStage composites a 3D frame onto a painted stage backdrop,
// replacing missed palette-background clusters on silhouette edges with the
// underlying stage cell's color so warm desk lighting shows no dark halo.
func (s *npScreen) drawDeckOverStage(f *deck.Frame, x, y int) {
	if f == nil {
		return
	}
	for i := range f.Cells {
		c := &f.Cells[i]
		if c.Ch == "" {
			continue
		}
		cx, cy := x+i%f.W, y+i/f.W
		if c.Shadow > 0 && s.shade(cx, cy, c.Shadow) {
			continue
		}
		var under color.Color
		if ex := s.buf.CellAt(cx, cy); ex != nil && ex.Style.Bg != nil {
			under = ex.Style.Bg
		}
		bg, fg := color.Color(&c.Bg), color.Color(&c.Fg)
		if under != nil && npNearBg(c.Bg, s.pal.Bg) {
			bg = under
		}
		if under != nil && npNearBg(c.Fg, s.pal.Bg) {
			fg = under
		}
		st := uv.Style{Bg: bg}
		if c.Ch != " " {
			st.Fg = fg
		}
		s.set(cx, cy, c.Ch, 1, st)
	}
}

func npNearBg(a, b color.RGBA) bool {
	dr, dg, db := int(a.R)-int(b.R), int(a.G)-int(b.G), int(a.B)-int(b.B)
	return dr*dr+dg*dg+db*db <= 36
}

// shade darkens the cell at (x, y) by k and reports whether it did; blank
// cells on the page background are left for the shadow's own glyphs.
func (s *npScreen) shade(x, y int, k float64) bool {
	c := s.buf.CellAt(x, y)
	switch {
	case c == nil:
		return true
	case c.Width == 0: // the tail of a wide character: its head carries the color
		return true
	case (c.Content == " " || c.Content == "") && (c.Style.Bg == nil || npSameColor(c.Style.Bg, s.pal.Bg)):
		return false
	}
	black := color.RGBA{A: 0xff}
	if c.Style.Fg != nil {
		c.Style.Fg = npLerp(c.Style.Fg, black, k)
	}
	bg := c.Style.Bg
	if bg == nil {
		bg = s.pal.Bg
	}
	c.Style.Bg = npLerp(bg, black, k)
	return true
}

func npClock(d time.Duration) string {
	sec := max(int(d/time.Second), 0)
	return fmt.Sprintf("%02d:%02d", sec/60, sec%60)
}
