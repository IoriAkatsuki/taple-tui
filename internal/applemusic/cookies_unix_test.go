//go:build !windows

package applemusic

import (
	"os"
	"path/filepath"
	"testing"
)

func secureCookieTestFile(t *testing.T, path string) {
	t.Helper()
	if err := os.Chmod(path, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestCookieFileRejectsPublicAndSymlinkInput(t *testing.T) {
	file := filepath.Join(t.TempDir(), "synthetic")
	if err := os.WriteFile(file, []byte(`[{"domain":"music.apple.com","name":"a","value":"fixture"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCookieFile(file); err == nil {
		t.Fatal("public input accepted")
	}
	if err := os.Chmod(file, 0600); err != nil {
		t.Fatal(err)
	}
	link := file + "-link"
	if err := os.Symlink(file, link); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCookieFile(link); err == nil {
		t.Fatal("symlink input accepted")
	}
}
