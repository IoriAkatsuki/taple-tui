package deck

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Diagnostic-only reconstruction of the CPU scene. The regression below checks
// its fitted pixels against RenderWalkman, including flight and storage cover.
func parityCPUSamples(v View, pose Pose, st WalkmanState) (image.Rectangle, []rgb, []bool, []mat) {
	cam := walkmanCamera(v)
	rot, scale := rotation(pose.Yaw, pose.Pitch, pose.Roll), scaleOf(pose)
	cassetteRot := rot.mul(walkmanVerticalRot)
	m := cassetteMaterials(testPal, st.Style, st.TrackNum)
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
		u, from := max(st.Flight, 0), st.FlightPose
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
	n := bounds.Dx() * bounds.Dy() * gc * gr
	pixels, hits, mats := make([]rgb, n), make([]bool, n), make([]mat, n)
	o := rot.applyT(vec3{pose.X, pose.Y, camDist}.mul(-1)).mul(1 / scale)
	for i := range n {
		cell, bit := i/(gc*gr), i%(gc*gr)
		x, y := bounds.Min.X+cell%bounds.Dx(), bounds.Min.Y+cell/bounds.Dx()
		fx, fy := float64(x)+(float64(bit%gc)+.5)/float64(gc), (float64(y)+(float64(bit/gc)+.5)/float64(gr))*cam.aspect
		d := vec3{(fx - cam.cx) / cam.focal, (fy - cam.cy) / cam.focal, 1}
		pixels[i], mats[i], hits[i] = sc.sample(o, rot.applyT(d).mul(1/scale), d.norm().mul(-1))
	}
	return bounds, pixels, hits, mats
}

type parityTotals struct {
	Samples, NonFinite, HitDelta int
	AbsRGB, SignedRGB            [3]float64
	Max                         float64
	// Maximum absolute channel difference: <=.01, <=.5, <=2, <=16, >16.
	Bins [5]int
}

func (s *parityTotals) add(a, b rgb, ah, bh bool) float64 {
	s.Samples++
	if ah != bh {
		s.HitDelta++
	}
	d := [3]float64{b.r - a.r, b.g - a.g, b.b - a.b}
	for _, x := range d {
		if math.IsNaN(x) || math.IsInf(x, 0) {
			s.NonFinite++
			return -1
		}
	}
	var worst float64
	for i, x := range d {
		s.AbsRGB[i] += math.Abs(x)
		s.SignedRGB[i] += x
		worst = max(worst, math.Abs(x))
	}
	s.Max = max(s.Max, worst)
	bin := 0
	for _, limit := range []float64{.01, .5, 2, 16} {
		if worst <= limit {
			break
		}
		bin++
	}
	s.Bins[bin]++
	return worst
}

type paritySample struct {
	X, Y, Bit, Material int
	CPU, GPU            [3]float64
	CPUHit, GPUHit      bool
	Max                 float64
}

type parityEvidence struct {
	Backend, Device, Variant string
	Tier                    Tier
	Style                   uint8
	Phase                   int
	Bounds                  image.Rectangle
	Raw, Fitted             parityTotals
	ByCPUMaterial           map[mat]*parityTotals
	WorstRaw, WorstFitted   []paritySample
}

func parityKeepWorst(samples []paritySample, s paritySample) []paritySample {
	if s.Max <= 0 || (len(samples) == 12 && s.Max <= samples[11].Max) {
		return samples
	}
	samples = append(samples, s)
	sort.SliceStable(samples, func(i, j int) bool { return samples[i].Max > samples[j].Max })
	return samples[:min(len(samples), 12)]
}

func parityFittedImage(t *testing.T, v View, f *Frame) *image.RGBA {
	t.Helper()
	gc, gr := v.Tier.Grid()
	out := image.NewRGBA(image.Rect(0, 0, v.W*gc, v.H*gr))
	for i, cell := range f.Cells {
		mask := 0
		if cell.Ch != "" {
			mask = glyphMask(t, v.Tier, cell.Ch)
		}
		for bit := range gc * gr {
			c := testPal.Bg
			if cell.Ch != "" {
				c = cell.Bg
				if mask&(1<<bit) != 0 {
					c = cell.Fg
				}
			}
			out.SetRGBA((i%v.W)*gc+bit%gc, (i/v.W)*gr+bit/gc, c)
		}
	}
	return out
}

