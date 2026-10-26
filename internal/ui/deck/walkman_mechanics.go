package deck

import "math"

const walkmanCassetteScale = .42

// walkmanVerticalRoll rotates the cassette 90 degrees around its own Z axis so
// it sits vertically inside the Sony Walkman TPS-L2 bay (reels stacked top/bottom).
const walkmanVerticalRoll = -math.Pi / 2

var (
	walkmanVerticalRot  = rotation(0, 0, walkmanVerticalRoll)
	walkmanVerticalRotT = rotation(0, 0, -walkmanVerticalRoll)
)

// WalkmanFitAspect is the width-to-height ratio of a box the Walkman exactly fills.
func WalkmanFitAspect() float64 {
	return (faceW*walkmanSize.x + fitMargin) / (faceH*walkmanSize.y + fitMargin)
}

// Centered on the vertical observation window (u in 10..36 -> u_c=23, v in 18.8..37.2 -> v_c=28).
var walkmanCassetteCenter = vec3{
	(23.0 - faceW/2) * walkmanSize.x,
	(28.0 - faceH/2) * walkmanSize.y,
	-depth*walkmanSize.z/2 + depth*walkmanCassetteScale/2 + 0.05,
}

func walkmanCassettePosition(amount float64) vec3 {
	return walkmanCassetteCenter.add(vec3{-amount * 28, 0, -amount * 3.5})
}

func walkmanCassettePlacement(cam camera, pose Pose, st WalkmanState) (vec3, float64) {
	if st.FlightFrom.W <= 0 || st.FlightFrom.H <= 0 {
		return walkmanCassettePosition(st.Cassette), walkmanCassetteScale
	}
	target := walkmanCassettePosition(1)
	u := min(max(st.Flight, 0), 1)
	if u == 1 {
		return target, walkmanCassetteScale
	}
	rot, size := rotation(pose.Yaw, pose.Pitch, pose.Roll), scaleOf(pose)
	source := newCamera(View{Fit: st.FlightFrom, Aspect: cam.aspect})
	z := camDist + rot.apply(target.mul(size)).z
	p := vec3{(source.cx - cam.cx) * z / cam.focal, (source.cy - cam.cy) * z / cam.focal, z - camDist}
	start := rot.applyT(p.sub(vec3{pose.X, pose.Y, 0})).mul(1 / size)
	startScale := source.k / cam.k * z / (camDist * size)
	// Keep the arc in view rows, independent of terminal aspect and device scale.
	arc := rot.applyT(vec3{0, -3 * math.Sin(u*math.Pi) * cam.aspect * z / cam.focal, 0}).mul(1 / size)
	return start.add(target.sub(start).mul(u)).add(arc), startScale + (walkmanCassetteScale-startScale)*u
}

func walkmanCamera(v View) camera {
	cam := newCamera(v)
	fit := v.Fit
	if fit.W <= 0 || fit.H <= 0 {
		fit = Rect{0, 0, float64(v.W), float64(v.H)}
	}
	cam.k = min(.9*fit.W/(faceW*walkmanSize.x+fitMargin), .9*fit.H*cam.aspect/(faceH*walkmanSize.y+fitMargin))
	cam.focal = cam.k * camDist
	return cam
}

// WalkmanHoverPlacement aligns a standalone cassette with Cassette=1 at RestPose.
// Give only Renderer the returned Fit/Pose, preserving W/H/Tier/Aspect; render
// the Walkman with the original v and RestPose, without another rotation or scale.
func WalkmanHoverPlacement(v View) (Rect, Pose) {
	cam := walkmanCamera(v)
	pose := RestPose()
	center := rotation(pose.Yaw, pose.Pitch, pose.Roll).apply(walkmanCassettePosition(1))
	// Normalize depth to the standalone renderer's fixed camera distance.
	k := camDist / (camDist + center.z)
	pose.X, pose.Y, pose.Scale = center.x*k, center.y*k, walkmanCassetteScale*k
	pose.Roll += walkmanVerticalRoll
	w := cam.k * (faceW + fitMargin) / .9
	h := cam.k * (faceH + fitMargin) / (.9 * cam.aspect)
	return Rect{cam.cx - w/2, cam.cy/cam.aspect - h/2, w, h}, pose
}

