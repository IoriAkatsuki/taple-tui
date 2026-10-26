package ui

import "testing"

func TestAppleMusicTrackOperationKinds(t *testing.T) {
	for _, tc := range []struct {
		kind   string
		track  bool
		direct bool
	}{
		{"play", true, true},
		{"jump", true, true},
		{"play-collection", true, true},
		{"play-station", true, true},
		{"next", true, false},
		{"previous", true, false},
		{"toggle", false, false},
		{"enqueue", false, false},
		{"seek", false, false},
		{"seek-relative", false, false},
		{"volume", false, false},
		{"volume-relative", false, false},
		{"login", false, false},
		{"", false, false},
		{"unknown", false, false},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			if got := appleTrackOperation(tc.kind); got != tc.track {
				t.Fatalf("track operation: got %t, want %t", got, tc.track)
			}
			if got := appleDirectPlayback(tc.kind); got != tc.direct {
				t.Fatalf("direct playback: got %t, want %t", got, tc.direct)
			}
		})
	}
}
