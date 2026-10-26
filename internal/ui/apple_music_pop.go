package ui

import (
	"time"

	"github.com/anhoder/foxful-cli/motion"
)

// applePopKey is one keyframe of a mechanical curve: at fraction u of the
// duration the normalized travel reaches f, eased in from the previous key.
type applePopKey struct {
	u, f float64
	ease func(float64) float64
}

// applePopCurve is a spring-loaded latch motion: normalized travel 0 is the
// start and 1 the target; values outside 0..1 are the press and overshoot.
type applePopCurve struct {
	keys     []applePopKey
	duration time.Duration
}

func (c *applePopCurve) at(u float64) float64 {
	prevU, prevF := 0.0, 0.0
	for _, k := range c.keys {
		if u <= k.u {
			return prevF + (k.f-prevF)*k.ease((u-prevU)/(k.u-prevU))
		}
		prevU, prevF = k.u, k.f
	}
	return prevF
}

func applePopKeys(ms float64, keys ...applePopKey) *applePopCurve {
	for i := range keys {
		keys[i].u /= ms
	}
	return &applePopCurve{keys: keys, duration: time.Duration(ms) * time.Millisecond}
}

var (
	// Half eject: press to unlock, spring past the catch, settle with a click.
	applePopRaise = applePopKeys(230,
		applePopKey{40, -.08, motion.OutCubic}, applePopKey{150, 1.16, motion.OutCubic},
		applePopKey{195, .97, motion.InOutCubic}, applePopKey{230, 1, motion.InOutCubic})
	// Pick up: a short dip off the catch, then draw it out.
	applePopLift = applePopKeys(200,
		applePopKey{40, -.08, motion.OutCubic}, applePopKey{200, 1, motion.OutCubic})
	// Stow: lift off the catch, drop to the floor and bounce once.
	applePopStow = applePopKeys(160,
		applePopKey{30, -.08, motion.OutCubic}, applePopKey{130, 1, motion.InQuad},
		applePopKey{145, .96, motion.OutCubic}, applePopKey{160, 1, motion.InCubic})
)

func applePopCurveFor(target float64) *applePopCurve {
	switch {
	case target <= 0:
		return applePopStow
	case target >= 1:
		return applePopLift
	}
	return applePopRaise
}

// applePopDuration is how long the rack takes to reach target at a level.
func applePopDuration(target float64, level motion.Level) time.Duration {
	switch level {
	case motion.Off:
		return 0
	case motion.Subtle:
		return applePopCurveFor(target).duration / 2
	}
	return applePopCurveFor(target).duration
}

// startPop moves the rack cassette with the latch curve; Subtle eases plainly.
func (t *appleTween) startPop(target float64, now time.Time, level motion.Level) {
	t.start(target, now, applePopDuration(target, level), level)
	if level == motion.Full {
		t.curve = applePopCurveFor(target)
	}
}
