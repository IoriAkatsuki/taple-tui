package deck

import (
	"math"
	"testing"
	"time"
)

func TestReelsWindingProportionalAndHold(t *testing.T) {
	for _, tt := range []struct{ delta, want time.Duration }{
		{0, 240 * time.Millisecond}, {time.Second, 240 * time.Millisecond},
		{5 * time.Second, 300 * time.Millisecond}, {-10 * time.Second, 400 * time.Millisecond},
		{35 * time.Second, 900 * time.Millisecond}, {2 * time.Hour, 900 * time.Millisecond},
	} {
		if got := WindDuration(tt.delta); got != tt.want {
			t.Fatalf("WindDuration(%v) = %v, want %v", tt.delta, got, tt.want)
		}
	}
	now := time.Unix(1000, 0)
	var reels Reels
	reels.Run()
	reels.WindWithDuration(1, now, .2, .6, 400*time.Millisecond)
	if got := reels.Progress(now.Add(200*time.Millisecond), .6); math.Abs(got-.55) > 1e-12 {
		t.Fatalf("halfway OutCubic progress = %g, want .55", got)
	}
	for i := 1; i <= 6; i++ {
		now = now.Add(100 * time.Millisecond)
		before := reels.Progress(now, .6)
		reels.WindWithDuration(1, now, .6, .8, 400*time.Millisecond)
		if got := reels.Progress(now, .8); got != before {
			t.Fatalf("repeated seek jumped from %g to %g", before, got)
		}
		reels.Advance(.1, true, now, ReelRadii(before), 1)
	}
	if reels.windMult != 8 || reels.Speed() > 8 || !reels.Winding(now.Add(399*time.Millisecond)) {
		t.Fatalf("held seek is not bounded/extended: multiplier=%g speed=%g", reels.windMult, reels.Speed())
	}
	for i := 1; i < 8; i++ {
		reels.Advance(.05, true, now.Add(time.Duration(i)*50*time.Millisecond), ReelRadii(.8), 1)
	}
	fast := reels.Speed()
	if reels.Blur() != 1 {
		t.Fatalf("fast reels are not fully blurred: %g", reels.Blur())
	}
	end := now.Add(400 * time.Millisecond)
	if reels.Winding(end) || reels.Progress(end, .8) != .8 {
		t.Fatal("seek did not finish at its duration")
	}
	travel := reels.Travel()
	for i := 0; i < 5; i++ {
		reels.Advance(.05, true, end.Add(time.Duration(i)*50*time.Millisecond), ReelRadii(.8), 1)
	}
	if reels.Speed() >= fast/2 || reels.Speed() <= 1 || reels.Travel() <= travel {
		t.Fatalf("250ms braking was not smooth: %g -> %g, travel %g -> %g", fast, reels.Speed(), travel, reels.Travel())
	}
	reels.WindWithDuration(-1, end.Add(time.Second), .8, .2, 900*time.Millisecond)
	for i := 0; i < 8; i++ {
		reels.Advance(.05, false, end.Add(time.Second+time.Duration(i)*50*time.Millisecond), ReelRadii(.5), 1)
	}
	if reels.Speed() >= 0 {
		t.Fatal("rewind did not reverse the tape")
	}
	reels.Reset(42)
	if reels.Winding(end.Add(time.Second)) || reels.Travel() != 0 || reels.Moving() || reels.Blur() != 0 {
		t.Fatal("Reset kept the old seek or tape travel")
	}
}

func TestReelsLegacyWindIsUnchanged(t *testing.T) {
	now := time.Unix(1000, 0)
	var reels Reels
	reels.Run()
	reels.Wind(1, now, .1, .9)
	reels.Advance(.05, true, now, ReelRadii(.5), 1)
	want := 9 - 8*1.65*math.Exp(-.65)
	if math.Abs(reels.Speed()-want) > 1e-12 || !reels.Winding(now.Add(399*time.Millisecond)) || reels.Winding(now.Add(400*time.Millisecond)) {
		t.Fatalf("legacy winding changed: speed=%g, want %g", reels.Speed(), want)
	}
}
