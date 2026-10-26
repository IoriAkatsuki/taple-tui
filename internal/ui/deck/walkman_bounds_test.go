package deck

import (
	"crypto/sha256"
	"fmt"
	"testing"
)

func TestWalkmanClippingKeepsReferencePixels(t *testing.T) {
	// Digests verified against the uncropped CPU renderer after removing housing logos.
	want := []string{
		"04218eff8f08ac0ae6d712b1ca3d0b89aac052ce7fb7ae36dd72d191313aff67",
		"50d3bb72acff9fe965f9c02bf6c90a73cb32f575fdb9355c4a7c1dac17a9751b",
		"3bec448e7d7aa9afd5ff6910774983ef74a78d59acbcc9df1bff9264c2031825",
		"5e3e926b1de99a4e57850aa262ba948422fcbdfcfde3af86c24dc04a5edd4420",
		"ec373a0b07e005a0dd8f330a47c23fdd4fb3d360d661b50d3f21580b09b27591",
		"34ffc6a6d3bdb3246cd9149deabea56e61168f01ecdecc684ac444438bf6bd60",
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
		h := sha256.New()
		for _, c := range f.Cells {
			h.Write([]byte(c.Ch))
			h.Write([]byte{0, c.Fg.R, c.Fg.G, c.Fg.B, c.Fg.A, c.Bg.R, c.Bg.G, c.Bg.B, c.Bg.A})
		}
		got := fmt.Sprintf("%x", h.Sum(nil))
		if got != want[i] {
			t.Fatalf("pose %d changed: %s", i, got)
		}
	}
}
