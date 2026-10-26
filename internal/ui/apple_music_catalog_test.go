package ui

import (
	"context"
	"errors"
	"fmt"
	"math"
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

type appleCatalogClient struct {
	appleMouseClient
	requests []applemusic.CatalogRequest
	page     applemusic.CatalogPage
	err      error
}

func (c *appleCatalogClient) Catalog(_ context.Context, q applemusic.CatalogRequest) (applemusic.CatalogPage, error) {
	c.requests = append(c.requests, q)
	return c.page, c.err
}

func TestAppleMusicCatalogDrilldownAndBack(t *testing.T) {
	c := &appleCatalogClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Off
	p.input.SetValue("Artist query")
	a := model.NewApp(model.DefaultOptions())
	run := func(cmd tea.Cmd) {
		t.Helper()
		if cmd == nil {
			t.Fatal("catalog navigation did not return a background request")
		}
		p.Update(cmd(), a)
	}
	c.page.Items = []applemusic.CatalogItem{{Type: "artists", ID: "artist-id", Name: "An Artist"}}
	cmd := p.handleKey(tea.KeyPressMsg{Code: '6'})
	if len(c.requests) != 0 {
		t.Fatal("catalog ran on UI thread")
	}
	run(cmd)
	if len(c.requests) != 1 || c.requests[0].Kind != "artists" || c.requests[0].Query != "Artist query" {
		t.Fatalf("wrong artist search: %+v", c.requests)
	}
	frame := ansi.Strip(p.draw(140, 45, time.Unix(100, 0), 2))
	if !strings.Contains(frame, "An Artist") {
		t.Fatal("artist result not rendered")
	}
	c.page.Items = []applemusic.CatalogItem{{Type: "albums", ID: "album-id", Name: "An Album", Artist: "An Artist"}}
	run(p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if c.requests[1].Kind != "artist-albums" || c.requests[1].ID != "artist-id" {
		t.Fatal("artist did not open albums")
	}
	c.page.Items = []applemusic.CatalogItem{{Type: "songs", ID: "song-id", Name: "A Song"}, {Type: "music-videos", ID: "video-id", Name: "A Video"}}
	run(p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter}))
	if c.requests[2].Kind != "album-tracks" || len(p.items()) != 1 || p.items()[0].ID != "song-id" {
		t.Fatal("album did not open playable songs only")
	}
	if len(c.actions) != 0 || p.inserting || len(p.styles) != 0 {
		t.Fatal("opening a container started playback or assigned cassette style")
	}
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	frame = ansi.Strip(p.draw(140, 45, time.Unix(100, 0), 2))
	if !strings.Contains(frame, "An Album") || len(c.requests) != 3 {
		t.Fatal("back did not restore the parent without refetching")
	}
}

func TestAppleMusicPopupStaysAnchoredAtListEdges(t *testing.T) {
	for _, size := range [][2]int{{140, 45}, {120, 38}, {80, 30}} {
		for _, amount := range []float64{.5, 1} {
			var offset float64
			for _, index := range []int{0, 5, 19} {
				p := NewAppleMusicPage(nil)
				p.level = motion.Full
				for i := range 20 {
					p.tracks = append(p.tracks, applemusic.Track{ID: fmt.Sprint(i), Name: "Track"})
				}
				p.selected, p.rackFocused = index, true
				now := time.Unix(100, 0)
				p.draw(size[0], size[1], now, 2)
				if amount == 1 {
					p.beginInsert(p.tracks[index], now.Add(200*time.Millisecond))
					now = p.insertStart.Add(-time.Nanosecond)
				} else {
					now = now.Add(200 * time.Millisecond)
				}
				p.draw(size[0], size[1], now, 2)
				found, visible := false, false
				for _, h := range p.hits {
					if h.kind == appleHitCassette && h.index == index {
						visible = true
					}
					if h.kind != appleHitTrack || h.index != index {
						continue
					}
					found = true
					got := h.cassette.Y - float64(h.slot.Min.Y)
					if index == 0 {
						offset = got
					} else if math.Abs(got-offset) > .01 {
						t.Errorf("%v pop=%g index=%d shifted from slot: offset %.2f, first %.2f", size, amount, index, got, offset)
					}
				}
				if !found || !visible {
					t.Errorf("%v pop=%g index=%d: missing visible source", size, amount, index)
				}
				p.Close()
			}
		}
	}
}

