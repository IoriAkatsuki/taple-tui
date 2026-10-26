package deck

import (
	"crypto/sha256"
	"fmt"
	"image/color"
	"math"
	"testing"
)

// Hash exact rendered pixels, not equivalent glyph/color polarity or Unicode quoting.
func framebufferHash(t *testing.T, tier Tier, cells []Cell) string {
	t.Helper()
	hash := sha256.New()
	cols, rows := tier.Grid()
	fmt.Fprintf(hash, "%d,%d\n", cols, rows)
	for _, c := range cells {
		if c.Ch == "" {
			fmt.Fprintf(hash, "0|%016x\n", math.Float64bits(c.Shadow))
			continue
		}
		fmt.Fprintf(hash, "1|%016x", math.Float64bits(c.Shadow))
		mask := glyphMask(t, tier, c.Ch)
		for bit := range cols * rows {
			pixel := c.Bg
			if mask&(1<<bit) != 0 {
				pixel = c.Fg
			}
			fmt.Fprintf(hash, "|%02x%02x%02x%02x", pixel.R, pixel.G, pixel.B, pixel.A)
		}
		fmt.Fprintln(hash)
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func TestFramebufferHashPreservesExactPixelBytes(t *testing.T) {
	cells := []Cell{{}, {Ch: "\U0001cd00", Fg: color.RGBA{1, 2, 3, 4}, Bg: color.RGBA{5, 6, 7, 8}, Shadow: .5}}
	// The octant paints bit 2 of an eight-pixel cell; every RGBA byte is retained.
	const want = "08efa769057a80f81fe7dc717b4a570a4b98095d0b79c02ddca979e456aa1911"
	if got := framebufferHash(t, Octant, cells); got != want {
		t.Fatalf("stable framebuffer encoding = %s, want %s", got, want)
	}
	cells[1].Shadow = math.Nextafter(.5, 1)
	if framebufferHash(t, Octant, cells) == want {
		t.Fatal("framebuffer hash discarded a shadow bit")
	}
}

func TestFramebufferHashNormalizesEquivalentSubpixels(t *testing.T) {
	a := Cell{Ch: "▀", Fg: color.RGBA{1, 2, 3, 255}, Bg: color.RGBA{4, 5, 6, 255}, Shadow: .5}
	b := Cell{Ch: "▄", Fg: a.Bg, Bg: a.Fg, Shadow: a.Shadow}
	want := framebufferHash(t, Quadrant, []Cell{a})
	if framebufferHash(t, Quadrant, []Cell{b}) != want {
		t.Fatal("complementary glyph with swapped colors changed rendered pixels")
	}
	for _, name := range []string{"pixel", "alpha", "coverage", "shadow"} {
		t.Run(name, func(t *testing.T) {
			changed := a
			switch name {
			case "pixel":
				changed.Ch = "▘"
			case "alpha":
				changed.Fg.A--
			case "coverage":
				changed.Ch = ""
			case "shadow":
				changed.Shadow = math.Nextafter(changed.Shadow, 1)
			}
			if framebufferHash(t, Quadrant, []Cell{changed}) == want {
				t.Fatal("rendered framebuffer difference was discarded")
			}
		})
	}
	if framebufferHash(t, Quadrant, []Cell{{}}) == framebufferHash(t, Quadrant, []Cell{{Ch: " "}}) {
		t.Fatal("empty cell and drawn blank have different coverage")
	}
}
