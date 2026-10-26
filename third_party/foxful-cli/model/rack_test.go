package model

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/ansix"
	"github.com/anhoder/foxful-cli/style"
	"github.com/charmbracelet/x/ansi"
)

func newRackMain(t *testing.T, width int) (*App, *Main) {
	t.Helper()
	a, m := newTracklistMain(t, width, 30)
	m.options.ListStyle = ListCassetteRack
	m.options.RackDecorator = func(*Main, int, MenuItem) RackRowDecor {
		return RackRowDecor{Shell: color.RGBA{26, 16, 32, 255}, Label: color.RGBA{240, 236, 228, 255}, Ink: color.RGBA{255, 43, 214, 255}, Pattern: "╱", Tag: "C60"}
	}
	m.Update(tea.WindowSizeMsg{Width: width, Height: 30}, a)
	return a, m
}

func TestCassetteRackRowsAndPullOut(t *testing.T) {
	for _, width := range []int{1, 12, 24, 35, 36, 51, 52, 80, 120, 200} {
		_, m := newRackMain(t, width)
		m.menuList[0] = MenuItem{Title: "Signal", Subtitle: "Artist", Meta: "3:21"}
		cols := m.trackColumns(m.getCurPageMenus())
		m.selectedIndex = 1
		plain := m.trackRow(0, cols)
		m.selectedIndex = 0
		selected := m.trackRow(0, cols)
		for _, row := range []string{plain, selected} {
			if ansix.Width(row) != m.ContentWidth() || ansi.GraphemeWidth.StringWidth(row) != m.ContentWidth() || strings.Contains(row, "\n") {
				t.Fatalf("width %d: bad rack row %q", width, row)
			}
		}
		if width >= 36 {
			p, s := ansi.Strip(plain), ansi.Strip(selected)
			if !strings.Contains(p, "│01│") || column([]string{s}, "╱") != column([]string{p}, "╱")+1 {
				t.Fatalf("width %d: index/pull-out missing: %q -> %q", width, p, s)
			}
			if strings.Contains(p, "C60") != (width >= 52) || strings.Contains(p, "Artist") != (width >= 52) {
				t.Fatalf("width %d: wrong responsive columns: %q", width, p)
			}
		}
		if rect := m.SelectedRowRect(); rect.Min.X < 0 || rect.Max.X > m.ContentWidth() {
			t.Fatalf("row bounds escaped content: %v", rect)
		}
	}
}

type rackSliceColor []uint8

func (c rackSliceColor) RGBA() (uint32, uint32, uint32, uint32) {
	return uint32(c[0]) * 257, uint32(c[1]) * 257, uint32(c[2]) * 257, 65535
}

func TestCassetteRackCacheAndTransparentBackground(t *testing.T) {
	_, m := newRackMain(t, 80)
	ink := rackSliceColor{255, 43, 214}
	m.options.RackDecorator = func(*Main, int, MenuItem) RackRowDecor {
		return RackRowDecor{Shell: color.RGBA{10, 20, 30, 255}, Ink: ink, Pattern: "界", Tag: "MIX"}
	}
	cols := m.trackColumns(m.getCurPageMenus())
	first := m.trackRow(0, cols)
	ink[0] = 100
	if second := m.trackRow(0, cols); first == second {
		t.Fatal("decorator color changes did not invalidate the cached row")
	}
	m.selectedIndex = 1
	theme := warmNightTestTheme()
	theme.Background = nil
	theme.AppBackground.Bg = nil
	style.SetStyleSet(style.NewStyleSet(theme))
	if row := m.trackRow(0, cols); strings.Contains(row, "48;2;0;0;0") {
		t.Fatal("transparent rack invented a black background")
	}
}
