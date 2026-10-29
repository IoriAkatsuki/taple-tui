package errorx

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestTapleWrapperIdentity(t *testing.T) {
	if childEnv != "TAPLE_INTERNAL_CHILD" {
		t.Fatalf("unexpected internal environment: %s", childEnv)
	}
	t.Setenv("TAPLE_INTERNAL_CHILD", "")
	t.Setenv("MUSICFOX_INTERNAL_CHILD", "1")
	cmd := exec.Command(os.Args[0], "-test.run=^TestTapleWrapperHelper$")
	cmd.Env = append(os.Environ(), "TAPLE_TEST_WRAPPER=1", "TAPLE_ROOT="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "taple-child\nPASS" {
		t.Fatalf("wrapper marker/exit propagation: %v %q", err, out)
	}
}

func TestTapleWrapperHelper(t *testing.T) {
	if os.Getenv("TAPLE_TEST_WRAPPER") != "1" {
		return
	}
	RunWrapped(func() {
		if os.Getenv("TAPLE_INTERNAL_CHILD") != "1" {
			os.Exit(3)
		}
		fmt.Println("taple-child")
		os.Exit(0)
	})
}
