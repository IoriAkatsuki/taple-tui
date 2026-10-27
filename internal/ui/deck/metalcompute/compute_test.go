package metalcompute

import (
	"go/build"
	"testing"
)

func TestNativeBuildSelection(t *testing.T) {
	for _, goos := range []string{"linux", "windows", "darwin", "freebsd"} {
		for _, arch := range []string{"amd64", "arm64"} {
			for _, cgo := range []bool{false, true} {
				ctx := build.Default
				ctx.GOOS, ctx.GOARCH, ctx.CgoEnabled = goos, arch, cgo
				pkg, err := ctx.ImportDir(".", 0)
				if err != nil {
					t.Fatal(err)
				}
				want := goos == "darwin" && cgo
				if (len(pkg.CgoFiles) != 0) != want || (len(pkg.MFiles) != 0) != want {
					t.Fatalf("%s/%s cgo=%v: incorrect Metal build selection: CGO=%v ObjC=%v", goos, arch, cgo, pkg.CgoFiles, pkg.MFiles)
				}
			}
		}
	}
}
