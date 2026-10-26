package ui

import (
	"image/color"
	"math"
	"strings"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

func widgetScreen(w, h int) *npScreen {
	s := &npScreen{pal: npPalette{
		Bg: color.RGBA{A: 255}, Text: color.RGBA{200, 200, 200, 255},
		Rule: color.RGBA{60, 60, 60, 255}, Hero: color.RGBA{255, 0, 127, 255},
		Accent: color.RGBA{0, 220, 255, 255}, Accent2: color.RGBA{255, 180, 0, 255},
	}}
	s.reset(w, h)
	return s
}

func TestFramebufferClipsWideTextAndControlCharacters(t *testing.T) {
	for _, size := range [][2]int{{1, 1}, {5, 2}, {60, 20}, {140, 45}} {
		s := widgetScreen(size[0], size[1])
		s.text(-1, 0, "夜行 ❤️ A\x1b[31m\n文本", uv.Style{Fg: s.pal.Text}, size[0]+3)
		s.text(size[0]-1, size[1]-1, "界", uv.Style{Fg: s.pal.Text}, 2)
		s.fillBackground()
		lines := strings.Split(s.render(), "\n")
		if len(lines) != size[1] {
			t.Fatalf("%v: %d rows", size, len(lines))
		}
		for _, line := range lines {
			if ansi.WcWidth.StringWidth(line) != size[0] || ansi.GraphemeWidth.StringWidth(line) != size[0] {
				t.Fatalf("%v: invalid row width: %q", size, line)
			}
		}
	}
}

func TestMeterUsesOnlyValidMeasuredSignal(t *testing.T) {
	s := widgetScreen(36, 4)
	drawLEDMeter(s, 0, 0, 36, "VU OFF", applemusic.MeterFrame{Levels: [2]float64{1, 1}})
	idle := s.buf.CellAt(2, 2).Style.Fg
	drawLEDMeter(s, 0, 0, 36, "SYSTEM MIX", applemusic.MeterFrame{Ready: true, Levels: [2]float64{1, math.NaN()}})
	if npSameColor(s.buf.CellAt(2, 2).Style.Fg, idle) {
		t.Fatal("measured channel was not lit")
	}
	if !npSameColor(s.buf.CellAt(2, 3).Style.Fg, idle) {
		t.Fatal("invalid sample lit a meter")
	}
	if s.buf.CellAt(3, 2).Content != " " {
		t.Fatal("LED segments must retain a one-cell gap")
	}
}

func TestCounterDrawDoesNotAdvanceState(t *testing.T) {
	now := time.Unix(100, 0)
	state := CounterState{}
	state.Update(now, 59*time.Second)
	state.Update(now.Add(time.Second), time.Minute)
	before := state
	s := widgetScreen(21, 4)
	DrawNixieCounter(s, 0, 0, time.Minute, true, now.Add(time.Second+20*time.Millisecond), &state)
	if state != before {
		t.Fatal("rendering advanced counter state")
	}
}
