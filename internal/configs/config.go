package configs

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	Main MainConfig `toml:"main"`
}

type MainConfig struct {
	Motion      string `toml:"motion"`
	BlockGlyphs string `toml:"blockGlyphs"`
	FrameRate   int    `toml:"frameRate"`
	Language    string `toml:"language"`
}

var AppConfig = defaultConfig()

func defaultConfig() *Config {
	return &Config{Main: MainConfig{Motion: "full", BlockGlyphs: "auto", FrameRate: 60, Language: "en"}}
}

// Read only the current settings; unrelated user preferences remain untouched.
func NewConfigFromTomlFile(path string) (*Config, error) {
	cfg := defaultConfig()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse configuration: %w", err)
	}
	switch cfg.Main.Motion {
	case "full", "subtle", "off":
	default:
		return nil, fmt.Errorf("main.motion must be full, subtle or off")
	}
	switch cfg.Main.BlockGlyphs {
	case "auto", "half", "quadrant", "sextant", "octant", "braille":
	default:
		return nil, fmt.Errorf("invalid main.blockGlyphs")
	}
	switch cfg.Main.Language {
	case "", "en", "zh":
		if cfg.Main.Language == "" {
			cfg.Main.Language = "en"
		}
	default:
		return nil, fmt.Errorf("main.language must be en or zh")
	}
	if cfg.Main.FrameRate < 1 || cfg.Main.FrameRate > 240 {
		return nil, fmt.Errorf("main.frameRate must be between 1 and 240")
	}
	return cfg, nil
}