func TestAppleMusicCatalogLatestRequestAndPagination(t *testing.T) {
	c := &appleCatalogClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Off
	a := model.NewApp(model.DefaultOptions())
	p.input.SetValue("First")
	old := p.handleKey(tea.KeyPressMsg{Code: '5'})
	p.input.SetValue("Latest")
	latest := p.handleKey(tea.KeyPressMsg{Code: '6'})
	c.page = applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: "artists", ID: "one", Name: "Latest Artist"}}, Next: "/v1/catalog/us/search?offset=25"}
	p.Update(latest(), a)
	p.Update(old(), a)
	if p.catalog.request.Query != "Latest" || p.catalog.request.Kind != "artists" {
		t.Fatal("stale search overwrote current results")
	}
	p.selected = 0
	page := p.handleKey(tea.KeyPressMsg{Code: 'l'})
	if page == nil || p.handleKey(tea.KeyPressMsg{Code: 'l'}) != nil {
		t.Fatal("pagination missing or dispatched twice")
	}
	c.err = errors.New("temporary failure")
	p.Update(page(), a)
	if p.catalog.next == "" || len(p.catalog.items) != 1 || p.catalogError == "" {
		t.Fatal("failed page discarded results/cursor or hid failure")
	}
	c.err = nil
	c.page = applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: "artists", ID: "two", Name: "Second Artist"}}}
	p.Update(p.handleKey(tea.KeyPressMsg{Code: 'l'})(), a)
	if len(p.catalog.items) != 2 || p.catalog.next != "" || p.selected != 0 {
		t.Fatal("pagination did not append while preserving selection")
	}
	q := c.requests[len(c.requests)-1]
	if q.Query != "Latest" || q.Next != "/v1/catalog/us/search?offset=25" {
		t.Fatalf("wrong pagination context %+v", q)
	}
	if p.handleKey(tea.KeyPressMsg{Code: 'l'}) != nil {
		t.Fatal("requested beyond last page")
	}
}

func TestAppleMusicCatalogBackRestoresSelectionAndRejectsLateDrilldown(t *testing.T) {
	c := &appleCatalogClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Off
	a := model.NewApp(model.DefaultOptions())
	p.input.SetValue("Albums")
	for i := range 30 {
		c.page.Items = append(c.page.Items, applemusic.CatalogItem{Type: "albums", ID: fmt.Sprint(i), Name: fmt.Sprintf("Album %d", i)})
	}
	p.Update(p.handleKey(tea.KeyPressMsg{Code: '5'})(), a)
	p.selected = 20
	p.draw(120, 38, time.Unix(100, 0), 2)
	start := p.rackStart
	open := p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	c.page = applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: "songs", ID: "track", Name: "Track"}}}
	p.Update(open(), a)
	if !p.catalogContainers() || p.selected != 20 {
		t.Fatal("cancelled drilldown was applied late")
	}
	open = p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	p.Update(open(), a)
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if p.selected != 20 || p.rackStart != start || !p.catalogContainers() {
		t.Fatal("back lost the opened container's selection or scroll")
	}
}

func TestAppleMusicCatalogMouseAndFramebuffer(t *testing.T) {
	for _, size := range [][2]int{{140, 45}, {120, 24}, {80, 30}, {50, 20}} {
		c := &appleCatalogClient{}
		p := NewAppleMusicPage(c)
		p.level = motion.Off
		p.input.SetValue("Example")
		c.page.Items = []applemusic.CatalogItem{{Type: "albums", ID: "album", Name: "Example Album", Artist: "Example Artist"}}
		a := model.NewApp(model.DefaultOptions())
		p.Update(p.handleKey(tea.KeyPressMsg{Code: '5'})(), a)
		out := p.draw(size[0], size[1], time.Unix(100, 0), 2)
		if dir := os.Getenv("TAPEFOX_CATALOG_PREVIEW_DIR"); dir != "" {
			if err := os.MkdirAll(dir, 0700); err != nil {
				t.Fatal(err)
			}
			writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("albums-%dx%d.html", size[0], size[1])))
		}
		for _, line := range strings.Split(out, "\n") {
			if ansi.WcWidth.StringWidth(line) != size[0] {
				t.Fatal("catalog frame overflows")
			}
		}
		if len(p.styles) != 0 {
			t.Fatal("container consumed random cassette style")
		}
		clicked := false
		for _, h := range p.hits {
			if h.id != "album" {
				continue
			}
			c.page.Items = []applemusic.CatalogItem{{Type: "songs", ID: "song", Name: "Example Song"}}
			cmd := p.handleClick(h.x, h.y)
			if cmd == nil {
				t.Fatal("rendered container is not clickable")
			}
			p.Update(cmd(), a)
			clicked = true
			break
		}
		if !clicked || len(p.items()) != 1 || len(c.actions) != 0 {
			t.Fatal("container click failed or played music")
		}
		p.draw(size[0], size[1], time.Unix(100, 0), 2)
		if dir := os.Getenv("TAPEFOX_CATALOG_PREVIEW_DIR"); dir != "" {
			writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("album-tracks-%dx%d.html", size[0], size[1])))
		}
		p.Close()
	}
}

