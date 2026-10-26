//go:build !windows

package applemusic

import (
	"os"
	"testing"
)

func makeProfileTestLink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}

func assertProfilePrivate(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0700 {
		t.Fatal("profile permissions are not 0700")
	}
}
