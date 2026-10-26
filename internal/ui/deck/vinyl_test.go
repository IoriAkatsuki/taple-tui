package deck

import "testing"

func TestVinylStyleForIDIsDeterministic(t *testing.T) {
	a := VinylStyleForID("album-1001")
	b := VinylStyleForID("album-1001")
	if a != b || a < 1 || int(a) > len(VinylLabels) {
		t.Fatalf("unexpected style %d vs %d", a, b)
	}
}

func TestVinylRendererDrawsGroovesAndLabel(t *testing.T) {
	var r VinylRenderer
	v := View{W: 48, H: 18, Tier: Quadrant, Aspect: 2, Fit: Rect{X: 8, Y: 2, W: 32, H: 14}}
	f := r.RenderVinyl(v, Palette{}, Pose{Yaw: -.18, Pitch: .32, Scale: 1}, VinylState{Style: 2, Peek: .5})
	filled := 0
	for _, c := range f.Cells {
		if c.Ch != "" {
			filled++
		}
	}
	if filled < 40 {
		t.Fatalf("vinyl disc rendered too few cells: %d", filled)
	}
	f.Cells[0].Ch = "Z"
	if cached := r.RenderVinyl(v, Palette{}, Pose{Yaw: -.18, Pitch: .32, Scale: 1}, VinylState{Style: 2, Peek: .5}); cached.Cells[0].Ch != "Z" {
		t.Fatal("unchanged vinyl frame was not cached")
	}
}
