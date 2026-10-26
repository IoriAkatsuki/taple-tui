package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathsPreserveExistingConfigLocationAndPureIsolation(t *testing.T) {
	previous := pureRoot
	t.Cleanup(func() { pureRoot = previous })
	pureRoot = ""
	root := t.TempDir()
	t.Setenv("MUSICFOX_ROOT", root)
	if got := ConfigFilePath(); got != filepath.Join(root, "config.toml") {
		t.Fatalf("portable configuration: %s", got)
	}
	pure := t.TempDir()
	SetupPureRoot(pure)
	if ConfigFilePath() != filepath.Join(pure, "config.toml") || LogDir() != filepath.Join(pure, "log") {
		t.Fatal("pure paths escaped the temporary directory")
	}
	if _, err := os.Stat(filepath.Join(root, "log")); !os.IsNotExist(err) {
		t.Fatalf("existing root was modified: %v", err)
	}
	if _, err := os.Stat(ConfigFilePath()); !os.IsNotExist(err) {
		t.Fatalf("configuration path lookup created a file: %v", err)
	}
}

func TestXDGPathsDoNotCreateLegacyDataDirectories(t *testing.T) {
	previous := pureRoot
	t.Cleanup(func() { pureRoot = previous })
	pureRoot = ""
	root := t.TempDir()
	t.Setenv("MUSICFOX_ROOT", "")
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(root, "state"))
	if ConfigFilePath() != filepath.Join(root, "config", "go-musicfox", "config.toml") {
		t.Fatal("existing XDG configuration path changed")
	}
	if LogDir() != filepath.Join(root, "state", "go-musicfox", "log") {
		t.Fatal("existing log location changed")
	}
	if _, err := os.Stat(filepath.Join(root, "config")); !os.IsNotExist(err) {
		t.Fatalf("configuration lookup wrote directories: %v", err)
	}
}
