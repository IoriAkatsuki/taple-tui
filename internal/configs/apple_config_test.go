package configs

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestConfigReadsOnlyAppleSettingsWithoutRewriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	original := []byte("# keep old user preferences\n[main]\nmotion = 'off'\nblockGlyphs = 'quadrant'\nframeRate = 30\n[player]\nengine = 'beep'\n[unm]\nenable = true\n")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	before, err := file.Stat()
	file.Close()
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := NewConfigFromTomlFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Main.Motion != "off" || cfg.Main.BlockGlyphs != "quadrant" || cfg.Main.FrameRate != 30 {
		t.Fatalf("settings not loaded: %+v", cfg.Main)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, original) {
		t.Fatalf("user configuration was rewritten: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode() != before.Mode() || !os.SameFile(info, before) {
		t.Fatalf("user configuration permissions changed: %v", err)
	}
}

func TestMissingConfigUsesDefaultsWithoutCreatingFiles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg, err := NewConfigFromTomlFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Main.Motion != "full" || cfg.Main.BlockGlyphs != "auto" || cfg.Main.FrameRate != 60 || cfg.Main.Language != "en" {
		t.Fatalf("unexpected defaults: %+v", cfg.Main)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("configuration must not be created: %v", err)
	}
}

func TestConfigRejectsInvalidCurrentSettings(t *testing.T) {
	for _, value := range []string{"motion = 'invalid'", "blockGlyphs = 'invalid'", "frameRate = 0", "frameRate = 241", "frameRate = 'fast'", "["} {
		t.Run(value, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.toml")
			if err := os.WriteFile(path, []byte("[main]\n"+value+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := NewConfigFromTomlFile(path); err == nil {
				t.Fatal("invalid setting accepted")
			}
		})
	}
}
