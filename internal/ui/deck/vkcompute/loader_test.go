//go:build linux && cgo

package vkcompute

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVulkanLoaderDispatchAndPortability(t *testing.T) {
	runCHarness(t, "testdata/loader_test.c")
}

func TestPlatformLoaders(t *testing.T) {
	for _, platform := range []string{"windows"} {
		t.Run(platform, func(t *testing.T) {
			runCHarness(t, "testdata/loader_"+platform+"_test.c")
		})
	}
}

func runCHarness(t *testing.T, source string, flags ...string) {
	t.Helper()
	cc, err := exec.Command("go", "env", "CC").Output()
	if err != nil {
		t.Fatal(err)
	}
	compiler := strings.Fields(strings.TrimSpace(string(cc)))
	binary := filepath.Join(t.TempDir(), "loader-test")
	args := append(compiler[1:], strings.Fields(os.Getenv("CGO_CFLAGS"))...)
	args = append(args, flags...)
	args = append(args, "-std=c11", "-Wall", "-Wextra", "-Werror", "-I.", "-Itestdata", source, "-o", binary)
	if out, err := exec.Command(compiler[0], args...).CombinedOutput(); err != nil {
		t.Fatalf("compile loader regression: %v\n%s", err, out)
	}
	if out, err := exec.Command(binary).CombinedOutput(); err != nil {
		t.Fatalf("loader regression: %v\n%s", err, out)
	}
}
