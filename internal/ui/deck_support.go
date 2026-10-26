package ui

import (
	"os"

	"github.com/anhoder/foxful-cli/model"
	"github.com/go-musicfox/go-musicfox/internal/configs"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

func deckPaletteOf(p npPalette) deck.Palette {
	return deck.Palette{
		Bg: p.Bg, Surface: p.Surface, Text: p.Text, Hero: p.Hero,
		Secondary: p.Secondary, Rule: p.Rule, Accent: p.Accent,
	}
}

func deckTier(a *model.App) deck.Tier {
	if t, ok := deck.ParseTier(configs.AppConfig.Main.BlockGlyphs); ok {
		return t
	}
	version := ""
	if a != nil {
		version = a.TerminalVersion()
	}
	return deck.DetectTier(deck.Terminal{Getenv: os.Getenv, Version: version})
}

// Multiplexers may report made-up cell sizes; accept plausible shapes only.
func deckAspect(a *model.App) float64 {
	if a != nil {
		if ratio := a.CellAspect(); ratio >= 1.4 && ratio <= 3.2 {
			return ratio
		}
	}
	return 0
}
