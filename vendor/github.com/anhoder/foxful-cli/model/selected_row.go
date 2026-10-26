package model

import "image"

// SelectedRowRect is the on-screen list row, without importing host geometry.
func (m *Main) SelectedRowRect() image.Rectangle {
	index := m.selectedIndex - m.getPageStartIndex()
	if index < 0 || index >= m.menuPageSize || m.selectedIndex >= len(m.menuList) {
		return image.Rectangle{}
	}
	x0, x1, ok := m.menuItemTextBounds(m.selectedIndex)
	if m.useTracklist() {
		x0, x1, ok = m.tracklistBounds(m.selectedIndex)
	}
	if !ok {
		return image.Rectangle{}
	}
	y := m.menuListStartRow + index
	return image.Rect(x0, y, x1, y+1)
}
