package deck

import (
	"hash/fnv"
	"image"
	"image/color"
	"math"
)

// VinylLabel holds the center label palette of a 12-inch LP record.
type VinylLabel struct {
	Name         string
	Outer, Inner color.RGBA
	Rim, Spindle color.RGBA
}

// VinylLabels are the 5 deterministic retro center-label palettes for albums.
// Choosing one from an album ID never consumes cassette session RNG.
var VinylLabels = [5]VinylLabel{
	{"BLUE NOTE", color.RGBA{38, 92, 168, 255}, color.RGBA{18, 24, 44, 255}, color.RGBA{245, 185, 66, 255}, color.RGBA{14, 15, 24, 255}},
	{"VERMILION", color.RGBA{222, 68, 56, 255}, color.RGBA{34, 16, 20, 255}, color.RGBA{248, 220, 148, 255}, color.RGBA{14, 15, 24, 255}},
	{"AMBER WAX", color.RGBA{235, 156, 46, 255}, color.RGBA{42, 26, 12, 255}, color.RGBA{252, 238, 196, 255}, color.RGBA{14, 15, 24, 255}},
	{"PACIFIC", color.RGBA{46, 206, 188, 255}, color.RGBA{16, 34, 42, 255}, color.RGBA{255, 107, 74, 255}, color.RGBA{14, 15, 24, 255}},
	{"JADE", color.RGBA{34, 158, 122, 255}, color.RGBA{14, 34, 28, 255}, color.RGBA{240, 226, 188, 255}, color.RGBA{14, 15, 24, 255}},
}

// VinylStyleForID deterministically maps an album ID to 1..len(VinylLabels).
func VinylStyleForID(id string) uint8 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return uint8(h.Sum32()%uint32(len(VinylLabels))) + 1
}

// VinylState describes a 3D vinyl record and its peeking inner sleeve.
type VinylState struct {
	Style uint8   // 1..5 selects from VinylLabels
	Spin  float64 // radians of rotation for the anisotropic sheen and label
	Peek  float64 // 0 stowed .. 1 half-exposed above the slot lip
}

// VinylRenderer ray-casts a tilted 3D vinyl LP disc with concentric micro-grooves,
// dual diagonal specular bands, and a center label.
type VinylRenderer struct {
	frame Frame
	key   vinylKey
	valid bool
}

type vinylKey struct {
	v    View
	pose Pose
	st   VinylState
}

const (
	vinylRadius = 31.0
	vinylThick  = 2.2
	vinylLabelR = 11.2
	vinylHoleR  = 1.75
)

