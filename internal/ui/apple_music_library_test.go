package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/motion"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

type appleLibraryClient struct {
	appleMouseClient
	requests []applemusic.CatalogRequest
	page     applemusic.CatalogPage
	err      error
}

func TestAppleMusicLibraryRepliesStayInTheirDrawerAndLogoutClearsData(t *testing.T) {
	SetLanguage("zh")
	defer SetLanguage("en")
	c := &appleLibraryClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Off
	p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us"}
	p.tracks = []applemusic.Track{{ID: "search"}}
	first := p.handleKey(tea.KeyPressMsg{Code: '2'})
	latest := p.handleKey(tea.KeyPressMsg{Code: '6'})
	c.page.Items = []applemusic.CatalogItem{{ID: "i.song", Type: "library-songs", Name: "Library Song", Playable: true}}
	p.acceptCatalog(latest().(appleCatalogMsg))
	c.page.Items = []applemusic.CatalogItem{{ID: "p.old", Type: "library-playlists", Name: "Old Playlist"}}
	p.acceptCatalog(first().(appleCatalogMsg))
	if len(p.items()) != 1 || p.items()[0].ID != "i.song" {
		t.Fatal("late category reply replaced current library")
	}
	p.handleKey(tea.KeyPressMsg{Code: '1'})
	if p.items()[0].ID != "search" {
		t.Fatal("library replaced search")
	}
	if !p.editing || !p.input.Focused() {
		t.Fatal("search navigation did not focus the editor")
	}
	p.Update(appleStateMsg{state: applemusic.State{Ready: true, Storefront: "us"}}, model.NewApp(model.DefaultOptions()))
	if len(p.library.tracks) != 0 || len(p.library.catalog.items) != 0 {
		t.Fatal("logout retained private library data")
	}
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if p.handleKey(tea.KeyPressMsg{Code: '2'}) != nil {
		t.Fatal("unauthorized library attempted fetch")
	}
	out := ansi.Strip(p.draw(140, 45, time.Unix(100, 0), 2))
	if !strings.Contains(out, "登录后查看资料库") {
		t.Fatal("unauthorized library was presented as empty")
	}
}

func TestAppleMusicLibraryFailureWaitsForExplicitRefresh(t *testing.T) {
	SetLanguage("zh")
	defer SetLanguage("en")
	c := &appleLibraryClient{err: errors.New("temporary library error")}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Off
	p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us"}
	cmd := p.handleKey(tea.KeyPressMsg{Code: '2'})
	p.acceptCatalog(cmd().(appleCatalogMsg))
	if p.ensureLibrary() != nil || p.library.catalogError == "" {
		t.Fatal("library failure was hidden or automatically retried")
	}
	c.err = nil
	cmd = p.handleKey(tea.KeyPressMsg{Code: 'r'})
	if cmd == nil {
		t.Fatal("refresh did not retry library")
	}
	p.acceptCatalog(cmd().(appleCatalogMsg))
	if p.library.catalogError != "" || p.library.catalog.title != "我的歌单" {
		t.Fatal("empty successful library was treated as failure")
	}
}

func TestAppleMusicLibraryCancelRestoresTheVisibleCategory(t *testing.T) {
	c := &appleLibraryClient{page: applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: "library-playlists", ID: "p.fixture", Name: "Playlist"}}}}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Off
	p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us"}
	p.acceptCatalog(p.handleKey(tea.KeyPressMsg{Code: '2'})().(appleCatalogMsg))
	p.handleKey(tea.KeyPressMsg{Code: '5'})
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if p.libraryKind != applemusic.LibraryPlaylists || p.library.lastRequest.request.Kind != applemusic.LibraryPlaylists {
		t.Fatal("cancelled category left mismatched tabs or refresh target")
	}
	c.err = errors.New("albums unavailable")
	p.acceptCatalog(p.handleKey(tea.KeyPressMsg{Code: '5'})().(appleCatalogMsg))
	if p.libraryKind != applemusic.LibraryPlaylists || p.library.lastRequest.request.Kind != applemusic.LibraryAlbums {
		t.Fatal("failed category mislabeled retained data or lost explicit retry target")
	}
}