func logGPUParityDiagnostics(t *testing.T, g *GPU, v View, pose Pose, st WalkmanState, a, b *Frame, phase int, variant string) {
	t.Helper()
	bounds, cpu, hit, mats := parityCPUSamples(v, pose, st)
	g.mu.Lock()
	raw := append([]float32(nil), g.samples...)
	g.mu.Unlock()
	if len(raw) != len(cpu)*4 {
		t.Errorf("diagnostic sample layout mismatch: GPU=%d CPU=%d", len(raw), len(cpu)*4)
		return
	}
	gc, gr := v.Tier.Grid()
	e := parityEvidence{Backend: GPUBackend(), Device: g.Name(), Variant: variant, Tier: v.Tier, Style: st.Style, Phase: phase, Bounds: bounds, ByCPUMaterial: make(map[mat]*parityTotals)}
	images := map[string]*image.RGBA{"cpu-fit": parityFittedImage(t, v, a), "gpu-fit": parityFittedImage(t, v, b)}
	for _, name := range []string{"cpu-raw", "gpu-raw", "raw-diff-x4", "fit-diff-x4"} {
		images[name] = image.NewRGBA(images["cpu-fit"].Bounds())
	}
	for i, c := range cpu {
		cell, bit := i/(gc*gr), i%(gc*gr)
		x, y := bounds.Min.X+cell%bounds.Dx(), bounds.Min.Y+cell/bounds.Dx()
		d, dh := rgb{float64(raw[i*4]), float64(raw[i*4+1]), float64(raw[i*4+2])}, raw[i*4+3] > 0
		worst := e.Raw.add(c, d, hit[i], dh)
		if e.ByCPUMaterial[mats[i]] == nil {
			e.ByCPUMaterial[mats[i]] = new(parityTotals)
		}
		e.ByCPUMaterial[mats[i]].add(c, d, hit[i], dh)
		e.WorstRaw = parityKeepWorst(e.WorstRaw, paritySample{x, y, bit, int(mats[i]), [3]float64{c.r, c.g, c.b}, [3]float64{d.r, d.g, d.b}, hit[i], dh, worst})
		px, py := x*gc+bit%gc, y*gr+bit/gc
		if worst >= 0 {
			images["cpu-raw"].SetRGBA(px, py, c.rgba())
			images["gpu-raw"].SetRGBA(px, py, d.rgba())
			images["raw-diff-x4"].SetRGBA(px, py, rgb{4 * math.Abs(c.r-d.r), 4 * math.Abs(c.g-d.g), 4 * math.Abs(c.b-d.b)}.rgba())
		}
	}
	for y := range v.H * gr {
		for x := range v.W * gc {
			ac, bc := images["cpu-fit"].RGBAAt(x, y), images["gpu-fit"].RGBAAt(x, y)
			c, d := rgbOf(ac), rgbOf(bc)
			ci, bit := (y/gr)*v.W+x/gc, (y%gr)*gc+x%gc
			worst := e.Fitted.add(c, d, a.Cells[ci].Ch != "", b.Cells[ci].Ch != "")
			e.WorstFitted = parityKeepWorst(e.WorstFitted, paritySample{x / gc, y / gr, bit, -1, [3]float64{c.r, c.g, c.b}, [3]float64{d.r, d.g, d.b}, a.Cells[ci].Ch != "", b.Cells[ci].Ch != "", worst})
			images["fit-diff-x4"].SetRGBA(x, y, rgb{4 * math.Abs(c.r-d.r), 4 * math.Abs(c.g-d.g), 4 * math.Abs(c.b-d.b)}.rgba())
		}
	}
	data, err := json.Marshal(e)
	if err != nil {
		t.Error("encode parity evidence:", err)
		return
	}
	t.Logf("GPU parity evidence (RGB units 0..255, signed=GPU-CPU, material IDs=face.go): %s", data)
	rawTotal := e.Raw.AbsRGB[0] + e.Raw.AbsRGB[1] + e.Raw.AbsRGB[2]
	fitTotal := e.Fitted.AbsRGB[0] + e.Fitted.AbsRGB[1] + e.Fitted.AbsRGB[2]
	t.Logf("raw RGB MAE: bounds=%.6f viewport=%.6f; fitted viewport MAE=%.6f (includes coverage mismatches)", rawTotal/float64(max(e.Raw.Samples, 1)*3), rawTotal/float64(v.W*v.H*gc*gr*3), fitTotal/float64(max(e.Fitted.Samples, 1)*3))
	if dir := os.Getenv("TAPLE_GPU_DIAGNOSTICS_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Error(err)
			return
		}
		prefix := filepath.Join(dir, fmt.Sprintf("%s-%s-s%d-p%d", variant, v.Tier, st.Style, phase))
		if err := os.WriteFile(prefix+".json", data, 0600); err != nil {
			t.Error(err)
		}
		for name, img := range images {
			file, err := os.OpenFile(prefix+"-"+name+".png", os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
			if err != nil {
				t.Error(err)
				continue
			}
			encodeErr, closeErr := png.Encode(file, img), file.Close()
			if encodeErr != nil || closeErr != nil {
				t.Errorf("write parity PNG: encode=%v close=%v", encodeErr, closeErr)
			}
		}
		t.Log("saved raw/actual fitted framebuffer PNGs, x4 differences and JSON:", prefix)
	}
}

