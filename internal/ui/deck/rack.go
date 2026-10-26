package deck

import (
	"image/color"
	"math"
)

// RackMedia selects the stacked physical media silhouette inside the 3D crate.
type RackMedia uint8

const (
	RackCassette RackMedia = iota
	RackVinyl
	RackFolder
)

// RackState describes the cassette storage rack in rack-local cells: two rows
// per slot, the spine row first and the slot lip below it.
type RackState struct {
	Padding  int        // rows above the first slot
	Slots    int        // complete slots
	Filled   int        // slots holding a cassette, counted from the top
	Out      int        // slot whose cassette has left its well; -1 for none
	Tab      int        // slot whose index tab is lit; -1 for none
	Media    RackMedia  // physical media kind stacked in the crate
	TabColor color.RGBA // the lit tab colour
	TabMix   float64    // 0 neutral .. 1 fully lit
	Flash    float64    // 0..1 snap highlight on the lit slot's lip
}

// RackRenderer ray-casts the storage rack as a relief seen slightly from the
// top left, lit by the same key light as the Walkman.
type RackRenderer struct {
	frame Frame
	key   rackKey
	valid bool
}

type rackKey struct {
	v   View
	pal Palette
	st  RackState
}

// The relief is drawn with an oblique projection: a point h columns in front
// of the back wall appears h·rackOX columns right and h·rackOY rows lower.
const (
	rackOX     = .22
	rackOY     = .30
	rackTop    = 1.7 // the highest relief, in columns
	rackStep   = .07
	rackShadow = .52
)

// RackLabel returns the cells of slot i's printed spine label.
func RackLabel(w int, st RackState, i int) (x, y, width int) {
	return 6, st.Padding + 2*i, max(0, w-12)
}

type rackMat uint8

const (
	rackBack rackMat = iota
	rackRail
	rackLip
	rackSpine
	rackLabel
	rackTabMat
	rackMouth
)

type rackScene struct {
	w, rows float64
	st      RackState
	aspect  float64
	colors  [7]rgb
	tab     rgb
	flash   rgb
}

// height returns the relief at a world point in cells, front faces placed so
// that they land exactly on their screen rows.
func (sc *rackScene) height(x, y float64) (float64, rackMat) {
	if x < 0 || y < 0 || x >= sc.w || y >= sc.rows {
		return 0, rackBack
	}
	// The frame: rounded rails with vertical grip ribs, side posts with ribs.
	const rail = 1.6
	edge := min(x, sc.w-x, y*sc.aspect, (sc.rows-y)*sc.aspect)
	if y < 1-rail*rackOY || y >= sc.rows-1-rail*rackOY || x < 2-rail*rackOX || x >= sc.w-2-rail*rackOX {
		h := rail * smoothstep(0, .9, edge)
		if y < 1 || y >= sc.rows-1 {
			if math.Mod(x, 2) > 1.35 {
				h -= .28
			}
		} else if math.Mod(y, 2) > 1.55 {
			h -= .28
		}
		return h, rackRail
	}
	if sc.st.Slots <= 0 {
		return 0, rackBack
	}
	rel := y - float64(sc.st.Padding)
	i := int(math.Floor(rel / 2))
	if rel < -1 || i >= sc.st.Slots {
		return 0, rackBack
	}
	v := rel - 2*float64(i) // 0..2 within the slot, screen rows
	// Lip: the slot's front wall with a rounded, lit top edge.
	const lip = .6
	if lv := v + lip*rackOY; lv >= 1.45 && lv < 1.9 && x >= 3-lip*rackOX && x < sc.w-3-lip*rackOX {
		return lip * (.55 + .45*smoothstep(1.45, 1.62, lv)), rackLip
	}
	if i < 0 {
		return 0, rackBack
	}
	filled := i < sc.st.Filled
	// Index tab: a raised ear on every filled slot, staggered across 3 columns for folders/vinyl.
	const tab = .95
	if tv, tx := v+tab*rackOY, x+tab*rackOX; filled && tv >= .05 && tv < 1.05 {
		if tx >= 3 && tx < 5 {
			return tab * smoothstep(.05, .3, tv) * smoothstep(3, 3.35, tx), rackTabMat
		}
		tabX := 7.0 + float64(i%3)*10.0
		if sc.st.Media != RackCassette && tv < .36 && tx >= tabX && tx < tabX+7.5 && tx < sc.w-6 {
			return tab * .92 * smoothstep(.05, .24, tv), rackTabMat
		}
	}
	if !filled || i == sc.st.Out {
		if x >= 3 && x < sc.w-3 && v < 1.05 {
			return 0, rackMouth
		}
		return 0, rackBack
	}
	// Spine: the stowed cassette's top edge, standing behind the lip.
	const spine = .7
	sv, sx := v+spine*rackOY, x+spine*rackOX
	if sv >= .12 && sv < 1.05 && sx >= 5.2 && sx < sc.w-4 {
		h := spine * (.7 + .3*smoothstep(.12, .34, sv))
		if sx >= 6 && sx < sc.w-6 && sv >= .34 {
			return h, rackLabel
		}
		return h, rackSpine
	}
	if x >= 3 && x < sc.w-3 && v < 1.05 {
		return 0, rackMouth
	}
	return 0, rackBack
}

