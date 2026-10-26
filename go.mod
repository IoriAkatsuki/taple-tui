module github.com/go-musicfox/go-musicfox

go 1.26

require (
	charm.land/bubbles/v2 v2.1.0
	charm.land/bubbletea/v2 v2.0.8
	github.com/anhoder/foxful-cli v1.1.4
	github.com/charmbracelet/ultraviolet v0.0.0-20260703014108-f5a850f9c2b7
	github.com/charmbracelet/x/ansi v0.11.7
	github.com/gookit/gcli/v2 v2.3.4
	github.com/pelletier/go-toml/v2 v2.4.4-0.20260718201843-686c980c4758
	golang.org/x/sys v0.46.0
)

require (
	charm.land/glamour/v2 v2.0.1 // indirect
	charm.land/lipgloss/v2 v2.0.4 // indirect
	github.com/alecthomas/chroma/v2 v2.14.0 // indirect
	github.com/atotto/clipboard v0.1.4 // indirect
	github.com/aymerick/douceur v0.2.0 // indirect
	github.com/charmbracelet/colorprofile v0.4.3 // indirect
	github.com/charmbracelet/x/exp/slice v0.0.0-20250327172914-2fdc97757edf // indirect
	github.com/charmbracelet/x/term v0.2.2 // indirect
	github.com/charmbracelet/x/termios v0.1.1 // indirect
	github.com/charmbracelet/x/windows v0.2.2 // indirect
	github.com/clipperhouse/displaywidth v0.11.0 // indirect
	github.com/clipperhouse/uax29/v2 v2.7.0 // indirect
	github.com/dlclark/regexp2 v1.11.0 // indirect
	github.com/ebitengine/purego v0.10.1 // indirect
	github.com/fogleman/ease v0.0.0-20170301025033-8da417bf1776 // indirect
	github.com/gookit/color v1.5.3 // indirect
	github.com/gookit/goutil v0.6.10 // indirect
	github.com/gorilla/css v1.0.1 // indirect
	github.com/jezek/xgb v1.3.1 // indirect
	github.com/lucasb-eyer/go-colorful v1.4.0 // indirect
	github.com/mattn/go-runewidth v0.0.23 // indirect
	github.com/microcosm-cc/bluemonday v1.0.27 // indirect
	github.com/muesli/cancelreader v0.2.2 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/robotn/gohook v1.0.0-beta1 // indirect
	github.com/sahilm/fuzzy v0.1.1 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/vcaesar/go-wayland v0.40.0 // indirect
	github.com/vcaesar/keycode v0.20.0 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/yuin/goldmark v1.7.8 // indirect
	github.com/yuin/goldmark-emoji v1.0.5 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/term v0.43.0 // indirect
	golang.org/x/text v0.37.0 // indirect
)

replace (
	charm.land/bubbletea/v2 v2.0.8 => github.com/go-musicfox/bubbletea/v2 v2.0.8-foxful-1.2
	// The framework is maintained in-tree; see third_party/foxful-cli/UPSTREAM.md.
	github.com/anhoder/foxful-cli => ./third_party/foxful-cli
	github.com/gookit/gcli/v2 v2.3.4 => github.com/anhoder/gcli/v2 v2.3.5
)
