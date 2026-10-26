package deck

import (
	"fmt"
	"slices"
	"testing"
)

func TestWalkmanFlightPoseMatchesIndependentCassette(t *testing.T) {
	v := View{W: 140, H: 42, Tier: Quadrant, Fit: Rect{80, 2, 58, 34}}
	pose := RestPose()
	pose.X, pose.Y, pose.Scale = 2, -1, 1.2
	for _, source := range []Pose{{Pitch: 1.3}, {Yaw: .5, Pitch: .8, Roll: -.2}, {Scale: 1}} {
		t.Run(fmt.Sprint(source), func(t *testing.T) {
			st := WalkmanState{State: State{Style: 5, TrackNum: 42, Radii: ReelRadii(.4)}, Door: 1, Cassette: 1,
				FlightFrom: Rect{6, 10, 54, 14}, FlightPose: source, OnlyCassette: true}
			var renderer WalkmanRenderer
			frame := renderer.RenderWalkman(v, cassetteTestPalette, pose, st)
			cam := walkmanCamera(v)
			position, size := walkmanCassettePlacement(cam, pose, st)
			rot := rotation(source.Yaw, source.Pitch, source.Roll)
			center := rotation(pose.Yaw, pose.Pitch, pose.Roll).apply(position.mul(pose.Scale)).add(vec3{pose.X, pose.Y, 0})
			size *= pose.Scale
			m := cassetteMaterials(cassetteTestPalette, st.Style, st.TrackNum)
			gc, gr := v.Tier.Grid()
			oracle := scene{m: &m, rot: rot, inv: 1 / size, o: rot.applyT(vec3{0, 0, -camDist}.sub(center)).mul(1 / size),
				radii: st.Radii, lObj: rot.applyT(keyLight), detail: cam.k * size * camDist / (camDist + center.z) * float64(gr) / cam.aspect}
			hits := 0
			for y := range v.H {
				for x := range v.W {
					var pixels [8]rgb
					covered := false
					for q := range gc * gr {
						fx, fy := float64(x)+(float64(q%gc)+.5)/float64(gc), (float64(y)+(float64(q/gc)+.5)/float64(gr))*cam.aspect
						var hit bool
						pixels[q], _, hit = oracle.sample(vec3{(fx - cam.cx) / cam.focal, (fy - cam.cy) / cam.focal, 1})
						covered = covered || hit
					}
					want := Cell{}
					if covered {
						want = fitCell(pixels[:gc*gr], v.Tier)
						hits++
					}
					if got := frame.Cells[y*v.W+x]; got != want {
						t.Fatalf("source pose or lighting differs at (%d,%d): got %+v, want %+v", x, y, got, want)
					}
				}
			}
			if hits < 20 {
				t.Fatalf("only %d source cells checked", hits)
			}
		})
	}
}

func TestWalkmanOnlyCassetteMatchesSceneOutsideHousing(t *testing.T) {
	v := View{W: 140, H: 42, Tier: Sextant, Fit: Rect{80, 2, 58, 34}}
	st := WalkmanState{State: State{Style: 3, Radii: ReelRadii(.4)}, Door: 1, Cassette: 1,
		FlightFrom: Rect{6, 10, 54, 14}, FlightPose: Pose{Pitch: 1.1, Roll: -.08}}
	var r WalkmanRenderer
	full := slices.Clone(r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells)
	st.OnlyCassette = true
	only := r.RenderWalkman(v, cassetteTestPalette, RestPose(), st)
	hits := 0
	for y := range v.H {
		for x := range v.W {
			i := y*v.W + x
			if x < 65 {
				if only.Cells[i] != full[i] {
					t.Fatalf("rack-to-flight handoff changes source cell (%d,%d)", x, y)
				}
				if only.Cells[i].Ch != "" {
					hits++
				}
			} else if only.Cells[i].Ch != "" {
				t.Fatalf("OnlyCassette leaves device cell (%d,%d)", x, y)
			}
		}
	}
	if hits < 20 {
		t.Fatalf("only %d source cells checked", hits)
	}
	st.Cassette = -1
	for i, cell := range r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells {
		if cell.Ch != "" {
			t.Fatalf("empty OnlyCassette frame leaves cell %d", i)
		}
	}
}

func TestWalkmanFlightPoseDoesNotRotateHousing(t *testing.T) {
	v := View{W: 140, H: 42, Tier: Quadrant, Fit: Rect{80, 2, 58, 34}}
	st := WalkmanState{State: State{Style: 5, Radii: ReelRadii(.4)}, Door: .6, Cassette: 1,
		FlightFrom: Rect{6, 10, 54, 14}, Keys: KeyPlay}
	var r WalkmanRenderer
	base := slices.Clone(r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells)
	st.FlightPose = RestPose()
	if !slices.Equal(base, r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells) {
		t.Fatal("zero source pose does not preserve the device orientation")
	}
	st.FlightPose = Pose{Pitch: 1.3}
	changed := r.RenderWalkman(v, cassetteTestPalette, RestPose(), st)
	sourceChanges := 0
	for i, cell := range changed.Cells {
		if i%v.W >= 65 && cell != base[i] {
			t.Fatalf("source pose changes device cell (%d,%d)", i%v.W, i/v.W)
		}
		if i%v.W < 65 && cell != base[i] {
			sourceChanges++
		}
	}
	if sourceChanges < 20 {
		t.Fatalf("source pose changes only %d source cells", sourceChanges)
	}
}

func TestWalkmanFlightPoseSurvivesViewportCropping(t *testing.T) {
	global := View{W: 160, H: 48, Tier: Octant, Aspect: 2.4, Fit: Rect{88, 5, 62, 34}}
	local := global
	local.W, local.H, local.Fit.X, local.Fit.Y = 136, 40, global.Fit.X-12, global.Fit.Y-4
	for _, only := range []bool{false, true} {
		for _, progress := range []float64{0, .5, 1} {
			st := WalkmanState{State: State{Style: 2, Radii: ReelRadii(.4)}, Door: 1, Cassette: 1,
				FlightFrom: Rect{18, 12, 48, 12}, FlightPose: Pose{Pitch: 1.3, Roll: -.1}, Flight: progress, OnlyCassette: only}
			var a, b WalkmanRenderer
			full := a.RenderWalkman(global, cassetteTestPalette, RestPose(), st)
			st.FlightFrom.X, st.FlightFrom.Y = st.FlightFrom.X-12, st.FlightFrom.Y-4
			crop := b.RenderWalkman(local, cassetteTestPalette, RestPose(), st)
			for y := range local.H {
				for x := range local.W {
					if crop.Cells[y*local.W+x] != full.Cells[(y+4)*global.W+x+12] {
						t.Fatalf("OnlyCassette=%t Flight=%g: crop changes (%d,%d)", only, progress, x, y)
					}
				}
			}
		}
	}
}