// WalkmanWindowFit centers and enlarges the seated vertical cassette within the device view.
// Use a front-facing Pose{Scale: 1}; Fit may extend beyond the device viewport.
// Clip the same Walkman's rendered result rather than switching close-up models.
func WalkmanWindowFit(v View, zoom float64) Rect {
	if v.W <= 0 || v.H <= 0 {
		return Rect{}
	}
	if zoom <= 0 {
		zoom = 1
	}
	cam := walkmanCamera(v)
	k := min(float64(v.W)/42, float64(v.H)*cam.aspect/44) * zoom
	doorZ := -depth * walkmanSize.z / 2
	wantK := k * (camDist + doorZ) / (camDist * walkmanCassetteScale)
	w := wantK * (faceW*walkmanSize.x + fitMargin) / .9
	h := wantK * (faceH*walkmanSize.y + fitMargin) / (.9 * cam.aspect)
	cx := float64(v.W)/2 - k*walkmanCassetteCenter.x/walkmanCassetteScale
	cy := float64(v.H)/2 - k*walkmanCassetteCenter.y/(walkmanCassetteScale*cam.aspect)
	return Rect{cx - w/2, cy - h/2, w, h}
}

// WalkmanDoorEdge projects the glass window's upper edge into view cells.
func WalkmanDoorEdge(v View, pose Pose, door float64) (x0, y0, x1, y1 float64) {
	cam := walkmanCamera(v)
	rot, scale := rotation(pose.Yaw, pose.Pitch, pose.Roll), scaleOf(pose)
	doorRot, hinge := walkmanDoorTransform(door)
	project := func(u float64) (float64, float64) {
		p := doorRot.apply(vec3{(u - 70) * walkmanSize.x, (19 - faceH/2) * walkmanSize.y, 0}).add(hinge)
		x, y := cam.project(rot.apply(p.mul(scale)).add(vec3{pose.X, pose.Y, camDist}))
		return x, y / cam.aspect
	}
	x0, y0 = project(10)
	x1, y1 = project(66)
	return
}

