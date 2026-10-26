package ui

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/motion"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

// These white-box tests observe private generations and mechanical previews
// without exporting UI state or starting browser goroutines.
func catalogStateReply(t *testing.T, cmd tea.Cmd) appleCatalogMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("catalog request did not return a command")
	}
	msg, ok := cmd().(appleCatalogMsg)
	if !ok {
		t.Fatal("catalog command did not return a catalog reply")
	}
	return msg
}

type catalogCategoryCase struct {
	name, root, target, outcome string
	depth                       int
}

func TestAppleMusicSearchCategoryRestoresVisibleRoot(t *testing.T) {
	for _, tc := range []catalogCategoryCase{
		{name: "songs", root: applemusic.CatalogSongs, target: applemusic.CatalogAlbums},
		{name: "albums", root: applemusic.CatalogAlbums, target: applemusic.CatalogArtists},
		{name: "artists", root: applemusic.CatalogArtists, target: applemusic.CatalogAlbums},
		{name: "album-tracks", root: applemusic.CatalogAlbums, target: applemusic.CatalogArtists, depth: 1},
		{name: "artist-albums", root: applemusic.CatalogArtists, target: applemusic.CatalogAlbums, depth: 1},
		{name: "artist-album-tracks", root: applemusic.CatalogArtists, target: applemusic.CatalogAlbums, depth: 2},
	} {
		for _, tc.outcome = range []string{"cancel", "failure"} {
			t.Run(tc.name+"/"+tc.outcome, tc.run)
		}
	}
}

func (tc catalogCategoryCase) page(t *testing.T) *AppleMusicPage {
	t.Helper()
	c := &appleCatalogClient{page: applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: tc.root, ID: "root", Name: "Visible root"}}}}
	p := NewAppleMusicPage(c)
	t.Cleanup(func() { p.Close() })
	p.level = motion.Off
	p.input.SetValue("Visible")
	p.acceptCatalog(catalogStateReply(t, p.searchCatalog(tc.root)))
	for level := range tc.depth {
		id, kind := "root", "songs"
		if tc.root == applemusic.CatalogArtists && level == 0 {
			kind = "albums"
		}
		if level > 0 {
			id = "child"
		}
		c.page = applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: kind, ID: "child", Name: "Visible child"}}}
		p.acceptCatalog(catalogStateReply(t, p.openCatalog(0, id)))
		if tc.root != p.searchKind {
			t.Fatalf("drilldown category: want %q, got %q", tc.root, p.searchKind)
		}
	}
	return p
}

func (tc catalogCategoryCase) run(t *testing.T) {
	p := tc.page(t)
	c := p.client.(*appleCatalogClient)
	before := p.appleCatalogState
	p.input.SetValue("Replacement")
	c.page = applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: tc.target, ID: "replacement", Name: "Replacement"}}}
	if tc.outcome == "failure" {
		c.err = errors.New("category unavailable")
	}
	reply := catalogStateReply(t, p.searchCatalog(tc.target))
	generation, wantRequest := p.searchGeneration, reply.request
	if tc.outcome == "cancel" {
		p.backCatalog()
		wantRequest = before.catalog.request
		if p.searchGeneration != generation+1 {
			t.Error("cancel did not invalidate the request generation")
		}
	} else {
		p.acceptCatalog(reply)
		if p.catalogError != c.err.Error() {
			t.Error("failed category hid its retry error")
		}
	}
	tc.assertVisible(t, p, before)
	if p.searching || p.lastRequest == nil || p.lastRequest.request != wantRequest {
		t.Fatalf("wrong refresh/retry target after %s: %+v", tc.outcome, p.lastRequest)
	}
	if tc.outcome == "cancel" {
		cancelled, status := p.appleCatalogState, p.status
		p.acceptCatalog(reply)
		reply.err = errors.New("late failure")
		p.acceptCatalog(reply)
		if !reflect.DeepEqual(p.appleCatalogState, cancelled) || p.status != status || p.searchKind != tc.root {
			t.Error("cancelled response changed the catalog, category or status")
		}
	}
	tc.retry(t, p, wantRequest)
}

