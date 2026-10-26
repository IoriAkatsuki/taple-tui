// Package the native CGO build using an explicit, credential-free file list.
package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"debug/buildinfo"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"
)

func main() {
	version := flag.String("version", os.Getenv("VERSION"), "release version")
	binary := flag.String("binary", "", "native CGO binary (default bin/taple-tui[.exe])")
	output := flag.String("output", "dist", "output directory")
	flag.Parse()
	if err := packageRelease(*version, *binary, *output); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func releaseName(version, goos, goarch string) (string, error) {
	if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._+-]*$`).MatchString(version) {
		return "", fmt.Errorf("invalid release version %q", version)
	}
	if (goos != "linux" && goos != "darwin" && goos != "windows") || (goarch != "amd64" && goarch != "arm64") {
		return "", fmt.Errorf("unsupported release platform %s/%s", goos, goarch)
	}
	return "taple-tui_" + version + "_" + goos + "_" + goarch, nil
}

func packageRelease(version, binary, output string) error {
	name, err := releaseName(version, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return err
	}
	extension, executable := ".tar.gz", "taple-tui"
	if runtime.GOOS == "windows" {
		extension, executable = ".zip", "taple-tui.exe"
	}
	if binary == "" {
		binary = filepath.Join("bin", executable)
	}
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		return fmt.Errorf("read native build metadata: %w", err)
	}
	settings := make(map[string]string)
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	if settings["GOOS"] != runtime.GOOS || settings["GOARCH"] != runtime.GOARCH || settings["CGO_ENABLED"] != "1" {
		return fmt.Errorf("release requires a native %s/%s CGO_ENABLED=1 binary", runtime.GOOS, runtime.GOARCH)
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(output, ".package-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	hash := sha256.New()
	err = writeArchive(io.MultiWriter(f, hash), ".", binary, name, runtime.GOOS)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	archive := filepath.Join(output, name+extension)
	if err := os.Rename(f.Name(), archive); err != nil {
		return err
	}
	checksum := fmt.Sprintf("%x  %s\n", hash.Sum(nil), filepath.Base(archive))
	if err := os.WriteFile(archive+".sha256", []byte(checksum), 0644); err != nil {
		return err
	}
	fmt.Print(checksum)
	return nil
}

func writeArchive(dst io.Writer, root, binary, name, goos string) error {
	executable := "taple-tui"
	if goos == "windows" {
		executable += ".exe"
	}
	files := []struct {
		path, name string
		mode       os.FileMode
	}{
		{binary, executable, 0755},
		{filepath.Join(root, "README.md"), "README.md", 0644},
		{filepath.Join(root, "README.en.md"), "README.en.md", 0644},
		{filepath.Join(root, "LICENSE"), "LICENSE", 0644},
	}
	var zw *zip.Writer
	var gz *gzip.Writer
	var tw *tar.Writer
	if goos == "windows" {
		zw = zip.NewWriter(dst)
	} else {
		gz = gzip.NewWriter(dst)
		tw = tar.NewWriter(gz)
	}
	for _, file := range files {
		info, err := os.Lstat(file.path)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("release input is not a regular file: %s", file.path)
		}
		data, err := os.ReadFile(file.path)
		if err != nil {
			return err
		}
		entry := name + "/" + file.name
		var writer io.Writer = tw
		if zw != nil {
			header := &zip.FileHeader{Name: entry, Method: zip.Deflate}
			header.SetMode(file.mode)
			header.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
			writer, err = zw.CreateHeader(header)
		} else {
			err = tw.WriteHeader(&tar.Header{Name: entry, Mode: int64(file.mode), Size: int64(len(data)), ModTime: time.Unix(0, 0), Format: tar.FormatUSTAR})
		}
		if err != nil {
			return err
		}
		if _, err := writer.Write(data); err != nil {
			return err
		}
	}
	if zw != nil {
		return zw.Close()
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return gz.Close()
}