func (r *WalkmanRenderer) RenderWalkman(v View, pal Palette, pose Pose, st WalkmanState) *Frame {
	if v.Tier == 0 || v.Tier == Braille {
		v.Tier = Quadrant
	}
	key := renderKey{v, pal, pose, st.State}
	if r.valid && r.key == key && r.state == st {
		return &r.frame
	}
	r.valid, r.key, r.state = true, key, st
	w, h := max(v.W, 0), max(v.H, 0)
	n := w * h
	if cap(r.frame.Cells) < n {
		r.frame.Cells = make([]Cell, n)
	}
	r.frame.W, r.frame.H, r.frame.Cells = w, h, r.frame.Cells[:n]
	clear(r.frame.Cells)
	if n == 0 {
		return &r.frame
	}
	cam := walkmanCamera(v)
	rot, scale := rotation(pose.Yaw, pose.Pitch, pose.Roll), scaleOf(pose)
	cassetteRot := rot.mul(walkmanVerticalRot)
	o := rot.applyT(vec3{pose.X, pose.Y, camDist}.mul(-1)).mul(1 / scale)
	m := cassetteMaterials(pal, st.State.Style, st.TrackNum)
	gc, gr := v.Tier.Grid()
	position, cassetteScale := walkmanCassettePlacement(cam, pose, st)
	center := rot.apply(position.mul(scale))
	detail := cam.k * scale * cassetteScale * camDist / (camDist + center.z) * float64(gr) / cam.aspect
	sc := walkmanScene{st: st, position: position, scale: cassetteScale, bodyRot: rot,
		cassette: scene{m: &m, rot: cassetteRot, radii: st.Radii, angles: st.Angles, blur: st.Blur, lObj: cassetteRot.applyT(keyLight), detail: detail}}
	if st.StorageCover > 0 {
		sc.storage = &materials{
			bg: m.bg, hero: rgb{188, 205, 214}, shell: rgb{38, 55, 69}, label: rgb{55, 72, 84},
			ink: rgb{188, 205, 214}, stripe: rgb{72, 109, 122}, glass: rgb{16, 29, 39}, tape: rgb{25, 38, 47},
			hub: rgb{148, 168, 180}, hole: rgb{9, 17, 25}, clamp: rgb{72, 109, 122}, screw: rgb{118, 139, 153},
		}
	}
	if st.FlightFrom.W > 0 && st.FlightFrom.H > 0 && st.Flight < 1 {
		u := max(st.Flight, 0)
		from := st.FlightPose
		if from == (Pose{}) {
			from = pose
		}
		toYaw, toPitch, toRoll := pose.Yaw, pose.Pitch, pose.Roll+walkmanVerticalRoll
		if st.OnlyCassette {
			toYaw, toPitch, toRoll = from.Yaw, from.Pitch, from.Roll
		}
		sc.flightRot = rotation(from.Yaw+(toYaw-from.Yaw)*u, from.Pitch+(toPitch-from.Pitch)*u, from.Roll+(toRoll-from.Roll)*u)
		if sc.flightRot == cassetteRot {
			sc.flightRot = mat3{}
		}
	}
	bounds := walkmanBounds(v, cam, pose, &sc)
	if bounds.Empty() {
		return &r.frame
	}
	if r.gpu != nil && r.gpu.render(v, cam, pose, &sc, &r.frame, bounds) {
		return &r.frame
	}
	parallel(bounds.Dy(), func(start, end int) {
		var pixels [8]rgb
		for y := bounds.Min.Y + start; y < bounds.Min.Y+end; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				covered := false
				for q := range gc * gr {
					fx, fy := float64(x)+(float64(q%gc)+.5)/float64(gc), (float64(y)+(float64(q/gc)+.5)/float64(gr))*cam.aspect
					d := vec3{(fx - cam.cx) / cam.focal, (fy - cam.cy) / cam.focal, 1}
					c, _, hit := sc.sample(o, rot.applyT(d).mul(1/scale), d.norm().mul(-1))
					covered = covered || hit
					pixels[q] = c
				}
				if covered {
					r.frame.Cells[y*w+x] = fitCell(pixels[:gc*gr], v.Tier)
				}
			}
		}
	})
	return &r.frame
}

type walkmanScene struct {
	st        WalkmanState
	position  vec3
	scale     float64
	bodyRot   mat3
	cassette  scene
	storage   *materials
	flightRot mat3 // camera-space cassette rotation; zero shares the seated vertical orientation
}

func walkmanNormalize(p vec3) vec3 {
	return vec3{p.x / walkmanSize.x, p.y / walkmanSize.y, p.z / walkmanSize.z}
}

func (sc *walkmanScene) lightBody(albedo rgb, nObj, view vec3, gloss float64) rgb {
	body := sc.cassette
	if sc.bodyRot != (mat3{}) {
		body.rot = sc.bodyRot
	}
	return body.light(albedo, nObj, view, gloss)
}