// march finds the first relief surface along the oblique ray through a
// screen point.
func (sc *rackScene) march(fx, fy float64) (x, y, h float64, m rackMat) {
	prev := rackTop
	for t := rackTop; t >= 0; t -= rackStep {
		x, y = fx-t*rackOX, fy-t*rackOY
		hh, mm := sc.height(x, y)
		if hh >= t {
			lo, hi := t, prev
			for range 3 {
				mid := (lo + hi) / 2
				if hm, _ := sc.height(fx-mid*rackOX, fy-mid*rackOY); hm >= mid {
					lo = mid
				} else {
					hi = mid
				}
			}
			x, y = fx-lo*rackOX, fy-lo*rackOY
			_, mm = sc.height(x, y)
			return x, y, lo, mm
		}
		prev = t
	}
	x, y = fx, fy
	_, m = sc.height(x, y)
	return x, y, 0, m
}

func (sc *rackScene) shade(fx, fy float64) rgb {
	x, y, h, m := sc.march(fx, fy)
	const e = .14
	hx0, _ := sc.height(x-e, y)
	hx1, _ := sc.height(x+e, y)
	hy0, _ := sc.height(x, y-e/sc.aspect)
	hy1, _ := sc.height(x, y+e/sc.aspect)
	n := vec3{-(hx1 - hx0) / (2 * e), -(hy1 - hy0) / (2 * e), -1}.norm()
	diffuse := ambient + (1-ambient)*max(n.dot(keyLight), 0)/-keyLight.z
	// Shadow: walk towards the light and look for relief above the ray.
	lx, ly := keyLight.x/-keyLight.z, keyLight.y/-keyLight.z/sc.aspect
	for t := .12; h+t < rackTop; t += .12 {
		if hh, _ := sc.height(x+lx*t, y+ly*t); hh > h+t+.02 {
			diffuse *= rackShadow
			break
		}
	}
	albedo := sc.colors[m]
	if m == rackTabMat && sc.st.Tab >= 0 && int(math.Floor((y-float64(sc.st.Padding))/2)) == sc.st.Tab {
		albedo = albedo.mix(sc.tab, sc.st.TabMix)
	}
	if m == rackLip && sc.st.Flash > 0 && int(math.Floor((y-float64(sc.st.Padding))/2)) == sc.st.Tab {
		albedo = albedo.mix(sc.flash, sc.st.Flash*.6)
	}
	c := albedo.scale(diffuse)
	if n.y < -.4 { // rounded top edges catch a soft specular glint
		c = c.mix(sc.colors[rackLabel].mix(rgb{230, 240, 248}, .5), min(-n.y-.4, .5)*.5)
	}
	// Depth cueing: the back wall recedes into the well colour.
	if h < .05 && m == rackMouth {
		c = c.scale(.7)
	}
	return c
}

