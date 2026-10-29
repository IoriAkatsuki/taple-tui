//go:build darwin && cgo

package deck

import (
	"os"
	"strings"
	"testing"

	"github.com/go-musicfox/go-musicfox/internal/ui/deck/metalcompute"
)

// An opt-in A/B experiment, not a replacement for the .8 parity gate. The
// generated shader and production compiler options remain unchanged.
func TestMetalNormalizeDiagnostic(t *testing.T) {
	if os.Getenv("TAPLE_GPU_DIAGNOSTICS") != "1" {
		t.Skip("set TAPLE_GPU_DIAGNOSTICS=1 for native Metal precision diagnostics")
	}
	if !strings.Contains(walkmanMSL, "precise::normalize(") {
		t.Fatal("generated shader no longer contains precise normalization")
	}
	for _, variant := range []string{"fast-normalize", "precise-normalize"} {
		t.Run(variant, func(t *testing.T) {
			source := walkmanMSL
			if variant == "fast-normalize" {
				source = strings.ReplaceAll(source, "precise::normalize(", "fast::normalize(")
			}
			d, err := metalcompute.Open(source)
			if err != nil {
				t.Fatal(err)
			}
			g := &GPU{device: d}
			defer g.Close()
			t.Logf("diagnostic-only %s: MSL 2.0, fastMathEnabled=NO, substitutions=%d; CPU uses float64 1/sqrt(dot)", variant, strings.Count(walkmanMSL, "precise::normalize("))
			v := View{W: 96, H: 28, Tier: Quadrant, Aspect: 2}
			pose := RestPose()
			st := WalkmanState{State: State{TrackNum: 29, Radii: PackRadii(.37, false), Angles: [2]float64{.3, 1.2}}}
			var cpu, gpu WalkmanRenderer
			gpu.SetGPU(g)
			a, b := cpu.RenderWalkman(v, testPal, pose, st), gpu.RenderWalkman(v, testPal, pose, st)
			if err := g.Err(); err != nil {
				t.Fatal(err)
			}
			logGPUParityDiagnostics(t, g, v, pose, st, a, b, 0, variant)
		})
	}
}
