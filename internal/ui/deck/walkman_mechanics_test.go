package deck

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestWalkmanPartsChangeFrame(t *testing.T) {
	v := View{W: 96, H: 36, Tier: Quadrant}
	st := WalkmanState{State: State{Style: 2, TrackNum: 42, Radii: ReelRadii(.4)}}
	var r WalkmanRenderer
	base := slices.Clone(r.RenderWalkman(v, testPal, RestPose(), st).Cells)
	for _, change := range []struct {
		name string
		st   WalkmanState
	}{
		{"door", func() WalkmanState { s := st; s.Door = 1; return s }()},
		{"ejected", func() WalkmanState { s := st; s.Door, s.Cassette = 1, 1; return s }()},
		{"empty", func() WalkmanState { s := st; s.Cassette = -1; return s }()},
		{"style", func() WalkmanState { s := st; s.State.Style = 5; return s }()},
	} {
		t.Run(change.name, func(t *testing.T) {
			frame := r.RenderWalkman(v, testPal, RestPose(), change.st)
			if slices.Equal(base, frame.Cells) {
				t.Fatal("changed state did not redraw the corresponding part")
			}
			var fresh WalkmanRenderer
			if !slices.Equal(frame.Cells, fresh.RenderWalkman(v, testPal, RestPose(), change.st).Cells) {
				t.Fatal("cached state does not match a fresh render")
			}
		})
	}
	for key := KeyRew; key <= KeyEject; key <<= 1 {
		s := st
		s.Keys = key
		if slices.Equal(base, r.RenderWalkman(v, testPal, RestPose(), s).Cells) {
			t.Errorf("key %d does not depress", key)
		}
	}
}

func TestWalkmanWindowFitCentersSeatedReels(t *testing.T) {
	for _, v := range []View{{W: 140, H: 35, Tier: Quadrant}, {W: 120, H: 40, Tier: Sextant}, {W: 80, H: 24, Aspect: 2.4}, {W: 64, H: 18, Aspect: 1.6}} {
		t.Run(fmt.Sprintf("%dx%d-%g", v.W, v.H, v.Aspect), func(t *testing.T) {
			st := State{Style: 5, Radii: ReelRadii(.4), Angles: [2]float64{.6, 1.3}}
			zoom := .82
			near := v
			near.Fit = WalkmanWindowFit(v, zoom)
			var walkman WalkmanRenderer
			walkman.RenderWalkman(near, cassetteTestPalette, Pose{Scale: 1}, WalkmanState{State: st})
			cam := walkmanCamera(near)
			m := cassetteMaterials(cassetteTestPalette, st.Style, st.TrackNum)
			sc := scene{m: &m, rot: walkmanVerticalRot, radii: st.Radii, angles: st.Angles, detail: 2}
			ws := walkmanScene{st: WalkmanState{State: st}, position: walkmanCassettePosition(0), scale: walkmanCassetteScale, bodyRot: rotation(0, 0, 0), cassette: sc}
			projected := measureWalkmanSpindles(t, v, func(x, y float64) mat {
				d := vec3{(x - cam.cx) / cam.focal, (y*cam.aspect - cam.cy) / cam.focal, 1}
				_, material, _ := ws.sample(vec3{0, 0, -camDist}, d, d.norm().mul(-1))
				return material
			})
			k := min(float64(v.W)/42, float64(v.H)*cam.aspect/44) * zoom
			want := [2][2]float64{{float64(v.W) / 2, float64(v.H)/2 + 16*k/cam.aspect}, {float64(v.W) / 2, float64(v.H)/2 - 16*k/cam.aspect}}
			for i := range projected {
				for j, got := range projected[i] {
					if math.Abs(got-want[i][j]) > .25 {
						t.Errorf("reel %d coordinate %d: ray-traced %.3f, want %.3f", i, j, got, want[i][j])
					}
				}
			}
			t.Logf("seated vertical spindle centers: %v", projected)
		})
	}
}

