package ui

// White-box tests exercise private navigation state and rendered cells/hits without
// exporting production state or executing browser, renderer or polling commands.
import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

const navigationEdgeDisclosure = "SYNTHETIC FRAMEBUFFER TEST / NOT LIVE PLAYBACK"

func TestAppleMusicPanelBlocksBackgroundInput(t *testing.T) {
	lang := currentLang()
	t.Cleanup(func() { SetLanguage(lang) })
	SetLanguage("en")
	for _, tc := range []struct {
		name  string
		input tea.Msg
		mouse bool
	}{
		{"paste/keyboard-close", tea.PasteMsg{Content: "hidden"}, false},
		{"paste/mouse-close", tea.PasteMsg{Content: "hidden"}, true},
		{"text/keyboard-close", tea.KeyPressMsg{Code: 'x', Text: "x"}, false},
		{"text/mouse-close", tea.KeyPressMsg{Code: 'x', Text: "x"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := appleNavigationPage(t)
			p.input.SetValue("")
			p.Update(tea.KeyPressMsg{Code: '/'}, nil)
			p.Update(tea.PasteMsg{Content: "query"}, nil)
			assertNavigationEdgeEditor(t, p, "query")
			p.draw(100, 30, time.Now(), 2)
			clickNavigationEdgeKey(t, p, "s")
			if p.panel == "" {
				t.Fatal("rendered settings entry did not open the panel")
			}
			p.Update(tc.input, nil)
			assertNavigationEdgeEditor(t, p, "query")
			state := p.state
			state.Volume = .37
			p.Update(appleStateMsg{state: state, generation: p.playGeneration, pollEpoch: p.pollEpoch}, nil)
			if p.state.Volume != state.Volume || p.panel == "" {
				t.Error("panel blocked background state updates or closed unexpectedly")
			}
			closeNavigationEdgePanel(t, p, tc.mouse)
			assertNavigationEdgeEditor(t, p, "query")
			p.Update(tea.PasteMsg{Content: "visible"}, nil)
			assertNavigationEdgeEditor(t, p, "queryvisible")
		})
	}
}

func assertNavigationEdgeEditor(t *testing.T, p *AppleMusicPage, want string) {
	t.Helper()
	if p.input.Value() != want || !p.editing || !p.input.Focused() {
		t.Errorf("want query=%q with editing/focus retained; got query=%q editing=%t focus=%t", want, p.input.Value(), p.editing, p.input.Focused())
	}
}

func clickNavigationEdgeKey(t *testing.T, p *AppleMusicPage, key string) {
	t.Helper()
	h := appleNavigationHit(t, p, key)
	p.Update(tea.MouseClickMsg{X: h.x, Y: h.y, Button: tea.MouseLeft}, nil)
}

func closeNavigationEdgePanel(t *testing.T, p *AppleMusicPage, mouse bool) {
	t.Helper()
	p.draw(100, 30, time.Now(), 2)
	if mouse {
		clickNavigationEdgeKey(t, p, "esc")
	} else {
		p.Update(tea.KeyPressMsg{Code: tea.KeyEscape}, nil)
	}
	if p.panel != "" {
		t.Fatal("rendered/keyboard Esc did not close the panel")
	}
}

func TestAppleMusicLongParentNavigation(t *testing.T) {
	lang := currentLang()
	t.Cleanup(func() { SetLanguage(lang) })
	for _, tc := range []struct{ lang, kind, query string }{
		{"en", "ascii", strings.Repeat("A", 70)},
		{"en", "cjk", strings.Repeat("中文", 35)},
		{"zh", "ascii", strings.Repeat("A", 70)},
		{"zh", "cjk", strings.Repeat("中文", 35)},
	} {
		for _, width := range []int{80, 99, 100, 60, 59} {
			name := fmt.Sprintf("long-parent-%s-%s-%d", tc.lang, tc.kind, width)
			t.Run(name, func(t *testing.T) {
				SetLanguage(tc.lang)
				p := navigationLongParentPage(t, tc.query)
				frame := p.draw(width, 30, time.Unix(100, 0), 2)
				assertPresentationFrame(t, p, frame)
				assertLongParentNavigation(t, p)
				exportPresentation(t, p, name)
				checkLongParentSearchEntry(t, p)
				checkLongParentReturns(t, p)
			})
		}
	}
}

func navigationLongParentPage(t *testing.T, query string) *AppleMusicPage {
	t.Helper()
	p := appleNavigationPage(t)
	client := &appleCatalogClient{page: applemusic.CatalogPage{Items: []applemusic.CatalogItem{
		{Type: "albums", ID: "edge-album", Name: "Child Album"},
	}}}
	p.client = client
	p.input.SetValue(query)
	p.acceptCatalog(catalogStateReply(t, p.searchCatalog(applemusic.CatalogAlbums)))
	if want := T("type.album") + " · " + query; p.catalog.title != want {
		t.Fatalf("want real search title %q, got %q", want, p.catalog.title)
	}
	client.page.Items = []applemusic.CatalogItem{{Type: "songs", ID: "edge-song", Name: "Synthetic song"}}
	p.acceptCatalog(catalogStateReply(t, p.openCatalog(0, "edge-album")))
	if len(p.catalogHistory) != 1 || p.catalog.request.Kind != applemusic.CatalogAlbumTracks {
		t.Fatal("album drilldown did not retain its real parent")
	}
	p.status = navigationEdgeDisclosure
	return p
}

