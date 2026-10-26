package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReleaseNameRejectsPathsAndUnknownPlatforms(t *testing.T) {
	for _, version := range []string{"", "../secret", "v1/escape", "v1\nextra", "v1 bad"} {
		if _, err := releaseName(version, "linux", "amd64"); err == nil {
			t.Fatalf("accepted unsafe version %q", version)
		}
	}
	for _, platform := range [][2]string{{"plan9", "amd64"}, {"linux", "../arm64"}} {
		if _, err := releaseName("v1.1.0-rc.1", platform[0], platform[1]); err == nil {
			t.Fatalf("accepted unsupported platform %v", platform)
		}
	}
	if name, err := releaseName("v1.1.0-rc.1", "darwin", "arm64"); err != nil || name != "taple-tui_v1.1.0-rc.1_darwin_arm64" {
		t.Fatalf("name=%q error=%v", name, err)
	}
}

func TestArchiveHasOnlyReleaseFilesAndIsDeterministic(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{
		"binary": "native binary", "README.md": "Chinese readme", "README.en.md": "English readme", "LICENSE": "license",
		"secret.env": "must not ship", "profile": "must not ship",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, goos := range []string{"linux", "darwin", "windows"} {
		t.Run(goos, func(t *testing.T) {
			name, err := releaseName("v1.1.0-rc.1", goos, "amd64")
			if err != nil {
				t.Fatal(err)
			}
			var first, second bytes.Buffer
			for _, dst := range []*bytes.Buffer{&first, &second} {
				if err := writeArchive(dst, root, filepath.Join(root, "binary"), name, goos); err != nil {
					t.Fatal(err)
				}
			}
			if !bytes.Equal(first.Bytes(), second.Bytes()) {
				t.Fatal("same input produced different archive bytes")
			}
			binary := "taple-tui"
			if goos == "windows" {
				binary += ".exe"
			}
			want := map[string]string{
				name + "/" + binary: "native binary", name + "/README.md": "Chinese readme",
				name + "/README.en.md": "English readme", name + "/LICENSE": "license",
			}
			got := make(map[string]string)
			if goos == "windows" {
				zr, err := zip.NewReader(bytes.NewReader(first.Bytes()), int64(first.Len()))
				if err != nil {
					t.Fatal(err)
				}
				for _, f := range zr.File {
					r, err := f.Open()
					if err != nil {
						t.Fatal(err)
					}
					data, err := io.ReadAll(r)
					r.Close()
					if err != nil {
						t.Fatal(err)
					}
					got[f.Name] = string(data)
				}
			} else {
				gz, err := gzip.NewReader(bytes.NewReader(first.Bytes()))
				if err != nil {
					t.Fatal(err)
				}
				defer gz.Close()
				tr := tar.NewReader(gz)
				for {
					h, err := tr.Next()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatal(err)
					}
					if h.Typeflag != tar.TypeReg || h.Uid != 0 || h.Gid != 0 || !h.ModTime.IsZero() && h.ModTime.Unix() != 0 {
						t.Fatalf("unexpected metadata: %+v", h)
					}
					if h.Name == name+"/"+binary && h.Mode != 0755 {
						t.Fatal("binary is not executable")
					}
					data, err := io.ReadAll(tr)
					if err != nil {
						t.Fatal(err)
					}
					got[h.Name] = string(data)
				}
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("payload=%v want=%v", got, want)
			}
		})
	}
}

func TestArchiveRejectsSymlinkInput(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("secret.env", filepath.Join(root, "binary")); err != nil {
		t.Skip("symlink creation unavailable:", err)
	}
	var dst bytes.Buffer
	if err := writeArchive(&dst, root, filepath.Join(root, "binary"), "release", "linux"); err == nil {
		t.Fatal("accepted symlink input")
	}
}