func TestWalkmanHoverPlacementMatchesRayHits(t *testing.T) {
	for _, v := range []View{
		{W: 120, H: 40, Tier: Quadrant, Fit: Rect{2, 2, 116, 27}},
		{W: 80, H: 24, Tier: Sextant, Aspect: 2.4, Fit: Rect{2, 2, 76, 7}},
		{W: 100, H: 20, Tier: Quadrant, Aspect: 1.6, Fit: Rect{2, 2, 96, 13}},
	} {
		flying := v
		var pose Pose
		flying.Fit, pose = WalkmanHoverPlacement(v)
		var renderer Renderer
		renderer.Render(flying, testPal, pose, State{Style: 1, Radii: ReelRadii(.4)})
		cam, sc := walkmanCamera(v), renderer.pass.sc
		rest := RestPose()
		rot := rotation(rest.Yaw, rest.Pitch, rest.Roll)
		o := walkmanVerticalRot.applyT(rot.applyT(vec3{0, 0, -camDist}).sub(walkmanCassettePosition(1))).mul(1 / walkmanCassetteScale)
		hits := 0
		for y := .25; y < float64(v.H); y += .5 {
			for x := .25; x < float64(v.W); x += .5 {
				d := vec3{(x - cam.cx) / cam.focal, (y*cam.aspect - cam.cy) / cam.focal, 1}
				bodyHit, bodyOK := intersectShell(o, walkmanVerticalRot.applyT(rot.applyT(d)).mul(1/walkmanCassetteScale))
				fc := renderer.pass.cam
				fd := vec3{(x - fc.cx) / fc.focal, (y*fc.aspect - fc.cy) / fc.focal, 1}
				flyingHit, flyingOK := intersectShell(sc.o, sc.rot.applyT(fd).mul(sc.inv))
				if bodyOK != flyingOK || bodyOK && flyingHit.p.sub(bodyHit.p).dot(flyingHit.p.sub(bodyHit.p)) > 1e-12 {
					t.Fatalf("%dx%d: cassette jumps at handoff ray (%g,%g)", v.W, v.H, x, y)
				}
				if bodyOK {
					hits++
				}
			}
		}
		if hits == 0 {
			t.Fatal("hovering cassette is outside the viewport")
		}
	}
}

// Measure visible vertically-stacked spindle centers from production ray hits.
func measureWalkmanSpindles(t *testing.T, v View, sample func(float64, float64) mat) [2][2]float64 {
	t.Helper()
	var reels [2][2]float64
	for i := range reels {
		count, cx, cy := 0, 0.0, 0.0
		lo, hi := float64((1-i)*v.H)/2, float64((2-i)*v.H)/2
		for y := lo + .125; y < hi; y += .25 {
			for x := .125; x < float64(v.W); x += .25 {
				m := sample(x, y)
				if m == matSpindle || m == matTooth {
					cx, cy, count = cx+x, cy+y, count+1
				}
			}
		}
		if count == 0 {
			t.Fatalf("reel %d has no visible spindle", i)
		}
		reels[i] = [2]float64{cx / float64(count), cy / float64(count)}
	}
	return reels
}

