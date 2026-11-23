package ui

import (
	"image/color"
	"math"
	"time"

	"github.com/anhoder/foxful-cli/motion"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

type appleDeskStage struct {
	w, h, bucket int
	tier         deck.Tier
	camKey       int64
	valid        bool
	stop         diurnalStop
	gbuf         []deskSubsample
	frame        deck.Frame
	cam          deskCam3D
}

type diurnalStop struct {
	hour                            float64
	skyTop, skyMid, skyBot          color.RGBA
	wood, blotter                   color.RGBA
	lampCore, lampHalo              color.RGBA
	lampInt, stars, sunSlats, bokeh float64
}

// 4-phase cozy late-night study room diurnal stops aligned with the velvet-plum & oak palette.
var martinaiseStops = [4]diurnalStop{
	{6.5, color.RGBA{28, 26, 46, 255}, color.RGBA{76, 64, 96, 255}, color.RGBA{188, 134, 114, 255},
		color.RGBA{86, 56, 40, 255}, color.RGBA{30, 24, 42, 255}, color.RGBA{244, 198, 112, 255}, color.RGBA{124, 78, 44, 255}, .58, 0, .40, .14},
	{13.0, color.RGBA{44, 62, 88, 255}, color.RGBA{92, 118, 142, 255}, color.RGBA{206, 194, 178, 255},
		color.RGBA{108, 74, 52, 255}, color.RGBA{36, 30, 48, 255}, color.RGBA{246, 200, 108, 255}, color.RGBA{122, 76, 42, 255}, .45, 0, .55, .08},
	{18.0, color.RGBA{42, 22, 54, 255}, color.RGBA{148, 62, 68, 255}, color.RGBA{238, 124, 78, 255},
		color.RGBA{98, 64, 45, 255}, color.RGBA{34, 25, 44, 255}, color.RGBA{250, 206, 118, 255}, color.RGBA{182, 92, 44, 255}, .86, .22, .82, .50},
	{23.0, color.RGBA{19, 15, 30, 255}, color.RGBA{28, 22, 44, 255}, color.RGBA{44, 34, 64, 255},
		color.RGBA{78, 52, 37, 255}, color.RGBA{26, 20, 38, 255}, color.RGBA{255, 214, 124, 255}, color.RGBA{232, 146, 76, 255}, 1.0, 1.0, 0, 1.0},
}

const (
	deskSurfSky uint8 = iota
	deskSurfWall
	deskSurfWoodTrim
	deskSurfTop
	deskSurfPad
	deskSurfProp
	deskSurfBulb
)

// deskSubsample caches static 3D G-buffer geometry and surface shading per square subpixel
// so dynamic 3D lamp lighting, Walkman contact shadow, and firefly bounce execute with 0 allocs.
type deskSubsample struct {
	r, g, b, surf uint8
	topW          float32
	px, py, pz    float32
	nx, ny, nz    float32
}

type vec3d struct {
	x, y, z float64
}

func (a vec3d) add(b vec3d) vec3d   { return vec3d{a.x + b.x, a.y + b.y, a.z + b.z} }
func (a vec3d) sub(b vec3d) vec3d   { return vec3d{a.x - b.x, a.y - b.y, a.z - b.z} }
func (a vec3d) mul(s float64) vec3d { return vec3d{a.x * s, a.y * s, a.z * s} }
func (a vec3d) dot(b vec3d) float64 { return a.x*b.x + a.y*b.y + a.z*b.z }
func (a vec3d) len2() float64       { return a.x*a.x + a.y*a.y + a.z*a.z }
func (a vec3d) norm() vec3d {
	d := math.Sqrt(a.len2())
	if d < 1e-12 {
		return vec3d{}
	}
	return vec3d{a.x / d, a.y / d, a.z / d}
}

var deskKeyLight = vec3d{-0.45, -0.70, -0.55}.norm()

// deskCam3D uses the same right-handed coordinate convention as internal/ui/deck:
// +X right, +Y down, +Z away from viewer.
type deskCam3D struct {
	eye, rt, dn, fwd vec3d
	cx, cy, focal    float64
	aspect           float64
	xScale, rScale   float64
	screenW, screenH float64
}

func newDeskCam3D(w, h int, aspect, panX, panY, yaw, zoom float64) deskCam3D {
	if aspect <= 0 {
		aspect = 2.0
	}
	sw, sh := float64(w), float64(h)*aspect
	pitch := 12.0*math.Pi/180.0 + panY*0.035
	sinP, cosP := math.Sin(pitch), math.Cos(pitch)
	sinY, cosY := math.Sin(yaw), math.Cos(yaw)

	rt := vec3d{cosY, 0, -sinY}
	dn := vec3d{-sinY * sinP, cosP, -cosY * sinP}
	fwd := vec3d{sinY * cosP, sinP, cosY * cosP}

	focal := sh * 0.96 * (1.0 + 0.14*zoom)
	xScale := min(max(sw/max(sh, 1.0), 0.85), 1.85)
	rScale := min(max(math.Sqrt(xScale), 0.92), 1.16)
	return deskCam3D{
		eye:     vec3d{panX, -4.80 + panY, 0},
		rt:      rt,
		dn:      dn,
		fwd:     fwd,
		cx:      sw * 0.5,
		cy:      sh * 0.40,
		focal:   focal,
		aspect:  aspect,
		xScale:  xScale,
		rScale:  rScale,
		screenW: sw,
		screenH: sh,
	}
}

func (c deskCam3D) ray(sx, sy float64) vec3d {
	ux := (sx - c.cx) / c.focal
	uy := (sy - c.cy) / c.focal
	return c.rt.mul(ux).add(c.dn.mul(uy)).add(c.fwd).norm()
}

func (c deskCam3D) project(p vec3d) (sx, sy, depth float64, ok bool) {
	rel := p.sub(c.eye)
	pz := rel.dot(c.fwd)
	if pz <= 0.1 {
		return 0, 0, 0, false
	}
	px := rel.dot(c.rt)
	py := rel.dot(c.dn)
	return c.cx + c.focal*px/pz, c.cy + c.focal*py/pz, pz, true
}

type deskLayout3D struct {
	showProps bool
	xScale    float64
	rScale    float64

	// Room & window bay
	zWall, zWinBack, zSky float64
	sideWallX             float64
	winHalfW              float64
	winTopY, winBotY      float64
	sillFrontZ, sillBotY  float64
	mullionX, mullionHalf float64

	// Desk & blotter pad
	deskY, zDeskFront      float64
	bevelR, apronBotY      float64
	padHalfW, padZ0, padZ1 float64
	padH                   float64

	// Study lamp (left)
	lampBase             vec3d
	lampBaseR, lampBaseH float64
	lampShoulder         vec3d
	lampElbow            vec3d
	lampNeck             vec3d
	armR, jointR         float64
	spotDir              vec3d
	shadeLen             float64
	shadeR0, shadeR1     float64
	bulb3D               vec3d
	bulbR                float64

	// Sage notebook (right back)
	bookCenter       vec3d
	bookHalf         vec3d
	bookCos, bookSin float64

	// Terracotta coffee mug (right front)
	mugCenter     vec3d
	mugR, mugInR  float64
	mugH, coffeeY float64
	handlePts     [4]vec3d
	handleR       float64
}

func newDeskLayout3D(w, h int, cam deskCam3D) deskLayout3D {
	xs, rs := cam.xScale, cam.rScale
	deskY := 6.40
	lx := -8.6 * xs
	lz := 21.8
	neck := vec3d{lx + 0.55*rs, deskY - 6.75, lz - 0.70}
	spotDir := vec3d{0.78, 0.54, -0.31}.norm()
	shadeLen := 2.45 * rs

	mx := 8.8 * xs
	mz := 22.0
	mugR := 1.38 * rs
	mugH := 2.45
	hx0 := mx + mugR*0.86
	hx1 := mx + mugR + 0.68*rs
	mugTop := deskY - mugH
	bookYaw := -0.36

	return deskLayout3D{
		showProps:    w >= 24 && h >= 10,
		xScale:       xs,
		rScale:       rs,
		zWall:        32.0,
		zWinBack:     34.4,
		zSky:         52.0,
		sideWallX:    13.8 * xs,
		winHalfW:     9.4 * xs,
		winTopY:      -9.8,
		winBotY:      0.25,
		sillFrontZ:   30.7,
		sillBotY:     0.92,
		mullionX:     3.15 * xs,
		mullionHalf:  0.28 * rs,
		deskY:        deskY,
		zDeskFront:   14.8,
		bevelR:       0.42,
		apronBotY:    7.62,
		padHalfW:     6.8 * xs,
		padZ0:        17.6,
		padZ1:        29.0,
		padH:         0.16,
		lampBase:     vec3d{lx, deskY, lz},
		lampBaseR:    1.65 * rs,
		lampBaseH:    0.62,
		lampShoulder: vec3d{lx - 0.15*rs, deskY - 0.56, lz},
		lampElbow:    vec3d{lx - 2.15*rs, deskY - 3.65, lz + 0.55},
		lampNeck:     neck,
		armR:         0.42 * rs,
		jointR:       0.64 * rs,
		spotDir:      spotDir,
		shadeLen:     shadeLen,
		shadeR0:      0.68 * rs,
		shadeR1:      1.68 * rs,
		bulb3D:       neck.add(spotDir.mul(1.32 * rs)),
		bulbR:        0.76 * rs,
		bookCenter:   vec3d{6.2 * xs, deskY - 0.36, 26.2},
		bookHalf:     vec3d{2.05 * rs, 0.36, 1.50 * rs},
		bookCos:      math.Cos(bookYaw),
		bookSin:      math.Sin(bookYaw),
		mugCenter:    vec3d{mx, deskY, mz},
		mugR:         mugR,
		mugInR:       1.08 * rs,
		mugH:         mugH,
		coffeeY:      deskY - mugH + 0.42,
		handlePts: [4]vec3d{
			{hx0, mugTop + 0.62, mz},
			{hx1, mugTop + 0.72, mz},
			{hx1, mugTop + 1.82, mz},
			{hx0, mugTop + 1.92, mz},
		},
		handleR: 0.22 * rs,
	}
}

func appleStageHour(now time.Time) float64 {
	if now.Year() < 2000 {
		return 18.0 // synthetic unit-test timestamps default to golden hour
	}
	local := now.Local()
	return float64(local.Hour()) + float64(local.Minute())/60.0
}

func sampleDiurnalStop(hour float64) diurnalStop {
	h := math.Mod(math.Mod(hour, 24)+24, 24)
	ring := [6]diurnalStop{
		martinaiseStops[3], martinaiseStops[0], martinaiseStops[1],
		martinaiseStops[2], martinaiseStops[3], martinaiseStops[0],
	}
	ring[0].hour, ring[5].hour = -1.0, 30.5
	a, b := ring[0], ring[1]
	for i := range 5 {
		if h >= ring[i].hour && h <= ring[i+1].hour {
			a, b = ring[i], ring[i+1]
			break
		}
	}
	t := (h - a.hour) / max(b.hour-a.hour, 1e-3)
	s := t * t * (3 - 2*t)
	return diurnalStop{
		hour:     h,
		skyTop:   deskMixRGBA(a.skyTop, b.skyTop, s),
		skyMid:   deskMixRGBA(a.skyMid, b.skyMid, s),
		skyBot:   deskMixRGBA(a.skyBot, b.skyBot, s),
		wood:     deskMixRGBA(a.wood, b.wood, s),
		blotter:  deskMixRGBA(a.blotter, b.blotter, s),
		lampCore: deskMixRGBA(a.lampCore, b.lampCore, s),
		lampHalo: deskMixRGBA(a.lampHalo, b.lampHalo, s),
		lampInt:  a.lampInt + (b.lampInt-a.lampInt)*s,
		stars:    a.stars + (b.stars-a.stars)*s,
		sunSlats: a.sunSlats + (b.sunSlats-a.sunSlats)*s,
		bokeh:    a.bokeh + (b.bokeh-a.bokeh)*s,
	}
}

func deskMixRGBA(a, b color.RGBA, t float64) color.RGBA {
	t = min(max(t, 0), 1)
	return color.RGBA{
		R: uint8(math.Round(float64(a.R)*(1-t) + float64(b.R)*t)),
		G: uint8(math.Round(float64(a.G)*(1-t) + float64(b.G)*t)),
		B: uint8(math.Round(float64(a.B)*(1-t) + float64(b.B)*t)),
		A: 0xff,
	}
}

func deskScaleRGBA(c color.RGBA, k float64) color.RGBA {
	return color.RGBA{
		R: uint8(min(max(math.Round(float64(c.R)*k), 0), 255)),
		G: uint8(min(max(math.Round(float64(c.G)*k), 0), 255)),
		B: uint8(min(max(math.Round(float64(c.B)*k), 0), 255)),
		A: 0xff,
	}
}

// drawMartinaiseStage renders the 3D ray-cast study alcove + recessed window + receding oak
// tabletop + 3D desk props, then shades them in real time (0 allocs/op) with the 4.2s
// 3D lamp spotlight, Walkman 3D contact shadow, rising steam, and orbiting golden firefly.
func (p *AppleMusicPage) drawMartinaiseStage(x, y, w, h int, now time.Time) {
	if w <= 0 || h <= 0 {
		return
	}
	tier := p.tier
	if tier == 0 || tier == deck.Braille {
		tier = deck.Half
	}
	gw, gh := tier.Grid()
	subN := gw * gh
	aspect := p.cellAspect
	if aspect <= 0 {
		aspect = 2.0
	}

	hour := appleStageHour(now)
	bucket := int(hour * 60)

	var panX, panY, yaw, zoom float64
	if p.level != motion.Off {
		mix := p.scenePosition(now)
		panX = (mix - 0.5) * 0.75
		zoom = p.camera.value(now)
		if p.level == motion.Full {
			nx, ny := p.stagePointerOffset()
			panX += nx * 0.85
			panY += ny * 0.40
			yaw = -nx * 0.032
		}
	}
	camKey := (int64(math.Round(panX*64)) << 36) ^
		(int64(math.Round(panY*64)) << 22) ^
		(int64(math.Round(zoom*64)) << 8) ^
		int64(math.Round(aspect*16))

	n := w * h
	needSub := n * subN
	if !p.desk.valid || p.desk.w != w || p.desk.h != h || p.desk.bucket != bucket || p.desk.tier != tier || p.desk.camKey != camKey || len(p.desk.gbuf) < needSub {
		p.desk.valid, p.desk.w, p.desk.h, p.desk.bucket, p.desk.tier, p.desk.camKey = true, w, h, bucket, tier, camKey
		p.desk.stop = sampleDiurnalStop(hour)
		if cap(p.desk.frame.Cells) < n {
			p.desk.frame.Cells = make([]deck.Cell, n)
		}
		p.desk.frame.W, p.desk.frame.H, p.desk.frame.Cells = w, h, p.desk.frame.Cells[:n]
		if cap(p.desk.gbuf) < needSub {
			p.desk.gbuf = make([]deskSubsample, needSub)
		}
		p.desk.gbuf = p.desk.gbuf[:needSub]
		p.desk.cam = newDeskCam3D(w, h, aspect, panX, panY, yaw, zoom)
		lay := newDeskLayout3D(w, h, p.desk.cam)
		buildDeskGBuffer(&p.desk, w, h, gw, gh, &lay)
	}

	phase := p.emptyMotionPhase(now)
	perkUp := !p.stageRect.Empty() && p.hoverX >= p.stageRect.Min.X && p.hoverX < p.stageRect.Max.X && p.hoverY >= p.stageRect.Min.Y && p.hoverY < p.stageRect.Max.Y
	lampPower := 0.58
	if phase != 0 {
		amp := 0.14
		if p.level == motion.Subtle {
			amp = 0.07
		}
		lampPower = 0.52 + amp*math.Sin(phase)
	}
	if perkUp {
		lampPower = 0.92
	}
	lampPower *= 0.65 + 0.35*p.desk.stop.lampInt

	cam := &p.desk.cam
	lay := newDeskLayout3D(w, h, *cam)
	sc := min(1.0, min(cam.screenW/44.0, cam.screenH/22.0))

	// Walkman 3D grounding parameters on the velvet-plum blotter plane.
	z := p.camera.value(now)
	pose := p.walkmanPose(now, z)
	walkX := (pose.X*0.16 - pose.Yaw*2.4) * cam.rScale
	walkZ := 22.8 + z*1.1
	poseScale := pose.Scale
	if poseScale <= 0 {
		poseScale = 0.80
	}
	walkRX := 5.6 * poseScale * cam.rScale
	walkRZ := 2.75 * poseScale * cam.rScale
	cosW, sinW := math.Cos(pose.Yaw), math.Sin(pose.Yaw)
	invWalkRX2 := 1.0 / max(walkRX*walkRX, 0.25)
	invWalkRZ2 := 1.0 / max(walkRZ*walkRZ, 0.25)

	// Orbiting golden firefly in 3D space above/right of the mug.
	fx3 := lay.mugCenter.x - 0.8*cam.xScale + 1.25*cam.rScale*math.Cos(phase)
	fy3 := lay.deskY - 4.4*cam.rScale + 1.15*cam.rScale*math.Sin(phase*1.3)
	fz3 := lay.mugCenter.z + 1.8 + 0.9*math.Sin(phase*0.8)
	if perkUp {
		fx3 = lay.mugCenter.x - 1.4*cam.xScale
		fy3 = lay.deskY - 4.9*cam.rScale
		fz3 = lay.mugCenter.z + 1.1
	}
	fireflyGlow := 1.0
	if phase != 0 {
		fireflyGlow = 0.72 + 0.28*math.Sin(phase*2.1)
	}
	if perkUp {
		fireflyGlow = 1.28
	}

	bulbSX, bulbSY, _, bulbVis := cam.project(lay.bulb3D)
	fireSX, fireSY, fireDepth, fireVis := cam.project(vec3d{fx3, fy3, fz3})
	steamSX, steamSY, _, steamVis := cam.project(vec3d{lay.mugCenter.x, lay.coffeeY - 0.15, lay.mugCenter.z})
	coneTarget := lay.bulb3D.add(lay.spotDir.mul(lay.shadeLen * 2.15))

	colBulb := color.RGBA{0xff, 0xf2, 0xac, 0xff}
	colAmber := color.RGBA{0xff, 0xb8, 0x5c, 0xff}
	colSteam := color.RGBA{0xd9, 0xbe, 0xa5, 0xff}

	var pxArr [8]color.RGBA
	gbuf := p.desk.gbuf
	for cy := range h {
		for cx := range w {
			cellOff := (cy*w + cx) * subN
			for q := range subN {
				g := gbuf[cellOff+q]
				cr, cg, cb := float64(g.r), float64(g.g), float64(g.b)
				sx := float64(cx) + (float64(q%gw)+0.5)/float64(gw)
				sy := (float64(cy) + (float64(q/gw)+0.5)/float64(gh)) * aspect

				// 1. Real 3D point/spot illumination from the desk lamp bulb + normal bounce.
				if g.surf != deskSurfSky && g.surf != deskSurfBulb {
					lx := lay.bulb3D.x - float64(g.px)
					ly := lay.bulb3D.y - float64(g.py)
					lz := lay.bulb3D.z - float64(g.pz)
					d2 := lx*lx + ly*ly + lz*lz
					if d2 < 320.0 && d2 > 1e-4 {
						invD := 1.0 / math.Sqrt(d2)
						ldx, ldy, ldz := lx*invD, ly*invD, lz*invD
						ndotl := max(float64(g.nx)*ldx+float64(g.ny)*ldy+float64(g.nz)*ldz, 0)
						spotCos := -(ldx*lay.spotDir.x + ldy*lay.spotDir.y + ldz*lay.spotDir.z)
						spot := smoothstepDesk(-0.15, 0.72, spotCos)
						atten := 1.0 / (1.0 + 0.022*d2)
						glowAmt := (0.14*atten + 0.92*ndotl*(0.24+0.76*spot)*atten) * lampPower
						if g.surf == deskSurfTop || g.surf == deskSurfPad {
							glowAmt *= 1.18
						}
						// Warm specular bounce toward camera on tabletop & ceramic props.
						vx, vy, vz := cam.eye.x-float64(g.px), cam.eye.y-float64(g.py), cam.eye.z-float64(g.pz)
						vd := 1.0 / max(math.Sqrt(vx*vx+vy*vy+vz*vz), 1e-4)
						hx, hy, hz := ldx+vx*vd, ldy+vy*vd, ldz+vz*vd
						hd := 1.0 / max(math.Sqrt(hx*hx+hy*hy+hz*hz), 1e-4)
						ndoth := max((float64(g.nx)*hx+float64(g.ny)*hy+float64(g.nz)*hz)*hd, 0)
						spec := math.Pow(ndoth, 16) * (0.28 * atten * (0.3 + 0.7*spot) * lampPower)
						wMix := min(glowAmt*0.58, 0.62)
						cr += (float64(colAmber.R)-cr)*wMix + float64(colBulb.R)*spec
						cg += (float64(colAmber.G)-cg)*wMix + float64(colBulb.G)*spec
						cb += (float64(colAmber.B)-cb)*wMix + float64(colBulb.B)*spec
					}
				} else if g.surf == deskSurfBulb {
					boost := 0.72 + 0.35*lampPower
					cr = min(cr*boost+30*lampPower, 255)
					cg = min(cg*boost+24*lampPower, 255)
					cb = min(cb*boost+10*lampPower, 255)
				}

				// 2. Real 3D Walkman contact & cast shadow strictly on horizontal blotter/desk plane.
				if g.topW > 0 {
					dx := float64(g.px) - walkX
					dz := float64(g.pz) - walkZ
					rx := dx*cosW + dz*sinW
					rz := -dx*sinW + dz*cosW
					ed2 := rx*rx*invWalkRX2 + rz*rz*invWalkRZ2
					if ed2 < 1.0 {
						sh := math.Pow(1.0-ed2, 1.45) * 0.54 * float64(g.topW)
						mul := 1.0 - sh
						cr *= mul
						cg *= mul
						cb *= mul
					}
				}

				// 3. Soft 3D volumetric lamp beam in front of the shade + bulb radial bloom.
				if lay.showProps && float64(g.pz) > lay.bulb3D.z-1.2 {
					rd := cam.ray(sx, sy)
					beamW := volumetricConeWeight(cam.eye, rd, lay.bulb3D, coneTarget, lay.spotDir, float64(g.pz))
					if beamW > 0 {
						bm := beamW * 0.24 * lampPower
						if g.surf == deskSurfSky {
							bm *= 0.45
						}
						cr += (float64(colAmber.R) - cr) * bm
						cg += (float64(colAmber.G) - cg) * bm
						cb += (float64(colAmber.B) - cb) * bm
					}
					if bulbVis {
						if bd := math.Hypot(sx-bulbSX, sy-bulbSY); bd < 4.0*sc*cam.rScale {
							u := 1.0 - bd/(4.0*sc*cam.rScale)
							bw := u * u * 0.46 * lampPower
							cr += (float64(colBulb.R) - cr) * bw
							cg += (float64(colBulb.G) - cg) * bw
							cb += (float64(colBulb.B) - cb) * bw
						}
					}
				}

				// 4. 3D depth-tested hot coffee steam ribbons rising above the terracotta mug.
				if lay.showProps && steamVis && float64(g.pz) >= lay.mugCenter.z-1.4 && sy < steamSY-0.3*sc && sy > steamSY-7.8*sc {
					for sIdx := range 2 {
						sxBase := steamSX - 0.75*sc + float64(sIdx)*1.5*sc
						if math.Abs(sx-sxBase) < 1.6*sc {
							driftY := sy - steamSY
							sOff := float64(sIdx) * math.Pi * 0.9
							waveX := sxBase + 0.50*sc*math.Sin(driftY*0.95/max(sc, 0.25)-phase*1.1+sOff)
							dist := math.Abs(sx - waveX)
							fade := 1.0 - min(1.0, math.Abs(driftY)/(7.8*sc))
							if dist < 0.72*sc {
								a := (1.0 - dist/(0.72*sc)) * fade * 0.58
								cr += (float64(colSteam.R) - cr) * a
								cg += (float64(colSteam.G) - cg) * a
								cb += (float64(colSteam.B) - cb) * a
							}
						}
					}
				}

				// 5. 3D orbiting golden firefly point-light bounce + projected core/halo.
				if w >= 20 && h >= 8 && fireVis {
					if g.surf != deskSurfSky {
						dfx := fx3 - float64(g.px)
						dfy := fy3 - float64(g.py)
						dfz := fz3 - float64(g.pz)
						fd2 := dfx*dfx + dfy*dfy + dfz*dfz
						if fd2 < 42.0 {
							b := 0.42 * fireflyGlow / (1.0 + 0.42*fd2)
							cr += (float64(colAmber.R) - cr) * b
							cg += (float64(colAmber.G) - cg) * b
							cb += (float64(colAmber.B) - cb) * b
						}
					}
					if float64(g.pz) >= fireDepth-0.5 {
						fDist := math.Hypot(sx-fireSX, sy-fireSY) - 0.50*sc
						if fDist < 2.8*sc {
							if fDist < 0.42*sc {
								a := emptySmoothAlpha(fDist, 0.42*sc)
								cr += (float64(colBulb.R) - cr) * a
								cg += (float64(colBulb.G) - cg) * a
								cb += (float64(colBulb.B) - cb) * a
							} else {
								halo := 1.0 - (fDist-0.42*sc)/(2.38*sc)
								a := halo * halo * 0.62 * fireflyGlow
								cr += (float64(colAmber.R) - cr) * a
								cg += (float64(colAmber.G) - cg) * a
								cb += (float64(colAmber.B) - cb) * a
							}
						}
					}
				}

				pxArr[q] = color.RGBA{
					R: uint8(min(max(math.Round(cr), 0), 255)),
					G: uint8(min(max(math.Round(cg), 0), 255)),
					B: uint8(min(max(math.Round(cb), 0), 255)),
					A: 0xff,
				}
			}
			if tier == deck.Half {
				if pxArr[0] == pxArr[1] {
					p.desk.frame.Cells[cy*w+cx] = deck.Cell{Ch: " ", Bg: pxArr[1]}
				} else {
					p.desk.frame.Cells[cy*w+cx] = deck.Cell{Ch: "▀", Fg: pxArr[0], Bg: pxArr[1]}
				}
			} else {
				p.desk.frame.Cells[cy*w+cx] = deck.FitCellRGBA(pxArr[:subN], tier)
			}
		}
	}
	p.screen.drawDeck(&p.desk.frame, x, y)
	p.drawMartinaiseDetails(x, y, w, h, gw, gh, &lay, &p.desk.stop, now)
}

func volumetricConeWeight(eye, rd, bulb, target, dir vec3d, hitZ float64) float64 {
	// Closest approach between camera ray (eye + t*rd) and finite cone axis segment (bulb -> target).
	u := target.sub(bulb)
	w0 := eye.sub(bulb)
	a := rd.dot(rd)
	b := rd.dot(u)
	c := u.dot(u)
	d := rd.dot(w0)
	e := u.dot(w0)
	den := a*c - b*b
	var t, s float64
	if den > 1e-8 {
		t = (b*e - c*d) / den
		s = (a*e - b*d) / den
	}
	if t <= 0 {
		return 0
	}
	s = min(max(s, 0.05), 1.0)
	closestRay := eye.add(rd.mul(t))
	if closestRay.z > hitZ+0.3 {
		return 0
	}
	closestAxis := bulb.add(u.mul(s))
	dist := math.Sqrt(closestRay.sub(closestAxis).len2())
	coneR := 0.9 + 4.4*s
	if dist >= coneR {
		return 0
	}
	radial := 1.0 - dist/coneR
	axial := math.Sqrt(1.0 - 0.65*s)
	return radial * radial * axial
}

var ssaa2x2 = [4][2]float64{{-0.25, -0.25}, {0.25, -0.25}, {-0.25, 0.25}, {0.25, 0.25}}

func buildDeskGBuffer(st *appleDeskStage, w, h, gw, gh int, lay *deskLayout3D) {
	subN := gw * gh
	stepX := 1.0 / float64(gw)
	stepY := st.cam.aspect / float64(gh)
	for cy := range h {
		for cx := range w {
			cellOff := (cy*w + cx) * subN
			for q := range subN {
				baseSX := float64(cx) + (float64(q%gw)+0.5)/float64(gw)
				baseSY := (float64(cy) + (float64(q/gw)+0.5)/float64(gh)) * st.cam.aspect
				var sumR, sumG, sumB float64
				var sumPX, sumPY, sumPZ float64
				var sumNX, sumNY, sumNZ float64
				var sumTop float64
				var centerSurf uint8
				for sIdx, off := range ssaa2x2 {
					rd := st.cam.ray(baseSX+off[0]*stepX, baseSY+off[1]*stepY)
					col, hit := traceAndShadeDesk3D(st.cam.eye, rd, lay, &st.stop)
					sumR += float64(col.R)
					sumG += float64(col.G)
					sumB += float64(col.B)
					sumPX += hit.p.x
					sumPY += hit.p.y
					sumPZ += hit.p.z
					sumNX += hit.n.x
					sumNY += hit.n.y
					sumNZ += hit.n.z
					if hit.isTop {
						sumTop += 1.0
					}
					if sIdx == 0 {
						centerSurf = hit.surf
					}
				}
				n := vec3d{sumNX, sumNY, sumNZ}.norm()
				st.gbuf[cellOff+q] = deskSubsample{
					r:    uint8(min(max(math.Round(sumR*0.25), 0), 255)),
					g:    uint8(min(max(math.Round(sumG*0.25), 0), 255)),
					b:    uint8(min(max(math.Round(sumB*0.25), 0), 255)),
					surf: centerSurf,
					topW: float32(sumTop * 0.25),
					px:   float32(sumPX * 0.25),
					py:   float32(sumPY * 0.25),
					pz:   float32(sumPZ * 0.25),
					nx:   float32(n.x),
					ny:   float32(n.y),
					nz:   float32(n.z),
				}
			}
		}
	}
}

type deskHit3D struct {
	t      float64
	p, n   vec3d
	albedo color.RGBA
	gloss  float64
	surf   uint8
	isTop  bool
	unlit  bool
}

func traceAndShadeDesk3D(ro, rd vec3d, lay *deskLayout3D, th *diurnalStop) (color.RGBA, deskHit3D) {
	hit := traceDeskScene3D(ro, rd, lay, th)
	if hit.unlit {
		return hit.albedo, hit
	}
	// Key light + cool/warm window skylight fill normalized cleanly like deck/shade.go.
	ndotl := max(hit.n.dot(deskKeyLight), 0)
	ambient := 0.38
	if hit.surf == deskSurfWall || hit.surf == deskSurfWoodTrim {
		ambient = 0.44
	}
	shadow := 1.0
	if ndotl > 0.02 && lay.showProps {
		shadow = deskKeyShadow3D(hit.p.add(hit.n.mul(0.03)), deskKeyLight, lay)
	}
	diffuse := ambient + (1.0-ambient)*ndotl*shadow/0.78

	// Soft fill from window opening (-Z/up) slightly tinted by current sky horizon.
	winDir := vec3d{0, -hit.p.y, lay.zWall - hit.p.z}.norm()
	winFill := max(hit.n.dot(winDir), 0) * 0.14
	c := deskScaleRGBA(hit.albedo, diffuse)
	if winFill > 0.005 {
		c = deskMixRGBA(c, th.skyBot, winFill)
	}
	if hit.gloss > 0 && shadow > 0.5 {
		view := rd.mul(-1)
		half := deskKeyLight.add(view).norm()
		x := max(hit.n.dot(half), 0)
		x4 := x * x * x * x
		spec := x4 * x4 * x4 * x4 * 0.22 * hit.gloss
		if spec > 0.004 {
			c = deskMixRGBA(c, color.RGBA{0xff, 0xf2, 0xd8, 0xff}, min(spec, 0.26))
		}
	}
	return c, hit
}

func deskKeyShadow3D(o, dir vec3d, lay *deskLayout3D) float64 {
	// Bounding shadow casters against directional keyLight.
	sh := 1.0
	if t, ok := hitSphereFast(o, dir, vec3d{lay.mugCenter.x, lay.deskY - lay.mugH*0.5, lay.mugCenter.z}, lay.mugR*1.05); ok && t > 0.05 {
		sh *= 0.48
	}
	if t, ok := hitSphereFast(o, dir, lay.bookCenter, 1.95*lay.rScale); ok && t > 0.05 {
		sh *= 0.62
	}
	if t, ok := hitSphereFast(o, dir, vec3d{lay.lampBase.x, lay.lampBase.y - 0.3, lay.lampBase.z}, lay.lampBaseR); ok && t > 0.05 {
		sh *= 0.54
	}
	if t, ok := hitSphereFast(o, dir, lay.bulb3D, lay.shadeR1*0.95); ok && t > 0.05 {
		sh *= 0.68
	}
	return sh
}

func hitSphereFast(ro, rd, center vec3d, r float64) (float64, bool) {
	oc := ro.sub(center)
	b := oc.dot(rd)
	c := oc.dot(oc) - r*r
	h := b*b - c
	if h < 0 {
		return 0, false
	}
	t := -b - math.Sqrt(h)
	if t <= 0 {
		t = -b + math.Sqrt(h)
	}
	return t, t > 0
}

func traceDeskScene3D(ro, rd vec3d, lay *deskLayout3D, th *diurnalStop) deskHit3D {
	best := deskHit3D{t: 1e9, surf: deskSurfWall, albedo: color.RGBA{22, 18, 32, 255}, n: vec3d{0, 0, -1}}
	accept := func(cand deskHit3D) {
		if cand.t > 1e-4 && cand.t < best.t {
			best = cand
		}
	}

	// 1. 3D Study Room Enclosure + Recessed Window Alcove + Far Twilight Sky.
	accept(intersectRoomAndWindow3D(ro, rd, lay, th))

	// 2. 3D Oak Desk Slab (receding horizontal top, rounded front bevel, vertical front oak apron, shadow recess).
	accept(intersectDeskSlab3D(ro, rd, lay, th))

	// 3. Raised 3D Velvet-Plum Stitched Desk Blotter Pad.
	padBox, ok := intersectAABB3D(ro, rd,
		vec3d{-lay.padHalfW, lay.deskY - lay.padH, lay.padZ0},
		vec3d{+lay.padHalfW, lay.deskY + 0.02, lay.padZ1})
	if ok {
		col := th.blotter
		edgeX := lay.padHalfW - math.Abs(padBox.p.x)
		edgeZ := min(padBox.p.z-lay.padZ0, lay.padZ1-padBox.p.z)
		if padBox.n.y < -0.5 {
			col = deskMixRGBA(color.RGBA{34, 27, 48, 255}, th.blotter, 0.55)
			if min(edgeX, edgeZ) > 0.25 && min(edgeX, edgeZ) < 0.45 {
				col = deskMixRGBA(col, color.RGBA{128, 102, 138, 255}, 0.55)
			}
			padBox.isTop = true
		} else {
			col = deskMixRGBA(th.blotter, color.RGBA{72, 56, 86, 255}, 0.35)
		}
		padBox.albedo = col
		padBox.surf = deskSurfPad
		padBox.gloss = 0.08
		accept(padBox)
	}

	if !lay.showProps {
		return best
	}

	// 4. Right-Back: Yawed Hardcover Sage Notebook resting on tabletop.
	if nb, ok := intersectYawedBox3D(ro, rd, lay.bookCenter, lay.bookHalf, lay.bookCos, lay.bookSin); ok {
		col := color.RGBA{0xf3, 0xeb, 0xdd, 0xff} // cream page stack on sides/front
		if nb.n.y < -0.5 {
			col = color.RGBA{0x6c, 0x8e, 0x7f, 0xff} // sage top cloth cover
			// Spine crease along notebook's local -X edge.
			rel := nb.p.sub(lay.bookCenter)
			locX := rel.x*lay.bookCos + rel.z*lay.bookSin
			if locX < -lay.bookHalf.x+0.48 {
				col = color.RGBA{0x4d, 0x6a, 0x5d, 0xff}
			}
			nb.isTop = true
		} else if nb.n.z > 0.3 {
			col = color.RGBA{0x5a, 0x78, 0x6b, 0xff}
		}
		nb.albedo = col
		nb.surf = deskSurfProp
		nb.gloss = 0.14
		accept(nb)
	}

	// 5. Right-Front: Hollow 3D Terracotta Mug with glazed rim, recessed coffee disk & 3D handle.
	if mug, ok := intersectMug3D(ro, rd, lay); ok {
		accept(mug)
	}
	for i := range 3 {
		if hHit, ok := intersectCapsule3D(ro, rd, lay.handlePts[i], lay.handlePts[i+1], lay.handleR); ok {
			hHit.albedo = color.RGBA{0xbd, 0x66, 0x47, 0xff}
			hHit.surf = deskSurfProp
			hHit.gloss = 0.32
			accept(hHit)
		}
	}

	// 6. Left: Articulated 3D Study Lamp (cylindrical base, tubular arms, sphere joint, hollow hood & bulb).
	if base, ok := intersectCylinderY3D(ro, rd, lay.lampBase.x, lay.lampBase.z, lay.deskY-lay.lampBaseH, lay.deskY, lay.lampBaseR); ok {
		base.albedo = color.RGBA{0xe2, 0x88, 0x45, 0xff}
		if base.n.y < -0.5 {
			base.albedo = color.RGBA{0xf0, 0xa2, 0x5d, 0xff}
		}
		base.surf = deskSurfProp
		base.gloss = 0.38
		accept(base)
	}
	if arm1, ok := intersectCapsule3D(ro, rd, lay.lampShoulder, lay.lampElbow, lay.armR); ok {
		arm1.albedo = color.RGBA{0xa8, 0x82, 0x64, 0xff}
		arm1.surf = deskSurfProp
		arm1.gloss = 0.25
		accept(arm1)
	}
	if arm2, ok := intersectCapsule3D(ro, rd, lay.lampElbow, lay.lampNeck, lay.armR); ok {
		arm2.albedo = color.RGBA{0xa8, 0x82, 0x64, 0xff}
		arm2.surf = deskSurfProp
		arm2.gloss = 0.25
		accept(arm2)
	}
	if jt, ok := intersectSphere3D(ro, rd, lay.lampElbow, lay.jointR); ok {
		jt.albedo = color.RGBA{0xf5, 0xe8, 0xd6, 0xff}
		jt.surf = deskSurfProp
		jt.gloss = 0.42
		accept(jt)
	}
	if bulb, ok := intersectSphere3D(ro, rd, lay.bulb3D, lay.bulbR); ok {
		bulb.albedo = color.RGBA{0xff, 0xf2, 0xac, 0xff}
		bulb.surf = deskSurfBulb
		bulb.unlit = true
		accept(bulb)
	}
	if shade, ok := intersectLampHood3D(ro, rd, lay.lampNeck, lay.spotDir, lay.shadeLen, lay.shadeR0, lay.shadeR1); ok {
		accept(shade)
	}

	return best
}

func intersectRoomAndWindow3D(ro, rd vec3d, lay *deskLayout3D, th *diurnalStop) deskHit3D {
	best := deskHit3D{t: 1e9, surf: deskSurfWall, albedo: color.RGBA{26, 22, 39, 255}, n: vec3d{0, 0, -1}}
	consider := func(c deskHit3D) {
		if c.t > 1e-4 && c.t < best.t {
			best = c
		}
	}

	// 3D left & right side walls of the study alcove.
	if math.Abs(rd.x) > 1e-6 {
		for _, sx := range [2]float64{-lay.sideWallX, lay.sideWallX} {
			t := (sx - ro.x) / rd.x
			if t > 0 {
				p := ro.add(rd.mul(t))
				if p.z >= lay.zDeskFront-2.0 && p.z <= lay.zWall+0.05 && p.y <= lay.deskY+0.02 {
					nx := 1.0
					if sx > 0 {
						nx = -1.0
					}
					wallT := min(max((p.y-lay.winTopY)/max(lay.deskY-lay.winTopY, 1.0), 0), 1)
					col := deskMixRGBA(color.RGBA{30, 24, 46, 255}, th.blotter, 0.35*wallT)
					if p.y > lay.deskY-0.75 {
						col = deskMixRGBA(th.wood, color.RGBA{64, 48, 60, 255}, 0.45)
					}
					consider(deskHit3D{t: t, p: p, n: vec3d{nx, 0, 0}, albedo: col, surf: deskSurfWall})
				}
			}
		}
	}

	// Main back wall at z = zWall (excluding window opening).
	if rd.z > 1e-6 {
		t := (lay.zWall - ro.z) / rd.z
		if t > 0 {
			p := ro.add(rd.mul(t))
			inWinX := math.Abs(p.x) < lay.winHalfW
			inWinY := p.y > lay.winTopY && p.y < lay.winBotY
			if !inWinX || !inWinY {
				alcoveT := min(max((p.y-lay.winBotY)/max(lay.deskY-lay.winBotY, 1), 0), 1)
				col := deskMixRGBA(color.RGBA{26, 22, 39, 255}, th.blotter, 0.32+0.25*alcoveT)
				surf := deskSurfWall
				if p.y >= lay.deskY-0.82 && p.y <= lay.deskY+0.02 {
					col = deskMixRGBA(th.wood, color.RGBA{72, 54, 68, 255}, 0.45)
					surf = deskSurfWoodTrim
				}
				// Soft painted arch shadow & sun slats on the back wall.
				if th.sunSlats > 0.05 && p.y > lay.winBotY && p.y < lay.deskY {
					if slat := math.Sin(p.x*0.95 - p.y*1.25); slat > 0.28 && p.x > -2.0*lay.xScale {
						col = deskMixRGBA(col, th.skyBot, th.sunSlats*0.16*(slat-0.28))
					}
				}
				if p.y > lay.apronBotY {
					col = color.RGBA{14, 11, 22, 255}
				}
				consider(deskHit3D{t: t, p: p, n: vec3d{0, 0, -1}, albedo: col, surf: surf})
			}
		}
	}

	// Window inner jamb walls (x = ±winHalfW, z in [zWall, zWinBack]).
	if math.Abs(rd.x) > 1e-6 {
		for _, jx := range [2]float64{-lay.winHalfW, lay.winHalfW} {
			t := (jx - ro.x) / rd.x
			if t > 0 {
				p := ro.add(rd.mul(t))
				if p.z >= lay.zWall-0.02 && p.z <= lay.zWinBack+0.02 && p.y >= lay.winTopY && p.y <= lay.winBotY {
					nx := 1.0
					if jx > 0 {
						nx = -1.0
					}
					col := deskMixRGBA(color.RGBA{56, 44, 72, 255}, th.wood, 0.35)
					consider(deskHit3D{t: t, p: p, n: vec3d{nx, 0, 0}, albedo: col, surf: deskSurfWoodTrim})
				}
			}
		}
	}
	// Window top lintel underside (y = winTopY, z in [zWall, zWinBack]).
	if math.Abs(rd.y) > 1e-6 {
		t := (lay.winTopY - ro.y) / rd.y
		if t > 0 {
			p := ro.add(rd.mul(t))
			if p.z >= lay.zWall-0.02 && p.z <= lay.zWinBack+0.02 && math.Abs(p.x) <= lay.winHalfW {
				consider(deskHit3D{t: t, p: p, n: vec3d{0, 1, 0}, albedo: color.RGBA{34, 27, 46, 255}, surf: deskSurfWoodTrim})
			}
		}
	}

	// Protruding 3D wooden window sill ledge (x in [-winHalfW-0.7, +winHalfW+0.7]).
	if sill, ok := intersectAABB3D(ro, rd,
		vec3d{-lay.winHalfW - 0.75, lay.winBotY, lay.sillFrontZ},
		vec3d{+lay.winHalfW + 0.75, lay.sillBotY, lay.zWinBack}); ok {
		sill.albedo = deskMixRGBA(th.wood, color.RGBA{86, 66, 84, 255}, 0.42)
		if sill.n.y < -0.5 {
			sill.albedo = deskMixRGBA(sill.albedo, color.RGBA{181, 132, 91, 255}, 0.38)
		}
		sill.surf = deskSurfWoodTrim
		sill.gloss = 0.20
		consider(sill)
	}

	// 2 Recessed 3D vertical wooden mullions inside the window bay.
	for _, mx := range [2]float64{-lay.mullionX, lay.mullionX} {
		if mHit, ok := intersectAABB3D(ro, rd,
			vec3d{mx - lay.mullionHalf, lay.winTopY - 0.1, lay.zWinBack - 0.55},
			vec3d{mx + lay.mullionHalf, lay.winBotY + 0.05, lay.zWinBack}); ok {
			mHit.albedo = color.RGBA{46, 36, 62, 255}
			if math.Abs(mHit.n.x) > 0.5 {
				mHit.albedo = color.RGBA{64, 50, 82, 255}
			}
			mHit.surf = deskSurfWoodTrim
			consider(mHit)
		}
	}

	// Distant 3D sky & neighborhood rooftops at z = zSky seen only through unobstructed glass panes.
	if rd.z > 1e-6 && best.t >= 1e8 {
		t := (lay.zSky - ro.z) / rd.z
		if t > 0 {
			p := ro.add(rd.mul(t))
			skyCol := sampleSky3D(p, lay, th)
			consider(deskHit3D{t: t, p: p, n: vec3d{0, 0, -1}, albedo: skyCol, surf: deskSurfSky, unlit: true})
		}
	}
	return best
}

func sampleSky3D(p vec3d, lay *deskLayout3D, th *diurnalStop) color.RGBA {
	// Map distant sky-plane coordinates cleanly across twilight gradient & city skyline.
	topSkyY := -16.8
	botSkyY := 0.8
	skyT := min(max((p.y-topSkyY)/(botSkyY-topSkyY), 0), 1)
	var col color.RGBA
	if skyT < 0.55 {
		col = deskMixRGBA(th.skyTop, th.skyMid, skyT/0.55)
	} else {
		col = deskMixRGBA(th.skyMid, th.skyBot, (skyT-0.55)/0.45)
	}
	skyHalfW := lay.winHalfW * (lay.zSky / lay.zWall)
	nx := min(max((p.x+skyHalfW)/(2*skyHalfW), 0), 1)

	if th.bokeh > 0.05 {
		bokehs := [4]struct {
			fx, fy, rad float64
			col         color.RGBA
		}{
			{.26, .48, 2.8, color.RGBA{108, 200, 176, 255}},
			{.44, .40, 3.6, color.RGBA{250, 110, 82, 255}},
			{.78, .50, 3.0, color.RGBA{242, 177, 60, 255}},
			{.16, .58, 2.5, color.RGBA{250, 110, 82, 255}},
		}
		for _, bk := range bokehs {
			bx := -skyHalfW + 2*skyHalfW*bk.fx
			by := topSkyY + (botSkyY-topSkyY)*bk.fy
			du, dv := p.x-bx, p.y-by
			if d2 := du*du + dv*dv*1.5; d2 < bk.rad*bk.rad {
				w := math.Pow(1-d2/(bk.rad*bk.rad), 1.5) * th.bokeh * 0.36
				col = deskMixRGBA(col, bk.col, w)
			}
		}
	}

	roofFrac, facadeTint := shibuyaRoofHeight(nx)
	if skyT >= 1.0-roofFrac*0.92 {
		bldg := deskMixRGBA(color.RGBA{22, 18, 34, 255}, th.skyMid, 0.16+facadeTint*0.45)
		if th.sunSlats > 0.2 && math.Mod(p.x+50, 5.2) < 1.1 {
			bldg = deskMixRGBA(bldg, th.skyBot, 0.18*th.sunSlats)
		}
		col = bldg
		gx := int(math.Floor((p.x + 40) * 1.4))
		gy := int(math.Floor((p.y + 40) * 1.8))
		if gx%2 == 0 && gy%2 == 0 {
			lit := 0.20 + 0.55*th.bokeh
			h := gx*13 + gy*29
			if h%6 == 0 {
				col = deskMixRGBA(col, color.RGBA{242, 177, 60, 255}, 0.45*lit)
			} else if h%9 == 0 {
				col = deskMixRGBA(col, color.RGBA{250, 110, 82, 255}, 0.32*lit)
			}
		}
	}
	return col
}

func intersectDeskSlab3D(ro, rd vec3d, lay *deskLayout3D, th *diurnalStop) deskHit3D {
	best := deskHit3D{t: 1e9}
	consider := func(c deskHit3D) {
		if c.t > 1e-4 && c.t < best.t {
			best = c
		}
	}

	// Horizontal oak tabletop plane y = deskY, z in [zDeskFront + bevelR, zWall].
	if math.Abs(rd.y) > 1e-6 {
		t := (lay.deskY - ro.y) / rd.y
		if t > 0 {
			p := ro.add(rd.mul(t))
			if p.z >= lay.zDeskFront+lay.bevelR && p.z <= lay.zWall+0.05 && math.Abs(p.x) <= lay.sideWallX+0.1 {
				grain := 0.5 + 0.5*math.Sin(p.x*0.72+math.Sin(p.z*1.35)*1.9)
				plank := math.Sin(p.z * 2.2)
				oak := color.RGBA{181, 132, 91, 255}
				col := deskMixRGBA(th.wood, oak, 0.50+grain*0.18)
				if plank > 0.92 {
					col = deskScaleRGBA(col, 0.88)
				}
				consider(deskHit3D{t: t, p: p, n: vec3d{0, -1, 0}, albedo: col, gloss: 0.22, surf: deskSurfTop, isTop: true})
			}
		}
	}

	// Quarter-cylinder rounded front edge bevel: axis along X at (y = deskY + bevelR, z = zDeskFront + bevelR).
	cyAxis := lay.deskY + lay.bevelR
	czAxis := lay.zDeskFront + lay.bevelR
	oy := ro.y - cyAxis
	oz := ro.z - czAxis
	a := rd.y*rd.y + rd.z*rd.z
	if a > 1e-8 {
		b := oy*rd.y + oz*rd.z
		c := oy*oy + oz*oz - lay.bevelR*lay.bevelR
		disc := b*b - a*c
		if disc >= 0 {
			sqrtD := math.Sqrt(disc)
			t := (-b - sqrtD) / a
			if t <= 1e-4 {
				t = (-b + sqrtD) / a
			}
			if t > 1e-4 {
				p := ro.add(rd.mul(t))
				if p.y <= cyAxis+1e-4 && p.z <= czAxis+1e-4 && math.Abs(p.x) <= lay.sideWallX+0.1 {
					ny := (p.y - cyAxis) / lay.bevelR
					nz := (p.z - czAxis) / lay.bevelR
					col := deskMixRGBA(th.wood, color.RGBA{204, 152, 106, 255}, 0.55)
					consider(deskHit3D{t: t, p: p, n: vec3d{0, ny, nz}.norm(), albedo: col, gloss: 0.28, surf: deskSurfTop})
				}
			}
		}
	}

	// Front vertical wooden desk apron z = zDeskFront, y in [deskY + bevelR, apronBotY].
	if math.Abs(rd.z) > 1e-6 {
		t := (lay.zDeskFront - ro.z) / rd.z
		if t > 0 {
			p := ro.add(rd.mul(t))
			if p.y >= lay.deskY+lay.bevelR && p.y <= lay.apronBotY && math.Abs(p.x) <= lay.sideWallX+0.1 {
				grain := 0.5 + 0.5*math.Sin(p.x*0.85+p.y*2.4)
				col := deskMixRGBA(th.wood, color.RGBA{136, 94, 62, 255}, 0.48+0.14*grain)
				if p.y > lay.apronBotY-0.25 {
					col = deskScaleRGBA(col, 0.72)
				}
				consider(deskHit3D{t: t, p: p, n: vec3d{0, 0, -1}, albedo: col, gloss: 0.14, surf: deskSurfWoodTrim})
			}
		}
	}

	// Underside soffit shadow lip y = apronBotY, z >= zDeskFront (receding into dark cavity under desk).
	if math.Abs(rd.y) > 1e-6 {
		t := (lay.apronBotY - ro.y) / rd.y
		if t > 0 {
			p := ro.add(rd.mul(t))
			if p.z >= lay.zDeskFront && p.z <= lay.zWall && math.Abs(p.x) <= lay.sideWallX+0.1 {
				consider(deskHit3D{t: t, p: p, n: vec3d{0, 1, 0}, albedo: color.RGBA{18, 14, 26, 255}, surf: deskSurfWall})
			}
		}
	}

	return best
}

func intersectAABB3D(ro, rd, bmin, bmax vec3d) (deskHit3D, bool) {
	t0, t1 := 1e-4, 1e9
	var nHit vec3d
	checkAxis := func(o, d, mn, mx, axX, axY, axZ float64) bool {
		if math.Abs(d) < 1e-9 {
			return o >= mn && o <= mx
		}
		inv := 1.0 / d
		ta := (mn - o) * inv
		tb := (mx - o) * inv
		sign := -1.0
		if ta > tb {
			ta, tb = tb, ta
			sign = 1.0
		}
		if ta > t0 {
			t0 = ta
			nHit = vec3d{axX * sign, axY * sign, axZ * sign}
		}
		if tb < t1 {
			t1 = tb
		}
		return t0 <= t1
	}
	if !checkAxis(ro.x, rd.x, bmin.x, bmax.x, 1, 0, 0) ||
		!checkAxis(ro.y, rd.y, bmin.y, bmax.y, 0, 1, 0) ||
		!checkAxis(ro.z, rd.z, bmin.z, bmax.z, 0, 0, 1) {
		return deskHit3D{}, false
	}
	if nHit.len2() < 0.5 {
		return deskHit3D{}, false
	}
	return deskHit3D{t: t0, p: ro.add(rd.mul(t0)), n: nHit}, true
}

func intersectYawedBox3D(ro, rd, center, half vec3d, cosY, sinY float64) (deskHit3D, bool) {
	rel := ro.sub(center)
	lo := vec3d{rel.x*cosY + rel.z*sinY, rel.y, -rel.x*sinY + rel.z*cosY}
	ld := vec3d{rd.x*cosY + rd.z*sinY, rd.y, -rd.x*sinY + rd.z*cosY}
	h, ok := intersectAABB3D(lo, ld, half.mul(-1), half)
	if !ok {
		return deskHit3D{}, false
	}
	h.p = ro.add(rd.mul(h.t))
	h.n = vec3d{h.n.x*cosY - h.n.z*sinY, h.n.y, h.n.x*sinY + h.n.z*cosY}.norm()
	return h, true
}

func intersectSphere3D(ro, rd, center vec3d, r float64) (deskHit3D, bool) {
	t, ok := hitSphereFast(ro, rd, center, r)
	if !ok {
		return deskHit3D{}, false
	}
	p := ro.add(rd.mul(t))
	return deskHit3D{t: t, p: p, n: p.sub(center).norm()}, true
}

func intersectCapsule3D(ro, rd, pa, pb vec3d, r float64) (deskHit3D, bool) {
	ba := pb.sub(pa)
	oa := ro.sub(pa)
	baba := ba.dot(ba)
	if baba < 1e-8 {
		return intersectSphere3D(ro, rd, pa, r)
	}
	bard := ba.dot(rd)
	baoa := ba.dot(oa)
	rdrd := rd.dot(rd)
	oaord := oa.dot(rd)
	oaoa := oa.dot(oa)

	k2 := baba*rdrd - bard*bard
	k1 := baba*oaord - baoa*bard
	k0 := baba*oaoa - baoa*baoa - r*r*baba
	h := k1*k1 - k2*k0
	if h < 0 {
		return deskHit3D{}, false
	}
	h = math.Sqrt(h)
	t := (-k1 - h) / k2
	if t > 1e-4 {
		y := baoa + t*bard
		if y > 0 && y < baba {
			p := ro.add(rd.mul(t))
			n := p.sub(pa.add(ba.mul(y / baba))).norm()
			return deskHit3D{t: t, p: p, n: n}, true
		}
	}
	// End-cap spheres
	bestT := 1e9
	var bestN, bestP vec3d
	for _, end := range [2]vec3d{pa, pb} {
		if ts, ok := hitSphereFast(ro, rd, end, r); ok && ts < bestT {
			bestT = ts
			bestP = ro.add(rd.mul(ts))
			bestN = bestP.sub(end).norm()
		}
	}
	if bestT < 1e8 {
		return deskHit3D{t: bestT, p: bestP, n: bestN}, true
	}
	return deskHit3D{}, false
}

func intersectCylinderY3D(ro, rd vec3d, cx, cz, yMin, yMax, r float64) (deskHit3D, bool) {
	best := deskHit3D{t: 1e9}
	ox, oz := ro.x-cx, ro.z-cz
	a := rd.x*rd.x + rd.z*rd.z
	if a > 1e-9 {
		b := ox*rd.x + oz*rd.z
		c := ox*ox + oz*oz - r*r
		if disc := b*b - a*c; disc >= 0 {
			if t := (-b - math.Sqrt(disc)) / a; t > 1e-4 {
				p := ro.add(rd.mul(t))
				if p.y >= yMin && p.y <= yMax {
					best = deskHit3D{t: t, p: p, n: vec3d{(p.x - cx) / r, 0, (p.z - cz) / r}}
				}
			}
		}
	}
	if rd.y > 1e-6 {
		if t := (yMin - ro.y) / rd.y; t > 1e-4 && t < best.t {
			p := ro.add(rd.mul(t))
			if (p.x-cx)*(p.x-cx)+(p.z-cz)*(p.z-cz) <= r*r {
				best = deskHit3D{t: t, p: p, n: vec3d{0, -1, 0}}
			}
		}
	}
	return best, best.t < 1e8
}

func intersectMug3D(ro, rd vec3d, lay *deskLayout3D) (deskHit3D, bool) {
	best := deskHit3D{t: 1e9}
	cx, cz := lay.mugCenter.x, lay.mugCenter.z
	yTop, yBot := lay.deskY-lay.mugH, lay.deskY
	ox, oz := ro.x-cx, ro.z-cz
	a := rd.x*rd.x + rd.z*rd.z

	// Outer terracotta body.
	if a > 1e-9 {
		b := ox*rd.x + oz*rd.z
		c := ox*ox + oz*oz - lay.mugR*lay.mugR
		if disc := b*b - a*c; disc >= 0 {
			if t := (-b - math.Sqrt(disc)) / a; t > 1e-4 {
				p := ro.add(rd.mul(t))
				if p.y >= yTop && p.y <= yBot {
					best = deskHit3D{
						t: t, p: p, n: vec3d{(p.x - cx) / lay.mugR, 0, (p.z - cz) / lay.mugR},
						albedo: color.RGBA{0xd9, 0x7b, 0x59, 0xff}, gloss: 0.35, surf: deskSurfProp,
					}
				}
			}
		}
	}
	// Glazed cream top rim annulus (mugInR <= r <= mugR).
	if rd.y > 1e-6 {
		if t := (yTop - ro.y) / rd.y; t > 1e-4 && t < best.t {
			p := ro.add(rd.mul(t))
			r2 := (p.x-cx)*(p.x-cx) + (p.z-cz)*(p.z-cz)
			if r2 >= lay.mugInR*lay.mugInR && r2 <= lay.mugR*lay.mugR {
				best = deskHit3D{
					t: t, p: p, n: vec3d{0, -1, 0},
					albedo: color.RGBA{0xf6, 0xdf, 0xc7, 0xff}, gloss: 0.45, surf: deskSurfProp,
				}
			}
		}
	}
	// Inside concave ceramic wall & recessed dark-roast crema coffee surface.
	if a > 1e-9 {
		b := ox*rd.x + oz*rd.z
		c := ox*ox + oz*oz - lay.mugInR*lay.mugInR
		if disc := b*b - a*c; disc >= 0 {
			// Far intersection hits the visible back inner wall of the hollow mug.
			if t := (-b + math.Sqrt(disc)) / a; t > 1e-4 && t < best.t {
				p := ro.add(rd.mul(t))
				if p.y >= yTop && p.y <= lay.coffeeY {
					best = deskHit3D{
						t: t, p: p, n: vec3d{-(p.x - cx) / lay.mugInR, 0, -(p.z - cz) / lay.mugInR},
						albedo: color.RGBA{0xc8, 0xa8, 0x8e, 0xff}, gloss: 0.25, surf: deskSurfProp,
					}
				}
			}
		}
	}
	if rd.y > 1e-6 {
		if t := (lay.coffeeY - ro.y) / rd.y; t > 1e-4 && t < best.t {
			p := ro.add(rd.mul(t))
			r2 := (p.x-cx)*(p.x-cx) + (p.z-cz)*(p.z-cz)
			if r2 <= lay.mugInR*lay.mugInR {
				radial := math.Sqrt(r2) / lay.mugInR
				col := deskMixRGBA(color.RGBA{0x44, 0x28, 0x1c, 0xff}, color.RGBA{0xb8, 0x84, 0x56, 0xff}, radial*0.72)
				best = deskHit3D{
					t: t, p: p, n: vec3d{0, -1, 0},
					albedo: col, gloss: 0.55, surf: deskSurfProp,
				}
			}
		}
	}
	return best, best.t < 1e8
}

func intersectLampHood3D(ro, rd, apex, dir vec3d, length, r0, r1 float64) (deskHit3D, bool) {
	// Frustum along unit dir from s in [0, length] with linear radius r(s) = r0 + m*s.
	m := (r1 - r0) / max(length, 1e-5)
	oc := ro.sub(apex)
	z0 := oc.dot(dir)
	dz := rd.dot(dir)
	r0oc := r0 + m*z0
	a := rd.dot(rd) - dz*dz*(1.0+m*m)
	b := 2.0 * (oc.dot(rd) - dz*(z0+m*r0oc))
	c := oc.dot(oc) - z0*z0 - r0oc*r0oc
	disc := b*b - 4.0*a*c
	best := deskHit3D{t: 1e9}

	checkConeT := func(t float64, inside bool) {
		if t <= 1e-4 || t >= best.t {
			return
		}
		p := ro.add(rd.mul(t))
		s := p.sub(apex).dot(dir)
		if s < 0 || s > length {
			return
		}
		proj := apex.add(dir.mul(s))
		radVec := p.sub(proj)
		radLen := math.Sqrt(radVec.len2())
		if radLen < 1e-6 {
			return
		}
		n := radVec.mul(1.0 / radLen).sub(dir.mul(m)).norm()
		col := color.RGBA{0xe8, 0x92, 0x4c, 0xff}
		unlit := false
		if inside {
			n = n.mul(-1)
			col = color.RGBA{0xff, 0xe4, 0x9e, 0xff}
			unlit = true
		}
		best = deskHit3D{t: t, p: p, n: n, albedo: col, gloss: 0.42, surf: deskSurfProp, unlit: unlit}
	}

	if math.Abs(a) > 1e-8 && disc >= 0 {
		sqrtD := math.Sqrt(disc)
		t1 := (-b - sqrtD) / (2.0 * a)
		t2 := (-b + sqrtD) / (2.0 * a)
		if t1 > t2 {
			t1, t2 = t2, t1
		}
		checkConeT(t1, false)
		if best.t >= 1e8 {
			checkConeT(t2, true)
		}
	}
	// Back cap at s = 0
	if math.Abs(dz) > 1e-6 {
		if t := -z0 / dz; t > 1e-4 && t < best.t {
			p := ro.add(rd.mul(t))
			if p.sub(apex).len2() <= r0*r0 {
				best = deskHit3D{t: t, p: p, n: dir.mul(-1), albedo: color.RGBA{0xd6, 0x7e, 0x3e, 0xff}, gloss: 0.35, surf: deskSurfProp}
			}
		}
	}
	return best, best.t < 1e8
}

func (p *AppleMusicPage) drawMartinaiseDetails(x, y, w, h, gw, gh int, lay *deskLayout3D, th *diurnalStop, now time.Time) {
	subN := gw * gh
	if th.stars > 0.15 && h > 3 && len(p.desk.gbuf) == w*h*subN {
		skyStars := [4]struct {
			wx, wy float64
			glyph  string
		}{
			{-1.4 * lay.xScale, -9.2, "✦"},
			{+0.8 * lay.xScale, -6.4, "·"},
			{+5.8 * lay.xScale, -8.5, "✧"},
			{+7.4 * lay.xScale, -5.2, "⋆"},
		}
		aspect := p.desk.cam.aspect
		for i, st := range skyStars {
			sxPix, syPix, _, ok := p.desk.cam.project(vec3d{st.wx, st.wy, lay.zSky})
			if !ok {
				continue
			}
			cx, cy := int(math.Floor(sxPix)), int(math.Floor(syPix/aspect))
			if cx <= 0 || cx >= w-1 || cy <= 0 || cy >= h-1 {
				continue
			}
			// Depth-test against the 3D G-buffer so mullions/window trim occlude distant stars.
			if p.desk.gbuf[(cy*w+cx)*subN].surf != deskSurfSky {
				continue
			}
			twinkle := 0.75
			if p.level != motion.Off {
				twinkle = 0.55 + 0.45*math.Sin(float64(now.UnixMilli())/1800+float64(i)*1.7)
			}
			bg := appleNeon.Bg
			if c, ok := p.screen.cellAt(x+cx, y+cy); ok && c.HasBg {
				bg = c.Bg
			}
			fg := deskMixRGBA(bg, color.RGBA{244, 232, 206, 255}, th.stars*twinkle)
			p.screen.setRGBA(x+cx, y+cy, st.glyph, 1, fg, bg, true, true, false)
		}
	}
	if p.level == motion.Off || th.lampInt <= 0.3 || w < 20 || h < 8 {
		return
	}
	// Ultra-slow drifting 3D dust motes inside the warm desk lamp beam (never audio-driven).
	aspect := p.desk.cam.aspect
	tSec := float64(now.UnixMilli()) / 1000.0
	for i := range 2 {
		mp := lay.bulb3D.add(lay.spotDir.mul(1.6 + float64(i)*1.4))
		mp.x += math.Sin(tSec/6.0+float64(i)*2.1) * 0.9
		mp.y += math.Cos(tSec/7.5+float64(i)*1.5) * 0.55
		sxPix, syPix, _, ok := p.desk.cam.project(mp)
		if !ok {
			continue
		}
		mx, my := x+int(math.Round(sxPix)), y+int(math.Round(syPix/aspect))
		if mx > x+1 && mx < x+w-2 && my > y+1 && my < y+h-2 {
			if c, ok := p.screen.cellAt(mx, my); ok && c.HasBg {
				p.screen.setRGBA(mx, my, "·", 1, npLerpRGBA(c.Bg, th.lampCore, 0.62*th.lampInt), c.Bg, true, true, false)
			}
		}
	}
}

func shibuyaRoofHeight(nx float64) (roofFrac float64, shade float64) {
	switch {
	case nx < .14:
		return .28, .18
	case nx < .26:
		return .18, .26
	case nx < .36:
		return .34, .14
	case nx < .50:
		return .22, .24
	case nx < .62:
		return .38, .12
	case nx < .74:
		return .20, .24
	case nx < .86:
		return .30, .16
	default:
		return .20, .22
	}
}

func smoothstepDesk(e0, e1, x float64) float64 {
	t := min(max((x-e0)/(e1-e0), 0), 1)
	return t * t * (3 - 2*t)
}
