package ui

import (
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

type appleCatalogView struct {
	request     applemusic.CatalogRequest
	title, next string
	items       []applemusic.CatalogItem
}

type appleCatalogState struct {
	tracks           []applemusic.Track
	catalog          appleCatalogView
	catalogHistory   []appleCatalogParent
	catalogError     string
	searching        bool
	searchGeneration uint64
	waitingSearch    *appleCatalogMsg
	lastRequest      *appleCatalogMsg
}

func (p *AppleMusicPage) catalogState(drawer appleDrawer) *appleCatalogState {
	if drawer == appleLibrary {
		return &p.library
	}
	return &p.appleCatalogState
}

type appleCatalogParent struct {
	view            appleCatalogView
	tracks          []applemusic.Track
	selected, start int
}

type appleCatalogMsg struct {
	drawer     appleDrawer
	request    applemusic.CatalogRequest
	title      string
	parent     *appleCatalogParent
	generation uint64
	page       applemusic.CatalogPage
	err        error
}

func (p *AppleMusicPage) catalogContainers() bool {
	if p.drawer == appleQueue {
		return false
	}
	switch p.catalogState(p.drawer).catalog.request.Kind {
	case "", applemusic.CatalogSongs, applemusic.CatalogAlbumTracks, applemusic.CatalogPlaylistTracks, applemusic.LibrarySongs, applemusic.LibraryPlaylistTracks, applemusic.LibraryAlbumTracks:
		return false
	default:
		return true
	}
}

func (p *AppleMusicPage) listCount() int {
	if p.catalogContainers() {
		return len(p.catalogState(p.drawer).catalog.items)
	}
	return len(p.items())
}

func (p *AppleMusicPage) searchCatalog(kind string) tea.Cmd {
	p.searchKind = kind
	query := strings.TrimSpace(p.input.Value())
	if query == "" {
		p.status = T("status.input_empty")
		p.editing = true
		return p.input.Focus()
	}
	p.editing = false
	p.input.Blur()
	label := map[string]string{applemusic.CatalogSongs: T("type.song"), applemusic.CatalogAlbums: T("type.album"), applemusic.CatalogArtists: T("type.artist")}[kind]
	return p.requestCatalog(applemusic.CatalogRequest{Kind: kind, Query: query}, label+" · "+query, false)
}

func (p *AppleMusicPage) requestCatalog(q applemusic.CatalogRequest, title string, push bool) tea.Cmd {
	refresh := q.Refresh
	q.Refresh = false
	drawer := p.drawer
	c := p.catalogState(drawer)
	c.searchGeneration++
	generation := c.searchGeneration
	var parent *appleCatalogParent
	if push {
		parent = &appleCatalogParent{c.catalog, c.tracks, p.selected, p.rackStart}
	}
	c.searching, c.waitingSearch, c.catalogError = true, nil, ""
	c.lastRequest = &appleCatalogMsg{drawer: drawer, request: q, title: title, parent: parent, generation: generation}
	p.status = T("status.cat_loading")
	return func() tea.Msg {
		request := q
		request.Refresh = refresh
		page, err := p.client.Catalog(p.ctx, request)
		return appleCatalogMsg{drawer: drawer, request: q, title: title, parent: parent, generation: generation, page: page, err: err}
	}
}

func (p *AppleMusicPage) openCatalog(index int, id string) tea.Cmd {
	c := p.catalogState(p.drawer)
	if c.searching || index < 0 || index >= len(c.catalog.items) || c.catalog.items[index].ID != id {
		return nil
	}
	item := c.catalog.items[index]
	p.selected = index
	if item.Type == "stations" {
		return p.playCatalogItem(item)
	}
	kind := map[string]string{"albums": applemusic.CatalogAlbumTracks, "artists": applemusic.CatalogArtistAlbums, "playlists": applemusic.CatalogPlaylistTracks,
		"library-albums": applemusic.LibraryAlbumTracks, "library-playlists": applemusic.LibraryPlaylistTracks, "personal-recommendation": applemusic.RecommendationContents}[item.Type]
	if kind == "" {
		return nil
	}
	return p.requestCatalog(applemusic.CatalogRequest{Kind: kind, ID: id}, item.Name, true)
}

func (p *AppleMusicPage) moreCatalog() tea.Cmd {
	c := p.catalogState(p.drawer)
	if p.drawer == appleQueue || c.searching || c.catalog.next == "" {
		return nil
	}
	q := c.catalog.request
	q.Next = c.catalog.next
	return p.requestCatalog(q, c.catalog.title, false)
}

func (p *AppleMusicPage) backCatalog() {
	if p.drawer == appleQueue {
		return
	}
	c := p.catalogState(p.drawer)
	if c.searching {
		c.searchGeneration++
		c.searching, c.waitingSearch = false, nil
		if c.catalog.title != "" {
			c.lastRequest = &appleCatalogMsg{drawer: p.drawer, request: c.catalog.request, title: c.catalog.title}
		}
		p.restoreCatalogCategory(p.drawer)
		p.status = T("status.cat_canceled")
		return
	}
	if len(c.catalogHistory) == 0 {
		return
	}
	c.searchGeneration++
	c.searching, c.waitingSearch, c.catalogError = false, nil, ""
	parent := c.catalogHistory[len(c.catalogHistory)-1]
	c.catalogHistory = c.catalogHistory[:len(c.catalogHistory)-1]
	c.catalog, c.tracks = parent.view, parent.tracks
	p.restoreCatalogCategory(p.drawer)
	c.lastRequest = &appleCatalogMsg{drawer: p.drawer, request: c.catalog.request, title: c.catalog.title}
	p.selected, p.rackStart = parent.selected, parent.start
	p.selections[p.drawer] = p.selected
	p.hits = nil
	p.hoverX, p.hoverY, p.rackHover = -1, -1, -1
	p.resetRackPickup()
	p.status = T("status.cat_returned", c.catalog.title)
}

func (p *AppleMusicPage) acceptCatalog(m appleCatalogMsg) {
	c := p.catalogState(m.drawer)
	if m.generation != c.searchGeneration || (m.drawer == appleLibrary && !p.state.Authorized) {
		return
	}
	c.searching, c.waitingSearch = false, nil
	if m.err != nil {
		c.catalogError = m.err.Error()
		if c.catalog.title != "" {
			p.restoreCatalogCategory(m.drawer)
		}
		if p.drawer == m.drawer {
			p.status = T("status.cat_retry", c.catalogError)
		}
		return
	}
	c.catalogError = ""
	if m.request.Next == "" {
		if m.parent != nil {
			c.catalogHistory = append(c.catalogHistory, *m.parent)
		} else if m.request != c.catalog.request {
			c.catalogHistory = nil
		}
		c.catalog = appleCatalogView{request: m.request, title: m.title}
		c.tracks = nil
		p.selections[m.drawer] = 0
		p.rackStarts[m.drawer] = 0
		if p.drawer == m.drawer {
			p.selected, p.rackStart = 0, 0
			p.resetRackPickup()
			p.hoverX, p.hoverY, p.rackHover = -1, -1, -1
			p.hits = nil
		}
	}
	for _, item := range m.page.Items {
		if item.ID == "" {
			continue
		}
		switch m.request.Kind {
		case applemusic.CatalogSongs, applemusic.CatalogAlbumTracks, applemusic.CatalogPlaylistTracks, applemusic.LibrarySongs, applemusic.LibraryPlaylistTracks, applemusic.LibraryAlbumTracks:
			if item.Type == "songs" || item.Type == "library-songs" {
				c.tracks = append(c.tracks, applemusic.Track{ID: item.ID, Type: item.Type, Playable: item.Playable, CatalogID: item.CatalogID, Name: item.Name, Artist: item.Artist, Album: item.Album, Duration: item.Duration})
			}
		case applemusic.CatalogArtists:
			if item.Type == "artists" {
				c.catalog.items = append(c.catalog.items, item)
			}
		default:
			if catalogItemLabel(item.Type) != "" {
				c.catalog.items = append(c.catalog.items, item)
			}
		}
	}
	c.catalog.next = m.page.Next
	p.restoreCatalogCategory(m.drawer)
	if p.drawer == m.drawer {
		p.status = T("status.cat_loaded_n", len(c.tracks)+len(c.catalog.items))
		if c.catalog.next != "" {
			p.status += T("status.cat_more")
		}
	}
}

// drawCatalogTabs renders Swiss-grid category tabs above the media crate.
func (p *AppleMusicPage) drawCatalogTabs(x, y, w int, kinds []string, keys []struct{ key, label string }, selected string) {
	if p.screen.w < 60 || p.screen.h < 20 || w < 34 {
		for i := range keys {
			if selected == kinds[i] {
				keys[i].label = "[" + keys[i].label + "]"
			}
		}
		p.drawKeys(x, y, w, keys)
		return
	}
	cx := x
	for i, k := range keys {
		active := selected == kinds[i]
		body := " " + k.label + " "
		tabW := npWidth(body) + 1
		if cx+tabW > x+w {
			break
		}
		hovered := p.hoverX >= cx && p.hoverX < cx+tabW && p.hoverY == y
		if active {
			p.screen.set(cx, y, "▍", 1, uv.Style{Fg: appleNeon.Accent, Bg: appleNeon.Surface})
			p.screen.text(cx+1, y, body, uv.Style{Fg: appleNeon.Text, Bg: appleNeon.Surface, Attrs: uv.AttrBold}, tabW-1)
		} else {
			fg := color.Color(appleNeon.Secondary)
			if hovered {
				fg = appleNeon.Hero
			}
			p.screen.set(cx, y, " ", 1, uv.Style{Fg: appleNeon.Rule})
			p.screen.text(cx+1, y, body, uv.Style{Fg: fg}, tabW-1)
		}
		p.addHit(appleHit{kind: appleHitKey, x: cx, y: y, w: tabW, h: 1, key: k.key})
		cx += tabW + 1
	}
}

func (p *AppleMusicPage) drawCatalogHeader(x, y, w int) int {
	c := p.catalogState(p.drawer)
	kinds := []string{applemusic.CatalogSongs, applemusic.CatalogAlbums, applemusic.CatalogArtists}
	keys := []struct{ key, label string }{{"4", T("cat.songs")}, {"5", T("cat.albums")}, {"6", T("cat.artists")}}
	selectedKind := p.searchKind
	if p.drawer == appleLibrary {
		kinds = libraryKinds
		keys = []struct{ key, label string }{
			{"4", T("cat.playlists")},
			{"5", T("cat.lib_albums")},
			{"6", T("cat.lib_songs")},
			{"7", T("cat.made_for_you")},
			{"8", T("cat.stations")},
		}
		selectedKind = p.libraryKind
	}
	p.drawCatalogTabs(x, y, w, kinds, keys, selectedKind)
	y++
	if len(c.catalogHistory) > 0 {
		p.drawKeys(x, y, w, []struct{ key, label string }{{"backspace", T("cat.back")}})
		crumbs := []string{c.catalogHistory[0].view.title}
		for _, parent := range c.catalogHistory[1:] {
			crumbs = append(crumbs, parent.view.title)
		}
		crumbs = append(crumbs, c.catalog.title)
		p.screen.text(x+9, y, strings.Join(crumbs, " / "), uv.Style{Fg: appleNeon.Secondary}, max(0, w-9))
		y++
	}
	if p.drawer == appleLibrary {
		p.drawKeys(x, y, w, []struct{ key, label string }{{"r", T("cat.refresh")}, {"P", T("cat.play_collection")}})
		y++
	}
	return y
}

func (p *AppleMusicPage) catalogEmptyLabel() string {
	c := p.catalogState(p.drawer)
	if p.drawer == appleLibrary && !p.state.Authorized {
		return T("empty.sign_in_lib")
	}
	if c.searching {
		return T("empty.loading")
	}
	if c.catalogError != "" {
		return c.catalogError + " · r " + T("cat.refresh")
	}
	if p.drawer == appleLibrary && c.catalog.title == "" {
		return T("empty.not_loaded")
	}
	return T("empty.none")
}

func (p *AppleMusicPage) drawCatalogList(x, y, w, rows int, now time.Time) {
	if w < 8 || rows < 2 {
		return
	}
	if p.catalogAlbumCrate() {
		p.rackGraphical = false
		p.rackPickup.current, p.rackPickup.wanted = appleRackChoice{index: -1}, appleRackChoice{index: -1}
		p.rackPickup.pop = appleTween{}
		p.rackPickup.style = 0
		if p.drawAlbumCrate(x, y, w, rows, now) {
			return
		}
	}
	p.resetVinylPickup()
	p.drawList(x, y, w, rows, now)
}
