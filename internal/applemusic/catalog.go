package applemusic

import (
	"context"
	"errors"
	"log/slog"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	CatalogSongs           = "songs"
	CatalogAlbums          = "albums"
	CatalogArtists         = "artists"
	CatalogAlbumTracks     = "album-tracks"
	CatalogArtistAlbums    = "artist-albums"
	CatalogPlaylistTracks  = "playlist-tracks"
	LibraryPlaylists       = "library-playlists"
	LibraryAlbums          = "library-albums"
	LibrarySongs           = "library-songs"
	LibraryPlaylistTracks  = "library-playlist-tracks"
	LibraryAlbumTracks     = "library-album-tracks"
	Recommendations        = "recommendations"
	RecommendationContents = "recommendation-contents"
	PersonalStation        = "personal-station"
)

type CatalogRequest struct {
	Kind    string `json:"kind"`
	Query   string `json:"query,omitempty"`
	ID      string `json:"id,omitempty"`
	Next    string `json:"next,omitempty"`
	Refresh bool   `json:"refresh,omitempty"`
}

type CatalogItem struct {
	Type      string  `json:"type"`
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Artist    string  `json:"artist"`
	Album     string  `json:"album"`
	Duration  float64 `json:"duration"`
	Playable  bool    `json:"playable"`
	CatalogID string  `json:"catalogId"`
}

type CatalogPage struct {
	Items []CatalogItem `json:"items"`
	Next  string        `json:"next"`
}

func (b *Browser) Catalog(ctx context.Context, q CatalogRequest) (CatalogPage, error) {
	switch q.Kind {
	case CatalogSongs, CatalogAlbums, CatalogArtists:
		q.Query = strings.TrimSpace(q.Query)
		if q.Query == "" || len(q.Query) > 512 || !utf8.ValidString(q.Query) {
			return CatalogPage{}, errors.New("请输入搜索词（最多 512 字节）")
		}
	case CatalogAlbumTracks, CatalogArtistAlbums, CatalogPlaylistTracks, LibraryPlaylistTracks, LibraryAlbumTracks, RecommendationContents:
		if q.ID == "" || len(q.ID) > 256 || !utf8.ValidString(q.ID) || strings.IndexFunc(q.ID, unicode.IsControl) >= 0 {
			return CatalogPage{}, errors.New("无效的 Apple Music 曲库条目")
		}
	case LibraryPlaylists, LibraryAlbums, LibrarySongs, Recommendations, PersonalStation:
	default:
		return CatalogPage{}, errors.New("无效的 Apple Music 曲库请求")
	}
	if q.Next != "" {
		u, err := url.Parse(q.Next)
		if err != nil {
			return CatalogPage{}, errors.New("无效的 Apple Music 曲库分页")
		}
		personalPath := map[string]string{LibraryPlaylists: "/v1/me/library/playlists", LibraryAlbums: "/v1/me/library/albums", LibrarySongs: "/v1/me/library/songs",
			LibraryPlaylistTracks: "/v1/me/library/playlists/" + q.ID + "/tracks", LibraryAlbumTracks: "/v1/me/library/albums/" + q.ID + "/tracks",
			Recommendations: "/v1/me/recommendations", RecommendationContents: "/v1/me/recommendations/" + q.ID + "/contents"}[q.Kind]
		allowed := strings.HasPrefix(q.Next, "/v1/catalog/")
		if personalPath != "" {
			allowed = u.Path == personalPath
		}
		if err != nil || !allowed || u.Scheme != "" || u.Host != "" || u.User != nil ||
			strings.ContainsAny(q.Next, "\\#") || strings.IndexFunc(q.Next, unicode.IsControl) >= 0 {
			return CatalogPage{}, errors.New("无效的 Apple Music 曲库分页")
		}
		// Keep encoded ID separators opaque while rejecting URL dot-segment traversal.
		pathname := strings.ReplaceAll(strings.ToLower(u.EscapedPath()), "%2e", ".")
		if path.Clean(pathname) != pathname {
			return CatalogPage{}, errors.New("无效的 Apple Music 曲库分页")
		}
		if _, err := url.ParseQuery(u.RawQuery); err != nil {
			return CatalogPage{}, errors.New("无效的 Apple Music 曲库分页")
		}
	}
	b.lifecycle.RLock()
	defer b.lifecycle.RUnlock()
	if b.stopped {
		return CatalogPage{}, errors.New("播放器已关闭")
	}
	var page CatalogPage
	started := time.Now()
	err := b.evaluate(ctx, struct {
		Op string `json:"op"`
		CatalogRequest
	}{"catalog", q}, &page)
	slog.Info("apple_music_catalog", "kind", q.Kind, "elapsed_ms", time.Since(started).Milliseconds(), "refresh", q.Refresh, "items", len(page.Items), "ok", err == nil)
	return page, err
}
