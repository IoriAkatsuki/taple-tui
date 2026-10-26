package deck

import (
	"hash/fnv"
	"image/color"
	"math"
)

// FolderPalette holds the cardstock, tab, inner paper, and brass clip colors of a 3D dossier folder.
type FolderPalette struct {
	Name        string
	Body, Tab   color.RGBA
	Inner, Clip color.RGBA
}

// FolderPalettes are the 4 deterministic retro filing-folder palettes for container items.
// Selecting one from a container ID never consumes session cassette RNG.
var FolderPalettes = [4]FolderPalette{
	{"MANILA", color.RGBA{204, 166, 102, 255}, color.RGBA{232, 194, 126, 255}, color.RGBA{245, 240, 226, 255}, color.RGBA{245, 192, 68, 255}},
	{"SLATE", color.RGBA{58, 88, 116, 255}, color.RGBA{82, 120, 154, 255}, color.RGBA{230, 238, 248, 255}, color.RGBA{0, 229, 255, 255}},
	{"KRAFT", color.RGBA{152, 102, 60, 255}, color.RGBA{182, 126, 76, 255}, color.RGBA{240, 230, 214, 255}, color.RGBA{255, 176, 0, 255}},
	{"REVACHOL", color.RGBA{128, 52, 46, 255}, color.RGBA{164, 72, 62, 255}, color.RGBA{244, 232, 222, 255}, color.RGBA{255, 196, 84, 255}},
}

// FolderStyleForID deterministically maps a container ID to 1..len(FolderPalettes).
func FolderStyleForID(id string) uint8 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return uint8(h.Sum32()%uint32(len(FolderPalettes))) + 1
}

// FolderState describes a 3D hinged file folder and its staggered top index tab.
type FolderState struct {
	Style   uint8   // 1..4 selects from FolderPalettes
	TabSlot uint8   // 0=left, 1=center, 2=right staggered index tab ear
	Open    float64 // 0 stowed .. 1 half-exposed and hinged open in the V-parting gap
}

// FolderRenderer ray-casts a 3D hinged file folder with a protruding stepped index tab,
// peeking white inner dossier pages, a brass fastener clip, and a Mylar window.
type FolderRenderer struct {
	frame Frame
	key   folderKey
	valid bool
}

type folderKey struct {
	v    View
	pose Pose
	st   FolderState
}

const (
	folderHalfW    = 36.0
	folderHalfH    = 20.0
	folderTabWidth = 19.0
	folderTabStep  = 21.0
	folderTabTopY  = -25.2
	folderFlapTilt = 0.34
)

// RenderFolder renders the 3D hinged dossier folder into v.W×v.H cells fitted to v.Fit.
func (r *FolderRenderer) RenderFolder(v View, pose Pose, st FolderState) *Frame {
	if v.Tier == 0 || v.Tier == Braille {
		v.Tier = Quadrant
	}
	if v.Aspect <= 0 {
		v.Aspect = 2
	}
	key := folderKey{v, pose, st}
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
	if n == 0 || st.Open <= 0 {
		return &r.frame
	}
	sc := newFolderScene(v, pose, st)
	r.traceFolder(v, &sc)
	return &r.frame
}

type folderScene struct {
	cam               camera
	rot, flapRot      mat3
	origin            vec3
	scale             float64
	tabX0, tabX1      float64
	body, tab         rgb
	inner, clip, dark rgb
}

func newFolderScene(v View, pose Pose, st FolderState) folderScene {
	pal := FolderPalettes[0]
	if st.Style >= 1 && int(st.Style) <= len(FolderPalettes) {
		pal = FolderPalettes[st.Style-1]
	}
	rot, scale := rotation(pose.Yaw, pose.Pitch, pose.Roll), scaleOf(pose)
	slot := float64(st.TabSlot % 3)
	tabX0 := -folderHalfW + 4.0 + slot*folderTabStep
	flapAngle := min(max(st.Open*1.6, 0), 1) * folderFlapTilt
	return folderScene{
		cam:     newCamera(v),
		rot:     rot,
		flapRot: rotation(0, flapAngle, 0),
		origin:  rot.applyT(vec3{pose.X, pose.Y, camDist}.mul(-1)).mul(1 / scale),
		scale:   scale,
		tabX0:   tabX0,
		tabX1:   tabX0 + folderTabWidth,
		body:    rgbOf(pal.Body),
		tab:     rgbOf(pal.Tab),
		inner:   rgbOf(pal.Inner),
		clip:    rgbOf(pal.Clip),
		dark:    rgb{20, 30, 44},
	}
}

