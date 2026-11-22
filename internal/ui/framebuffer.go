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

var npASCII [128]string

func init() {
	for i := range npASCII {
		npASCII[i] = string(byte(i))
	}
}

// npPalette holds the color roles of the framebuffer. Text is near
// neutral; Accent marks what is happening now and Accent2 switches that are on.
// Ghost, for unlit segments and lamps, is derived from Bg and Rule.
type npPalette struct {
	Bg, Surface, Text, Hero, Secondary, Rule, Ghost, Accent, Accent2 color.RGBA
}

type npCell struct {
	Content string
	Width   int
	Fg      color.RGBA
	Bg      color.RGBA
	HasFg   bool
	HasBg   bool
	Bold    bool
}

var npEmptyCell = npCell{Content: " ", Width: 1}

// npBuffer stores a flat grid of cells and serializes frames to SGR bytes
// without per-cell color interface boxing.
type npBuffer struct {
	w, h  int
	cells []npCell
	out   []byte
}

func (b *npBuffer) reset(w, h int) {
	w, h = max(w, 0), max(h, 0)
	need := w * h
	if cap(b.cells) >= need {
		b.cells = b.cells[:need]
	} else {
		b.cells = make([]npCell, need)
	}
	b.w, b.h = w, h
	b.Clear()
}

func (b *npBuffer) Clear() {
	for i := range b.cells {
		b.cells[i] = npEmptyCell
	}
}

// CellAt returns an ultraviolet view of the cell for tests and debug callers.
func (b *npBuffer) CellAt(x, y int) *uv.Cell {
	if b == nil || x < 0 || y < 0 || x >= b.w || y >= b.h {
		return nil
	}
	c := &b.cells[y*b.w+x]
	var st uv.Style
	if c.HasFg {
		st.Fg = c.Fg
	}
	if c.HasBg {
		st.Bg = c.Bg
	}
	if c.Bold {
		st.Attrs = uv.AttrBold
	}
	return &uv.Cell{
		Content: c.Content,
		Width:   c.Width,
		Style:   st,
	}
}

func clearWideOverlap(row []npCell, x int) {
	if x < 0 || x >= len(row) {
		return
	}
	prev := row[x]
	if pw := prev.Width; pw > 1 {
		for j := 0; j < pw && x+j < len(row); j++ {
			blank := prev
			blank.Content = " "
			blank.Width = 1
			row[x+j] = blank
		}
		return
	}
	if prev.Width == 0 {
		for j := 1; x-j >= 0; j++ {
			wide := row[x-j]
			if wide.Width > 1 && j < wide.Width {
				for k := 0; k < wide.Width && x-j+k < len(row); k++ {
					blank := wide
					blank.Content = " "
					blank.Width = 1
					row[x-j+k] = blank
				}
				return
			}
			if wide.Width > 0 {
				return
			}
		}
	}
}

func (b *npBuffer) setCell(x, y int, c npCell) {
	if b == nil || x < 0 || y < 0 || x >= b.w || y >= b.h {
		return
	}
	row := b.cells[y*b.w : (y+1)*b.w]
	span := max(c.Width, 1)
	for pos := x; pos < x+span && pos < b.w; pos++ {
		clearWideOverlap(row, pos)
	}
	cw := c.Width
	if cw > 1 && x+cw > b.w {
		blank := c
		blank.Content = " "
		blank.Width = 1
		for i := x; i < b.w; i++ {
			row[i] = blank
		}
		return
	}
	row[x] = c
	if cw > 1 {
		for j := 1; j < cw && x+j < b.w; j++ {
			row[x+j] = npCell{}
		}
	}
}

func appendUint8(b []byte, v uint8) []byte {
	if v >= 100 {
		b = append(b, '0'+v/100)
		v %= 100
		return append(b, '0'+v/10, '0'+v%10)
	}
	if v >= 10 {
		return append(b, '0'+v/10, '0'+v%10)
	}
	return append(b, '0'+v)
}

