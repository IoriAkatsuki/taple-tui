package deck

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"testing"
)

func TestWalkmanStorageCoverHidesStyleAndTrackNumber(t *testing.T) {
	v := View{W: 140, H: 48, Tier: Octant, Fit: Rect{80, 2, 58, 34}}
	for _, pose := range []Pose{{Scale: 1}, {Yaw: .5, Pitch: .8, Roll: -.2}, {Yaw: 2.8, Pitch: -.2}} {
		st := WalkmanState{State: State{Radii: ReelRadii(.4), Angles: [2]float64{.6, 1.3}},
			Cassette: 1, FlightFrom: Rect{4, 6, 68, 28}, FlightPose: pose, OnlyCassette: true, StorageCover: 1}
		var r WalkmanRenderer
		want := slices.Clone(r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells)
		for _, style := range []uint8{0, 1, 2, 3, 4, 5, 255} {
			for _, track := range []int{-1, 0, 42, 43, 1001} {
				st.Style, st.TrackNum = style, track
				if !slices.Equal(want, r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells) {
					t.Fatalf("covered style=%d track=%d pose=%+v leaks its material or pattern", style, track, pose)
				}
			}
		}
	}
}

func TestWalkmanStorageCoverZeroKeepsOriginalFrame(t *testing.T) {
	for _, tt := range []struct {
		name string
		view View
		st   WalkmanState
		want string
	}{
		{"source", View{W: 140, H: 42, Tier: Quadrant, Fit: Rect{80, 2, 58, 34}}, WalkmanState{
			Cassette: 1, FlightFrom: Rect{6, 10, 54, 14}, FlightPose: Pose{Pitch: 1.3}, OnlyCassette: true}, "1d415415bac6a10b77ffc8949e5e72ee1f4d8afda857337d0d89e6b27ff46294"},
		{"flight", View{W: 140, H: 42, Tier: Sextant, Aspect: 2.4, Fit: Rect{80, 2, 58, 34}}, WalkmanState{
			Door: .6, Cassette: 1, FlightFrom: Rect{6, 10, 54, 14}, FlightPose: Pose{Pitch: 1.3}, Flight: .5}, "ab6dd9c79cb879f0908bdd04f626f5164c329c4ac7ff7f620096e44cedf9185a"},
		{"device", View{W: 100, H: 34, Tier: Octant, Fit: Rect{40, 1, 58, 30}}, WalkmanState{
			Door: .4, Cassette: .25, Keys: KeyPlay}, "bbec044e112f41735de1319ee4c5c5700cdadbdfc868a2f6b8508eb0ce117078"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tt.st.State = State{Style: 4, TrackNum: 42, Radii: ReelRadii(.4), Angles: [2]float64{.6, 1.3}}
			var r WalkmanRenderer
			frame := r.RenderWalkman(tt.view, cassetteTestPalette, RestPose(), tt.st)
			hash := sha256.New()
			for _, cell := range frame.Cells {
				fmt.Fprintf(hash, "%#v\n", cell)
			}
			if got := fmt.Sprintf("%x", hash.Sum(nil)); got != tt.want {
				t.Fatalf("uncovered framebuffer hash = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWalkmanStorageCoverRevealsFromTopWithoutChangingCoverage(t *testing.T) {
	v := View{W: 140, H: 48, Tier: Octant, Fit: Rect{80, 2, 58, 34}}
	st := WalkmanState{State: State{Style: 5, TrackNum: 42, Radii: ReelRadii(.4)},
		Cassette: 1, FlightFrom: Rect{4, 6, 68, 28}, FlightPose: Pose{Scale: 1}, OnlyCassette: true}
	var r WalkmanRenderer
	exposed := slices.Clone(r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells)
	st.StorageCover = 1
	covered := slices.Clone(r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells)
	st.StorageCover = .5
	middle := r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells
	top, bottom := 0, 0
	for i, cell := range middle {
		if (cell.Ch == "") != (exposed[i].Ch == "") || (cell.Ch == "") != (covered[i].Ch == "") {
			t.Fatalf("cover changes cassette coverage at (%d,%d)", i%v.W, i/v.W)
		}
		if exposed[i] == covered[i] {
			continue
		}
		if i/v.W < 19 && cell == exposed[i] {
			top++
		}
		if i/v.W > 20 && cell == covered[i] {
			bottom++
		}
	}
	if top < 30 || bottom < 30 {
		t.Fatalf("half cover does not expose the top and preserve the neutral bottom: top=%d bottom=%d", top, bottom)
	}
}

func TestWalkmanStorageCoverEndpointsAreContinuous(t *testing.T) {
	v := View{W: 140, H: 48, Tier: Octant, Fit: Rect{80, 2, 58, 34}}
	for _, pose := range []Pose{{Scale: 1}, {Yaw: .5, Pitch: 1.1}, {Yaw: 2.8, Pitch: -.2}} {
		st := WalkmanState{State: State{Style: 4, Radii: ReelRadii(.4)}, Cassette: 1,
			FlightFrom: Rect{4, 6, 68, 28}, FlightPose: pose, OnlyCassette: true}
		var r WalkmanRenderer
		for _, endpoint := range []float64{0, 1} {
			st.StorageCover = endpoint
			want := slices.Clone(r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells)
			st.StorageCover = 1e-9 + endpoint*(1-2e-9)
			if !slices.Equal(want, r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells) {
				t.Fatalf("cover jumps at endpoint=%g pose=%+v", endpoint, pose)
			}
		}
	}
}

func TestWalkmanStorageCoverDoesNotTintDeviceOrBackground(t *testing.T) {
	v := View{W: 140, H: 42, Tier: Sextant, Fit: Rect{80, 2, 58, 34}}
	st := WalkmanState{State: State{Style: 4, TrackNum: 42, Radii: ReelRadii(.4)}, Door: .6, Cassette: 1,
		FlightFrom: Rect{6, 10, 54, 14}, FlightPose: Pose{Pitch: 1.1}, Keys: KeyPlay}
	var r WalkmanRenderer
	base := slices.Clone(r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells)
	for _, cover := range []float64{.5, 1} {
		st.StorageCover = cover
		changes := 0
		for i, cell := range r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells {
			if (i%v.W >= 65 || base[i].Ch == "") && cell != base[i] {
				t.Fatalf("cover=%g changes device or background at (%d,%d)", cover, i%v.W, i/v.W)
			}
			if cell != base[i] {
				changes++
			}
		}
		if changes < 20 {
			t.Fatalf("cover=%g changes only %d cassette cells", cover, changes)
		}
	}
	st.Cassette, st.StorageCover = -1, 0
	empty := slices.Clone(r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells)
	st.StorageCover = 1
	if !slices.Equal(empty, r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells) {
		t.Fatal("cover changes the empty device")
	}
}

func TestWalkmanStorageCoverReleasePreservesHandoffs(t *testing.T) {
	v := View{W: 140, H: 42, Tier: Octant, Fit: Rect{80, 2, 58, 34}}
	st := WalkmanState{State: State{Style: 3, TrackNum: 42, Radii: ReelRadii(.4)}, Door: 1, Cassette: 1,
		FlightFrom: Rect{6, 10, 54, 14}, FlightPose: Pose{Pitch: 1.1, Roll: -.08}, OnlyCassette: true, StorageCover: 1}
	var r WalkmanRenderer
	r.RenderWalkman(v, cassetteTestPalette, RestPose(), st)
	st.StorageCover = 0
	source := slices.Clone(r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells)
	st.OnlyCassette = false
	for i, cell := range r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells {
		if i%v.W < 65 && cell != source[i] {
			t.Fatalf("released source changes at flight handoff (%d,%d)", i%v.W, i/v.W)
		}
	}
	st.Flight = 1
	flight := slices.Clone(r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells)
	st.FlightFrom, st.FlightPose, st.Flight = Rect{}, Pose{}, 0
	if !slices.Equal(flight, r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells) {
		t.Fatal("released cassette changes at bay handoff")
	}
}
