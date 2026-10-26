package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRPMPrereleaseVersionKeepsBinaryVersion(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("RPM packaging targets Linux")
	}
	script, err := os.ReadFile("../rpm.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ tag, version string }{
		{"v1.1.0-rc.1", "1.1.0~rc.1"},
		{"v1.1.0-rc-2-extra", "1.1.0~rc.2.extra"},
		{"v1.1.0", "1.1.0"},
	} {
		t.Run(tc.tag, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"hack/rpm.sh": string(script), "LICENSE": "license", "README.md": "readme", "README.en.md": "readme",
				"deploy/taple-tui.desktop": "desktop",
				"tools/git": `#!/bin/sh
case "$1" in
  describe) printf '%s\n' "$RPM_TEST_TAG" ;;
  log) echo 20261027 ;;
  rev-parse) echo abc123 ;;
  *) exit 1 ;;
esac
`,
				"tools/make": `#!/bin/sh
set -eu
test "$1" = build
mkdir -p bin
printf '%s' "$VERSION" > bin/taple-tui
`,
				"tools/rpmbuild": `#!/bin/sh
set -eu
for arg do
  case "$arg" in
    '_topdir '*) top=${arg#_topdir } ;;
    'deck_version '*) version=${arg#deck_version } ;;
  esac
done
case "$version" in *-*) echo 'RPM Version cannot contain hyphens' >&2; exit 1 ;; esac
mkdir -p "$top/RPMS"
printf '%s' "$version" > "$top/RPMS/taple-tui.rpm"
`,
			}
			for name, data := range files {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(data), 0755); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", filepath.Join(root, "tools")+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("RPM_TEST_TAG", tc.tag)
			t.Setenv("VERSION", "v1.1.0-rc.1-custom")
			if out, err := exec.Command("bash", filepath.Join(root, "hack/rpm.sh")).CombinedOutput(); err != nil {
				t.Fatalf("RPM script failed: %v\n%s", err, out)
			}
			for file, want := range map[string]string{
				"dist/taple-tui.rpm": tc.version + "^20261027gitabc123",
				"bin/taple-tui":      "v1.1.0-rc.1-custom",
			} {
				got, err := os.ReadFile(filepath.Join(root, file))
				if err != nil || string(got) != want {
					t.Fatalf("%s=%q, want %q; error=%v", file, got, want, err)
				}
			}
		})
	}
}
