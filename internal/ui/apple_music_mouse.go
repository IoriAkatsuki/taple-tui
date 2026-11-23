package ui

import (
	"image"
	"image/color"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

type appleHitKind uint8

const (
	appleHitKey appleHitKind = iota
	appleHitTrack
	appleHitSeek
	appleHitVolume
	appleHitStage
	appleHitCassette
	appleHitMoving
	appleHitCatalog
)

type appleHit struct {
	kind       appleHitKind
	x, y, w, h int
	key        string
	index      int
	id         string
	cassette   deck.Rect
	pose       deck.Pose
	slot       image.Rectangle
}

func (p *AppleMusicPage) addHit(hit appleHit) {
	if hit.x < 0 || hit.y < 0 || hit.x >= p.screen.w || hit.y >= p.screen.h {
		return
	}
	hit.w = min(hit.w, p.screen.w-hit.x)
	hit.h = min(hit.h, p.screen.h-hit.y)
	if hit.w > 0 && hit.h > 0 {
		p.hits = append(p.hits, hit)
	}
}

func (p *AppleMusicPage) hitAt(x, y int) *appleHit {
	for i := len(p.hits) - 1; i >= 0; i-- {
		h := &p.hits[i]
		if x >= h.x && x < h.x+h.w && y >= h.y && y < h.y+h.h {
			return h
		}
	}
	return nil
}

func (p *AppleMusicPage) handleClick(x, y int) tea.Cmd {
	h := p.hitAt(x, y)
	if h == nil {
		return nil
	}
	if h.kind == appleHitKey {
		return p.handleClickKey(h.key)
	}
	if p.panel != "" {
		return nil
	}
	p.editing = false
	p.input.Blur()
	switch h.kind {
	case appleHitTrack, appleHitCassette:
		if h.index < 0 || h.index >= len(p.items()) || p.items()[h.index].ID != h.id {
			return nil
		}
		p.selected = h.index
		p.rackFocused = true
		return p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	case appleHitCatalog:
		if p.catalogContainers() {
			return p.openCatalog(h.index, h.id)
		}
	case appleHitStage:
		return p.handleKey(tea.KeyPressMsg{Code: 'z'})
	case appleHitSeek:
		if p.state.Track.Duration <= 0 || math.IsNaN(p.state.Track.Duration) || math.IsInf(p.state.Track.Duration, 0) {
			return nil
		}
		return p.dispatch(applemusic.Action{Kind: "seek", ID: p.state.Track.ID, Value: float64(x-h.x) / float64(max(1, h.w-1)) * p.state.Track.Duration})
	case appleHitVolume:
		return p.dispatch(applemusic.Action{Kind: "volume", Value: float64(x-h.x) / float64(max(1, h.w-1))})
	}
	return nil
}

func (p *AppleMusicPage) handlePanelClickKey(key string) tea.Cmd {
	switch key {
	case "l":
		return p.handleKey(tea.KeyPressMsg{Code: 'l'})
	case "esc":
		return p.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	case "b", "B", "y":
		return p.handleKey(tea.KeyPressMsg{Code: rune(key[0])})
	}
	return nil
}

func (p *AppleMusicPage) handleClickKey(key string) tea.Cmd {
	if p.panel != "" {
		return p.handlePanelClickKey(key)
	}
	switch key {
	case "esc":
		return p.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	case "/", "1":
		return p.editSearch(time.Now())
	case "s", "o":
		p.openPanel(time.Now())
		return nil
	case "b", "B":
		return p.openAccount(key == "B")
	case "search":
		if p.searching {
			return nil
		}
		p.editing = true
		return p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	}
	p.editing = false
	p.input.Blur()
	keys := map[string]rune{"space": tea.KeySpace, "enter": tea.KeyEnter, "tab": tea.KeyTab, "left": tea.KeyLeft, "right": tea.KeyRight, "backspace": tea.KeyBackspace}
	code, ok := keys[key]
	if !ok && len(key) == 1 {
		code, ok = rune(key[0]), true
	}
	if ok {
		return p.handleKey(tea.KeyPressMsg{Code: code})
	}
	return nil
}

func (p *AppleMusicPage) drawKeys(x, y, w int, keys []struct{ key, label string }) int {
	if len(keys) == 0 || w <= 0 {
		return 0
	}
	// Pick padded (▐ label ▌), compact capped (▐label▌), or uncapped layout across the row
	// so narrow widths (e.g. w=59 footer) never drop actionable buttons.
	sumW := 0
	for _, item := range keys {
		sumW += npWidth(item.label)
	}
	paddedNeed := sumW + len(keys)*4 + max(0, len(keys)-1)*2
	compactCapNeed := sumW + len(keys)*2 + max(0, len(keys)-1)
	capOverhead, gap, padded := 0, 2, false
	switch {
	case paddedNeed <= w:
		capOverhead, gap, padded = 4, 2, true
	case compactCapNeed <= w:
		capOverhead, gap, padded = 2, 1, false
	}

	var rowBg color.RGBA
	hasRowBg := false
	if c, ok := p.screen.cellAt(x, y); ok && c.HasBg {
		rowBg, hasRowBg = c.Bg, true
	}
	startX := x
	for _, item := range keys {
		width := npWidth(item.label) + capOverhead
		if width > w {
			break
		}
		hovered := p.hoverX >= x && p.hoverX < x+width && p.hoverY == y
		active := (len(item.key) == 1 && item.key[0] >= '1' && item.key[0] <= '3' && int(p.drawer) == int(item.key[0]-'1')) ||
			(item.key == "i" && p.scene == appleListen) || (item.key == "z" && p.zoomed)
		capCol := appleNeon.Rule
		pillBg := color.RGBA{0x22, 0x1c, 0x30, 0xff}
		keyFg := appleNeon.Accent2
		labelFg := appleNeon.Text
		labelBold := false
		switch item.key {
		case "space", "/":
			capCol = appleNeon.Hero
			pillBg = appleCozyPill
			keyFg = appleNeon.Hero
			labelBold = true
		case "b", "B", "y":
			capCol = appleNeon.Accent2
			pillBg = appleCozyPill
			keyFg = appleNeon.Accent2
			labelBold = true
		}
		if capOverhead == 0 {
			labelFg = appleNeon.Secondary
		}
		if active {
			capCol = appleNeon.Hero
			pillBg = color.RGBA{0x31, 0x25, 0x42, 0xff}
			keyFg = appleNeon.Hero
			labelFg = appleNeon.Text
			labelBold = true
		}
		if hovered {
			capCol = appleNeon.Accent2
			pillBg = appleCozyPillHover
			keyFg = appleNeon.Accent2
			labelFg = appleNeon.Text
			labelBold = true
		}

		bodyX, bodyW := x, width
		if capOverhead > 0 {
			p.screen.setRGBA(x, y, "▐", 1, capCol, rowBg, true, hasRowBg, false)
			p.screen.setRGBA(x+width-1, y, "▌", 1, capCol, rowBg, true, hasRowBg, false)
			bodyX, bodyW = x+1, width-2
			p.screen.fillRGBA(bodyX, y, bodyW, 1, pillBg)
			if padded {
				bodyX++
				bodyW -= 2
			}
		} else {
			p.screen.fillRGBA(bodyX, y, bodyW, 1, pillBg)
		}
		if idx := strings.IndexByte(item.label, ' '); idx > 0 {
			kw := p.screen.textRGBA(bodyX, y, item.label[:idx], keyFg, pillBg, true, true, true, bodyW)
			p.screen.textRGBA(bodyX+kw, y, item.label[idx:], labelFg, pillBg, true, true, labelBold, max(0, bodyW-kw))
		} else {
			p.screen.textRGBA(bodyX, y, item.label, appleNeon.Text, pillBg, true, true, true, bodyW)
		}
		p.addHit(appleHit{kind: appleHitKey, x: x, y: y, w: width, h: 1, key: item.key})
		step := width + gap
		x += step
		w -= step
	}
	return x - startX
}

func (p *AppleMusicPage) stagePointerOffset() (float64, float64) {
	r := p.stageRect
	if p.hoverX < r.Min.X || p.hoverX >= r.Max.X || p.hoverY < r.Min.Y || p.hoverY >= r.Max.Y || r.Dx() <= 2 || r.Dy() <= 2 {
		return 0, 0
	}
	cx := float64(r.Min.X) + float64(r.Dx())/2
	cy := float64(r.Min.Y) + float64(r.Dy())/2
	nx := min(max((float64(p.hoverX)-cx)/max(float64(r.Dx())/2, 1), -1), 1)
	ny := min(max((float64(p.hoverY)-cy)/max(float64(r.Dy())/2, 1), -1), 1)
	return nx, ny
}
