package deck

import (
	"slices"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestBrailleUsesUnicodeDotOrder(t *testing.T) {
	for mask, want := range map[uint8]string{0: "⠀", 1: "⠁", 2: "⠈", 4: "⠂", 16: "⠄", 64: "⡀", 128: "⢀", 255: "⣿"} {
		if got := glyphFor(Braille, mask); got != want {
			t.Errorf("%08b: %q != %q", mask, got, want)
		}
	}
	for mask := range 256 {
		g := glyphFor(Braille, uint8(mask))
		if ansi.WcWidth.StringWidth(g) != 1 || ansi.GraphemeWidth.StringWidth(g) != 1 {
			t.Fatalf("wide glyph %q", g)
		}
	}
}

func TestWalkmanHasIndependentBodyAndSafeFrames(t *testing.T) {
	for _, v := range []View{{W: 64, H: 22}, {W: 28, H: 9}, {W: 1, H: 1}, {W: 0, H: 0}} {
		var r WalkmanRenderer
		pose := RestPose()
		st := WalkmanState{State: State{Radii: ReelRadii(.4)}}
		f := r.RenderWalkman(v, testPal, pose, st)
		if len(f.Cells) != v.W*v.H {
			t.Fatal("incorrect frame extent")
		}
		for _, c := range f.Cells {
			if c.Ch != "" && ansi.WcWidth.StringWidth(c.Ch) != 1 {
				t.Fatalf("invalid cell %q", c.Ch)
			}
		}
		if !slices.Equal(slices.Clone(f.Cells), r.RenderWalkman(v, testPal, pose, st).Cells) {
			t.Fatal("unstable idle cache")
		}
	}
}

func TestWalkmanWindowReelsMoveButLabelsDoNot(t *testing.T) {
	var r WalkmanRenderer
	v := View{W: 64, H: 24, Tier: Quadrant}
	st := WalkmanState{State: State{Radii: ReelRadii(.4)}}
	a := slices.Clone(r.RenderWalkman(v, testPal, RestPose(), st).Cells)
	st.Angles = [2]float64{1, 2}
	b := r.RenderWalkman(v, testPal, RestPose(), st)
	if slices.Equal(a, b.Cells) {
		t.Fatal("window reels do not move")
	}
	var fresh WalkmanRenderer
	if !slices.Equal(b.Cells, fresh.RenderWalkman(v, testPal, RestPose(), st).Cells) {
		t.Fatal("cached render differs from fresh frame")
	}
}
