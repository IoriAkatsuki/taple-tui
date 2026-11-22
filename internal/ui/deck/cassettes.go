package deck

import (
	"image/color"
	"math"
	"strconv"
)

type Pattern uint8

const (
	PatternDefault Pattern = iota
	PatternLaser
	PatternGrid
	PatternNixie
	PatternXRay
	PatternOutrun
)

type CassetteStyle struct {
	Name         string
	Shell, Label color.RGBA
	Ink, Accent  color.RGBA
	Translucency float64
	Pattern      Pattern
	Glyph        string
	Model        string
}

var CassetteStyles = [5]CassetteStyle{
	{"SHIBUYA", color.RGBA{28, 22, 34, 255}, color.RGBA{22, 18, 28, 255}, color.RGBA{255, 107, 74, 255}, color.RGBA{46, 216, 198, 255}, .20, PatternLaser, "╱", "TYPE II · CrO₂"},
	{"CHROME", color.RGBA{26, 36, 48, 255}, color.RGBA{196, 202, 212, 255}, color.RGBA{46, 216, 198, 255}, color.RGBA{255, 107, 74, 255}, .26, PatternGrid, "┼", "METAL · TYPE IV"},
	{"NIXIE", color.RGBA{54, 34, 14, 255}, color.RGBA{22, 16, 10, 255}, color.RGBA{245, 185, 66, 255}, color.RGBA{255, 107, 74, 255}, .24, PatternNixie, "▪", "TYPE I · NORMAL"},
	{"X-RAY", color.RGBA{214, 208, 198, 255}, color.RGBA{244, 236, 222, 255}, color.RGBA{46, 216, 198, 255}, color.RGBA{255, 107, 74, 255}, .68, PatternXRay, "┊", "CrO₂ · C60"},
	{"CITYPOP", color.RGBA{236, 226, 208, 255}, color.RGBA{244, 236, 222, 255}, color.RGBA{255, 107, 74, 255}, color.RGBA{245, 185, 66, 255}, 0, PatternOutrun, "≡", "HIGH BIAS · C90"},
}

func cassetteMaterials(p Palette, style uint8, trackNum int) materials {
	m := materialsFor(p)
	if style == 0 || int(style) > len(CassetteStyles) {
		return m
	}
	s := CassetteStyles[style-1]
	m.shell, m.label, m.ink, m.stripe = rgbOf(s.Shell), rgbOf(s.Label), rgbOf(s.Ink), rgbOf(s.Accent)
	m.hero, m.glass, m.tape = rgb{217, 210, 236}, rgb{6, 34, 42}, rgb{43, 26, 31}
	m.hub, m.hole, m.clamp, m.screw = rgb{184, 188, 208}, rgb{4, 6, 10}, m.stripe, rgb{155, 160, 180}
	m.pattern, m.translucency = s.Pattern, s.Translucency
	if s.Pattern == PatternNixie {
		m.trackLabel = strconv.Itoa((trackNum%100+100)%100 + 100)[1:]
	}
	return m
}

func cassetteLabelAt(m *materials, u, v, detail float64) faceSample {
	c, material := m.label, matLabel
	if detail < lineDetail {
		return faceSample{albedo: c, n: flatOut, gloss: .1, m: material}
	}
	switch m.pattern {
	case PatternLaser:
		if math.Abs((u-18)+(v-14)*1.4) < 1.8 || math.Abs((u-26)+(v-14)*1.4) < 1.1 {
			c, material = m.ink, matInk
		}
	case PatternGrid:
		if v >= 10 && v <= 15 {
			c, material = m.ink, matStripe
		} else if v < 10 && (math.Mod(u, 6) < .7 || math.Mod(v, 2.5) < .7) {
			c, material = m.shell, matInk
		}
	case PatternNixie:
		if v >= 10.5 && v <= 13.5 {
			c, material = m.shell, matStripe
		} else if u >= 56 && u <= 68 && v >= 4.5 && v <= 10 && PixSample(m.trackLabel, u-56, v-4.5, 1, 5.5/7, 2) {
			c, material = m.ink, matInk
		}
	case PatternXRay:
		switch {
		case v >= 11 && v < 11.9:
			c, material = rgb{255, 107, 74}, matStripe
		case v >= 11.9 && v < 12.8:
			c, material = rgb{245, 185, 66}, matStripe
		case v >= 12.8 && v <= 13.7:
			c, material = m.ink, matInk
		}
	case PatternOutrun:
		if v >= 6 && v <= 14 && !(v >= 9.2 && v <= 9.8 || v >= 11.2 && v <= 11.8 || v >= 13 && v <= 13.5) {
			t := (v - 6) / 8
			orange := rgb{255, 118, 56}
			c = m.ink.mix(orange, t*2)
			if t > .5 {
				c = orange.mix(m.stripe, (t-.5)*2)
			}
			material = matStripe
		}
	}
	return faceSample{albedo: c, n: flatOut, gloss: .1, m: material}
}
