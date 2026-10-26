package deck

import (
	"fmt"
	"testing"
)

func TestWalkmanKeepsUnbrandedBlueSilverBody(t *testing.T) {
	for _, tt := range []struct {
		name string
		u, v float64
		want rgb
	}{
		{"blue shell", 45, 20, rgb{34, 70, 123}},
		{"silver sideband", 70, 20, rgb{156, 168, 186}},
		{"upper plain fascia", 10.5, 10.5, rgb{34, 70, 123}},
		{"lower plain fascia", 8.5, 40.5, rgb{34, 70, 123}},
	} {
		h := shellHit{p: vec3{tt.u - faceW/2, tt.v - faceH/2, -depth / 2}, surf: surfFront}
		if got := walkmanBody(h); got != tt.want {
			t.Errorf("%s changed material: got %+v, want %+v", tt.name, got, tt.want)
		}
	}
}

func TestWalkmanFormerLabelsUseBodyMaterial(t *testing.T) {
	for _, region := range []struct {
		x, y       float64
		cols, rows int
	}{
		{10, 10.2, 128, 21},
		{8, 40.2, 174, 21},
	} {
		for i := range region.cols * region.rows {
			u := region.x + float64(i%region.cols)*.25
			v := region.y + float64(i/region.cols)*.25
			h := shellHit{p: vec3{u - faceW/2, v - faceH/2, -depth / 2}, surf: surfFront}
			want := rgb{34, 70, 123}
			if got := walkmanBody(h); got != want {
				t.Fatalf("fascia (%g,%g): want body material %+v, got %+v", u, v, want, got)
			}
		}
	}
}

func TestWalkmanDoorDoesNotCoverUpperFascia(t *testing.T) {
	m := cassetteMaterials(cassetteTestPalette, 2, 42)
	sc := walkmanScene{st: WalkmanState{Cassette: -1}, cassette: scene{m: &m, rot: rotation(0, 0, 0)}}
	o := vec3{0, 0, -camDist}
	// The plain upper fascia must stay above the door and cassette opening.
	p := vec3{(10.5 - faceW/2) * walkmanSize.x, (10.5 - faceH/2) * walkmanSize.y, -depth * walkmanSize.z / 2}
	d := p.sub(o)
	want := sc.lightBody(rgb{34, 70, 123}, vec3{0, 0, -1}, d.norm().mul(-1), .45)
	for _, amount := range []float64{0, .5, 1} {
		sc.st.Door = amount
		got, material, _ := sc.sample(o, d, d.norm().mul(-1))
		if material != matShell || got.dist2(want) > 1e-12 {
			t.Errorf("door %g obscures upper fascia: %v/%d, want %v", amount, got, material, want)
		}
	}
}

func TestWalkmanHandoffKeepsCassetteMaterials(t *testing.T) {
	v := View{W: 120, H: 40, Tier: Quadrant, Fit: Rect{2, 2, 116, 27}}
	for style := uint8(0); style <= 5; style++ {
		t.Run(fmt.Sprint(style), func(t *testing.T) {
			st := State{Style: style, TrackNum: 42, Radii: ReelRadii(.4), Angles: [2]float64{.6, 1.3}}
			flying := v
			var pose Pose
			flying.Fit, pose = WalkmanHoverPlacement(v)
			var standalone Renderer
			standalone.Render(flying, cassetteTestPalette, pose, st)
			rest := RestPose()
			rot := rotation(rest.Yaw, rest.Pitch, rest.Roll)
			ws := walkmanScene{st: WalkmanState{State: st, Door: 1, Cassette: 1}, position: walkmanCassettePosition(1), scale: walkmanCassetteScale, bodyRot: rot, cassette: standalone.pass.sc}
			var walkman WalkmanRenderer
			frame := walkman.RenderWalkman(v, cassetteTestPalette, rest, ws.st)
			pixels := make([][4]rgb, len(frame.Cells))
			covered := make([]uint8, len(frame.Cells))
			cam := walkmanCamera(v)
			o := rot.applyT(vec3{0, 0, -camDist})
			hits := 0
			for y := .25; y < float64(v.H); y += .5 {
				for x := .25; x < float64(v.W); x += .5 {
					d := vec3{(x - cam.cx) / cam.focal, (y*cam.aspect - cam.cy) / cam.focal, 1}
					localD := rot.applyT(d)
					h, ok := intersectShell(walkmanVerticalRot.applyT(o.sub(walkmanCassettePosition(1))).mul(1/walkmanCassetteScale), walkmanVerticalRot.applyT(localD).mul(1/walkmanCassetteScale))
					if !ok {
						continue
					}
					if body, ok := intersectShell(walkmanNormalize(o), walkmanNormalize(localD)); ok && body.t <= h.t {
						continue
					}
					if door, _, _, ok := walkmanDoorHit(o, localD, 1); ok && door.t <= h.t {
						continue
					}
					fc := standalone.pass.cam
					fd := vec3{(x - fc.cx) / fc.focal, (y*fc.aspect - fc.cy) / fc.focal, 1}
					want, wantMaterial, wantHit := standalone.pass.sc.sample(fd)
					got, gotMaterial, gotHit := ws.sample(o, localD, d.norm().mul(-1))
					if gotHit != wantHit || gotMaterial != wantMaterial || got.dist2(want) > 1e-12 {
						t.Fatalf("cassette changes at handoff (%g,%g): inserted %v/%d, flying %v/%d", x, y, got, gotMaterial, want, wantMaterial)
					}
					i, q := int(y)*v.W+int(x), int(y*2)%2*2+int(x*2)%2
					pixels[i][q], covered[i] = want, covered[i]|(1<<q)
					hits++
				}
			}
			if hits < 100 {
				t.Fatalf("only %d unoccluded cassette rays checked", hits)
			}
			for i, mask := range covered {
				if mask == 15 {
					if got, want := frame.Cells[i], fitCell(pixels[i][:], Quadrant); got != want {
						t.Fatalf("framebuffer changes at handoff cell (%d,%d): inserted %+v, flying %+v", i%v.W, i/v.W, got, want)
					}
				}
			}
		})
	}
}
