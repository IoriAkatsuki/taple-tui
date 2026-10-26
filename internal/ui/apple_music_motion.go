package ui

import (
	"math"
	"time"

	"github.com/anhoder/foxful-cli/motion"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

type appleTween struct {
	from, to float64
	at       time.Time
	duration time.Duration
	curve    *applePopCurve // nil eases with InOutCubic
}

func (t appleTween) value(now time.Time) float64 {
	if t.duration <= 0 {
		return t.to
	}
	u := min(max(float64(now.Sub(t.at))/float64(t.duration), 0), 1)
	if t.curve != nil {
		return t.from + (t.to-t.from)*t.curve.at(u)
	}
	return t.from + (t.to-t.from)*motion.InOutCubic(u)
}
func (t appleTween) active(now time.Time) bool {
	return t.from != t.to && now.Before(t.at.Add(t.duration))
}
func (t *appleTween) start(target float64, now time.Time, duration time.Duration, level motion.Level) {
	from := t.value(now)
	if level == motion.Off {
		duration = 0
	}
	*t = appleTween{from: from, to: target, at: now, duration: duration}
}

func (p *AppleMusicPage) setZoom(zoom bool, now time.Time) {
	// A new camera intent must not change the projection at the cassette handoff.
	if p.inserting || !p.swapStart.IsZero() {
		p.zoomed, p.restoreZoom = zoom, zoom
		return
	}
	p.restoreZoom = false
	p.autoZoomAfterFold = false
	p.zoomed = zoom
	target := 0.0
	if zoom {
		target = 1
	}
	p.camera.start(target, now, 350*time.Millisecond, p.level)
}

func (p *AppleMusicPage) enterDetail(now time.Time) {
	position := p.scenePosition(now)
	p.inserting = false
	p.rackFocused = false
	p.hoverX, p.hoverY = -1, -1
	p.resetRackPickup()
	p.scene = appleListen
	p.sceneMotion = appleTween{from: position, to: position}
	dur := 640 * time.Millisecond
	if p.level == motion.Subtle {
		dur = 200 * time.Millisecond
	}
	p.sceneMotion.start(1, now, dur, p.level)
	p.autoZoomAfterFold = true
	if !p.sceneMotion.active(now) {
		p.autoZoomAfterFold = false
		p.setZoom(true, now)
	}
}

func (p *AppleMusicPage) scenePosition(now time.Time) float64 {
	if p.inserting {
		u := min(max(float64(now.Sub(p.insertStart)-760*time.Millisecond)/float64(320*time.Millisecond), 0), 1)
		return p.insertSceneFrom + (1-p.insertSceneFrom)*motion.InOutCubic(u)
	}
	return p.sceneMotion.value(now)
}

func (p *AppleMusicPage) browse(now time.Time) {
	position := p.scenePosition(now)
	p.inserting = false
	p.autoZoomAfterFold = false
	p.restoreZoom = false
	if p.zoomed || p.camera.value(now) > 0 {
		p.setZoom(false, now)
	}
	p.rackFocused = false
	p.hoverX, p.hoverY = -1, -1
	p.resetRackPickup()
	p.scene = appleBrowse
	p.sceneMotion = appleTween{from: position, to: position}
	p.sceneMotion.start(0, now, 640*time.Millisecond, p.level)
}

func (p *AppleMusicPage) beginInsert(track applemusic.Track, now time.Time) {
	previous := p.mechanics(now)
	oldPresent := previous.Cassette >= 0 && (p.bayID != "" || p.state.Track.ID != "" || p.inserting)
	continuingEjection := p.inserting && p.insertOldPresent && now.Before(p.insertStart)
	oldEjectAt := p.insertEjectAt
	if p.inserting && now.Before(p.insertStart) {
		previous, oldPresent = p.insertOld, p.insertOldPresent
	}
	position := p.scenePosition(now)
	p.inserting = false
	style := uint8(0)
	if p.rackPickup.current.id == track.ID && p.rackPickup.current.index == p.selected && p.rackPickup.style > 0 {
		style = p.rackPickup.style
	} else {
		style = p.nextCassetteStyle()
	}
	p.insertID, p.insertStyle = track.ID, style
	p.styles[track.ID] = p.insertStyle
	p.rackFocused = true
	p.insertSourceCaptured = false
	p.insertNumber = p.selected + 1
	p.insertOrigin = deck.Rect{X: 3, Y: 8, W: 14, H: 4}
	p.insertPose = deck.RestPose()
	for _, hit := range p.hits {
		if (hit.kind == appleHitTrack || hit.kind == appleHitCassette) && hit.index == p.selected && hit.id == track.ID {
			p.insertOrigin = deck.Rect{X: float64(hit.x), Y: float64(hit.y), W: float64(min(hit.w, 20)), H: 4}
			if hit.cassette.W > 0 {
				p.insertOrigin = hit.cassette
				p.insertPose = hit.pose
			}
			break
		}
	}
	delay := time.Duration(0)
	if p.camera.value(now) > 0 && p.level != motion.Off {
		delay = 350 * time.Millisecond
	}
	p.swapStart, p.swap = time.Time{}, appleSwapState{}
	p.setZoom(false, now)
	p.scene = appleListen
	p.inserting = p.level == motion.Full
	p.insertSceneFrom = position
	p.insertOld, p.insertOldPresent = previous, oldPresent
	p.insertOld.Keys = 0
	p.reels = deck.Reels{}
	p.insertEjectAt = now.Add(delay)
	if continuingEjection {
		p.insertEjectAt = oldEjectAt
	}
	p.insertStart = p.insertEjectAt
	if p.inserting && oldPresent {
		p.insertStart = p.insertStart.Add(320 * time.Millisecond)
	}
	if p.insertStart.Before(now.Add(delay)) {
		p.insertStart = now.Add(delay)
	}
	if p.inserting {
		if popReady := now.Add(p.prepareRackPop(now)); popReady.After(p.insertStart) {
			p.insertStart = popReady
		}
		p.restoreZoom = true
	} else {
		p.bayID, p.bayStyle, p.bayNumber = p.insertID, p.insertStyle, p.insertNumber
		p.sceneMotion.start(1, now.Add(delay), 200*time.Millisecond, p.level)
		p.autoZoomAfterFold = true
		if !p.sceneMotion.active(now) {
			p.autoZoomAfterFold = false
			p.setZoom(true, now)
		}
	}
}

func (p *AppleMusicPage) beginSwap(now time.Time) {
	old := p.mechanics(now)
	p.inserting = false
	p.sceneMotion = appleTween{from: float64(p.scene), to: float64(p.scene)}
	delay := time.Duration(0)
	restore := p.zoomed || p.restoreZoom
	if !p.swapStart.IsZero() {
		if !p.swap.insertAt.IsZero() && !now.Before(p.swap.insertAt) {
			p.swap.queued = true
		} else {
			p.swap.nextID, p.swap.insertAt = "", time.Time{}
		}
		p.restoreZoom = restore
		return
	}
	if p.camera.value(now) > 0 && p.level != motion.Off {
		delay = 350 * time.Millisecond
	}
	p.setZoom(false, now)
	p.restoreZoom = restore
	if p.level == motion.Full {
		p.swapStart = now.Add(delay)
		p.swap = appleSwapState{old: old}
	} else if restore {
		p.setZoom(true, now)
	}
}

// The mechanical timeline has no authority over backend success or failure.
func (p *AppleMusicPage) mechanics(now time.Time) deck.WalkmanState {
	if !p.swapStart.IsZero() {
		return p.swapMechanics(now)
	}
	if p.swap.armed && !p.inserting {
		st := p.swap.old
		st.Keys, st.Blur = 0, 0
		return st
	}
	style := p.styles[p.state.Track.ID]
	if p.bayID != "" {
		style = p.bayStyle
	}
	st := deck.WalkmanState{State: deck.State{Style: style, TrackNum: p.bayNumber, Radii: deck.PackRadii(p.progress(), false), Angles: p.reels.Angles}}
	if p.inserting {
		st.TrackNum = p.insertNumber
	}
	if !p.ready() {
		st.Angles = [2]float64{}
	}
	if p.state.Track.ID == "" && p.bayID == "" {
		st.Cassette = -1
	}
	if p.ready() && p.state.Playing {
		st.Keys = deck.KeyPlay
		st.Blur = p.reels.Blur()
	}
	if p.inserting && now.Before(p.insertStart) {
		if p.insertOldPresent {
			old := p.insertOld
			t := max(0, float64(now.Sub(p.insertEjectAt))/float64(time.Millisecond))
			old.Door += (1 - old.Door) * motion.OutCubic(min(t/80, 1))
			old.Cassette += (1 - old.Cassette) * motion.InOutCubic(min(max((t-80)/120, 0), 1))
			return old
		}
		st.Cassette = -1
		st.Keys = 0
		return st
	}
	if p.inserting && !now.Before(p.insertStart) {
		st.State.Style = p.insertStyle
		st.Radii, st.Angles, st.Blur = deck.PackRadii(0, false), [2]float64{}, 0
		t := float64(now.Sub(p.insertStart)) / float64(time.Millisecond)
		st.Keys = 0
		switch {
		case t < 380:
			st.Cassette = -1
			st.Door = motion.OutCubic(min(max((t-80)/300, 0), 1))
			if p.insertOldPresent {
				st.Door = 1
			}
		case t < 500:
			st.Cassette, st.Door = 1, 1
		case t < 660:
			st.Door, st.Cassette = 1, 1-motion.InQuad((t-500)/160) // accelerate into the stop
		case t < 760:
			st.Door = 1 - motion.OutCubic((t-660)/100)
			st.Cassette = 0
			st.Keys = deck.KeyPlay
		default:
			st.Cassette = 0
		}
		if t >= 660 && p.ready() {
			st.Radii, st.Angles, st.Blur = deck.PackRadii(p.progress(), false), p.reels.Angles, p.reels.Blur()
		}
	}
	return st
}

func (p *AppleMusicPage) sceneBasePose(now time.Time) deck.Pose {
	pose := deck.RestPose()
	mix := p.scenePosition(now)
	// Face left (+14 deg) when the playlist is on the left (mix=0);
	// face right (-14 deg, original RestPose) when lyrics/details are on the right (mix=1).
	pose.Yaw *= 2*mix - 1
	return pose
}

func (p *AppleMusicPage) walkmanPose(now time.Time, z float64) deck.Pose {
	pose := p.sceneBasePose(now)
	if p.level != motion.Off && !p.inserting && p.swapStart.IsZero() {
		ms := (now.UnixMilli() / 40) * 40
		t := float64(ms) / 1000.0
		scale := 1.0
		if p.level == motion.Subtle {
			scale = .6
		}
		if p.state.Playing {
			scale *= .35
		}
		pose.Yaw += (.042*math.Sin(2*math.Pi*t/4.0) + .016*math.Sin(2*math.Pi*t/7.3)) * scale
		pose.Pitch += (.026*math.Sin(2*math.Pi*t/5.3+1.1) + .010*math.Sin(2*math.Pi*t/3.1)) * scale
		pose.Roll += .015 * math.Sin(2*math.Pi*t/6.7+.4) * scale
		pose.Y += .42 * math.Sin(2*math.Pi*t/4.7) * scale
		if p.level == motion.Full {
			nx, ny := p.stagePointerOffset()
			pose.Yaw += -nx * .11
			pose.Pitch += ny * .07
			pose.Roll += -nx * .025
		}
	}
	if p.sceneMotion.active(now) && !p.inserting {
		mix := p.scenePosition(now)
		bank := math.Sin(mix * math.Pi)
		pose.Pitch += .035 * bank
	} else if p.inserting {
		ms := float64(now.Sub(p.insertStart)) / float64(time.Millisecond)
		pose.Y += appleSeatJolt(ms, p.level)
		if ms > 760 && ms < 1080 {
			bank := math.Sin((ms - 760) / 320 * math.Pi)
			pose.Pitch += .03 * bank
		}
	}
	if z > 0 {
		pose.Yaw *= 1 - z
		pose.Pitch *= 1 - z
		pose.Roll *= 1 - z
	}
	return pose
}

// appleSeatJolt is the housing's damped recoil, in face units, when the
// cassette hits the bottom of the bay at 660ms.
func appleSeatJolt(ms float64, level motion.Level) float64 {
	const start, length, amplitude = 660.0, 80.0, 3.0
	if level != motion.Full || ms < start || ms >= start+length {
		return 0
	}
	u := (ms - start) / length
	return amplitude * math.Sin(u*2*math.Pi) * (1 - u)
}

func (p *AppleMusicPage) cassette(w, h int, now time.Time, aspect float64) *deck.Frame {
	v := deck.View{W: w, H: h, Aspect: aspect, Tier: p.tier}
	z := p.camera.value(now)
	pose := p.walkmanPose(now, z)
	if z > 0 {
		fit := deck.WalkmanWindowFit(v, .82)
		v.Fit = deck.Rect{X: fit.X * z, Y: fit.Y * z, W: float64(w) + (fit.W-float64(w))*z, H: float64(h) + (fit.H-float64(h))*z}
	}
	return p.renderer.RenderWalkman(v, deckPaletteOf(appleNeon), pose, p.mechanics(now))
}
