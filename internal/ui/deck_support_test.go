package ui

import (
	"testing"

	"github.com/go-musicfox/go-musicfox/internal/configs"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

func TestDeckTierUsesConfiguredGlyphsWithoutLegacyUI(t *testing.T) {
	previous := configs.AppConfig
	t.Cleanup(func() { configs.AppConfig = previous })
	configs.AppConfig = &configs.Config{Main: configs.MainConfig{BlockGlyphs: "half"}}
	if got := deckTier(nil); got != deck.Half {
		t.Fatalf("configured tier = %v", got)
	}
	configs.AppConfig.Main.BlockGlyphs = "auto"
	t.Setenv("TERM", "xterm-kitty")
	t.Setenv("TMUX", "")
	t.Setenv("TERM_PROGRAM", "")
	if got := deckTier(nil); got != deck.Octant {
		t.Fatalf("detected tier = %v", got)
	}
	if deckAspect(nil) != 0 {
		t.Fatal("unknown cell geometry must not be invented")
	}
}