func TestWalkmanDoorAndKeysHaveMechanicalTravel(t *testing.T) {
	o := vec3{0, 0, -camDist}
	hinge := vec3{(70 - faceW/2) * walkmanSize.x, 0, -depth * walkmanSize.z / 2}
	for _, amount := range []float64{0, .5, 1} {
		angle := amount * 35 * math.Pi / 180
		p := hinge.add(vec3{-20 * math.Cos(angle), 0, -20 * math.Sin(angle)})
		h, _, _, ok := walkmanDoorHit(o, p.sub(o), amount)
		if !ok || math.Abs(h.t-1) > 1e-9 || h.p.sub(p).dot(h.p.sub(p)) > 1e-12 {
			t.Fatalf("door %g did not intersect the rotated plane: %+v", amount, h)
		}
		if math.Abs(math.Acos(-h.n.z)-angle) > 1e-9 {
			t.Fatalf("door normal did not rotate %g degrees: %+v", amount*35, h.n)
		}
	}
	for i := range 5 {
		lo, hi := walkmanKeyBox(i, 0)
		pressedLo, pressedHi := walkmanKeyBox(i, 1<<i)
		if math.Abs(pressedLo.y-lo.y-1.2) > 1e-9 || math.Abs(pressedHi.y-hi.y-1.2) > 1e-9 || pressedLo.x != lo.x || pressedHi.z != hi.z {
			t.Fatalf("key %d did not sink 1.2 face units", i)
		}
		if otherLo, otherHi := walkmanKeyBox(i, 31^(1<<i)); lo != otherLo || hi != otherHi {
			t.Fatalf("unpressed key %d follows another key", i)
		}
	}
}

func TestWalkmanDoorEdgeMatchesRayHits(t *testing.T) {
	v := View{W: 120, H: 40, Tier: Quadrant, Fit: Rect{X: 2, Y: 2, W: 116, H: 27}}
	cam := walkmanCamera(v)
	for _, pose := range []Pose{RestPose(), {Yaw: .2, Pitch: -.1, Roll: .08, X: 2, Y: 1, Scale: 1.3}} {
		rot, scale := rotation(pose.Yaw, pose.Pitch, pose.Roll), scaleOf(pose)
		o := rot.applyT(vec3{pose.X, pose.Y, camDist}.mul(-1)).mul(1 / scale)
		for _, door := range []float64{0, .5, 1} {
			x0, y0, x1, y1 := WalkmanDoorEdge(v, pose, door)
			for i, p := range [][2]float64{{x0, y0}, {x1, y1}} {
				d := vec3{(p[0] - cam.cx) / cam.focal, (p[1]*cam.aspect - cam.cy) / cam.focal, 1}
				_, u, vv, hit := walkmanDoorHit(o, rot.applyT(d).mul(1/scale), door)
				if !hit || math.Abs(u-(10+float64(i)*56)) > 1e-8 || math.Abs(vv-19) > 1e-8 {
					t.Fatalf("door %g projected edge %d hit (%g,%g), visible %t", door, i, u, vv, hit)
				}
			}
		}
	}
}

func TestWalkmanEmptyBayHasNoMovingTape(t *testing.T) {
	v := View{W: 96, H: 36, Tier: Quadrant}
	st := WalkmanState{State: State{Style: 4, Radii: ReelRadii(.4)}, Cassette: -1}
	var r WalkmanRenderer
	a := slices.Clone(r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells)
	st.Angles, st.Radii = [2]float64{1, 2}, ReelRadii(.8)
	if !slices.Equal(a, r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells) {
		t.Fatal("empty bay shows a moving cassette")
	}
	st.Cassette = 0
	b := slices.Clone(r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells)
	st.Door, st.Cassette = 1, 1.35
	c := r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells
	firstCol := func(cells []Cell) int {
		minX := v.W
		for i, c := range cells {
			if c.Ch != "" {
				minX = min(minX, i%v.W)
			}
		}
		return minX
	}
	if firstCol(c) >= firstCol(b) {
		t.Fatal("ejected cassette did not travel out the left opening of the player")
	}
}

