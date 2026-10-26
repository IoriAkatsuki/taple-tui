package ui

import (
	"math"
	"testing"
	"time"

	"github.com/anhoder/foxful-cli/motion"
)

func TestApplePopRaisePressesOvershootsAndSettles(t *testing.T) {
	now := time.Unix(100, 0)
	var tw appleTween
	tw.startPop(.5, now, motion.Full)
	at := func(ms int) float64 { return tw.value(now.Add(time.Duration(ms) * time.Millisecond)) }
	if got := at(40); math.Abs(got-(-.04)) > 1e-9 {
		t.Fatalf("unlock press = %v, want -0.04", got)
	}
	if got := at(150); math.Abs(got-applePopPeak) > 1e-9 {
		t.Fatalf("overshoot peak = %v, want %v", got, applePopPeak)
	}
	if got := at(230); got != .5 {
		t.Fatalf("settled = %v, want 0.5", got)
	}
	if tw.active(now.Add(230 * time.Millisecond)) {
		t.Fatal("raise still animating after 230ms")
	}
}

func TestApplePopStowLiftsOffTheCatchFirst(t *testing.T) {
	now := time.Unix(100, 0)
	tw := appleTween{from: .5, to: .5}
	tw.startPop(0, now, motion.Full)
	if got := tw.value(now.Add(30 * time.Millisecond)); math.Abs(got-.54) > 1e-9 {
		t.Fatalf("stow lift = %v, want 0.54", got)
	}
	if got := tw.value(now.Add(160 * time.Millisecond)); got != 0 {
		t.Fatalf("stowed = %v, want 0", got)
	}
}

func TestApplePopSubtleAndOffDropTheSpring(t *testing.T) {
	now := time.Unix(100, 0)
	var subtle appleTween
	subtle.startPop(.5, now, motion.Subtle)
	for ms := 0; ms <= 115; ms += 5 {
		if v := subtle.value(now.Add(time.Duration(ms) * time.Millisecond)); v < 0 || v > .5 {
			t.Fatalf("subtle left 0..0.5 at %dms: %v", ms, v)
		}
	}
	if subtle.value(now.Add(115*time.Millisecond)) != .5 {
		t.Fatal("subtle raise not done at half duration")
	}
	var off appleTween
	off.startPop(.5, now, motion.Off)
	if off.value(now) != .5 || off.active(now) {
		t.Fatal("off did not jump straight to the catch")
	}
}

func TestApplePopFlashOnlyAtTheCatch(t *testing.T) {
	now := time.Unix(100, 0)
	var tw appleTween
	tw.startPop(.5, now, motion.Full)
	if applePopFlash(tw, now.Add(100*time.Millisecond)) != 0 || applePopFlash(tw, now.Add(240*time.Millisecond)) != 0 {
		t.Fatal("flash outside the settle window")
	}
	if applePopFlash(tw, now.Add(160*time.Millisecond)) <= 0 {
		t.Fatal("no flash when the cassette snaps into its catch")
	}
	tw.startPop(0, now.Add(time.Second), motion.Full)
	if applePopFlash(tw, now.Add(time.Second+160*time.Millisecond)) != 0 {
		t.Fatal("stowing flashed the catch")
	}
}

func TestAppleSeatJoltOnlyAtBottomOutInFull(t *testing.T) {
	if appleSeatJolt(650, motion.Full) != 0 || appleSeatJolt(740, motion.Full) != 0 {
		t.Fatal("jolt outside 660-740ms")
	}
	if appleSeatJolt(680, motion.Full) == 0 {
		t.Fatal("no recoil when the cassette bottoms out")
	}
	if appleSeatJolt(680, motion.Subtle) != 0 || appleSeatJolt(680, motion.Off) != 0 {
		t.Fatal("reduced motion kept the recoil")
	}
}

func TestAppleRackWellIsEmptyAfterTheCassetteDeparts(t *testing.T) {
	p := storageTestPage()
	defer p.Close()
	now := time.Unix(100, 0)
	p.selected, p.rackFocused = 2, true
	p.draw(140, 45, now, 2)
	p.draw(140, 45, now.Add(300*time.Millisecond), 2)
	p.beginInsert(p.tracks[2], now.Add(300*time.Millisecond))
	p.draw(140, 45, p.insertStart.Add(-time.Nanosecond), 2)
	p.draw(140, 45, p.insertStart.Add(200*time.Millisecond), 2)
	if !p.insertSourceCaptured || p.rackPickup.rack.LastState().Out != 2-p.rackStart {
		t.Fatalf("departed cassette still stowed in its well: out=%d", p.rackPickup.rack.LastState().Out)
	}
}