func TestAppleMusicLibraryDistinguishesConnectingAndCancelledFromEmpty(t *testing.T) {
	SetLanguage("zh")
	defer SetLanguage("en")
	p := NewAppleMusicPage(&appleLibraryClient{})
	defer p.Close()
	p.level = motion.Off
	p.handleKey(tea.KeyPressMsg{Code: '2'})
	if !strings.Contains(ansi.Strip(p.draw(140, 45, time.Unix(100, 0), 2)), "正在连接 Apple Music") {
		t.Fatal("unknown session was presented as logged out")
	}
	p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us"}
	p.ensureLibrary()
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !strings.Contains(p.catalogEmptyLabel(), "尚未加载") {
		t.Fatal("cancelled initial request was presented as an empty library")
	}
}

func TestAppleMusicLibraryCategoryPreviews(t *testing.T) {
	for _, tc := range []struct{ kind, resource string }{{applemusic.LibraryPlaylists, "library-playlists"}, {applemusic.LibraryAlbums, "library-albums"}, {applemusic.LibrarySongs, "library-songs"}, {applemusic.Recommendations, "personal-recommendation"}, {applemusic.PersonalStation, "stations"}, {applemusic.RecommendationContents, "playlists"}} {
		for _, size := range [][2]int{{255, 68}, {140, 45}, {120, 24}, {80, 30}, {50, 20}} {
			c := &appleLibraryClient{}
			for i := range 12 {
				c.page.Items = append(c.page.Items, applemusic.CatalogItem{Type: tc.resource, ID: fmt.Sprintf("fixture.%d", i), Name: fmt.Sprintf("资源 %02d / %s", i, tc.resource), Artist: "FRAMEBUFFER TEST", Playable: true})
			}
			p := NewAppleMusicPage(c)
			p.level = motion.Full
			p.drawer = appleLibrary
			p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us"}
			p.acceptCatalog(p.requestCatalog(applemusic.CatalogRequest{Kind: tc.kind, ID: "fixture"}, "测试分类", false)().(appleCatalogMsg))
			p.selected, p.rackFocused = 2, true
			now := time.Unix(100, 0)
			p.draw(size[0], size[1], now, 2)
			p.status = "FRAMEBUFFER TEST · 合成资料库，非真实账户"
			out := p.draw(size[0], size[1], now.Add(time.Second), 2)
			for _, line := range strings.Split(out, "\n") {
				if ansi.WcWidth.StringWidth(line) != size[0] {
					t.Fatal("category overflow")
				}
			}
			if dir := os.Getenv("TAPEFOX_LIBRARY_PREVIEW_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("%s-%dx%d.html", tc.kind, size[0], size[1])))
			}
			p.Close()
		}
	}
}

func TestAppleMusicLibrarySongKeepsOpaqueIDAndConfirmsCatalogAlias(t *testing.T) {
	c := &appleLibraryClient{page: applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: "library-songs", ID: "i.opaque", CatalogID: "123", Name: "Mapped song", Playable: true}}}}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Off
	p.drawer = appleLibrary
	p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us"}
	p.acceptCatalog(p.loadLibrary(applemusic.LibrarySongs)().(appleCatalogMsg))
	cmd := p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("library song was not playable")
	}
	m := cmd().(appleActionMsg)
	p.Update(m, model.NewApp(model.DefaultOptions()))
	if len(c.actions) != 1 || c.actions[0].ID != "i.opaque" || c.actions[0].ResourceType != "library-songs" {
		t.Fatal("library ID was treated as a catalog song")
	}
	style := p.insertStyle
	p.acceptState(applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Storefront: "us", Track: applemusic.Track{ID: "123"}}, time.Now())
	if p.pending || p.styles["123"] != style {
		t.Fatal("documented catalog alias lost playback confirmation or cassette identity")
	}
}

func TestAppleMusicQueuedLibrarySongDoesNotRequireAnotherPlayParamsLookup(t *testing.T) {
	c := &appleMouseClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Off
	p.drawer = appleQueue
	p.state = applemusic.State{Ready: true, Authorized: true, QueueTotal: 1, Queue: []applemusic.Track{{ID: "i.queued", Type: "library-songs", Name: "Queued song"}}}
	if strings.Contains(ansi.Strip(p.draw(140, 45, time.Unix(100, 0), 2)), "不可播放") {
		t.Fatal("queued item was mislabeled unavailable")
	}
	cmd := p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("existing queued library item was blocked by missing catalog metadata")
	}
	cmd()
	if len(c.actions) != 1 || c.actions[0].Kind != "jump" || c.actions[0].ID != "i.queued" {
		t.Fatal("queued library item did not use validated queue jump")
	}
}