func TestAppleMusicCatalogAlignmentPreview(t *testing.T) {
	dir := os.Getenv("TAPEFOX_CATALOG_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set TAPEFOX_CATALOG_PREVIEW_DIR for real framebuffer exports")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{140, 45}, {120, 24}, {80, 30}, {50, 20}} {
		for _, index := range []int{0, 5, 19} {
			p := NewAppleMusicPage(nil)
			p.level = motion.Full
			for i := range 20 {
				p.tracks = append(p.tracks, applemusic.Track{ID: fmt.Sprint(i), Name: fmt.Sprintf("Track %02d / 测试歌曲", i+1), Artist: "FRAMEBUFFER TEST"})
			}
			p.selected, p.rackFocused = index, true
			now := time.Unix(100, 0)
			p.draw(size[0], size[1], now, 2)
			p.draw(size[0], size[1], now.Add(200*time.Millisecond), 2)
			writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("popup-%dx%d-%02d.html", size[0], size[1], index)))
			p.Close()
		}
	}
}

func TestAppleMusicNarrowPopupKeepsMultipleSlots(t *testing.T) {
	p := storageTestPage()
	defer p.Close()
	p.draw(80, 30, time.Unix(100, 0), 2)
	count := 0
	for _, h := range p.hits {
		if h.kind == appleHitTrack {
			count++
		}
	}
	if count < 2 {
		t.Fatal("popup headroom displaced the narrow song list")
	}
}

func TestAppleMusicEnterOpensHoveredContainer(t *testing.T) {
	c := &appleCatalogClient{page: applemusic.CatalogPage{Items: []applemusic.CatalogItem{
		{Type: "albums", ID: "first", Name: "First"}, {Type: "albums", ID: "hovered", Name: "Hovered"},
	}}}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.input.SetValue("Albums")
	a := model.NewApp(model.DefaultOptions())
	p.Update(p.handleKey(tea.KeyPressMsg{Code: '5'})(), a)
	p.draw(120, 38, time.Unix(100, 0), 2)
	for _, h := range p.hits {
		if h.id == "hovered" {
			p.Update(tea.MouseMotionMsg{X: h.x, Y: h.y}, a)
			break
		}
	}
	cmd := p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("hovered album did not open")
	}
	cmd()
	if c.requests[len(c.requests)-1].ID != "hovered" {
		t.Fatal("Enter opened the keyboard selection instead of the hovered album")
	}
}

func TestAppleMusicVinylAlbumCrateDoesNotConsumeCassetteStyles(t *testing.T) {
	c := &appleCatalogClient{page: applemusic.CatalogPage{Items: []applemusic.CatalogItem{
		{Type: "albums", ID: "alb-1", Name: "Midnight Vinyl", Artist: "Neon Artist"},
		{Type: "albums", ID: "alb-2", Name: "Second LP", Artist: "Neon Artist"},
		{Type: "albums", ID: "alb-3", Name: "Third LP", Artist: "Neon Artist"},
	}}}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Full
	p.input.SetValue("Vinyl")
	a := model.NewApp(model.DefaultOptions())
	p.Update(p.handleKey(tea.KeyPressMsg{Code: '5'})(), a)
	now := time.Unix(100, 0)
	p.draw(140, 45, now, 2)
	out := ansi.Strip(p.draw(140, 45, now.Add(230*time.Millisecond), 2))
	if !strings.Contains(out, "LP 01") || !strings.Contains(out, "Midnight Vinyl") {
		t.Fatalf("vinyl LP crate did not render album sleeve or peeking record: %s", out)
	}
	if len(p.styles) != 0 {
		t.Fatal("vinyl album crate consumed random cassette styles")
	}
	if p.vinylPickup.pop.value(now.Add(230*time.Millisecond)) != .5 {
		t.Fatal("selected vinyl record did not settle at half peek")
	}
}
