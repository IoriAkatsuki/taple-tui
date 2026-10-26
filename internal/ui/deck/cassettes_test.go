package deck

import (
	"crypto/sha256"
	"fmt"
	"html"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCassetteClassicUnchanged(t *testing.T) {
	for _, tt := range []struct {
		name string
		view View
		pose Pose
		want string
	}{
		{"panel", View{W: 40, H: 14, Tier: Quadrant}, RestPose(), "6fca4db52f2decd3ade8799508307f15b497c80f50893a0d3bdbfd7d0d2b4a26"},
		{"page", View{W: 96, H: 32, Tier: Sextant}, RestPose(), "7706de87c89f1e2d5485bb33d78764662bd714ae1c636ddab6d0348c611eccee"},
		{"back", View{W: 64, H: 22, Tier: Octant}, Pose{Yaw: 2.8, Pitch: -.2, Scale: 1}, "0affcf293721d387fd1025770dcf076d048ebfdfadb6fe91fa5a8b11d0a585be"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var r Renderer
			frame := r.Render(tt.view, testPal, tt.pose, State{Radii: ReelRadii(.4), Angles: [2]float64{.7, 1.2}, Lift: 2})
			hash := sha256.New()
			for _, c := range frame.Cells {
				fmt.Fprintf(hash, "%#v\n", c)
			}
			if got := fmt.Sprintf("%x", hash.Sum(nil)); got != tt.want {
				t.Fatalf("classic framebuffer hash = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCassetteStylesRenderDistinct(t *testing.T) {
	v := View{W: 96, H: 36, Tier: Quadrant}
	var frames [5][]Cell
	for i, style := range CassetteStyles {
		var r Renderer
		frame := r.Render(v, cassetteTestPalette, RestPose(), State{Style: uint8(i + 1), TrackNum: 42, Radii: ReelRadii(.4), Lift: 2})
		frames[i] = slices.Clone(frame.Cells)
		for j := 0; j < i; j++ {
			different := 0
			for k, c := range frame.Cells {
				if c != frames[j][k] {
					different++
				}
			}
			if different < len(frame.Cells)/5 {
				t.Errorf("%s and %s differ in only %d/%d cells", style.Name, CassetteStyles[j].Name, different, len(frame.Cells))
			}
		}
		if dir := os.Getenv("TAPEFOX_PREVIEW_DIR"); dir != "" {
			writeCassettePreview(t, filepath.Join(dir, fmt.Sprintf("cassette-%d-%s.html", i+1, strings.ToLower(style.Name))), style.Name, frame)
		}
	}
}

func TestCassettePatternsAtSpecifiedCoordinates(t *testing.T) {
	for _, tt := range []struct {
		style uint8
		u, v  float64
		want  color.RGBA
	}{
		{1, 18, 14, color.RGBA{255, 43, 214, 255}},
		{1, 40, 5, color.RGBA{18, 10, 26, 255}},
		{2, 30, 12, color.RGBA{0, 229, 255, 255}},
		{2, 24.2, 6, color.RGBA{14, 42, 51, 255}},
		{2, 26, 6, color.RGBA{154, 163, 184, 255}},
		{3, 40, 12, color.RGBA{58, 36, 8, 255}},
		{3, 57.5, 6.5, color.RGBA{255, 176, 0, 255}},
		{4, 40, 11.4, color.RGBA{255, 59, 92, 255}},
		{4, 40, 12.3, color.RGBA{255, 210, 63, 255}},
		{4, 40, 13.2, color.RGBA{0, 229, 255, 255}},
		{5, 40, 6, color.RGBA{255, 43, 214, 255}},
		{5, 40, 10, color.RGBA{255, 107, 0, 255}},
		{5, 40, 14, color.RGBA{255, 176, 0, 255}},
		{5, 40, 9.5, color.RGBA{244, 239, 230, 255}},
	} {
		m := cassetteMaterials(cassetteTestPalette, tt.style, 42)
		if got := labelAt(&m, tt.u, tt.v, 0, 2).albedo.rgba(); got != tt.want {
			t.Errorf("style %d at (%g,%g) = %v, want %v", tt.style, tt.u, tt.v, got, tt.want)
		}
		if got := labelAt(&m, tt.u, tt.v, 0, .2).albedo.rgba(); got != CassetteStyles[tt.style-1].Label {
			t.Errorf("style %d retains unresolved print at low LOD", tt.style)
		}
	}
	xray := cassetteMaterials(cassetteTestPalette, 4, 42)
	if got := face(&xray, 8, 8, 0, 2).m; got != matShell {
		t.Fatalf("X-RAY side is covered by a wide label: %v", got)
	}
	if got := face(&xray, 40, 8, 0, 2).m; got != matLabel {
		t.Fatalf("X-RAY narrow label is missing: %v", got)
	}
}

func TestCassetteXRayShellRevealsRotatingFlange(t *testing.T) {
	m := cassetteMaterials(cassetteTestPalette, 4, 42)
	sc := scene{m: &m, rot: rotation(0, 0, 0), inv: 1, o: vec3{0, 0, -camDist}, radii: ReelRadii(.5), lObj: keyLight, detail: 2}
	// At the left flange's outer edge, outside the window, label and head.
	u, v := 22-14.5/math.Sqrt2, 24+14.5/math.Sqrt2
	if face(&m, u, v, 0, 2).m != matShell {
		t.Fatal("fixture must hit the transparent shell, not the window")
	}
	d := vec3{u - faceW/2, v - faceH/2, camDist - depth/2}
	a, _, _ := sc.sample(d)
	sc.angles[0] = 3 * math.Pi / 4
	b, mt, _ := sc.sample(d)
	if a.dist2(b) < 25 || !mt.turns() {
		t.Fatalf("transparent shell did not reveal a rotating flange: %v -> %v, material %v", a, b, mt)
	}
	m.translucency = 0
	c, _, _ := sc.sample(d)
	sc.angles[0] = 0
	dull, _, _ := sc.sample(d)
	if c != dull {
		t.Fatal("opaque shell follows internal angles")
	}
}

func TestCassetteNixieTrackNumberAndThemeIndependence(t *testing.T) {
	var r Renderer
	v := View{W: 96, H: 36, Tier: Quadrant}
	st := State{Style: 3, TrackNum: 42, Radii: ReelRadii(.4)}
	a := slices.Clone(r.Render(v, cassetteTestPalette, RestPose(), st).Cells)
	st.TrackNum = 43
	if slices.Equal(a, r.Render(v, cassetteTestPalette, RestPose(), st).Cells) {
		t.Fatal("changing TrackNum did not update the NIXIE label")
	}
	st.TrackNum = 142
	if !slices.Equal(a, r.Render(v, cassetteTestPalette, RestPose(), st).Cells) {
		t.Fatal("NIXIE label did not use TrackNum modulo 100")
	}
	for style := uint8(1); style <= 5; style++ {
		light, dark := cassetteMaterials(testPal, style, 42), cassetteMaterials(cassetteTestPalette, style, 42)
		// Only the backdrop may follow the application theme.
		light.bg, dark.bg = rgb{}, rgb{}
		if light != dark {
			t.Errorf("style %d changed its physical materials with the theme", style)
		}
	}
}

func TestCassetteStyleCacheMatchesFreshRender(t *testing.T) {
	for _, tier := range []Tier{Quadrant, Sextant, Octant} {
		for _, pose := range []Pose{RestPose(), {Yaw: 2.8, Pitch: -.25, Scale: 1}} {
			v := View{W: 64, H: 24, Tier: tier}
			var cached Renderer
			for _, style := range []uint8{0, 1, 2, 3, 4, 5, 0, 255} {
				for i := range 4 {
					st := State{Style: style, TrackNum: 42 + i/2, Radii: ReelRadii(.5), Angles: [2]float64{float64(i), float64(i) / 2}, Blur: .3 * float64(i)}
					got := cached.Render(v, cassetteTestPalette, pose, st)
					var fresh Renderer
					if want := fresh.Render(v, cassetteTestPalette, pose, st); !slices.Equal(got.Cells, want.Cells) {
						t.Fatalf("style=%d track=%d tier=%v pose=%+v cached render differs", style, st.TrackNum, tier, pose)
					}
				}
			}
		}
	}
}

var cassetteTestPalette = Palette{
	Bg: color.RGBA{10, 6, 18, 255}, Surface: color.RGBA{20, 11, 34, 255}, Text: color.RGBA{217, 210, 236, 255},
	Hero: color.RGBA{255, 43, 214, 255}, Secondary: color.RGBA{142, 131, 176, 255}, Rule: color.RGBA{75, 53, 112, 255}, Accent: color.RGBA{0, 229, 255, 255},
}

func writeCassettePreview(t *testing.T, path, name string, f *Frame) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	fmt.Fprintf(&out, `<!doctype html><meta charset="utf-8"><title>%s cassette</title><style>body{margin:20px;background:#0A0612;color:#D9D2EC;font-family:monospace}#terminal{position:relative;font:16px/20px 'DejaVu Sans Mono',monospace;font-variant-ligatures:none;width:%dpx;height:%dpx}span{position:absolute;white-space:pre;height:20px}</style><h1>%s</h1><div id="terminal">`, html.EscapeString(name), f.W*10, f.H*20, html.EscapeString(name))
	for i, c := range f.Cells {
		if c.Ch == "" {
			continue
		}
		fmt.Fprintf(&out, `<span style="left:%dpx;top:%dpx;width:10px;color:#%02x%02x%02x;background:#%02x%02x%02x">%s</span>`, (i%f.W)*10, (i/f.W)*20, c.Fg.R, c.Fg.G, c.Fg.B, c.Bg.R, c.Bg.G, c.Bg.B, html.EscapeString(c.Ch))
	}
	out.WriteString("</div>")
	if err := os.WriteFile(path, []byte(out.String()), 0600); err != nil {
		t.Fatal(err)
	}
	t.Log(path)
}
