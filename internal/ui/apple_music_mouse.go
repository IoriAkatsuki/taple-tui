package ui

import (
	"image"
	"math"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
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

func (p *AppleMusicPage) drawKeys(x, y, w int, keys []struct{ key, label string }) {
	for _, item := range keys {
		width := npWidth(item.label)
		if width > w {
			break
		}
		hovered := p.hoverX >= x && p.hoverX < x+width && p.hoverY == y
		active := (len(item.key) == 1 && item.key[0] >= '1' && item.key[0] <= '3' && int(p.drawer) == int(item.key[0]-'1')) ||
			(item.key == "i" && p.scene == appleListen) || (item.key == "z" && p.zoomed)
		badgeSt := uv.Style{Fg: appleNeon.Text, Bg: appleNeon.Surface, Attrs: uv.AttrBold}
		labelSt := uv.Style{Fg: appleNeon.Secondary}
		if active {
			badgeSt.Fg = appleNeon.Accent
			labelSt.Fg, labelSt.Attrs = appleNeon.Text, uv.AttrBold
		}
		if hovered {
			badgeSt.Fg = appleNeon.Hero
			labelSt.Fg, labelSt.Bg = appleNeon.Hero, appleNeon.Surface
		}
		if idx := strings.IndexByte(item.label, ' '); idx > 0 {
			kw := p.screen.text(x, y, item.label[:idx], badgeSt, width)
			p.screen.text(x+kw, y, item.label[idx:], labelSt, max(0, width-kw))
		} else {
			p.screen.text(x, y, item.label, badgeSt, width)
		}
		p.addHit(appleHit{kind: appleHitKey, x: x, y: y, w: width, h: 1, key: item.key})
		x += width + 2
		w -= width + 2
	}
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
