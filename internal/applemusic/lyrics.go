package applemusic

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

type LyricsRequest struct {
	ID         string `json:"id"`
	Storefront string `json:"storefront"`
	Refresh    bool   `json:"refresh,omitempty"`
}

type LyricLine struct {
	Text       string
	Start, End time.Duration
}

type Lyrics struct {
	TrackID, CatalogID, Storefront   string
	Source, Status, Language, Detail string
	Synced                           bool
	Lines                            []LyricLine
}

func (b *Browser) Lyrics(ctx context.Context, q LyricsRequest) (Lyrics, error) {
	result := Lyrics{TrackID: q.ID, Storefront: q.Storefront, Source: "apple-web-undocumented"}
	for _, value := range []string{q.ID, q.Storefront} {
		if value == "" || len(value) > 256 || !utf8.ValidString(value) || strings.IndexFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 || value == "." || value == ".." {
			return result, errors.New("无效的歌词曲目或地区")
		}
	}
	b.lifecycle.RLock()
	defer b.lifecycle.RUnlock()
	if b.stopped {
		return result, errors.New("播放器已关闭")
	}
	var raw struct{ Status, TrackID, CatalogID, Storefront, TTML, Cache string }
	started := time.Now()
	err := b.evaluate(ctx, struct {
		Op string `json:"op"`
		LyricsRequest
	}{"lyrics", q}, &raw)
	slog.Info("apple_music_lyrics", "elapsed_ms", time.Since(started).Milliseconds(), "cache", raw.Cache, "status", raw.Status, "ok", err == nil)
	if err != nil {
		return result, err
	}
	result.Status, result.CatalogID = raw.Status, raw.CatalogID
	if raw.Status != "available" {
		return result, nil
	}
	if raw.TrackID != q.ID || raw.Storefront != q.Storefront {
		result.Status = "stale"
		return result, nil
	}
	parsed, err := parseLyricsTTML(raw.TTML)
	if err != nil {
		result.Status = "parse_error"
		result.Detail = err.Error()
		return result, nil
	}
	result.Language, result.Synced, result.Lines = parsed.Language, parsed.Synced, parsed.Lines
	if len(result.Lines) == 0 {
		result.Status = "unavailable"
	} else if !result.Synced {
		result.Status = "unsynced"
	}
	return result, nil
}
