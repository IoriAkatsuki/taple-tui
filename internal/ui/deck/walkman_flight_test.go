package deck

import (
	"fmt"
	"slices"
	"testing"
)

func TestWalkmanFlightHandoffHasIdenticalFrame(t *testing.T) {
	for _, v := range []View{
		{W: 140, H: 36, Tier: Quadrant, Fit: Rect{72, 1, 66, 29}},
		{W: 80, H: 24, Tier: Sextant, Aspect: 2.4, Fit: Rect{40, 1, 39, 20}},
		{W: 100, H: 20, Tier: Octant, Aspect: 1.6, Fit: Rect{55, 0, 44, 19}},
	} {
		for style := uint8(0); style <= 5; style++ {
			t.Run(fmt.Sprintf("%dx%d-style%d", v.W, v.H, style), func(t *testing.T) {
				st := WalkmanState{State: State{Style: style, TrackNum: 42, Radii: ReelRadii(.4), Angles: [2]float64{.6, 1.3}}, Door: 1, Cassette: 1}
				var r WalkmanRenderer
				want := slices.Clone(r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells)
				st.FlightFrom, st.Flight = Rect{3, 8, 40, 2}, 1
				for _, source := range []Pose{{}, {Scale: 1}, {Pitch: 1.3}, {Yaw: 2, Pitch: -.7, Roll: 1.1}} {
					st.FlightPose = source
					got := r.RenderWalkman(v, cassetteTestPalette, RestPose(), st).Cells
					if !slices.Equal(got, want) {
						t.Fatalf("free flight from %+v changes the framebuffer when ownership passes into the bay", source)
					}
				}
			})
		}
	}
}

func TestWalkmanFlightStartsInDrawerAndMovesToBay(t *testing.T) {
	v := View{W: 140, H: 42, Tier: Quadrant, Fit: Rect{80, 2, 58, 34}}
	st := WalkmanState{State: State{Style: 5, Radii: ReelRadii(.4)}, Door: 1, Cassette: 1, FlightFrom: Rect{10, 18, 50, 4}}
	var r WalkmanRenderer
	empty := st
	empty.Cassette, empty.FlightFrom = -1, Rect{}
	background := slices.Clone(r.RenderWalkman(v, cassetteTestPalette, RestPose(), empty).Cells)
	centers := make([]float64, 0, 3)
	for _, progress := range []float64{0, .5, 1} {
		st.Flight = progress
		frame := r.RenderWalkman(v, cassetteTestPalette, RestPose(), st)
		minX, minY, maxX, maxY := v.W, v.H, -1, -1
		for y := range v.H {
			for x := range v.W {
				if frame.Cells[y*v.W+x].Ch != "" && background[y*v.W+x].Ch == "" {
					minX, minY = min(minX, x), min(minY, y)
					maxX, maxY = max(maxX, x), max(maxY, y)
				}
			}
		}
		if progress == 0 && (minX < 10 || maxX >= 60 || minY < 18 || maxY >= 22 || maxX <= minX) {
			t.Fatalf("flight starts outside the selected row: (%d,%d)-(%d,%d)", minX, minY, maxX, maxY)
		}
		if progress < 1 {
			centers = append(centers, float64(minX+maxX)/2)
		}
		if progress == 1 && minX <= 60 {
			t.Fatal("cassette did not leave the drawer region at handoff")
		}
	}
	if centers[1] <= centers[0]+10 {
		t.Fatalf("cassette did not travel toward the bay: %v", centers)
	}
}

func TestWalkmanFlightHandoffSurvivesStageCropping(t *testing.T) {
	stage := Rect{72, 2, 66, 28}
	global := View{W: 140, H: 36, Tier: Quadrant, Fit: stage}
	local := View{W: 66, H: 28, Tier: Quadrant}
	st := WalkmanState{State: State{Style: 5, Radii: ReelRadii(.4)}, Door: 1, Cassette: 1, FlightFrom: Rect{3, 8, 40, 2}, Flight: 1}
	var globalRenderer, localRenderer WalkmanRenderer
	before := globalRenderer.RenderWalkman(global, cassetteTestPalette, RestPose(), st)
	st.FlightFrom, st.Flight = Rect{}, 0
	after := localRenderer.RenderWalkman(local, cassetteTestPalette, RestPose(), st)
	for y := range after.H {
		for x := range after.W {
			if before.Cells[(y+2)*before.W+x+72] != after.Cells[y*after.W+x] {
				t.Fatalf("global-to-stage handoff changes cell (%d,%d)", x, y)
			}
		}
	}
}