// RenderVinyl renders the 3D vinyl disc into v.W×v.H cells fitted to v.Fit.
func (r *VinylRenderer) RenderVinyl(v View, pal Palette, pose Pose, st VinylState) *Frame {
	if v.Tier == 0 || v.Tier == Braille {
		v.Tier = Quadrant
	}
	if v.Aspect <= 0 {
		v.Aspect = 2
	}
	key := vinylKey{v, pose, st}
	if r.valid && r.key == key {
		return &r.frame
	}
	r.valid, r.key = true, key
	w, h := max(v.W, 0), max(v.H, 0)
	n := w * h
	if cap(r.frame.Cells) < n {
		r.frame.Cells = make([]Cell, n)
	}
	r.frame.W, r.frame.H, r.frame.Cells = w, h, r.frame.Cells[:n]
	clear(r.frame.Cells)
	if n == 0 || st.Peek <= 0 {
		return &r.frame
	}
	cam := newCamera(v)
	rot, scale := rotation(pose.Yaw, pose.Pitch, pose.Roll), scaleOf(pose)
	bounds := vinylBounds(v, cam, pose, rot, scale)
	if bounds.Empty() {
		return &r.frame
	}
	o := rot.applyT(vec3{pose.X, pose.Y, camDist}.mul(-1)).mul(1 / scale)
	lbl := VinylLabels[0]
	if st.Style >= 1 && int(st.Style) <= len(VinylLabels) {
		lbl = VinylLabels[st.Style-1]
	}
	sc := vinylScene{
		rot:     rot,
		st:      st,
		outer:   rgbOf(lbl.Outer),
		inner:   rgbOf(lbl.Inner),
		rim:     rgbOf(lbl.Rim),
		spindle: rgbOf(lbl.Spindle),
		hero:    rgbOf(pal.Hero),
	}
	gc, gr := v.Tier.Grid()
	scols, srows := gc, gr
	if v.Tier == Half {
		scols, srows = 2, 4
	}
	parallel(bounds.Dy(), func(start, end int) {
		var pixels [8]rgb
		for y := bounds.Min.Y + start; y < bounds.Min.Y+end; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				if v.Tier == Half {
					var topSum, botSum rgb
					topHits, botHits := 0, 0
					for q := range 8 {
						fx := float64(x) + (float64(q%2)+.5)/2.0
						fy := (float64(y) + (float64(q/2)+.5)/4.0) * cam.aspect
						d := vec3{(fx - cam.cx) / cam.focal, (fy - cam.cy) / cam.focal, 1}
						c, hit := sc.sample(o, rot.applyT(d).mul(1/scale), d.norm().mul(-1))
						if hit {
							if q < 4 {
								topSum, topHits = topSum.add(c), topHits+1
							} else {
								botSum, botHits = botSum.add(c), botHits+1
							}
						}
					}
					if c := fitHalfMask(topSum, botSum, topHits, botHits); c.Ch != "" {
						r.frame.Cells[y*w+x] = c
					}
					continue
				}
				covered := false
				for q := range scols * srows {
					fx := float64(x) + (float64(q%scols)+.5)/float64(scols)
					fy := (float64(y) + (float64(q/scols)+.5)/float64(srows)) * cam.aspect
					d := vec3{(fx - cam.cx) / cam.focal, (fy - cam.cy) / cam.focal, 1}
					c, hit := sc.sample(o, rot.applyT(d).mul(1/scale), d.norm().mul(-1))
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

func vinylBounds(v View, cam camera, pose Pose, rot mat3, scale float64) image.Rectangle {
	loX, loY, hiX, hiY := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	translation := vec3{pose.X, pose.Y, camDist}
	for i := range 8 {
		q := vec3{-vinylRadius, -vinylRadius, -vinylThick / 2}
		if i&1 != 0 {
			q.x = vinylRadius
		}
		if i&2 != 0 {
			q.y = vinylRadius
		}
		if i&4 != 0 {
			q.z = vinylThick / 2
		}
		x, y := cam.project(rot.apply(q).mul(scale).add(translation))
		loX, hiX = min(loX, x), max(hiX, x)
		loY, hiY = min(loY, y/cam.aspect), max(hiY, y/cam.aspect)
	}
	if math.IsInf(loX, 1) {
		return image.Rectangle{}
	}
	return image.Rect(max(0, int(math.Floor(loX))-1), max(0, int(math.Floor(loY))-1), min(v.W, int(math.Ceil(hiX))+1), min(v.H, int(math.Ceil(hiY))+1)).Intersect(image.Rect(0, 0, v.W, v.H))
}

type vinylScene struct {
	rot                        mat3
	st                         VinylState
	outer, inner, rim, spindle rgb
	hero                       rgb
}

func (sc *vinylScene) sample(o, d, view vec3) (rgb, bool) {
	// Intersect the finite cylinder |x,y| <= vinylRadius, |z| <= vinylThick/2.
	tz0, tz1, okz := slab(o.z, d.z, -vinylThick/2, vinylThick/2)
	if !okz {
		return rgb{}, false
	}
	tc0, tc1, okc := circleT(o.x, o.y, d.x, d.y, 0, 0, vinylRadius)
	if !okc {
		return rgb{}, false
	}
	tin, tout := max(tz0, tc0), min(tz1, tc1)
	if tin <= 0 || tin > tout {
		return rgb{}, false
	}
	p := o.at(d, tin)
	rad := math.Hypot(p.x, p.y)
	// Center spindle hole passes through to the well behind it.
	if rad < vinylHoleR {
		return sc.spindle, true
	}
	if tz0 >= tc0 {
		return sc.shadeFace(p, view), true
	}
	// Outer rim of the vinyl disc with a bright specular bead.
	nObj := vec3{p.x / max(rad, 1e-6), p.y / max(rad, 1e-6), 0}
	n := sc.rot.apply(nObj)
	diff := ambient + (1-ambient)*max(n.dot(keyLight), 0)/restNL
	return rgb{46, 62, 78}.scale(diff).mix(sc.rim, .22), true
}

func (sc *vinylScene) shadeFace(p, view vec3) rgb {
	rad := math.Hypot(p.x, p.y)
	ang := math.Atan2(p.y, p.x) + sc.st.Spin
	n := sc.rot.apply(vec3{0, 0, -1})
	baseDiff := ambient + (1-ambient)*max(n.dot(keyLight), 0)/restNL
	// Outer lead-in / rim bevel.
	if rad > vinylRadius-1.1 {
		return rgb{72, 92, 112}.scale(baseDiff).mix(sc.rim, .28)
	}
	// Center paper label with dual rings and a two-tone split.
	if rad <= vinylLabelR {
		c := sc.outer
		if rad < vinylLabelR*.54 || (rad > vinylLabelR*.78 && rad < vinylLabelR*.88) {
			c = sc.inner
		}
		if math.Abs(rad-vinylLabelR*.83) < .38 || math.Abs(rad-vinylHoleR-.55) < .35 {
			c = sc.rim
		}
		if math.Sin(ang) > .15 && rad > vinylHoleR+1.1 && rad < vinylLabelR*.74 {
			c = c.mix(sc.rim, .32)
		}
		return c.scale(baseDiff)
	}
	// Black vinyl groove field with track bands and twin anisotropic sheen lobes.
	groove := .5 + .5*math.Sin(rad*2.9)
	band := 1.0
	if math.Abs(rad-17.5) < .55 || math.Abs(rad-24.2) < .55 {
		band = .62 // silent track separator band
	}
	albedo := rgb{16, 22, 32}.mix(rgb{34, 46, 62}, groove*band*.65)
	sheen1 := math.Pow(max(0, math.Cos(2*(ang+.68))), 10)
	sheen2 := math.Pow(max(0, math.Cos(2*(ang-.85))), 14)
	c := albedo.scale(baseDiff)
	c = c.mix(rgb{110, 148, 178}, sheen1*.52)
	c = c.mix(sc.outer.mix(rgb{220, 238, 252}, .55), sheen2*.38)
	half := keyLight.add(view).norm()
	if spec := max(n.dot(half), 0); spec > 0 {
		c = c.mix(sc.hero, math.Pow(spec, 16)*.16)
	}
	return c
}
