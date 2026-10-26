package deck

import "testing"

func TestPixFontGlyphs(t *testing.T) {
	for _, r := range "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789.·-/:" {
		bits := PixGlyph(r)
		var lit uint8
		for _, row := range bits {
			if row > 31 {
				t.Fatalf("%c exceeds 5 columns", r)
			}
			lit |= row
		}
		if lit == 0 {
			t.Fatalf("%c is empty", r)
		}
		for y := range 7 {
			for x := range 5 {
				if got := PixSample(string(r), float64(x)+.5, float64(y)+.5, 1, 1, 1); got != (bits[y]&(1<<(4-x)) != 0) {
					t.Fatalf("%c at %d,%d", r, x, y)
				}
			}
		}
	}
	for _, pt := range [][2]float64{{-1, 0}, {5, 0}, {0, 7}} {
		if PixSample("A", pt[0], pt[1], 1, 1, 1) {
			t.Fatal("sample outside glyph")
		}
	}
	if PixSample("A", 0, 0, 0, 1, 1) || PixGlyph(' ') != [7]uint8{} {
		t.Fatal("invalid pixel size or space")
	}
}