func assertLongParentNavigation(t *testing.T, p *AppleMusicPage) {
	t.Helper()
	path := T("scene.browse") + " / " + T("location.search") + " / " + T("type.album")
	if row := presentationRow(p, 2); !strings.Contains(row, path) {
		t.Errorf("real catalog path disappeared: want %q in %q", path, row)
	}
	footer := presentationRow(p, p.screen.h-2)
	if !strings.Contains(footer, T("bar.search")) {
		t.Errorf("footer search entry disappeared: %q", footer)
	}
	for _, y := range []int{2, p.screen.h - 2} {
		row, esc := presentationRow(p, y), false
		for _, hit := range p.hits {
			esc = esc || (hit.kind == appleHitKey && hit.key == "esc" && hit.y == y)
		}
		if !esc || !strings.Contains(row, "Esc") || !strings.Contains(row, "…") {
			t.Errorf("row %d lost its visible, truncated Esc target or hit: %q", y, row)
		}
	}
	for _, hit := range p.hits {
		if hit.kind != appleHitKey || (hit.y != 2 && hit.y != p.screen.h-2) {
			continue
		}
		for x := hit.x; x < hit.x+hit.w; x++ {
			if actual := p.hitAt(x, hit.y); actual == nil || actual.kind != hit.kind || actual.key != hit.key {
				t.Errorf("navigation hit overlaps another target: %+v at column %d", hit, x)
			}
		}
	}
}

func navigationEdgeFooterSearch(t *testing.T, p *AppleMusicPage) appleHit {
	t.Helper()
	for _, hit := range p.hits {
		if hit.kind == appleHitKey && hit.key == "/" && hit.y == p.screen.h-2 {
			return hit
		}
	}
	t.Fatal("missing rendered footer search hit")
	return appleHit{}
}

func checkLongParentSearchEntry(t *testing.T, p *AppleMusicPage) {
	t.Helper()
	hit := navigationEdgeFooterSearch(t, p)
	query, parent := p.input.Value(), p.catalogHistory[0]
	p.Update(tea.MouseClickMsg{X: hit.x, Y: hit.y, Button: tea.MouseLeft}, nil)
	assertNavigationEdgeEditor(t, p, query)
	if p.drawer != appleSearch || p.scene != appleBrowse {
		t.Fatal("footer search did not enter the visible search drawer")
	}
	p.status = navigationEdgeDisclosure
	assertPresentationFrame(t, p, p.draw(p.screen.w, p.screen.h, time.Unix(100, 0), 2))
	p.Update(tea.KeyPressMsg{Code: tea.KeyEscape}, nil)
	if p.editing || p.input.Focused() || len(p.catalogHistory) != 1 || !reflect.DeepEqual(parent, p.catalogHistory[0]) {
		t.Fatal("leaving input also changed the parent directory")
	}
}

func clickNavigationEdgeBack(t *testing.T, p *AppleMusicPage, row int) {
	t.Helper()
	for _, hit := range p.hits {
		if hit.kind == appleHitKey && hit.key == "esc" && hit.y == row {
			p.Update(tea.MouseClickMsg{X: hit.x, Y: hit.y, Button: tea.MouseLeft}, nil)
			return
		}
	}
	t.Fatalf("missing rendered Esc on row %d", row)
}

func checkLongParentReturns(t *testing.T, p *AppleMusicPage) {
	t.Helper()
	parent := p.catalogHistory[0]
	for _, route := range []string{"keyboard", "context", "footer"} {
		p.status = navigationEdgeDisclosure
		frame := p.draw(p.screen.w, p.screen.h, time.Unix(100, 0), 2)
		assertPresentationFrame(t, p, frame)
		if route == "keyboard" {
			p.Update(tea.KeyPressMsg{Code: tea.KeyEscape}, nil)
		} else {
			y := 2
			if route == "footer" {
				y = p.screen.h - 2
			}
			clickNavigationEdgeBack(t, p, y)
		}
		if len(p.catalogHistory) != 0 || !reflect.DeepEqual(parent.view, p.catalog) || !reflect.DeepEqual(parent.tracks, p.tracks) || p.selected != parent.selected || p.rackStart != parent.start {
			t.Fatalf("%s Esc did not restore the real parent directory, selection and viewport", route)
		}
		p.acceptCatalog(catalogStateReply(t, p.openCatalog(0, "edge-album")))
	}
}
