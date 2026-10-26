package ui

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

const appleLogoutPanel = "clear_local_login"

func (p *AppleMusicPage) openAccount(clear bool) tea.Cmd {
	if clear || p.state.Authorized {
		p.openPanel(time.Now())
		p.panel = appleLogoutPanel
		return nil
	}
	p.panel, p.hits = "", nil
	return p.dispatch(applemusic.Action{Kind: "login"})
}

func (p *AppleMusicPage) confirmLogout() tea.Cmd {
	if p.loggingOut {
		return nil
	}
	p.loggingOut = true
	// Invalidate every old reply before the active control releases the serial queue.
	p.pollEpoch++
	p.playGeneration = p.generation + 1
	p.operations = nil
	p.appleCatalogState = appleCatalogState{searchGeneration: p.searchGeneration + 1}
	p.library = appleCatalogState{searchGeneration: p.library.searchGeneration + 1}
	p.lyrics = appleLyricsState{generation: p.lyrics.generation + 1}
	p.state, p.stateAt = applemusic.State{QueueIndex: -1}, time.Time{}
	p.pending, p.pendingID, p.pendingCatalogID, p.previousID = false, "", "", ""
	p.lastAction, p.pendingIndex = applemusic.Action{}, -1
	p.scene, p.drawer = appleBrowse, appleSearch
	p.selected, p.rackStart = 0, 0
	p.selections, p.rackStarts = [3]int{}, [3]int{}
	p.input.SetValue("")
	p.input.Blur()
	p.editing = false
	p.resetRackPickup()
	p.camera, p.sceneMotion, p.drawerTween = appleTween{}, appleTween{}, appleTween{}
	p.zoomed, p.restoreZoom, p.autoZoomAfterFold = false, false, false
	p.inserting, p.insertOldPresent, p.insertSourceCaptured = false, false, false
	p.insertID, p.bayID = "", ""
	p.insertStyle, p.bayStyle, p.insertNumber, p.bayNumber = 0, 0, 0, 0
	p.insertStart, p.insertEjectAt, p.swapStart, p.glitchAt = time.Time{}, time.Time{}, time.Time{}, time.Time{}
	p.insertOld, p.swap, p.reels = deck.WalkmanState{}, appleSwapState{}, deck.Reels{}
	p.styles = make(map[string]uint8)
	p.hits = nil
	p.status = T("account.clearing")
	return p.dispatch(applemusic.Action{Kind: "logout"})
}

func (p *AppleMusicPage) drawLogoutPanel() {
	if p.screen.w < 40 || p.screen.h < 12 {
		p.screen.fill(0, 0, p.screen.w, p.screen.h, appleNeon.Surface)
		p.screen.text(0, 0, T("account.clear_title"), uv.Style{Fg: appleNeon.Accent2}, p.screen.w)
		p.screen.text(0, 1, T("account.resize"), uv.Style{Fg: appleNeon.Text}, p.screen.w)
		if !p.loggingOut {
			p.drawKeys(0, max(0, p.screen.h-1), p.screen.w, []struct{ key, label string }{{"esc", T("account.cancel")}})
		}
		return
	}
	w, h := min(p.screen.w-4, 66), min(p.screen.h-2, 13)
	x, y := (p.screen.w-w)/2, (p.screen.h-h)/2
	p.screen.fill(x, y, w, h, appleNeon.Surface)
	appleBezel(&p.screen, x, y, w, h, T("account.clear_title"))
	body := T("account.clear_body")
	if p.loggingOut {
		body = T("account.clearing_body")
	}
	for i, line := range strings.Split(ansi.WrapWc(body, w-4, ""), "\n") {
		if i >= h-4 {
			break
		}
		p.screen.text(x+2, y+2+i, line, uv.Style{Fg: appleNeon.Text}, w-4)
	}
	if !p.loggingOut {
		p.drawKeys(x+2, y+h-2, w-4, []struct{ key, label string }{{"esc", T("account.cancel")}, {"y", T("account.confirm")}})
	}
}