func TestAppleMusicRecommendationsAndStationsOnlyPlayOnActivation(t *testing.T) {
	c := &appleLibraryClient{page: applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: "personal-recommendation", ID: "rec", Name: "For You"}}}}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Off
	p.drawer = appleLibrary
	p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us"}
	p.acceptCatalog(p.loadLibrary(applemusic.Recommendations)().(appleCatalogMsg))
	c.page.Items = []applemusic.CatalogItem{{Type: "stations", ID: "ra.personal", Name: "My Station", Playable: true}}
	p.acceptCatalog(p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})().(appleCatalogMsg))
	if len(c.actions) != 0 {
		t.Fatal("opening recommendation started playback")
	}
	cmd := p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("station cannot play")
	}
	cmd()
	if c.actions[0].Kind != "play-station" || c.actions[0].ResourceType != "stations" || p.pendingID != "" {
		t.Fatal("station ID was mistaken for the eventual song ID")
	}
}

func TestAppleMusicLibraryFramesAndCollectionPlayback(t *testing.T) {
	for _, size := range [][2]int{{140, 45}, {120, 24}, {80, 30}, {50, 20}} {
		c := &appleLibraryClient{page: applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: "library-playlists", ID: "p.fixture", Name: "My Playlist / 测试歌单", Playable: true}}}}
		p := NewAppleMusicPage(c)
		p.level = motion.Off
		p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us"}
		p.acceptCatalog(p.handleKey(tea.KeyPressMsg{Code: '2'})().(appleCatalogMsg))
		p.status = "FRAMEBUFFER TEST · 合成资料库，非真实账户"
		out := p.draw(size[0], size[1], time.Unix(100, 0), 2)
		for _, line := range strings.Split(out, "\n") {
			if ansi.WcWidth.StringWidth(line) != size[0] {
				t.Fatal("library frame overflow")
			}
		}
		if dir := os.Getenv("TAPEFOX_LIBRARY_PREVIEW_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("library-%dx%d.html", size[0], size[1])))
		}
		cmd := p.handleKey(tea.KeyPressMsg{Code: 'P'})
		if cmd == nil {
			t.Fatal("collection play missing")
		}
		cmd()
		if len(c.actions) != 1 || c.actions[0].Kind != "play-collection" || c.actions[0].ID != "p.fixture" {
			t.Fatal("playlist play used a song endpoint")
		}
		p.Close()
	}
}

func (c *appleLibraryClient) Catalog(_ context.Context, q applemusic.CatalogRequest) (applemusic.CatalogPage, error) {
	c.requests = append(c.requests, q)
	return c.page, c.err
}

func TestAppleMusicLibraryLoadsPlaylistsAndPreservesSearch(t *testing.T) {
	c := &appleLibraryClient{page: applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: "library-playlists", ID: "p.opaque", Name: "My Playlist", Playable: true}}}}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Off
	p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us"}
	p.tracks = []applemusic.Track{{ID: "search-result", Name: "Search Song"}}
	a := model.NewApp(model.DefaultOptions())
	cmd := p.handleKey(tea.KeyPressMsg{Code: '2'})
	if cmd == nil || len(c.requests) != 0 {
		t.Fatal("library did not schedule an asynchronous read")
	}
	p.Update(cmd(), a)
	if len(c.requests) != 1 || c.requests[0].Kind != applemusic.LibraryPlaylists {
		t.Fatalf("library route: %+v", c.requests)
	}
	out := ansi.Strip(p.draw(140, 45, time.Unix(100, 0), 2))
	if !strings.Contains(out, "My Playlist") || strings.Contains(out, "资料库接入待验证") {
		t.Fatal("library remains a placeholder")
	}
	c.page.Items = []applemusic.CatalogItem{{Type: "library-songs", ID: "i.opaque", Name: "Library Song", Playable: true}}
	cmd = p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("playlist did not open")
	}
	p.Update(cmd(), a)
	if len(p.items()) != 1 || p.items()[0].ID != "i.opaque" || len(c.actions) != 0 {
		t.Fatal("playlist browsing played music or lost library IDs")
	}
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if !strings.Contains(ansi.Strip(p.draw(140, 45, time.Unix(100, 0), 2)), "My Playlist") {
		t.Fatal("library back failed")
	}
	p.handleKey(tea.KeyPressMsg{Code: '1'})
	if len(p.items()) != 1 || p.items()[0].ID != "search-result" {
		t.Fatal("library overwrote search context")
	}
}
