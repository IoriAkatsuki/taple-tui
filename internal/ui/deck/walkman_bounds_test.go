package deck

import "testing"

func TestWalkmanClippingKeepsReferencePixels(t *testing.T) {
	// Digests verified against the uncropped CPU renderer after removing housing logos.
	want := []string{
		"f95488d66bd96e359143dc2d314c7371927bb4d4e962c630a24e59ce8c7b2372",
		"b62a0bc1f8809b2e758dac7ff96bc773b7f827a62d0d3339622fa842bf63955f",
		"4c7f15c9d871ab4f50f6c8b34d3f594ae51970ccba65656e89fce334498737f7",
		"82456bc9603484b1adbf8dd90127398e5410a0ed37721e8753d69b265170072d",
		"c6603c455d9143267c06e4c68a174a1f2e075d02bb5e0b5c4de4a1348c570e7b",
		"7986395c85ad246087002a3bdc6206eee20811bdba747cd9af896e49c97aa644",
	}
	for i := range want {
		v := View{W: 140, H: 36, Tier: Octant, Aspect: 2}
		pose := RestPose()
		st := WalkmanState{State: State{Style: uint8(i), Radii: PackRadii(.4, false), Angles: [2]float64{.5, .9}}}
		switch i {
		case 0:
			st.Cassette = -1
		case 1:
			st.Door, st.Cassette = 1, 1
		case 2:
			st.OnlyCassette, st.Cassette = true, 1
			st.FlightFrom = Rect{8, 11, 36, 12}
			st.FlightPose = Pose{Yaw: -.24, Pitch: .35, Scale: 1}
		case 3:
			st.Door, st.Cassette, st.Flight = 1, 1, .45
			st.FlightFrom = Rect{8, 11, 36, 12}
			v.Fit = Rect{74, 1, 63, 28}
		case 4:
			v.Fit = WalkmanWindowFit(v, .82)
			pose = Pose{Scale: 1}
		case 5:
			st.Keys = KeyPlay
			pose = Pose{Yaw: .5, Pitch: -.3, Roll: .4, Scale: 1.1}
		}
		var r WalkmanRenderer
		f := r.RenderWalkman(v, testPal, pose, st)
		got := framebufferHash(t, v.Tier, f.Cells)
		if got != want[i] {
			t.Errorf("pose %d changed: %s", i, got)
		}
	}
}
