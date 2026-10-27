package deck

import (
	"crypto/sha256"
	"fmt"
	"go/build"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestGPUPlatformSelection(t *testing.T) {
	for _, goos := range []string{"linux", "windows", "darwin"} {
		for _, cgo := range []bool{false, true} {
			ctx := build.Default
			ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = goos, "arm64", cgo
			pkg, err := ctx.ImportDir(".", 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, backend := range []string{"vkcompute", "metalcompute"} {
				want := (backend == "metalcompute") == (goos == "darwin")
				if slices.Contains(pkg.Imports, "github.com/go-musicfox/go-musicfox/internal/ui/deck/"+backend) != want {
					t.Fatalf("%s cgo=%v selected incorrect backend %s", goos, cgo, backend)
				}
			}
		}
	}
}

func TestMetalShaderMatchesSPIRV(t *testing.T) {
	spv, err := os.ReadFile("walkman.comp.spv")
	if err != nil {
		t.Fatal(err)
	}
	msl, err := os.ReadFile("walkman.metal")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{fmt.Sprintf("// SPIR-V SHA256: %x", sha256.Sum256(spv)), "kernel void main0(", "[[buffer(0)]]", "[[buffer(1)]]", "[[thread_position_in_grid]]"} {
		if !strings.Contains(string(msl), required) {
			t.Fatalf("Metal shader missing %q; regenerate with compile_metal.py", required)
		}
	}
}
