package deck

import (
	"errors"
	"math"
	"os"
	"runtime"
	"testing"

	"github.com/go-musicfox/go-musicfox/internal/ui/deck/metalcompute"
)

func TestGPUWalkmanMatchesCPU(t *testing.T) {
	g, err := OpenGPU()
	if err != nil {
		if runtime.GOOS == "darwin" && !errors.Is(err, metalcompute.ErrUnavailable) {
			t.Fatal(err) // Shader/pipeline failures must not masquerade as absent hardware.
		}
		if os.Getenv("TAPEFOX_REQUIRE_GPU") == "1" || os.Getenv("TAPEFOX_REQUIRE_VULKAN") == "1" {
			t.Fatal(err)
		}
		t.Skip(err)
	}
	defer g.Close()
	t.Log(GPUBackend()+" device:", g.Name())
	var cpu, gpu WalkmanRenderer
	gpu.SetGPU(g)
	for _, tier := range []Tier{Quadrant, Sextant, Octant} {
		for style := uint8(0); style <= 5; style++ {
			for phase := range 4 {
				v := View{W: 96, H: 28, Tier: tier, Aspect: 2}
				pose := RestPose()
				st := WalkmanState{State: State{Style: style, TrackNum: 29, Radii: PackRadii(.37, false), Angles: [2]float64{.3, 1.2}}}
				switch phase {
				case 1:
					st.Door, st.Cassette = .6, .4
				case 2:
					st.OnlyCassette, st.Cassette, st.StorageCover = true, 1, .4
					st.FlightFrom = Rect{8, 6, 32, 11}
					st.FlightPose = Pose{Yaw: -.24, Pitch: .35, Scale: 1}
				case 3:
					v.Fit = WalkmanWindowFit(v, .82)
					pose = Pose{Scale: 1}
					st.Keys = KeyPlay
				}
				a, b := cpu.RenderWalkman(v, testPal, pose, st), gpu.RenderWalkman(v, testPal, pose, st)
				if err := g.Err(); err != nil {
					t.Fatal(err)
				}
				var total float64
				coverage := 0
				gc, gr := tier.Grid()
				for i, c := range a.Cells {
					d := b.Cells[i]
					if (c.Ch == "") != (d.Ch == "") {
						coverage++
						continue
					}
					if c.Ch == "" {
						continue
					}
					am, bm := glyphMask(t, tier, c.Ch), glyphMask(t, tier, d.Ch)
					for bit := range gc * gr {
						ac, bc := c.Bg, d.Bg
						if am&(1<<bit) != 0 {
							ac = c.Fg
						}
						if bm&(1<<bit) != 0 {
							bc = d.Fg
						}
						total += math.Abs(float64(ac.R)-float64(bc.R)) + math.Abs(float64(ac.G)-float64(bc.G)) + math.Abs(float64(ac.B)-float64(bc.B))
					}
				}
				mae := total / float64(v.W*v.H*gc*gr*3)
				if coverage > 2 || mae > .8 {
					t.Fatalf("%v style=%d phase=%d: coverage delta=%d color MAE=%.3f", tier, style, phase, coverage, mae)
				}
			}
		}
	}
}

func TestGPUCloseRejectsFurtherWork(t *testing.T) {
	g, err := OpenGPU()
	if err != nil {
		t.Skip(err)
	}
	g.Close()
	g.Close()
	var r WalkmanRenderer
	r.SetGPU(g)
	f := r.RenderWalkman(View{W: 20, H: 10, Tier: Quadrant}, testPal, RestPose(), WalkmanState{})
	if len(f.Cells) != 200 || g.Err() == nil {
		t.Fatal("closed GPU did not return to CPU rendering")
	}
}

func BenchmarkWalkmanBackend(b *testing.B) {
	for _, backend := range []string{"cpu", "gpu"} {
		b.Run(backend, func(b *testing.B) {
			var r WalkmanRenderer
			if backend == "gpu" {
				g, err := OpenGPU()
				if err != nil {
					b.Skip(err)
				}
				defer g.Close()
				r.SetGPU(g)
			}
			v := View{W: 140, H: 36, Tier: Octant, Aspect: 2, Fit: Rect{74, 1, 63, 28}}
			st := WalkmanState{State: State{Style: 4, Radii: PackRadii(.4, false)}, Cassette: 1, Door: 1, FlightFrom: Rect{24, 12, 36, 12}, FlightPose: Pose{Yaw: -.24, Pitch: .35, Scale: 1}}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				st.Flight = float64(i%100) / 100
				r.RenderWalkman(v, testPal, RestPose(), st)
			}
		})
	}
}
