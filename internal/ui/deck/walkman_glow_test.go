package deck

import "testing"

func TestWalkmanPausedWindowGlowDims(t *testing.T) {
	m := cassetteMaterials(cassetteTestPalette, 2, 1)
	sc := walkmanScene{position: walkmanCassettePosition(0), scale: walkmanCassetteScale, cassette: scene{m: &m, rot: rotation(0, 0, 0), radii: ReelRadii(.4)}}
	rot, hinge := walkmanDoorTransform(0)
	p := rot.apply(vec3{(10.2 - 70) * walkmanSize.x, (25 - faceH/2) * walkmanSize.y, 0}).add(hinge)
	o := vec3{0, 0, -camDist}
	d := p.sub(o)
	view := d.norm().mul(-1)
	idle, _, ok := sc.sample(o, d, view)
	if !ok {
		t.Fatal("window ray missed")
	}
	sc.st.Keys = KeyPlay
	playing, _, _ := sc.sample(o, d, view)
	if playing.g <= idle.g || playing.b <= idle.b {
		t.Fatalf("paused window did not dim: idle=%+v play=%+v", idle, playing)
	}
}
