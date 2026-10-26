package deck

import "testing"

func TestFolderStyleForIDIsDeterministic(t *testing.T) {
	a := FolderStyleForID("playlist-1001")
	b := FolderStyleForID("playlist-1001")
	if a != b || a < 1 || int(a) > len(FolderPalettes) {
		t.Fatalf("unexpected folder style %d vs %d", a, b)
	}
}

func TestFolderRendererDrawsHingedDossierAndCachesFrame(t *testing.T) {
	var r FolderRenderer
	v := View{W: 48, H: 18, Tier: Quadrant, Aspect: 2, Fit: Rect{X: 8, Y: 2, W: 32, H: 14}}
	f := r.RenderFolder(v, Pose{Yaw: -.18, Pitch: .32, Scale: 1}, FolderState{Style: 2, TabSlot: 1, Open: .5})
	filled := 0
	for _, c := range f.Cells {
		if c.Ch != "" {
			filled++
		}
	}
	if filled < 45 {
		t.Fatalf("hinged folder rendered too few cells: %d", filled)
	}
	f.Cells[0].Ch = "Z"
	if cached := r.RenderFolder(v, Pose{Yaw: -.18, Pitch: .32, Scale: 1}, FolderState{Style: 2, TabSlot: 1, Open: .5}); cached.Cells[0].Ch != "Z" {
		t.Fatal("unchanged folder frame was not cached")
	}
}
