package ui

import (
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/motion"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

type appleLyricsMsg struct {
	request    applemusic.LyricsRequest
	generation uint64
	data       applemusic.Lyrics
	err        error
}

type appleLyricRow struct {
	text               string
	line               int
	startFrac, endFrac float64
}

type appleLyricsState struct {
	request    applemusic.LyricsRequest
	generation uint64
	data       applemusic.Lyrics
	status     string
	manual     bool
	top        int
	scroll     appleTween
	scrollInit bool
	rows       []appleLyricRow
	wrapWidth  int
	wrapSource *applemusic.LyricLine
	wrapCount  int
	area       image.Rectangle
}

const appleLyricScrollDur = 260 * time.Millisecond

// The UI displays only the current song; the browser owns its session cache.
func (p *AppleMusicPage) loadLyrics(force bool) tea.Cmd {
	q := applemusic.LyricsRequest{ID: p.state.Track.ID, Storefront: p.state.Storefront}
	status := ""
	switch {
	case !p.state.Ready:
		status = "sdk_unavailable"
	case !p.state.Authorized:
		status = "login_required"
	case q.ID == "":
		status = "no_track"
	case q.Storefront == "":
		status = "storefront_loading"
	}
	if status != "" {
		if p.lyrics.request.ID != "" {
			p.lyrics = appleLyricsState{generation: p.lyrics.generation + 1}
		}
		p.lyrics.status = status
		return nil
	}
	if q == p.lyrics.request && (!force || p.lyrics.status == "loading") {
		return nil
	}
	p.lyrics = appleLyricsState{request: q, generation: p.lyrics.generation + 1, status: "loading"}
	generation := p.lyrics.generation
	return func() tea.Msg {
		request := q
		request.Refresh = force
		data, err := p.client.Lyrics(p.ctx, request)
		return appleLyricsMsg{request: q, generation: generation, data: data, err: err}
	}
}

func (p *AppleMusicPage) acceptLyrics(m appleLyricsMsg) {
	if m.generation != p.lyrics.generation || m.request != p.lyrics.request || m.request.ID != p.state.Track.ID || m.request.Storefront != p.state.Storefront || !p.state.Ready || !p.state.Authorized {
		return
	}
	if m.err != nil {
		p.lyrics.status = "request_error"
		return
	}
	if (m.data.Status == "available" || m.data.Status == "unsynced") && (m.data.TrackID != m.request.ID || m.data.Storefront != m.request.Storefront) {
		p.lyrics.status = "stale"
		return
	}
	p.lyrics.data, p.lyrics.status = m.data, m.data.Status
	p.lyrics.scrollInit = false
	p.lyrics.wrapSource = nil
}

func (p *AppleMusicPage) scrollLyrics(delta int) {
	if p.scene != appleListen || len(p.lyrics.rows) == 0 {
		return
	}
	p.lyrics.manual = true
	p.lyrics.top = min(max(0, p.lyrics.top+delta), max(0, len(p.lyrics.rows)-max(1, p.lyrics.area.Dy()-2)))
}

func (p *AppleMusicPage) updateLyricScroll(now time.Time) (int, float64) {
	target := float64(p.lyrics.top)
	if !p.lyrics.scrollInit {
		p.lyrics.scrollInit = true
		p.lyrics.scroll = appleTween{from: target, to: target}
		return p.lyrics.top, 0
	}
	if p.lyrics.scroll.to != target {
		dur := appleLyricScrollDur
		if p.level == motion.Subtle {
			dur /= 2
		}
		p.lyrics.scroll.start(target, now, dur, p.level)
	}
	val := p.lyrics.scroll.value(now)
	base := min(max(0, int(math.Round(val))), max(0, len(p.lyrics.rows)-1))
	return base, val - float64(base)
}

func (p *AppleMusicPage) lyricsPosition(now time.Time) (time.Duration, bool) {
	return appleDuration(p.state.Position), p.ready() && p.lyrics.data.Synced &&
		p.state.Position >= 0 && !math.IsNaN(p.state.Position) && !math.IsInf(p.state.Position, 0) &&
		p.lyrics.data.TrackID == p.state.Track.ID && p.lyrics.data.Storefront == p.state.Storefront &&
		!p.stateAt.IsZero() && !now.Before(p.stateAt) && now.Sub(p.stateAt) <= 2*time.Second
}

var appleLyricsKeys = map[string]string{
	"":                   "lyrics.auto_load",
	"no_track":           "lyrics.auto_load",
	"loading":            "lyrics.loading",
	"login_required":     "lyrics.sign_in",
	"storefront_loading": "lyrics.wait_region",
	"sdk_unavailable":    "lyrics.wait_conn",
	"no_lyrics":          "lyrics.none",
	"no_catalog_mapping": "lyrics.no_mapping",
	"unsupported_media":  "lyrics.unsupported",
	"unauthorized":       "lyrics.auth_expired",
	"access_denied":      "lyrics.denied",
	"rate_limited":       "lyrics.rate_limit",
	"unavailable":        "lyrics.unavailable",
	"timeout":            "lyrics.timeout",
	"stale":              "lyrics.stale",
	"request_error":      "lyrics.req_error",
	"parse_error":        "lyrics.parse_error",
}

func (p *AppleMusicPage) drawLyrics(r image.Rectangle, now time.Time, reveal float64) {
	if r.Empty() || reveal <= 0 {
		return
	}
	p.lyrics.area = r
	y, rows := r.Min.Y, r.Dy()
	position, sync := p.lyricsPosition(now)
	if rows >= 3 {
		p.drawLyricsChrome(r, y, sync, reveal)
		y++
		rows -= 2
	}
	if !p.lyricsHasText() {
		p.drawLyricsEmpty(r.Min.X, y, r.Dx(), rows, reveal)
		return
	}
	anchor := p.layoutLyricRows(r.Dx(), position, sync)
	p.lyrics.top = min(p.lyrics.top, max(0, len(p.lyrics.rows)-rows))
	baseTop, frac := p.updateLyricScroll(now)
	baseTop = min(baseTop, max(0, len(p.lyrics.rows)-rows))
	focusRow := baseTop + rows/2
	for i := baseTop; i < len(p.lyrics.rows) && i < baseTop+rows; i++ {
		if sync && p.lyrics.rows[i].line == anchor {
			focusRow = i
			break
		}
	}
	for i := 0; i < rows && baseTop+i < len(p.lyrics.rows); i++ {
		p.drawLyricRow(r.Min.X, y+i, r.Dx(), rows, i, baseTop+i, focusRow, frac, position, sync, reveal)
	}
}

func (p *AppleMusicPage) lyricsHasText() bool {
	data := &p.lyrics.data
	return len(data.Lines) > 0 && data.TrackID == p.state.Track.ID && data.Storefront == p.state.Storefront &&
		(p.lyrics.status == "available" || p.lyrics.status == "unsynced")
}

func (p *AppleMusicPage) lyricsLabel(now time.Time) string {
	if !p.lyricsHasText() {
		if key, ok := appleLyricsKeys[p.lyrics.status]; ok {
			return T(key)
		}
		return T("lyrics.unavailable")
	}
	if !p.lyrics.data.Synced {
		return T("lyrics.plain")
	}
	if p.state.Preview {
		return T("lyrics.preview")
	}
	if !p.ready() {
		return T("lyrics.unconfirmed")
	}
	if _, sync := p.lyricsPosition(now); !sync {
		return T("lyrics.clock_unavailable")
	}
	if p.lyrics.manual {
		return T("lyrics.manual")
	}
	if p.state.Paused {
		return T("lyrics.paused")
	}
	return T("lyrics.focus")
}

func (p *AppleMusicPage) drawLyricsChrome(r image.Rectangle, y int, sync bool, reveal float64) {
	p.screen.text(r.Min.X, y, p.lyricsLabel(p.lastFrame), uv.Style{Fg: npLerp(appleNeon.Bg, appleNeon.Secondary, reveal)}, r.Dx())
	keys := []struct{ key, label string }{}
	if p.lyricsHasText() {
		if !p.lyrics.data.Synced {
			keys = append(keys, struct{ key, label string }{"f", T("lyrics.btn_top")})
		} else if p.lyrics.manual && sync {
			keys = append(keys, struct{ key, label string }{"f", T("lyrics.btn_sync")})
		}
	}
	if p.lyrics.status != "loading" && p.state.Ready && p.state.Authorized && p.state.Track.ID != "" && p.state.Storefront != "" {
		keys = append(keys, struct{ key, label string }{"y", T("lyrics.btn_retry")})
	}
	p.drawKeys(r.Min.X, r.Max.Y-1, r.Dx(), keys)
}

func (p *AppleMusicPage) drawLyricsEmpty(x, y, w, rows int, reveal float64) {
	var label string
	if k, ok := appleLyricsKeys[p.lyrics.status]; ok {
		label = T(k)
	} else {
		label = T("lyrics.unavailable")
	}
	if p.lyrics.status == "parse_error" && p.lyrics.data.Detail != "" {
		label = T("lyrics.parse_detail", p.lyrics.data.Detail)
	}
	for i, text := range strings.Split(ansi.WrapWc(label, max(1, w), ""), "\n") {
		if i >= rows {
			break
		}
		p.screen.text(x, y+i, text, uv.Style{Fg: npLerp(appleNeon.Bg, appleNeon.Accent2, reveal)}, w)
	}
}

func (p *AppleMusicPage) layoutLyricRows(w int, position time.Duration, sync bool) int {
	anchor, latest := -1, time.Duration(-1)
	wrapW := max(1, w-4)
	var source *applemusic.LyricLine
	if len(p.lyrics.data.Lines) > 0 {
		source = &p.lyrics.data.Lines[0]
	}
	// Accepted lyric text is immutable; only width or a new response needs wrapping.
	rewrap := p.lyrics.wrapWidth != wrapW || p.lyrics.wrapSource != source || p.lyrics.wrapCount != len(p.lyrics.data.Lines)
	if rewrap {
		p.lyrics.rows = p.lyrics.rows[:0]
		p.lyrics.wrapWidth, p.lyrics.wrapSource, p.lyrics.wrapCount = wrapW, source, len(p.lyrics.data.Lines)
	}
	for i, line := range p.lyrics.data.Lines {
		if sync && line.Start <= position && line.Start >= latest {
			anchor, latest = i, line.Start
		}
		if !rewrap {
			continue
		}
		parts := strings.Split(ansi.WrapWc(npSanitize(line.Text), wrapW, ""), "\n")
		n := max(1, len(parts))
		for j, text := range parts {
			p.lyrics.rows = append(p.lyrics.rows, appleLyricRow{
				text: text, line: i,
				startFrac: float64(j) / float64(n), endFrac: float64(j+1) / float64(n),
			})
		}
	}
	if !p.lyrics.manual && sync {
		p.lyrics.top = 0
		rows := max(1, p.lyrics.area.Dy()-2)
		for i, row := range p.lyrics.rows {
			if row.line == anchor {
				p.lyrics.top = max(0, i-rows/2)
				break
			}
		}
	}
	return anchor
}

func (p *AppleMusicPage) drawLyricRow(x, y, w, rows, screenRow, idx, focusRow int, frac float64, position time.Duration, sync bool, reveal float64) {
	row := p.lyrics.rows[idx]
	line := p.lyrics.data.Lines[row.line]
	active := sync && line.Start <= position && position < line.End
	if active {
		p.drawLyricActiveRow(x, y, w, row, position, reveal)
		return
	}
	past := sync && line.End <= position
	dist := math.Abs(float64(idx-focusRow) - frac)
	rail, fg := "│", color.Color(appleLyricLight)
	railFg := npLerp(appleNeon.Ghost, appleNeon.Rule, max(0, 1-dist/5))
	if past {
		// Already-sung lines recede into muted dark violet.
		fade := min(max((dist-1)/4, 0), .75)
		fg = npLerp(appleNeon.Rule, appleNeon.Ghost, fade)
	} else {
		// Unplayed lines stay in a soft light lavender tier.
		fade := min(max((dist-1)/6, 0), .65)
		fg = npLerp(appleLyricLight, appleNeon.Secondary, fade)
	}
	if rows >= 7 && (screenRow == 0 || screenRow == rows-1) {
		fg = npLerp(fg, appleNeon.Ghost, .38)
	}
	if w >= 12 {
		p.screen.set(x, y, rail, 1, uv.Style{Fg: npLerp(appleNeon.Bg, railFg, reveal)})
		p.screen.text(x+1, y, "  "+row.text, uv.Style{Fg: npLerp(appleNeon.Bg, fg, reveal)}, w-1)
		return
	}
	p.screen.text(x, y, "  "+row.text, uv.Style{Fg: npLerp(appleNeon.Bg, fg, reveal)}, w)
}