// RenderRack draws the rack into a v.W×v.H frame. The frame stays valid until
// the next call; an unchanged state returns it without redrawing.
func (r *RackRenderer) RenderRack(v View, pal Palette, st RackState) *Frame {
	if v.Tier == 0 || v.Tier == Braille {
		v.Tier = Quadrant
	}
	if v.Aspect <= 0 {
		v.Aspect = 2
	}
	key := rackKey{v, pal, st}
	if r.valid && r.key == key {
		return &r.frame
	}
	// Only the lit slot animates: when nothing else changed, redraw the bands
	// of the previous and the new lit slot over the cached cabinet.
	lo, hi := 0, v.H
	prev := r.key
	if r.valid && rackSameCabinet(prev, key) {
		lo, hi = rackBand(prev.st, st, v.H)
	}
	r.valid, r.key = true, key
	w, h := max(v.W, 0), max(v.H, 0)
	n := w * h
	if lo == 0 && hi == v.H {
		if cap(r.frame.Cells) < n {
			r.frame.Cells = make([]Cell, n)
		}
		r.frame.W, r.frame.H, r.frame.Cells = w, h, r.frame.Cells[:n]
		clear(r.frame.Cells)
	}
	if n == 0 || lo >= hi {
		return &r.frame
	}
	sc := rackScene{w: float64(w), rows: float64(h), st: st, aspect: v.Aspect,
		tab: rgbOf(st.TabColor), flash: rgbOf(pal.Hero)}
	sc.colors = [7]rgb{
		rackBack:   {12, 23, 34},
		rackRail:   {43, 64, 80},
		rackLip:    {55, 72, 84},
		rackSpine:  {38, 55, 69},
		rackLabel:  {29, 44, 58},
		rackTabMat: {42, 62, 78},
		rackMouth:  {10, 18, 28},
	}
	gc, gr := v.Tier.Grid()
	parallel(hi-lo, func(start, end int) {
		var pixels [8]rgb
		for y := lo + start; y < lo+end; y++ {
			for x := range w {
				for q := range gc * gr {
					fx := float64(x) + (float64(q%gc)+.5)/float64(gc)
					fy := float64(y) + (float64(q/gc)+.5)/float64(gr)
					pixels[q] = sc.shade(fx, fy)
				}
				r.frame.Cells[y*w+x] = fitCell(pixels[:gc*gr], v.Tier)
			}
		}
	})
	return &r.frame
}

// rackSameCabinet reports whether two rack states differ only in which slot
// is out and how it is lit.
func rackSameCabinet(a, b rackKey) bool {
	a.st.Out, a.st.Tab, a.st.TabColor, a.st.TabMix, a.st.Flash = 0, 0, color.RGBA{}, 0, 0
	b.st.Out, b.st.Tab, b.st.TabColor, b.st.TabMix, b.st.Flash = 0, 0, color.RGBA{}, 0, 0
	return a == b
}

// rackBand returns the rows covering the lit slots of two states, with a row
// of margin for the oblique relief and its shadow.
func rackBand(a, b RackState, rows int) (lo, hi int) {
	lo, hi = rows, 0
	for _, st := range [2]RackState{a, b} {
		for _, i := range [2]int{st.Out, st.Tab} {
			if i >= 0 {
				lo, hi = min(lo, st.Padding+2*i-1), max(hi, st.Padding+2*i+3)
			}
		}
	}
	return max(lo, 0), min(hi, rows)
}

// LastState returns the state of the last rendered frame.
func (r *RackRenderer) LastState() RackState { return r.key.st }