func TestGPUParityDiagnosticCPUReference(t *testing.T) {
	for _, tier := range []Tier{Quadrant, Sextant, Octant} {
		for phase := range 4 {
			v, pose := View{W: 96, H: 28, Tier: tier, Aspect: 2}, RestPose()
			st := WalkmanState{State: State{Style: 5, TrackNum: 29, Radii: PackRadii(.37, false), Angles: [2]float64{.3, 1.2}}}
			switch phase {
			case 1:
				st.Door, st.Cassette = .6, .4
			case 2:
				st.OnlyCassette, st.Cassette, st.StorageCover = true, 1, .4
				st.FlightFrom, st.FlightPose = Rect{8, 6, 32, 11}, Pose{Yaw: -.24, Pitch: .35, Scale: 1}
			case 3:
				v.Fit, pose, st.Keys = WalkmanWindowFit(v, .82), Pose{Scale: 1}, KeyPlay
			}
			bounds, pixels, hits, _ := parityCPUSamples(v, pose, st)
			var renderer WalkmanRenderer
			want := renderer.RenderWalkman(v, testPal, pose, st)
			got := Frame{W: v.W, H: v.H, Cells: make([]Cell, v.W*v.H)}
			gc, gr := tier.Grid()
			n := gc * gr
			for i := 0; i < len(pixels); i += n {
				covered := false
				for _, hit := range hits[i : i+n] {
					covered = covered || hit
				}
				if covered {
					cell := i / n
					got.Cells[(bounds.Min.Y+cell/bounds.Dx())*v.W+bounds.Min.X+cell%bounds.Dx()] = fitCell(pixels[i:i+n], tier)
				}
			}
			if framebufferHash(t, tier, got.Cells) != framebufferHash(t, tier, want.Cells) {
				t.Fatalf("diagnostic CPU reconstruction diverged: tier=%v phase=%d", tier, phase)
			}
		}
	}
}

func TestGPUParityDiagnosticStatistics(t *testing.T) {
	var stats parityTotals
	stats.add(rgb{1, 2, 3}, rgb{2, 0, 3}, true, false)
	stats.add(rgb{}, rgb{math.NaN(), 0, 0}, false, false)
	if stats.Samples != 2 || stats.NonFinite != 1 || stats.HitDelta != 1 || stats.AbsRGB != [3]float64{1, 2, 0} || stats.SignedRGB != [3]float64{1, -2, 0} || stats.Bins[2] != 1 {
		t.Fatalf("incorrect diagnostic statistics: %+v", stats)
	}
	var worst []paritySample
	for i := range 100 {
		worst = parityKeepWorst(worst, paritySample{Max: float64(i)})
	}
	if len(worst) != 12 || worst[0].Max != 99 || worst[11].Max != 88 {
		t.Fatalf("diagnostic samples must be bounded and sorted: %v", worst)
	}
}
