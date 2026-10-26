package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/motion"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

func TestAppleMusicLibraryContainersUseSongRack(t *testing.T) {
	for _, tc := range []struct{ kind, resource, next string }{
		{applemusic.LibraryPlaylists, "library-playlists", applemusic.LibraryPlaylistTracks},
		{applemusic.Recommendations, "personal-recommendation", applemusic.RecommendationContents},
		{applemusic.PersonalStation, "stations", ""},
		{applemusic.RecommendationContents, "playlists", applemusic.CatalogPlaylistTracks},
		{applemusic.RecommendationContents, "albums", applemusic.CatalogAlbumTracks},
		{applemusic.RecommendationContents, "stations", ""},
	} {
		for _, size := range [][2]int{{255, 68}, {140, 45}, {80, 30}, {120, 24}, {50, 20}} {
			for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
				t.Run(fmt.Sprintf("%s/%s/%dx%d/%s", tc.kind, tc.resource, size[0], size[1], level), func(t *testing.T) {
					c := &appleLibraryClient{}
					for i := range 3 {
						c.page.Items = append(c.page.Items, applemusic.CatalogItem{ID: fmt.Sprintf("resource.%d", i), Type: tc.resource, Name: "夜航", Playable: true})
					}
					p := NewAppleMusicPage(c)
					defer p.Close()
					p.drawer, p.level = appleLibrary, level
					p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us"}
					p.acceptCatalog(p.requestCatalog(applemusic.CatalogRequest{Kind: tc.kind}, "测试资料库", false)().(appleCatalogMsg))
					now := time.Now()
					p.draw(size[0], size[1], now, 2)
					var first appleHit
					for _, hit := range p.hits {
						if hit.kind == appleHitCatalog && hit.index == 0 {
							first = hit
							break
						}
					}
					if first.slot.Dy() != 2 {
						t.Fatal("library container did not use the shared two-row rack slot")
					}
					if p.rackPickup.current.index >= 0 {
						t.Fatal("unfocused library popped a cassette")
					}
					p.Update(tea.MouseMotionMsg{X: first.slot.Min.X + 2, Y: first.slot.Min.Y}, model.NewApp(model.DefaultOptions()))
					p.draw(size[0], size[1], now.Add(10*time.Millisecond), 2)
					out := p.draw(size[0], size[1], now.Add(400*time.Millisecond), 2)
					if size[1] >= 30 && (!p.rackGraphical || p.rackPickup.current.id != "resource.0" || p.rackPickup.pop.value(now.Add(400*time.Millisecond)) != .5) {
						t.Fatal("library hover did not half-eject the same physical cassette as songs")
					}
					if len(c.actions) != 0 || len(p.styles) != 0 || len(p.library.tracks) != 0 {
						t.Fatal("container preview changed playback or masqueraded as a song")
					}
					for _, line := range strings.Split(out, "\n") {
						if ansi.WcWidth.StringWidth(line) != size[0] {
							t.Fatal("rack overflow")
						}
					}
					foundTitle := false
					for x := first.slot.Min.X; x < first.slot.Max.X; x++ {
						cell := p.screen.buf.CellAt(x, first.slot.Min.Y)
						if cell.Content == "夜" {
							foundTitle = npSameColor(cell.Style.Fg, appleNeon.Text) && cell.Style.Attrs&uv.AttrBold != 0
						}
					}
					if !foundTitle {
						t.Fatal("popped library title was hidden or lost its light selected style")
					}
					activation := first
					if p.rackGraphical {
						found := false
						for _, hit := range p.hits {
							if hit.kind == appleHitTrack || hit.kind == appleHitCassette {
								t.Fatal("container got a song playback hit target")
							}
							if hit.kind == appleHitCatalog && hit.index == 0 && hit.y < first.slot.Min.Y {
								activation, found = hit, true
								break
							}
						}
						if !found {
							t.Fatal("physical library cassette has no matching catalog hit target")
						}
					}
					p.library.catalog.items[0].ID = "replaced"
					if p.handleClick(activation.x, activation.y) != nil {
						t.Fatal("stale container hit activated a replacement")
					}
					p.library.catalog.items[0].ID = "resource.0"
					cmd := p.handleClick(activation.x, activation.y)
					if cmd == nil {
						t.Fatal("catalog activation missing")
					}
					msg := cmd()
					if tc.resource == "stations" {
						if len(c.actions) != 1 || c.actions[0].Kind != "play-station" || c.actions[0].ID != "resource.0" {
							t.Fatal("station did not retain typed playback")
						}
					} else {
						m, ok := msg.(appleCatalogMsg)
						if !ok || m.request.Kind != tc.next || m.request.ID != "resource.0" || len(c.actions) != 0 {
							t.Fatal("container did not browse its own resource")
						}
					}
				})
			}
		}
	}
}