func TestWalkmanFitAndSmallFrames(t *testing.T) {
	st := WalkmanState{State: State{Style: 3, Radii: ReelRadii(.4)}, Door: .6, Cassette: .5, Keys: KeyPlay}
	for _, v := range []View{{W: 64, H: 24, Tier: Octant}, {W: 28, H: 9, Tier: Sextant}, {W: 1, H: 1}, {W: 0, H: 0}, {W: -2, H: 4}} {
		var r WalkmanRenderer
		frame := r.RenderWalkman(v, cassetteTestPalette, RestPose(), st)
		if len(frame.Cells) != max(v.W, 0)*max(v.H, 0) {
			t.Fatalf("incorrect frame extent for %+v", v)
		}
		for _, c := range frame.Cells {
			if c.Ch != "" && (ansi.WcWidth.StringWidth(c.Ch) != 1 || ansi.GraphemeWidth.StringWidth(c.Ch) != 1) {
				t.Fatalf("invalid cell %q", c.Ch)
			}
		}
	}
	v := View{W: 80, H: 30, Tier: Quadrant}
	st.Door, st.Cassette = 0, 0
	var r WalkmanRenderer
	full := slices.Clone(r.RenderWalkman(v, cassetteTestPalette, Pose{Scale: 1}, st).Cells)
	v.Fit = Rect{20, 7.5, 40, 15}
	small := r.RenderWalkman(v, cassetteTestPalette, Pose{Scale: 1}, st).Cells
	covered := func(cells []Cell) int {
		n := 0
		for _, c := range cells {
			if c.Ch != "" {
				n++
			}
		}
		return n
	}
	if covered(small) >= covered(full)/2 {
		t.Fatal("Walkman ignores the Fit box scale")
	}
}

func TestWalkmanPreview(t *testing.T) {
	dir := os.Getenv("TAPEFOX_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set TAPEFOX_PREVIEW_DIR to export real framebuffer previews")
	}
	for _, tt := range []struct {
		name string
		st   WalkmanState
	}{
		{"closed", WalkmanState{Keys: KeyPlay}},
		{"open", WalkmanState{Door: 1}},
		{"ejected", WalkmanState{Door: 1, Cassette: 1}},
		{"empty", WalkmanState{Door: 1, Cassette: -1}},
	} {
		tt.st.State = State{Style: 2, TrackNum: 42, Radii: ReelRadii(.4), Angles: [2]float64{.6, 1.3}}
		var r WalkmanRenderer
		frame := r.RenderWalkman(View{W: 120, H: 40, Tier: Quadrant, Fit: Rect{X: 2, Y: 2, W: 116, H: 27}}, cassetteTestPalette, RestPose(), tt.st)
		writeCassettePreview(t, filepath.Join(dir, "walkman-"+tt.name+".html"), "SONY WALKMAN · "+tt.name, frame)
	}
	v := View{W: 120, H: 36, Tier: Quadrant}
	st := State{Style: 2, TrackNum: 42, Radii: ReelRadii(.4), Angles: [2]float64{.6, 1.3}}
	var walkman WalkmanRenderer
	near := v
	near.Fit = WalkmanWindowFit(v, .82)
	writeCassettePreview(t, filepath.Join(dir, "walkman-near.html"), "SONY WALKMAN · local camera", walkman.RenderWalkman(near, cassetteTestPalette, Pose{Scale: 1}, WalkmanState{State: st}))
	flying := View{W: 120, H: 40, Tier: Quadrant, Fit: Rect{X: 2, Y: 2, W: 116, H: 27}}
	var pose Pose
	flying.Fit, pose = WalkmanHoverPlacement(flying)
	var cassette Renderer
	writeCassettePreview(t, filepath.Join(dir, "walkman-handoff-cassette.html"), "Independent cassette at handoff", cassette.Render(flying, cassetteTestPalette, pose, st))
	flight := View{W: 140, H: 36, Tier: Quadrant, Fit: Rect{72, 2, 66, 28}}
	for _, progress := range []float64{0, .2, .5, .8, 1} {
		moving := WalkmanState{State: st, Door: 1, Cassette: 1, FlightFrom: Rect{3, 8, 50, 2}, Flight: progress}
		name := fmt.Sprintf("walkman-flight-%03.0f", progress*100)
		writeCassettePreview(t, filepath.Join(dir, name+".html"), name, walkman.RenderWalkman(flight, cassetteTestPalette, RestPose(), moving))
	}
}