func (b *npBuffer) render() string {
	if b == nil || b.w <= 0 || b.h <= 0 {
		return ""
	}
	b.out = b.out[:0]
	type penState struct {
		fg, bg color.RGBA
		hasFg  bool
		hasBg  bool
		bold   bool
	}
	for y := 0; y < b.h; y++ {
		if y > 0 {
			b.out = append(b.out, '\n')
		}
		var pen penState
		pendingSpaces := 0
		row := b.cells[y*b.w : (y+1)*b.w]
		for x := 0; x < len(row); x++ {
			c := row[x]
			if c.Width == 0 && c.Content == "" {
				continue
			}
			if c.Content == " " && c.Width == 1 && !c.HasFg && !c.HasBg && !c.Bold {
				if pen.hasFg || pen.hasBg || pen.bold {
					b.out = append(b.out, "\x1b[m"...)
					pen = penState{}
				}
				pendingSpaces++
				continue
			}
			if pendingSpaces > 0 {
				for range pendingSpaces {
					b.out = append(b.out, ' ')
				}
				pendingSpaces = 0
			}
			if !c.HasFg && !c.HasBg && !c.Bold {
				if pen.hasFg || pen.hasBg || pen.bold {
					b.out = append(b.out, "\x1b[m"...)
					pen = penState{}
				}
			} else if c.HasFg != pen.hasFg || c.HasBg != pen.hasBg || c.Bold != pen.bold ||
				(c.HasFg && c.Fg != pen.fg) || (c.HasBg && c.Bg != pen.bg) {
				b.out = append(b.out, "\x1b["...)
				sep := false
				if pen.bold && !c.Bold {
					b.out = append(b.out, "22"...)
					sep = true
				}
				if pen.hasFg && !c.HasFg {
					if sep {
						b.out = append(b.out, ';')
					}
					b.out = append(b.out, "39"...)
					sep = true
				}
				if pen.hasBg && !c.HasBg {
					if sep {
						b.out = append(b.out, ';')
					}
					b.out = append(b.out, "49"...)
					sep = true
				}
				if c.HasFg && (!pen.hasFg || c.Fg != pen.fg) {
					if sep {
						b.out = append(b.out, ';')
					}
					b.out = append(b.out, "38;2;"...)
					b.out = appendUint8(b.out, c.Fg.R)
					b.out = append(b.out, ';')
					b.out = appendUint8(b.out, c.Fg.G)
					b.out = append(b.out, ';')
					b.out = appendUint8(b.out, c.Fg.B)
					sep = true
				}
				if c.HasBg && (!pen.hasBg || c.Bg != pen.bg) {
					if sep {
						b.out = append(b.out, ';')
					}
					b.out = append(b.out, "48;2;"...)
					b.out = appendUint8(b.out, c.Bg.R)
					b.out = append(b.out, ';')
					b.out = appendUint8(b.out, c.Bg.G)
					b.out = append(b.out, ';')
					b.out = appendUint8(b.out, c.Bg.B)
					sep = true
				}
				if !pen.bold && c.Bold {
					if sep {
						b.out = append(b.out, ';')
					}
					b.out = append(b.out, '1')
				}
				b.out = append(b.out, 'm')
				pen = penState{fg: c.Fg, bg: c.Bg, hasFg: c.HasFg, hasBg: c.HasBg, bold: c.Bold}
			}
			if c.Content == "" {
				b.out = append(b.out, ' ')
			} else {
				b.out = append(b.out, c.Content...)
			}
		}
		if pen.hasFg || pen.hasBg || pen.bold {
			b.out = append(b.out, "\x1b[m"...)
		}
	}
	return string(b.out)
}

// npScreen is the page framebuffer. Cells left without a background get the
// page background at the end, so widgets only set backgrounds they own.
type npScreen struct {
	buf       *npBuffer
	w, h      int
	pal       npPalette
	glitchBuf []npCell
}

func (s *npScreen) reset(w, h int) {
	if s.buf == nil {
		s.buf = &npBuffer{}
	}
	s.buf.reset(w, h)
	s.w, s.h = w, h
}

// render serializes the frame: exactly h lines, no trailing newline.
func (s *npScreen) render() string {
	return s.buf.render()
}

