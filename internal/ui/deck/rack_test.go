package deck

import (
	"image/color"
	"testing"
)

func rackTestView() View { return View{W: 60, H: 24, Tier: Quadrant, Aspect: 2} }

func TestRackStowedSlotsShareOneMaterial(t *testing.T) {
	var r RackRenderer
	st := RackState{Padding: 4, Slots: 8, Filled: 8, Out: -1, Tab: -1}
	f := r.RenderRack(rackTestView(), Palette{}, st)
	_, y0, _ := RackLabel(60, st, 0)
	_, y1, _ := RackLabel(60, st, 1)
	for x := 8; x < 50; x++ {
		a, b := f.Cells[y0*f.W+x], f.Cells[y1*f.W+x]
		if a != b {
			t.Fatalf("stowed spines differ at column %d: %+v vs %+v", x, a, b)
		}
	}
}

func TestRackLightsOnlyTheOutSlotTab(t *testing.T) {
	var r RackRenderer
	base := RackState{Padding: 4, Slots: 6, Filled: 6, Out: -1, Tab: -1}
	plain := append([]Cell(nil), r.RenderRack(rackTestView(), Palette{}, base).Cells...)
	lit := base
	lit.Out, lit.Tab, lit.TabColor, lit.TabMix = 2, 2, color.RGBA{255, 43, 214, 255}, 1
	f := r.RenderRack(rackTestView(), Palette{}, lit)
	for i := range plain {
		if plain[i] == f.Cells[i] {
			continue
		}
		y := i / f.W
		if y < base.Padding+3 || y > base.Padding+6 {
			t.Fatalf("lighting slot 2 changed row %d", y)
		}
	}
}

func TestRackEmptySlotsHaveNoSpine(t *testing.T) {
	var r RackRenderer
	st := RackState{Padding: 4, Slots: 6, Filled: 2, Out: -1, Tab: -1}
	f := r.RenderRack(rackTestView(), Palette{}, st)
	_, yFull, _ := RackLabel(60, st, 1)
	_, yEmpty, _ := RackLabel(60, st, 3)
	if f.Cells[yFull*f.W+20] == f.Cells[yEmpty*f.W+20] {
		t.Fatal("empty slot drawn like a stowed cassette")
	}
}

func TestRackReturnsCachedFrame(t *testing.T) {
	var r RackRenderer
	st := RackState{Padding: 4, Slots: 6, Filled: 6, Out: -1, Tab: -1}
	a := r.RenderRack(rackTestView(), Palette{}, st)
	a.Cells[0].Ch = "X"
	if b := r.RenderRack(rackTestView(), Palette{}, st); b.Cells[0].Ch != "X" {
		t.Fatal("unchanged rack was redrawn")
	}
}

func BenchmarkRackRender(b *testing.B) {
	var r RackRenderer
	v := View{W: 66, H: 30, Tier: Octant, Aspect: 2}
	for i := 0; i < b.N; i++ {
		r.valid = false
		r.RenderRack(v, Palette{}, RackState{Padding: 4, Slots: 12, Filled: 12, Out: 3, Tab: 3, TabMix: float64(i%2) * .5})
	}
}

func TestRackBandRedrawMatchesFullRedraw(t *testing.T) {
	v := rackTestView()
	base := RackState{Padding: 4, Slots: 8, Filled: 8, Out: 1, Tab: 1, TabColor: color.RGBA{0, 229, 255, 255}, TabMix: .3}
	next := base
	next.Out, next.Tab, next.TabMix, next.Flash = 4, 4, .8, .5
	var incremental, full RackRenderer
	incremental.RenderRack(v, Palette{}, base)
	got := incremental.RenderRack(v, Palette{}, next)
	want := full.RenderRack(v, Palette{}, next)
	for i := range want.Cells {
		if got.Cells[i] != want.Cells[i] {
			t.Fatalf("band redraw differs at cell %d (row %d)", i, i/v.W)
		}
	}
}

func BenchmarkRackRenderBand(b *testing.B) {
	var r RackRenderer
	v := View{W: 66, H: 30, Tier: Octant, Aspect: 2}
	for i := 0; i < b.N; i++ {
		r.RenderRack(v, Palette{}, RackState{Padding: 4, Slots: 12, Filled: 12, Out: 3, Tab: 3, TabMix: float64(i%100) / 100})
	}
}