func (tc catalogCategoryCase) assertVisible(t *testing.T, p *AppleMusicPage, before appleCatalogState) {
	t.Helper()
	if tc.root != p.searchKind {
		t.Errorf("visible category: want root %q, got %q", tc.root, p.searchKind)
	}
	label := map[string]string{applemusic.CatalogSongs: "cat.songs", applemusic.CatalogAlbums: "cat.albums", applemusic.CatalogArtists: "cat.artists"}[tc.root]
	frame := ansi.Strip(p.draw(59, 24, time.Unix(100, 0), 2))
	if !strings.Contains(frame, "["+T(label)+"]") {
		t.Errorf("visible root tab is not selected: want [%s]", T(label))
	}
	if !reflect.DeepEqual(p.catalog, before.catalog) || !reflect.DeepEqual(p.tracks, before.tracks) || !reflect.DeepEqual(p.catalogHistory, before.catalogHistory) {
		t.Error("category interruption replaced the visible content or history")
	}
}

func (tc catalogCategoryCase) retry(t *testing.T, p *AppleMusicPage, request applemusic.CatalogRequest) {
	t.Helper()
	c := p.client.(*appleCatalogClient)
	c.err = nil
	if tc.outcome == "cancel" {
		c.page = applemusic.CatalogPage{}
	}
	reply := catalogStateReply(t, p.refreshCatalog())
	request.Next, request.Refresh = "", true
	if got := c.requests[len(c.requests)-1]; got != request {
		t.Errorf("refresh/retry request: want %+v, got %+v", request, got)
	}
	p.acceptCatalog(reply)
	root := tc.root
	if tc.outcome == "failure" {
		root = tc.target
	}
	if p.searchKind != root || p.catalogError != "" {
		t.Errorf("successful refresh/retry: want category %q without error, got %q / %q", root, p.searchKind, p.catalogError)
	}
}

type catalogReplyCase struct {
	drawer            appleDrawer
	outcome, delivery string
}

func TestAppleMusicInactiveCatalogReplyDoesNotChangeVisibleState(t *testing.T) {
	for _, drawer := range []appleDrawer{appleSearch, appleLibrary} {
		for _, outcome := range []string{"success", "error"} {
			for _, delivery := range []string{"current", "superseded", "cancelled"} {
				tc := catalogReplyCase{drawer, outcome, delivery}
				name := map[appleDrawer]string{appleSearch: "search", appleLibrary: "library"}[drawer]
				t.Run(name+"/"+outcome+"/"+delivery, tc.run)
			}
		}
	}
}

func (tc catalogReplyCase) page(t *testing.T) (*AppleMusicPage, appleCatalogMsg) {
	t.Helper()
	c := &appleLibraryClient{page: applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: "albums", ID: "visible-search", Name: "Visible search"}}}}
	p := NewAppleMusicPage(c)
	t.Cleanup(func() { p.Close() })
	p.level = motion.Off
	p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us"}
	p.input.SetValue("Visible")
	p.acceptCatalog(catalogStateReply(t, p.searchCatalog(applemusic.CatalogAlbums)))
	c.page = applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: "library-playlists", ID: "visible-library", Name: "Visible library"}}}
	p.acceptCatalog(catalogStateReply(t, p.switchDrawer(appleLibrary, time.Unix(100, 0))))
	p.switchDrawer(tc.drawer, time.Unix(100, 0))
	p.selected, p.rackStart = 7, 5
	var cmd tea.Cmd
	if tc.drawer == appleSearch {
		c.page = applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: "artists", ID: "replacement", Name: "Replacement"}}, Next: "next-page"}
		p.input.SetValue("Replacement")
		cmd = p.searchCatalog(applemusic.CatalogArtists)
	} else {
		c.page = applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: "library-albums", ID: "replacement", Name: "Replacement"}}, Next: "next-page"}
		cmd = p.loadLibrary(applemusic.LibraryAlbums)
	}
	if tc.outcome == "error" {
		c.err = errors.New("background catalog unavailable")
	}
	return p, catalogStateReply(t, cmd)
}

func (tc catalogReplyCase) run(t *testing.T) {
	p, reply := tc.page(t)
	switch tc.delivery {
	case "superseded":
		p.requestCatalog(reply.request, "Newer request", false)
	case "cancelled":
		p.backCatalog()
	}
	active := appleSearch
	if tc.drawer == appleSearch {
		active = appleLibrary
	}
	p.switchDrawer(active, time.Unix(100, 0))
	assertVisible := catalogActiveGuard(t, p)
	before := *p.catalogState(tc.drawer)
	searchKind, libraryKind := p.searchKind, p.libraryKind
	p.acceptCatalog(reply)
	assertVisible()
	if tc.delivery != "current" {
		if !reflect.DeepEqual(*p.catalogState(tc.drawer), before) || p.searchKind != searchKind || p.libraryKind != libraryKind {
			t.Error("obsolete response changed its drawer cache, loading or category")
		}
		return
	}
	tc.assertCache(t, p, before)
}