func npRGBA(c color.Color) (color.RGBA, bool) {
	if c == nil {
		return color.RGBA{}, false
	}
	if rgba, ok := c.(color.RGBA); ok {
		return rgba, true
	}
	if rgba, ok := c.(*color.RGBA); ok {
		if rgba == nil {
			return color.RGBA{}, false
		}
		return *rgba, true
	}
	r, g, b, a := c.RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}, true
}

func (s *npScreen) cellAt(x, y int) (npCell, bool) {
	if s.buf == nil || x < 0 || y < 0 || x >= s.w || y >= s.h {
		return npCell{}, false
	}
	return s.buf.cells[y*s.w+x], true
}

func (s *npScreen) setCell(x, y int, c npCell) {
	if s.buf == nil || x < 0 || y < 0 || y >= s.h || x+c.Width > s.w {
		return
	}
	s.buf.setCell(x, y, c)
}

func (s *npScreen) setRGBA(x, y int, content string, width int, fg, bg color.RGBA, hasFg, hasBg, bold bool) {
	if s.buf == nil || x < 0 || y < 0 || y >= s.h || x+width > s.w {
		return
	}
	s.buf.setCell(x, y, npCell{
		Content: content,
		Width:   width,
		Fg:      fg,
		Bg:      bg,
		HasFg:   hasFg,
		HasBg:   hasBg,
		Bold:    bold,
	})
}

func (s *npScreen) set(x, y int, content string, width int, st uv.Style) {
	if s.buf == nil || x < 0 || y < 0 || y >= s.h || x+width > s.w {
		return
	}
	fg, hasFg := npRGBA(st.Fg)
	bg, hasBg := npRGBA(st.Bg)
	s.buf.setCell(x, y, npCell{
		Content: content,
		Width:   width,
		Fg:      fg,
		Bg:      bg,
		HasFg:   hasFg,
		HasBg:   hasBg,
		Bold:    st.Attrs&uv.AttrBold != 0,
	})
}

func (s *npScreen) fillRGBA(x, y, w, h int, bg color.RGBA) {
	if s.buf == nil || w <= 0 || h <= 0 {
		return
	}
	x0, y0 := max(x, 0), max(y, 0)
	x1, y1 := min(x+w, s.w), min(y+h, s.h)
	cell := npCell{Content: " ", Width: 1, Bg: bg, HasBg: true}
	for yy := y0; yy < y1; yy++ {
		for xx := x0; xx < x1; xx++ {
			s.buf.setCell(xx, yy, cell)
		}
	}
}

// tintBg fills cells inside (x, y, w, h) that do not already own a background
// color, preserving all existing glyphs and foregrounds.
func (s *npScreen) tintBg(x, y, w, h int, bg color.RGBA) {
	if s.buf == nil || w <= 0 || h <= 0 {
		return
	}
	x0, y0 := max(x, 0), max(y, 0)
	x1, y1 := min(x+w, s.w), min(y+h, s.h)
	for yy := y0; yy < y1; yy++ {
		row := s.buf.cells[yy*s.w : (yy+1)*s.w]
		for xx := x0; xx < x1; xx++ {
			c := &row[xx]
			if c.Width == 0 && c.Content == "" {
				continue
			}
			if !c.HasBg {
				c.Bg = bg
				c.HasBg = true
			}
		}
	}
}

func (s *npScreen) fill(x, y, w, h int, bg color.Color) {
	if s.buf == nil || w <= 0 || h <= 0 {
		return
	}
	rgba, ok := npRGBA(bg)
	if ok {
		s.fillRGBA(x, y, w, h, rgba)
		return
	}
	x0, y0 := max(x, 0), max(y, 0)
	x1, y1 := min(x+w, s.w), min(y+h, s.h)
	for yy := y0; yy < y1; yy++ {
		for xx := x0; xx < x1; xx++ {
			s.buf.setCell(xx, yy, npEmptyCell)
		}
	}
}

func (s *npScreen) repeatRGBA(x, y, n int, ch string, fg, bg color.RGBA, hasFg, hasBg, bold bool) {
	for i := range max(n, 0) {
		s.setRGBA(x+i, y, ch, 1, fg, bg, hasFg, hasBg, bold)
	}
}

