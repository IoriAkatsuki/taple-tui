package deck

import "testing"

func TestWalkmanClippingKeepsReferencePixels(t *testing.T) {
	// Digests verified after updating window rim/key glow and Japanese tape palettes.
	want := []string{
		"9fad594ae8150f3df61a53699480f62f9b731eaa4806b069543c2d0fd8d6a9ac",
		"8cbbb5d2c867c06a4b03a004c55f5bbd971c612133c674661e88526ee013103f",
		"36e2bede23e0ce838e26631836d15309411a7b6dac185dc7ca1df47e8d40c54a",
		"d21ecc200d405949f29960359c8c88962a0457f9627de44ad908680d84e5dd23",
		"0d3474ef4079497f01d678752c19c7606c817c4c21922453532eee84e0b427e2",
		"5e3485551d5de0be3b9dfb23889bc66736e45c759dc305eff89126530c3961ba",
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
