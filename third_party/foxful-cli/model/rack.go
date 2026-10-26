package model

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/anhoder/foxful-cli/ansix"
	"github.com/anhoder/foxful-cli/style"
)

type rackDecorKey struct {
	colors       [3]color.RGBA
	set          [3]bool
	pattern, tag string
	playing      bool
}

func (a *App) cassetteRack() bool {
	return a.options != nil && a.options.ListStyle == ListCassetteRack
}

func rackKey(d RackRowDecor) rackDecorKey {
	k := rackDecorKey{pattern: d.Pattern, tag: d.Tag, playing: d.Playing}
	for i, c := range [3]color.Color{d.Shell, d.Label, d.Ink} {
		if !isNoColor(c) {
			k.colors[i] = color.RGBAModel.Convert(c).(color.RGBA)
			k.set[i] = true
		}
	}
	return k
}

func (m *Main) rackRow(index int, cols trackColumns) string {
	item := m.menuList[index]
	var decor RackRowDecor
	if f := m.options.RackDecorator; f != nil {
		decor = f(m, index, item)
	}
	selected, hovered := m.isSelected(index), index == m.hoveredMenuItemIdx
	pull := 0
	if selected || hovered {
		pull = 1
	}
	w := m.ContentWidth()
	key := trackRowKey{item: item, index: index, selected: selected, hovered: hovered, cols: cols, width: w, styleGen: style.StyleGeneration(), decor: rackKey(decor), pullOffset: pull}
	if entry, ok := m.trackCache[index]; ok && entry.key == key {
		return entry.row
	}
	ss := style.CurrentStyleSet()
	appBg := ss.AppBackground.GetBackground()
	if isNoColor(appBg) {
		appBg = nil
	}
	shell, ink := decor.Shell, decor.Ink
	if isNoColor(shell) {
		shell = ss.Subtitle.GetForeground()
	}
	if isNoColor(ink) {
		ink = ss.MenuItem.GetForeground()
	}
	if selected || decor.Playing {
		shell = ansix.Mix(shell, ink, .25)
	}
	bg, fg := ansix.Mix(appBg, decor.Label, .08), ss.MenuItem.GetForeground()
	if selected {
		bg, fg = ss.SelectedItem.GetBackground(), ss.SelectedItem.GetForeground()
	} else if hovered {
		fg = ss.MenuItemHover.GetForeground()
	}
	paint := func(text string, fg, bg color.Color) string {
		st := lipgloss.NewStyle()
		if !isNoColor(fg) {
			st = st.Foreground(fg)
		}
		if !isNoColor(bg) {
			st = st.Background(bg)
		}
		return st.Render(text)
	}
	pattern := ansix.Normalize(decor.Pattern)
	if ansix.Width(pattern) != 1 || strings.ContainsAny(pattern, "\r\n\t") {
		pattern = "▪"
	}
	tag := strings.NewReplacer("\r", " ", "\n", " ", "\t", " ").Replace(decor.Tag)
	artistW := cols.artistW
	if w < 52 {
		tag, artistW = "", 0
	}
	compact := w < 36
	sep := "│"
	if compact {
		sep = " "
	}
	idxW, metaW := max(cols.idxW, 2), min(cols.metaW, max(w/4, 0))
	start := max(cols.start+1, 0)
	available := max(cols.end-start-pull, 0)
	fixed := 3 + 1 + idxW + 1 + 1 // shell, separators, index, end cap
	if metaW > 0 {
		fixed += 1 + metaW
	}
	if tag != "" {
		fixed += 4
	}
	if artistW > 0 {
		artistW = min(artistW, max(available-fixed-12, 0))
	}
	titleW := available - fixed
	if artistW > 0 {
		titleW -= 2 + artistW
	}
	var b strings.Builder
	b.WriteString(paintBg(strings.Repeat(" ", start), appBg))
	if selected || hovered {
		b.WriteString(paint("▐", ss.Muted.GetForeground(), appBg))
	}
	b.WriteString(paint("▐", shell, appBg))
	b.WriteString(paint(pattern, ink, shell))
	b.WriteString(paint("▌", shell, appBg))
	b.WriteString(paint(sep+fmt.Sprintf("%0*d", idxW, index+1)+sep, ss.Subtitle.GetForeground(), bg))
	b.WriteString(paint(trackPad(item.Title, max(titleW, 0), false), fg, bg))
	if artistW > 0 {
		b.WriteString(paint("  "+trackPad(item.Subtitle, artistW, false), ss.Subtitle.GetForeground(), bg))
	}
	if tag != "" {
		b.WriteString(paint("│"+trackPad(tag, 3, false), ink, bg))
	}
	if metaW > 0 {
		b.WriteString(paint(sep+trackPad(item.Meta, metaW, true), ss.Subtitle.GetForeground(), bg))
	}
	b.WriteString(paint("▌", shell, appBg))
	if w >= 36 && cols.end < w {
		if !selected && !hovered {
			b.WriteString(paintBg(" ", appBg))
		}
		rail := "│"
		if selected || hovered {
			rail = "┃"
		}
		b.WriteString(paint(rail, ss.Muted.GetForeground(), appBg))
	}
	row := ansix.Fit(b.String(), w, backgroundSGR(appBg))
	if m.trackCache == nil {
		m.trackCache = map[int]trackRowEntry{}
	}
	m.trackCache[index] = trackRowEntry{key: key, row: row}
	return row
}
