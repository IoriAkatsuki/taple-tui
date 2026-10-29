package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestBuildScriptInstallsOnlyTapleAndPreservesLegacyFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Bash build script targets Linux/macOS")
	}
	script, err := os.ReadFile("../build.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, preserveLegacy := range []bool{false, true} {
		name := "clean install"
		if preserveLegacy {
			name = "existing legacy files"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"hack/build.sh": string(script),
				"tools/go": `#!/bin/sh
set -eu
case "$1" in
  env)
    case "$2" in
      GOOS) printf 'linux\n' ;;
      GOBIN) printf '%s\n' "$GOBIN" ;;
      *) exit 1 ;;
    esac ;;
  build)
    while [ "$#" -gt 0 ]; do
      if [ "$1" = -o ]; then shift; target=$1; fi
      shift
    done
    printf '%s' "$VERSION" > "$target"
    chmod 755 "$target" ;;
  *) exit 1 ;;
esac
`,
			}
			if preserveLegacy {
				for _, dir := range []string{"bin", "installed"} {
					for _, legacy := range []string{"tapefox", "taple-tui"} {
						files[dir+"/"+legacy] = "existing legacy binary"
					}
				}
			}
			for name, content := range files {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(content), 0755); err != nil {
					t.Fatal(err)
				}
			}
			cmd := exec.Command("bash", filepath.Join(root, "hack/build.sh"), "install")
			cmd.Env = append(os.Environ(),
				"GOBINARY="+filepath.Join(root, "tools/go"), "GOOS=linux", "VERSION=v1.2.1-test",
				"BUILD_TARGET=", "GOBIN="+filepath.Join(root, "installed"))
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("build/install failed: %v\n%s", err, out)
			}
			for _, dir := range []string{"bin", "installed"} {
				path := filepath.Join(root, dir, "taple")
				if got, err := os.ReadFile(path); err != nil || string(got) != "v1.2.1-test" {
					t.Errorf("%s=%q, error=%v", path, got, err)
				}
				for _, legacy := range []string{"tapefox", "taple-tui"} {
					path := filepath.Join(root, dir, legacy)
					if preserveLegacy {
						if got, err := os.ReadFile(path); err != nil || string(got) != "existing legacy binary" {
							t.Errorf("legacy file changed: %s=%q, error=%v", path, got, err)
						}
					} else if _, err := os.Lstat(path); !os.IsNotExist(err) {
						t.Errorf("created legacy artifact %s, error=%v", path, err)
					}
				}
			}
		})
	}
}

func TestReleaseScriptsUseTapleIdentity(t *testing.T) {
	paths := []string{
		"../build.sh", "../build.ps1", "../release-windows.ps1", "../rpm.sh", "../package-linux.sh",
		"../../deploy/taple.desktop", "../../deploy/rpm/taple.spec", "../../.github/workflows/platform-build.yml",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			content := string(data)
			// The real repository URL is intentionally not renamed.
			content = strings.ReplaceAll(content, "https://github.com/IoriAkatsuki/taple-tui", "repository-url")
			for _, legacy := range []string{"taple-tui", "tapefox", "TAPEFOX_", "MUSICFOX_ROOT"} {
				if strings.Contains(content, legacy) {
					t.Errorf("legacy identity %q in %s", legacy, path)
				}
			}
		})
	}
}