func (sc *walkmanScene) sample(o, d, view vec3) (rgb, mat, bool) {
	c, material, nearest := sc.cassette.m.bg, matBg, math.Inf(1)
	bodyRot := sc.bodyRot
	if bodyRot == (mat3{}) {
		bodyRot = sc.cassette.rot.mul(walkmanVerticalRotT)
	}
	var frontU, frontV float64
	var hitFront bool
	if !sc.st.OnlyCassette {
		if h, ok := intersectShell(walkmanNormalize(o), walkmanNormalize(d)); ok {
			u, v := h.p.x+faceW/2, h.p.y+faceH/2
			frontU, frontV, hitFront = u, v, h.surf == surfFront
			if h.surf == surfFront && u > 6 && u < 70 && v > 18 && v < 38 {
				// Project the two drive spindles centered symmetrically in the window (u_c=23, v_c=28).
				nearest = (depth*.3 - o.z) / d.z
				wp := vec3{(u - 23.0) * walkmanSize.x, (v - 28.0) * walkmanSize.y, 0}
				p := walkmanVerticalRot.applyT(wp.mul(1 / walkmanCassetteScale))
				c, material = rgb{19, 16, 29}, matWall
				for _, center := range reelCenters {
					dx, dy := p.x-(center[0]-faceW/2), p.y-(center[1]-faceH/2)
					r := hypot(dx, dy)
					if r < holeR*1.15 && (r < toothR*1.2 || angleNear(math.Atan2(dy, dx), 0, math.Pi/3, .28)) {
						c, material = rgb{184, 188, 208}, matSpindle
					}
				}
			} else {
				nearest = h.t
				c, material = walkmanBody(h), matShell
				c = sc.lightBody(c, walkmanNormalize(h.n).norm(), view, .45)
			}
		}
	}
	if sc.st.Cassette >= 0 {
		inWindow := sc.st.OnlyCassette || sc.st.Cassette > 0 || sc.st.Door > 0 || (hitFront && frontU > 10.0 && frontU < 36.0 && frontV > 18.8 && frontV < 37.2)
		if inWindow {
			cassette := sc.cassette
			pos := sc.position
			if !sc.st.OnlyCassette && sc.st.Cassette == 0 && sc.st.Door == 0 {
				// Compensate for the recess depth between the window plane and the seated reel plane
				// so the two spindles/reels stay centered in the window without parallax shift.
				dz := reelDepth*sc.scale + 0.05
				if math.Abs(d.z) > 1e-6 {
					pos = pos.add(vec3{d.x / d.z * dz, d.y / d.z * dz, 0})
				}
			}
			co := walkmanVerticalRot.applyT(o.sub(pos)).mul(1 / sc.scale)
			cd := walkmanVerticalRot.applyT(d).mul(1 / sc.scale)
			dCamera := bodyRot.apply(d)
			if sc.flightRot != (mat3{}) {
				cassette.rot, cassette.lObj = sc.flightRot, sc.flightRot.applyT(keyLight)
				co = cassette.rot.applyT(bodyRot.apply(o.sub(pos))).mul(1 / sc.scale)
				cd = cassette.rot.applyT(dCamera).mul(1 / sc.scale)
			}
			if h, ok := intersectShell(co, cd); ok && h.t < nearest {
				nearest = h.t
				// Reuse the flying cassette's surface, glass, label and lighting path.
				cassette.o, cassette.inv = co, 1/sc.scale
				cover := 0.0
				if sc.st.StorageCover > 0 {
					// Sweep the feather beyond both shell edges so the endpoints cannot pop.
					edge := (.5 - min(sc.st.StorageCover, 1)) * (faceH + 1)
					cover = smoothstep(edge-.5, edge+.5, h.p.y)
				}
				if cover < 1 {
					c, material, _ = cassette.sample(dCamera)
				}
				if cover > 0 {
					cassette.m = sc.storage
					neutral, neutralMaterial, _ := cassette.sample(dCamera)
					if cover == 1 {
						c, material = neutral, neutralMaterial
					} else {
						c = c.mix(neutral, cover)
					}
				}
			}
		}
	}
	if sc.st.OnlyCassette {
		return c, material, !math.IsInf(nearest, 1)
	}
	for i := range 5 {
		lo, hi := walkmanKeyBox(i, sc.st.Keys)
		if h, ok := walkmanBoxHit(o, d, lo, hi); ok && h.t < nearest {
			nearest, material = h.t, matSide
			if i == 0 {
				// Iconic orange HOT LINE button on the top-left of the TPS-L2.
				c = rgb{248, 138, 14}
				if h.n.y < -.5 {
					c = rgb{255, 168, 28}
				} else if h.n.y > -.5 {
					c = rgb{198, 96, 8}
				}
			} else {
				c = rgb{148, 160, 178}
				if sc.st.Keys&(1<<i) != 0 {
					c = rgb{255, 43, 214}
				} else if h.n.y > -.5 {
					c = c.scale(.65)
				}
			}
			c = sc.lightBody(c, h.n, view, .35)
		}
	}
	for i := range 3 {
		lo, hi := walkmanSideKeyBox(i, sc.st.Keys)
		if h, ok := walkmanBoxHit(o, d, lo, hi); ok && h.t < nearest {
			nearest, material = h.t, matSide
			c = rgb{142, 154, 172}
			if h.n.y < -.5 {
				c = rgb{186, 198, 214}
			} else if h.n.x > .5 {
				if int(math.Abs(h.p.y)*1.8)%2 == 0 {
					c = rgb{112, 124, 142}
				} else {
					c = rgb{164, 176, 194}
				}
			}
			c = sc.lightBody(c, h.n, view, .35)
		}
	}
	if h, u, v, ok := walkmanDoorHit(o, d, sc.st.Door); ok && h.t <= nearest {
		switch {
		case u > 10.0 && u < 36.0 && v > 18.8 && v < 37.2:
			// Tall vertical dark-framed observation window centered at (u=23, v=28),
			// exposing the vertically seated cassette and its top/bottom stacked reels.
			c = c.mix(rgb{6, 34, 42}, .35)
			if min(min(u-10.0, 36.0-u), min(v-18.8, 37.2-v)) < .45 {
				glow := .24
				if sc.st.Keys&KeyPlay != 0 {
					glow = .6
				}
				c = c.mix(rgb{0, 229, 255}, glow)
			}
		case u >= 8.8 && u <= 37.2 && v >= 18.0 && v <= 38.0:
			// Molded charcoal vertical window bezel surrounding the glass.
			c, material = rgb{16, 20, 32}, matShell
			if min(min(u-8.8, 37.2-u), min(v-18.0, 38.0-v)) < .50 {
				c = rgb{36, 44, 62}
			}
			c = sc.lightBody(c, h.n, view, .4)
		case u >= 56.0:
			// Right-hand full-height brushed silver sideband matching Image #3.
			silver := rgb{156, 168, 186}
			if u < 56.8 {
				c = rgb{14, 18, 32}
			} else if u < 57.5 {
				c = rgb{204, 214, 228}
			} else if hypot(u-63.0, v-36.0) < 1.3 {
				c = rgb{28, 36, 52}
			} else {
				c = silver
			}
			material = matShell
			c = sc.lightBody(c, h.n, view, .3)
		default:
			// Royal-blue door face surrounding the vertical window, with hollow upward arrow on the right.
			blue := rgb{34, 70, 123}
			c, material = blue, matShell
			if walkmanArrow(u, v) {
				c = rgb{196, 212, 232}
			} else if u < 7.0 {
				c = blue.scale(.86)
			}
			c = sc.lightBody(c, h.n, view, .35)
		}
		nearest = h.t
	}
	return c, material, !math.IsInf(nearest, 1)
}

