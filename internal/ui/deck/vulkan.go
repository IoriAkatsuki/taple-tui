package deck

import (
	_ "embed"
	"image"
	"sync"

	"github.com/go-musicfox/go-musicfox/internal/ui/deck/vkcompute"
)

//go:generate python3 compile_shader.py
//go:embed walkman.comp.spv
var walkmanSPIRV []byte

// Vulkan owns one hardware compute queue shared by the device and picked tape.
// Text composition and block-glyph fitting remain in the existing renderer.
type Vulkan struct {
	mu      sync.Mutex
	device  *vkcompute.Device
	err     error
	samples []float32
}

func OpenVulkan() (*Vulkan, error) {
	d, err := vkcompute.Open(walkmanSPIRV)
	if err != nil {
		return nil, err
	}
	return &Vulkan{device: d}, nil
}
func (g *Vulkan) Name() string { return g.device.Name() }
func (g *Vulkan) Err() error   { g.mu.Lock(); defer g.mu.Unlock(); return g.err }
func (g *Vulkan) Close()       { g.mu.Lock(); defer g.mu.Unlock(); g.device.Close() }

func (r *WalkmanRenderer) SetVulkan(g *Vulkan) { r.gpu, r.valid = g, false }

func (g *Vulkan) render(v View, cam camera, pose Pose, sc *walkmanScene, frame *Frame, bounds image.Rectangle) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.err != nil {
		return false
	}
	gc, gr := v.Tier.Grid()
	n := bounds.Dx() * bounds.Dy() * gc * gr * 4
	if n <= 0 || n > (1<<20)*4 {
		return false
	}
	if cap(g.samples) < n {
		g.samples = make([]float32, n)
	}
	g.samples = g.samples[:n]
	var p [128]float32
	put := func(i int, values ...float64) {
		for j, x := range values {
			p[i+j] = float32(x)
		}
	}
	vector := func(i int, v vec3) { put(i, v.x, v.y, v.z) }
	put(0, float64(v.W), float64(v.H), float64(gc), float64(gr), cam.aspect, cam.cx, cam.cy, cam.focal, 1/scaleOf(pose))
	vector(9, sc.bodyRot.applyT(vec3{pose.X, pose.Y, camDist}.mul(-1)).mul(1/scaleOf(pose)))
	put(12, sc.bodyRot[:]...)
	crot := sc.cassette.rot
	if sc.flightRot != (mat3{}) {
		crot = sc.flightRot
	}
	put(21, crot[:]...)
	vector(30, sc.position)
	put(33, sc.scale, sc.st.Cassette, sc.st.Door, float64(sc.st.Keys))
	if sc.st.OnlyCassette {
		p[37] = 1
	}
	m := sc.cassette.m
	put(38, sc.st.StorageCover, sc.cassette.detail, sc.st.Radii[0], sc.st.Radii[1], sc.st.Angles[0], sc.st.Angles[1], sc.st.Blur, m.translucency, float64(m.pattern), float64(sc.st.TrackNum))
	for i, c := range []rgb{m.bg, m.hero, m.shell, m.label, m.ink, m.stripe, m.glass, m.tape, m.hub, m.hole, m.clamp, m.screw} {
		put(48+i*3, c.r, c.g, c.b)
	}
	door, hinge := walkmanDoorTransform(sc.st.Door)
	put(84, door[:]...)
	vector(93, hinge)
	p[96] = float32(restNL)
	put(97, float64(bounds.Min.X), float64(bounds.Min.Y), float64(bounds.Dx()), float64(bounds.Dy()))
	if g.err = g.device.Compute(p[:], g.samples); g.err != nil {
		return false
	}
	var px [8]rgb
	for i := range bounds.Dx() * bounds.Dy() {
		covered := false
		for j := 0; j < gc*gr; j++ {
			at := (i*gc*gr + j) * 4
			px[j] = rgb{float64(g.samples[at]), float64(g.samples[at+1]), float64(g.samples[at+2])}
			covered = covered || g.samples[at+3] > 0
		}
		if covered {
			frame.Cells[(bounds.Min.Y+i/bounds.Dx())*v.W+bounds.Min.X+i%bounds.Dx()] = fitCell(px[:gc*gr], v.Tier)
		}
	}
	return true
}