func (r *FolderRenderer) traceFolder(v View, sc *folderScene) {
	w, h := v.W, v.H
	gc, gr := v.Tier.Grid()
	parallel(h, func(start, end int) {
		var pixels [8]rgb
		for y := start; y < end; y++ {
			for x := range w {
				covered := false
				for q := range gc * gr {
					fx := float64(x) + (float64(q%gc)+.5)/float64(gc)
					fy := (float64(y) + (float64(q/gc)+.5)/float64(gr)) * sc.cam.aspect
					d := vec3{(fx - sc.cam.cx) / sc.cam.focal, (fy - sc.cam.cy) / sc.cam.focal, 1}
					c, hit := sc.sample(sc.origin, sc.rot.applyT(d).mul(1/sc.scale), d.norm().mul(-1))
					covered = covered || hit
					pixels[q] = c
				}
				if covered {
					r.frame.Cells[y*w+x] = fitCell(pixels[:gc*gr], v.Tier)
				}
			}
		}
	})
}

func (sc *folderScene) sample(o, d, view vec3) (rgb, bool) {
	nearest := math.Inf(1)
	var out rgb
	// 1. Back cardstock leaf + protruding staggered 3D Index Tab ear.
	if h, ok := walkmanBoxHit(o, d, vec3{-folderHalfW, -folderHalfH, 0.4}, vec3{folderHalfW, folderHalfH, 2.2}); ok && h.t < nearest {
		nearest, out = h.t, sc.shade(sc.body, sc.rot.apply(h.n), view, .28)
	}
	if h, ok := walkmanBoxHit(o, d, vec3{sc.tabX0, folderTabTopY, 0.4}, vec3{sc.tabX1, -folderHalfH + 0.8, 2.3}); ok && h.t < nearest {
		nearest, out = h.t, sc.shade(sc.tab, sc.rot.apply(h.n), view, .42)
	}
	// 2. White/cream inner dossier track sheets peeking out of the V-opening.
	if h, ok := walkmanBoxHit(o, d, vec3{-folderHalfW + 3.2, -folderHalfH + 1.4, -0.9}, vec3{folderHalfW - 3.2, folderHalfH - 1.5, 0.4}); ok && h.t < nearest {
		albedo := sc.inner
		if math.Sin(h.p.y*1.4) < -.25 {
			albedo = albedo.scale(.86)
		}
		nearest, out = h.t, sc.shade(albedo, sc.rot.apply(h.n), view, .20)
	}
	// 3. Hinged front cover tilted forward, with brass fastener clip and Mylar cutout window.
	oFlap, dFlap := sc.flapRot.applyT(o), sc.flapRot.applyT(d)
	if h, ok := walkmanBoxHit(oFlap, dFlap, vec3{-folderHalfW, -folderHalfH + 3.6, -2.5}, vec3{folderHalfW, folderHalfH, -1.0}); ok && h.t < nearest {
		albedo := sc.flapAlbedo(h.p)
		nWorld := sc.rot.apply(sc.flapRot.apply(h.n))
		nearest, out = h.t, sc.shade(albedo, nWorld, view, .36)
	}
	return out, !math.IsInf(nearest, 1)
}

func (sc *folderScene) flapAlbedo(p vec3) rgb {
	if math.Abs(p.x) < 5.5 && p.y < -folderHalfH+6.8 {
		return sc.clip // gleaming brass prong fastener
	}
	if p.x > 8.0 && p.x < folderHalfW-4.5 && p.y > -folderHalfH+7.5 && p.y < folderHalfH-4.5 {
		if math.Sin(p.y*1.2) > .35 {
			return sc.clip // glowing index bars inside Mylar cutout window
		}
		return sc.dark
	}
	if p.y < -folderHalfH+5.2 {
		return sc.tab
	}
	return sc.body.scale(.92)
}

func (sc *folderScene) shade(albedo rgb, n, view vec3, gloss float64) rgb {
	diff := ambient + (1-ambient)*max(n.dot(keyLight), 0)/restNL
	c := albedo.scale(diff)
	half := keyLight.add(view).norm()
	if spec := max(n.dot(half), 0); spec > 0 {
		c = c.mix(rgb{240, 246, 255}, math.Pow(spec, 14)*gloss)
	}
	return c
}