func (s *npScreen) repeat(x, y, n int, ch string, st uv.Style) {
	fg, hasFg := npRGBA(st.Fg)
	bg, hasBg := npRGBA(st.Bg)
	s.repeatRGBA(x, y, n, ch, fg, bg, hasFg, hasBg, st.Attrs&uv.AttrBold != 0)
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

func npIsPrintableASCII(str string) bool {
	for i := 0; i < len(str); i++ {
		b := str[i]
		if b < 0x20 || b >= 0x7f {
			return false
		}
	}
	return true
}

func npSanitize(str string) string {
	clean := true
	for i := 0; i < len(str); {
		b := str[i]
		if b < 0x80 {
			if b < 0x20 || b == 0x7f {
				clean = false
				break
			}
			i++
			continue
		}
		r, size := utf8.DecodeRuneInString(str[i:])
		if unicode.IsControl(r) {
			clean = false
			break
		}
		i += size
	}
	if clean {
		return str
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, str)
}

func npWidth(str string) int {
	if npIsPrintableASCII(str) {
		return len(str)
	}
	w := 0
	for len(str) > 0 {
		_, cw, n := npNextCluster(str)
		w += cw
		str = str[n:]
	}
	return w
}

func (s *npScreen) textRGBA(x, y int, str string, fg, bg color.RGBA, hasFg, hasBg, bold bool, maxW int) int {
	if maxW <= 0 || str == "" {
		return 0
	}
	if npIsPrintableASCII(str) {
		limit := len(str)
		cut := limit > maxW
		if cut {
			limit = maxW - 1
		}
		for col := 0; col < limit; col++ {
			s.setRGBA(x+col, y, npASCII[str[col]], 1, fg, bg, hasFg, hasBg, bold)
		}
		if cut {
			s.setRGBA(x+limit, y, "…", 1, fg, bg, hasFg, hasBg, bold)
			return limit + 1
		}
		return limit
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
		s.setRGBA(x+col, y, cl, w, fg, bg, hasFg, hasBg, bold)
		col += w
	}
	if cut {
		s.setRGBA(x+col, y, "…", 1, fg, bg, hasFg, hasBg, bold)
		col++
	}
	return col
}

// text draws str at (x, y) within maxW columns, ending with "…" when cut, and
// returns the number of columns used.
func (s *npScreen) text(x, y int, str string, st uv.Style, maxW int) int {
	if maxW <= 0 || str == "" {
		return 0
	}
	fg, hasFg := npRGBA(st.Fg)
	bg, hasBg := npRGBA(st.Bg)
	return s.textRGBA(x, y, str, fg, bg, hasFg, hasBg, st.Attrs&uv.AttrBold != 0, maxW)
}

// npPixels is a pixel canvas drawn with half blocks: every cell holds a top
// and a bottom pixel, which are roughly square in a terminal font.
type npPixels struct {
	w, h  int
	px    []color.RGBA
	valid []bool
}

func newNPPixels(w, h int) *npPixels {
	h += h % 2
	n := w * h
	return &npPixels{w: w, h: h, px: make([]color.RGBA, n), valid: make([]bool, n)}
}

func (p *npPixels) set(x, y int, c color.RGBA) {
	if x >= 0 && y >= 0 && x < p.w && y < p.h {
		i := y*p.w + x
		p.px[i] = c
		p.valid[i] = true
	}
}

func (p *npPixels) at(x, y int) (color.RGBA, bool) {
	if x < 0 || y < 0 || x >= p.w || y >= p.h {
		return color.RGBA{}, false
	}
	i := y*p.w + x
	return p.px[i], p.valid[i]
}

func npSameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

func npSameRGBA(a, b color.RGBA) bool {
	return a.R == b.R && a.G == b.G && a.B == b.B
}

// blit copies the canvas to the screen; transparent pixels keep the screen
// background.
func (s *npScreen) blit(p *npPixels, x0, y0 int) {
	for cy := 0; cy < p.h/2; cy++ {
		for cx := 0; cx < p.w; cx++ {
			top, topOK := p.at(cx, cy*2)
			bot, botOK := p.at(cx, cy*2+1)
			switch {
			case !topOK && !botOK:
			case !botOK:
				s.setRGBA(x0+cx, y0+cy, "▀", 1, top, color.RGBA{}, true, false, false)
			case !topOK:
				s.setRGBA(x0+cx, y0+cy, "▄", 1, bot, color.RGBA{}, true, false, false)
			case top == bot:
				s.setRGBA(x0+cx, y0+cy, "█", 1, top, color.RGBA{}, true, false, false)
			default:
				s.setRGBA(x0+cx, y0+cy, "▀", 1, top, bot, true, true, false)
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

func npLerpRGBA(from, to color.RGBA, t float64) color.RGBA {
	t = min(max(t, 0), 1)
	mix := func(a, b uint8) uint8 {
		return uint8(float64(a)*(1-t) + float64(b)*t)
	}
	return color.RGBA{mix(from.R, to.R), mix(from.G, to.G), mix(from.B, to.B), 0xFF}
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
	if s.buf == nil {
		return
	}
	cells := s.buf.cells
	for i := range cells {
		c := &cells[i]
		if c.Width == 0 && c.Content == "" {
			continue // wide-cell placeholder
		}
		if !c.HasBg {
			c.Bg = s.pal.Bg
			c.HasBg = true
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
		if cx < 0 || cx >= s.w || cy < 0 || cy >= s.h {
			continue
		}
		if c.Shadow > 0 && s.shade(cx, cy, c.Shadow) {
			continue
		}
		hasFg := c.Ch != " "
		var fg color.RGBA
		if hasFg {
			fg = c.Fg
		}
		s.setRGBA(cx, cy, c.Ch, 1, fg, c.Bg, hasFg, true, false)
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
		if cx < 0 || cx >= s.w || cy < 0 || cy >= s.h {
			continue
		}
		if c.Shadow > 0 && s.shade(cx, cy, c.Shadow) {
			continue
		}
		var under color.RGBA
		var hasUnder bool
		if ex, ok := s.cellAt(cx, cy); ok && ex.HasBg {
			under, hasUnder = ex.Bg, true
		}
		bg, fg := c.Bg, c.Fg
		if hasUnder && npNearBg(c.Bg, s.pal.Bg) {
			bg = under
		}
		if hasUnder && npNearBg(c.Fg, s.pal.Bg) {
			fg = under
		}
		hasFg := c.Ch != " "
		if !hasFg {
			fg = color.RGBA{}
		}
		s.setRGBA(cx, cy, c.Ch, 1, fg, bg, hasFg, true, false)
	}
}

func npNearBg(a, b color.RGBA) bool {
	dr, dg, db := int(a.R)-int(b.R), int(a.G)-int(b.G), int(a.B)-int(b.B)
	return dr*dr+dg*dg+db*db <= 36
}

// shade darkens the cell at (x, y) by k and reports whether it did; blank
// cells on the page background are left for the shadow's own glyphs.
func (s *npScreen) shade(x, y int, k float64) bool {
	if s.buf == nil || x < 0 || y < 0 || x >= s.w || y >= s.h {
		return true
	}
	c := &s.buf.cells[y*s.w+x]
	switch {
	case c.Width == 0: // the tail of a wide character: its head carries the color
		return true
	case (c.Content == " " || c.Content == "") && (!c.HasBg || npSameRGBA(c.Bg, s.pal.Bg)):
		return false
	}
	black := color.RGBA{A: 0xff}
	if c.HasFg {
		c.Fg = npLerpRGBA(c.Fg, black, k)
	}
	bg := c.Bg
	if !c.HasBg {
		bg = s.pal.Bg
	}
	c.Bg = npLerpRGBA(bg, black, k)
	c.HasBg = true
	return true
}

func npClock(d time.Duration) string {
	sec := max(int(d/time.Second), 0)
	return fmt.Sprintf("%02d:%02d", sec/60, sec%60)
}
