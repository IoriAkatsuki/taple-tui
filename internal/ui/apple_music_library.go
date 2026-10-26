package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

var libraryKinds = []string{applemusic.LibraryPlaylists, applemusic.LibraryAlbums, applemusic.LibrarySongs, applemusic.Recommendations, applemusic.PersonalStation}

func libraryTitle(kind string) string {
	switch kind {
	case applemusic.LibraryPlaylists:
		return T("lib.playlists")
	case applemusic.LibraryAlbums:
		return T("lib.albums")
	case applemusic.LibrarySongs:
		return T("lib.songs")
	case applemusic.Recommendations:
		return T("lib.made_for_you")
	case applemusic.PersonalStation:
		return T("lib.station")
	default:
		return ""
	}
}

func catalogItemLabel(kind string) string {
	switch kind {
	case "albums", "library-albums":
		return T("type.album")
	case "artists":
		return T("type.artist")
	case "playlists", "library-playlists":
		return T("type.playlist")
	case "stations":
		return T("type.station")
	case "personal-recommendation":
		return T("type.recommend")
	default:
		return ""
	}
}

func (p *AppleMusicPage) loadLibrary(kind string) tea.Cmd {
	p.libraryKind = kind
	if !p.state.Ready {
		p.status = T("status.wait_backend")
		return nil
	}
	if !p.state.Authorized {
		p.status = T("status.sign_in_hint")
		return nil
	}
	if p.state.Storefront == "" {
		p.status = T("status.wait_region")
		return nil
	}
	return p.requestCatalog(applemusic.CatalogRequest{Kind: kind}, libraryTitle(kind), false)
}

func (p *AppleMusicPage) ensureLibrary() tea.Cmd {
	if p.drawer == appleLibrary && p.library.lastRequest == nil {
		return p.loadLibrary(p.libraryKind)
	}
	return nil
}

func (p *AppleMusicPage) restoreCatalogCategory(drawer appleDrawer) {
	c := p.catalogState(drawer)
	kind := c.catalog.request.Kind
	if len(c.catalogHistory) > 0 {
		kind = c.catalogHistory[0].view.request.Kind
	}
	if drawer == appleLibrary {
		if libraryTitle(kind) != "" {
			p.libraryKind = kind
		}
		return
	}
	if drawer == appleSearch {
		switch kind {
		case applemusic.CatalogSongs, applemusic.CatalogAlbums, applemusic.CatalogArtists:
			p.searchKind = kind
		}
	}
}

func (p *AppleMusicPage) invalidateLibrary() {
	if p.library.lastRequest == nil && p.library.catalog.title == "" {
		return
	}
	p.library = appleCatalogState{searchGeneration: p.library.searchGeneration + 1}
	p.selections[appleLibrary], p.rackStarts[appleLibrary] = 0, 0
	if p.drawer == appleLibrary {
		p.selected, p.rackStart = 0, 0
		p.resetRackPickup()
		p.hits = nil
	}
}

func (p *AppleMusicPage) refreshCatalog() tea.Cmd {
	if p.drawer == appleQueue {
		return nil
	}
	c := p.catalogState(p.drawer)
	if c.searching {
		return nil
	}
	if c.lastRequest == nil {
		return p.ensureLibrary()
	}
	q := c.lastRequest.request
	q.Next = ""
	push := c.lastRequest.parent != nil && q != c.catalog.request
	q.Refresh = true
	return p.requestCatalog(q, c.lastRequest.title, push)
}

func (p *AppleMusicPage) playCatalogItem(item applemusic.CatalogItem) tea.Cmd {
	if !item.Playable {
		p.status = "此条目未提供可播放信息"
		return nil
	}
	kind := "play-collection"
	if item.Type == "stations" {
		kind = "play-station"
	}
	p.enterDetail(time.Now())
	return p.dispatch(applemusic.Action{Kind: kind, ID: item.ID, ResourceType: item.Type})
}

func (p *AppleMusicPage) playCatalogCollection() tea.Cmd {
	if p.drawer == appleQueue {
		return nil
	}
	c := p.catalogState(p.drawer)
	if c.searching {
		return nil
	}
	if p.catalogContainers() && p.selected >= 0 && p.selected < len(c.catalog.items) {
		item := c.catalog.items[p.selected]
		if item.Type == "albums" || item.Type == "library-albums" || item.Type == "playlists" || item.Type == "library-playlists" || item.Type == "stations" {
			return p.playCatalogItem(item)
		}
	}
	resource := appleCollectionResource(c.catalog.request.Kind)
	if resource != "" {
		return p.playCatalogItem(applemusic.CatalogItem{ID: c.catalog.request.ID, Type: resource, Playable: true})
	}
	p.status = "先选择或打开歌单/专辑"
	return nil
}

func appleCollectionResource(kind string) string {
	return map[string]string{applemusic.CatalogAlbumTracks: "albums", applemusic.CatalogPlaylistTracks: "playlists", applemusic.LibraryAlbumTracks: "library-albums", applemusic.LibraryPlaylistTracks: "library-playlists"}[kind]
}
