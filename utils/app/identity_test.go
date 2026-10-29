package app

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestTapleRootControlsConfigAndLogsWithoutLegacyFallback(t *testing.T) {
	previous := pureRoot
	t.Cleanup(func() { pureRoot = previous })
	pureRoot = ""
	root := t.TempDir()
	canonical, legacy := filepath.Join(root, "canonical"), filepath.Join(root, "legacy")
	t.Setenv("TAPLE_ROOT", canonical)
	t.Setenv("MUSICFOX_ROOT", legacy)
	if got := ConfigFilePath(); got != filepath.Join(canonical, "config.toml") {
		t.Fatalf("canonical config root ignored: %s", got)
	}
	if got := LogDir(); got != filepath.Join(canonical, "log") {
		t.Fatalf("canonical log root ignored: %s", got)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatal("legacy override was accessed")
	}

	t.Setenv("TAPLE_ROOT", "")
	config := filepath.Join(root, "config")
	t.Setenv("XDG_CONFIG_HOME", config)
	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", config)
	case "darwin":
		t.Setenv("HOME", filepath.Join(root, "home"))
		config = filepath.Join(root, "home", "Library", "Application Support")
	}
	if got := ConfigFilePath(); got != filepath.Join(config, "go-musicfox", "config.toml") {
		t.Fatalf("legacy environment was not ignored or persistent path moved: %s", got)
	}
	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Fatal("legacy directory was created")
	}
}