func catalogActiveGuard(t *testing.T, p *AppleMusicPage) func() {
	t.Helper()
	p.status = "active drawer status"
	p.selected, p.rackStart = 2, 1
	p.selections[p.drawer], p.rackStarts[p.drawer] = 2, 1
	p.hits = []appleHit{{kind: appleHitCatalog, x: 9, y: 11, w: 3, h: 2, id: "active-hit"}}
	p.hoverX, p.hoverY, p.rackHover, p.rackFocused = 10, 11, 2, true
	choice := appleRackChoice{id: "active-preview", index: 2, drawer: p.drawer}
	p.rackPickup.current, p.rackPickup.wanted = choice, choice
	p.rackPickup.pop = appleTween{from: .25, to: .5, at: time.Unix(100, 0), duration: time.Second}
	p.rackPickup.style = 3
	p.vinylPickup.current, p.vinylPickup.wanted = 2, 2
	active, visible := p.drawer, *p.catalogState(p.drawer)
	hits := append([]appleHit(nil), p.hits...)
	pickup, vinyl := p.rackPickup, p.vinylPickup
	searchKind, libraryKind := p.searchKind, p.libraryKind
	return func() {
		t.Helper()
		if p.status != "active drawer status" {
			t.Errorf("inactive reply replaced active status with %q", p.status)
		}
		if p.drawer != active || p.selected != 2 || p.rackStart != 1 || p.selections[active] != 2 || p.rackStarts[active] != 1 {
			t.Error("inactive reply changed active selection or viewport")
		}
		if !reflect.DeepEqual(p.hits, hits) || p.hoverX != 10 || p.hoverY != 11 || p.rackHover != 2 || !p.rackFocused || !reflect.DeepEqual(p.rackPickup, pickup) || !reflect.DeepEqual(p.vinylPickup, vinyl) {
			t.Error("inactive reply changed active hits, hover or mechanical preview")
		}
		if !reflect.DeepEqual(*p.catalogState(active), visible) || (active == appleSearch && p.searchKind != searchKind) || (active == appleLibrary && p.libraryKind != libraryKind) {
			t.Error("inactive reply changed the active catalog or category")
		}
	}
}

func (tc catalogReplyCase) assertCache(t *testing.T, p *AppleMusicPage, before appleCatalogState) {
	t.Helper()
	c := p.catalogState(tc.drawer)
	if c.searching || c.waitingSearch != nil {
		t.Error("legal background response did not settle its own loading state")
	}
	wantCategory := c.lastRequest.request.Kind
	if tc.outcome == "error" {
		if c.catalogError != "background catalog unavailable" || !reflect.DeepEqual(c.catalog, before.catalog) || !reflect.DeepEqual(c.tracks, before.tracks) || !reflect.DeepEqual(c.catalogHistory, before.catalogHistory) {
			t.Error("background failure lost its own error, cached content or history")
		}
		if p.selections[tc.drawer] != 7 || p.rackStarts[tc.drawer] != 5 || !reflect.DeepEqual(c.lastRequest, before.lastRequest) {
			t.Error("background failure changed its saved selection or lost the retry target")
		}
		wantCategory = before.catalog.request.Kind
	} else {
		if c.catalogError != "" || c.catalog.request != c.lastRequest.request || len(c.catalog.items) != 1 || c.catalog.items[0].ID != "replacement" || c.catalog.next != "next-page" {
			t.Error("legal background success was dropped instead of updating its own cache")
		}
		if p.selections[tc.drawer] != 0 || p.rackStarts[tc.drawer] != 0 {
			t.Error("background root success did not reset its saved selection and viewport")
		}
	}
	gotCategory := p.searchKind
	if tc.drawer == appleLibrary {
		gotCategory = p.libraryKind
	}
	if gotCategory != wantCategory {
		t.Errorf("background category: want %q, got %q", wantCategory, gotCategory)
	}
}