func walkmanBody(h shellHit) rgb {
	blue, silver, dark := rgb{34, 70, 123}, rgb{156, 168, 186}, rgb{14, 18, 32}
	silverBright := rgb{206, 218, 234}
	u, v := h.p.x+faceW/2, h.p.y+faceH/2
	if h.surf == surfSide {
		if h.n.y < -.5 {
			// Top wall: royal blue on the left, brushed silver wrap on the right,
			// with two green HEADPHONES A/B jacks.
			pz := h.p.z
			if u >= 56.0 {
				return silver.scale(.84)
			}
			for _, jx := range [2]float64{32, 42} {
				r := hypot(u-jx, pz*1.2)
				if r < 1.3 {
					return dark
				}
				if r < 2.6 {
					return rgb{42, 208, 116}
				}
				if r < 3.4 {
					return silverBright
				}
			}
			return blue.scale(.82)
		}
		if h.n.x > .5 {
			// Right side wall: brushed silver control flank with volume slider slot.
			if v >= 26 && v <= 41 && math.Abs(h.p.z) < 2.2 {
				if v >= 31 && v <= 35 {
					return rgb{168, 180, 198}
				}
				return dark
			}
			return rgb{106, 118, 136}
		}
		if h.n.x < -.5 && v >= 18 && v <= 38 {
			if v <= 19.5 || v >= 36.5 {
				return blue.scale(.92)
			}
			return dark
		}
		if v >= 7.2 && v <= 8.2 {
			return dark
		}
		return blue.scale(.68)
	}
	if h.surf == surfBack {
		return blue.scale(.72)
	}
	// Right-hand full-height brushed silver sideband (u >= 56.0), matching Image #3.
	if u >= 56.0 {
		if u < 56.8 {
			return dark
		}
		if u < 57.5 {
			return silverBright
		}
		if u > faceW-1.2 {
			return silver.scale(.76)
		}
		if hypot(u-63.0, v-38.5) < 1.35 {
			return dark
		}
		return silver
	}
	// Clean royal-blue front edge chamfer.
	if u < 1.2 || v < 1.2 || v > faceH-1.2 {
		return blue.mix(silver, .24)
	}
	// Upper horizontal seam across the blue fascia (v around 7.5), matching Image #3.
	if v >= 7.2 && v < 8.2 {
		if v < 7.7 {
			return dark
		}
		return blue.mix(silver, .28)
	}
	return blue
}

