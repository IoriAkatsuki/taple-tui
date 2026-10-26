package deck

import (
	"image"
	"math"
)

// Bound in projection space, not screen space: flight and close-up retain the
// same camera, while empty parts of the terminal never need rays or readback.
func walkmanBounds(v View, cam camera, pose Pose, sc *walkmanScene) image.Rectangle {
	loX, loY, hiX, hiY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	translation, scale := vec3{pose.X, pose.Y, camDist}, scaleOf(pose)
	point := func(q vec3) {
		x, y := cam.project(q.add(translation))
		loX, hiX, loY, hiY = min(loX, x), max(hiX, x), min(loY, y/cam.aspect), max(hiY, y/cam.aspect)
	}
	box := func(lo, hi vec3, rot mat3, center vec3) {
		for i := range 8 {
			q := lo
			if i&1 != 0 {
				q.x = hi.x
			}
			if i&2 != 0 {
				q.y = hi.y
			}
			if i&4 != 0 {
				q.z = hi.z
			}
			point(rot.apply(q).add(center).mul(scale))
		}
	}
	if !sc.st.OnlyCassette {
		half := vec3{faceW / 2 * walkmanSize.x, faceH / 2 * walkmanSize.y, depth / 2 * walkmanSize.z}
		box(half.mul(-1), half, sc.bodyRot, vec3{})
		for i := range 5 {
			lo, hi := walkmanKeyBox(i, sc.st.Keys)
			box(lo, hi, sc.bodyRot, vec3{})
		}
		for i := range 3 {
			lo, hi := walkmanSideKeyBox(i, sc.st.Keys)
			box(lo, hi, sc.bodyRot, vec3{})
		}
		door, hinge := walkmanDoorTransform(sc.st.Door)
		for _, u := range []float64{6, 70} {
			for _, vv := range []float64{18, 38} {
				q := door.apply(vec3{(u - 70) * walkmanSize.x, (vv - faceH/2) * walkmanSize.y, 0}).add(hinge)
				point(sc.bodyRot.apply(q).mul(scale))
			}
		}
	}
	if sc.st.Cassette >= 0 {
		rot := sc.cassette.rot
		if sc.flightRot != (mat3{}) {
			rot = sc.flightRot
		}
		half := vec3{faceW / 2, faceH / 2, depth / 2}.mul(sc.scale)
		box(half.mul(-1), half, rot, sc.bodyRot.apply(sc.position))
	}
	if math.IsInf(loX, 1) {
		return image.Rectangle{}
	}
	return image.Rect(max(0, int(math.Floor(loX))-1), max(0, int(math.Floor(loY))-1), min(v.W, int(math.Ceil(hiX))+1), min(v.H, int(math.Ceil(hiY))+1)).Intersect(image.Rect(0, 0, v.W, v.H))
}
