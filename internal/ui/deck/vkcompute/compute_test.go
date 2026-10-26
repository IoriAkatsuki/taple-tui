package vkcompute

import (
	"go/build"
	"testing"
)

func TestNativeBuildSelection(t *testing.T) {
	for _, goos := range []string{"linux", "windows", "darwin", "freebsd"} {
		for _, cgo := range []bool{false, true} {
			t.Run(goos+map[bool]string{false: "/cpu", true: "/cgo"}[cgo], func(t *testing.T) {
				ctx := build.Default
				ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = goos, "amd64", cgo
				pkg, err := ctx.ImportDir(".", 0)
				if err != nil {
					t.Fatal(err)
				}
				wantNative := cgo && goos != "freebsd"
				if got := len(pkg.CgoFiles) != 0; got != wantNative {
					t.Fatalf("native Vulkan selected=%v, want %v; Go=%v CGO=%v", got, wantNative, pkg.GoFiles, pkg.CgoFiles)
				}
				if got := len(pkg.CFiles) != 0; got != wantNative {
					t.Fatalf("native C sources selected=%v, want %v; C=%v", got, wantNative, pkg.CFiles)
				}
			})
		}
	}
}