func walkmanDoorTransform(amount float64) (mat3, vec3) {
	rot := rotation(-min(max(amount, 0), 1)*35*math.Pi/180, 0, 0)
	hinge := vec3{(70 - faceW/2) * walkmanSize.x, 0, -depth * walkmanSize.z / 2}
	return rot, hinge
}

func walkmanDoorHit(o, d vec3, amount float64) (shellHit, float64, float64, bool) {
	rot, hinge := walkmanDoorTransform(amount)
	localO, localD := rot.applyT(o.sub(hinge)), rot.applyT(d)
	if math.Abs(localD.z) < 1e-9 {
		return shellHit{}, 0, 0, false
	}
	t := -localO.z / localD.z
	p := localO.at(localD, t)
	u, v := p.x/walkmanSize.x+70, p.y/walkmanSize.y+faceH/2
	if t <= 0 || u < 6 || u > 70 || v < 18 || v > 38 {
		return shellHit{}, 0, 0, false
	}
	return shellHit{t: t, p: o.at(d, t), n: rot.apply(vec3{0, 0, -1}), surf: surfFront}, u, v, true
}

func walkmanKeyBox(i int, keys uint8) (vec3, vec3) {
	x := (-24 + float64(i)*12) * walkmanSize.x
	y := -faceH*walkmanSize.y/2 - 1.6
	if keys&(1<<i) != 0 {
		y += 1.2
	}
	return vec3{x - 3.1, y, -3.8}, vec3{x + 3.1, y + 2.8, 3.8}
}

func walkmanBoxHit(o, d, lo, hi vec3) (shellHit, bool) {
	entry, exit := math.Inf(-1), math.Inf(1)
	n := vec3{}
	for axis, bounds := range [3][4]float64{{o.x, d.x, lo.x, hi.x}, {o.y, d.y, lo.y, hi.y}, {o.z, d.z, lo.z, hi.z}} {
		a, b, ok := slab(bounds[0], bounds[1], bounds[2], bounds[3])
		if !ok {
			return shellHit{}, false
		}
		if a > entry {
			entry = a
			n = vec3{}
			value := -math.Copysign(1, bounds[1])
			switch axis {
			case 0:
				n.x = value
			case 1:
				n.y = value
			case 2:
				n.z = value
			}
		}
		exit = min(exit, b)
	}
	if entry <= 0 || entry > exit {
		return shellHit{}, false
	}
	return shellHit{t: entry, p: o.at(d, entry), n: n, surf: surfSide}, true
}
