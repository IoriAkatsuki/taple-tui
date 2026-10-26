package model

import "testing"

func TestSelectedRowRectFollowsDrawnTrackRow(t *testing.T) {
	a, m := newTracklistMain(t, 120, 30)
	m.selectedIndex = 2
	_ = m.View(a)
	r := m.SelectedRowRect()
	x0, x1, _ := m.tracklistBounds(2)
	if r.Min.X != x0 || r.Max.X != x1 || r.Min.Y != m.MenuStartRow()+2 || r.Dy() != 1 {
		t.Fatalf("rect=%v", r)
	}
	m.selectedIndex = -1
	if !m.SelectedRowRect().Empty() {
		t.Fatal("invalid selection has geometry")
	}
}
