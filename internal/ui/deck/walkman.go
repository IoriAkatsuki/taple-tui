package deck

import "math"

const (
	KeyRew uint8 = 1 << iota
	KeyPlay
	KeyPause
	KeyFF
	KeyEject
)

// WalkmanState extends the cassette state with the player's mechanical parts.
type WalkmanState struct {
	State
	Door       float64 // 0 closed, 1 open by 35 degrees
	Cassette   float64 // 0 seated, 1 outside the left opening, negative for an empty bay
	Keys       uint8
	FlightFrom Rect    // source box in view cells; positive W/H enables free flight
	Flight     float64 // 0 at FlightFrom, 1 at the open bay's Cassette=1 position
	// FlightPose supplies camera-space Yaw/Pitch/Roll at Flight=0; X/Y/Scale are ignored.
	// Pose{} keeps the device orientation; Pose{Scale: 1} explicitly faces the camera.
	FlightPose   Pose
	OnlyCassette bool    // hide the housing, door and keys without changing the cassette
	StorageCover float64 // 0 exposes the cassette; 1 covers it in neutral storage materials
}

// WalkmanRenderer draws the unbranded blue-and-silver player and its cassette.
type WalkmanRenderer struct {
	gpu   *Vulkan
	frame Frame
	key   renderKey
	valid bool
	state WalkmanState
}

var walkmanSize = vec3{.74, 1.55, 2.1}

// walkmanArrow draws the TPS-L2 hollow upward direction arrow to the right of the vertical window.
func walkmanArrow(u, v float64) bool {
	cx := 44.5
	if v >= 23.0 && v <= 27.8 {
		half := (v - 23.0) * 0.52
		dx := math.Abs(u - cx)
		return dx <= half+0.42 && (dx >= half-0.48 || v > 27.0)
	}
	if v > 27.8 && v <= 33.8 {
		dx := math.Abs(u - cx)
		return (dx >= 0.35 && dx <= 1.05) || (v >= 33.0 && dx <= 1.05)
	}
	return false
}

// walkmanSideKeyBox returns the 3D bounding box for the protruding mechanical
// keys along the right-hand silver flank of the TPS-L2 housing.
func walkmanSideKeyBox(i int, keys uint8) (vec3, vec3) {
	x0 := (faceW/2 - 0.2) * walkmanSize.x
	x1 := (faceW/2 + 3.2) * walkmanSize.x
	var v0, v1 float64
	switch i {
	case 0:
		v0, v1 = 6.5, 11.0
	case 1:
		v0, v1 = 12.2, 16.8
	default:
		v0, v1 = 18.2, 27.0
		if keys&KeyPlay != 0 {
			x1 -= 1.0 * walkmanSize.x
		}
	}
	y0 := (v0 - faceH/2) * walkmanSize.y
	y1 := (v1 - faceH/2) * walkmanSize.y
	return vec3{x0, y0, -3.4}, vec3{x1, y1, 3.4}
}
