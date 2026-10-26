package ui

// These navigation tests intentionally inspect private scene state and rendered
// hit regions; exporting this tightly coupled UI state would widen the API.
import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/motion"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

func appleNavigationPage(t *testing.T) *AppleMusicPage {
	t.Helper()
	p := NewAppleMusicPage(&appleMouseClient{})
	t.Cleanup(func() { _ = p.Close() })
	p.level = motion.Off
	p.state = applemusic.State{Ready: true, Authorized: true, Storefront: "us"}
	p.input.SetValue("navigation-query")
	return p
}

func appleNavigationHit(t *testing.T, p *AppleMusicPage, key string) appleHit {
	t.Helper()
	for _, h := range p.hits {
		if h.kind == appleHitKey && h.key == key {
			return h
		}
	}
	t.Fatalf("missing rendered key %q", key)
	return appleHit{}
}

func appleSearchEntry(t *testing.T, p *AppleMusicPage, entry string) {
	t.Helper()
	switch entry {
	case "slash":
		p.handleKey(tea.KeyPressMsg{Code: '/'})
	case "one":
		p.handleKey(tea.KeyPressMsg{Code: '1'})
	default:
		key := "1"
		if entry == "footer" {
			key = "/"
		}
		h := appleNavigationHit(t, p, key)
		p.handleClick(h.x, h.y)
	}
}

func TestAppleMusicSearchEntryParity(t *testing.T) {
	for _, drawer := range []appleDrawer{appleSearch, appleLibrary, appleQueue} {
		for _, scene := range []appleScene{appleBrowse, appleListen} {
			t.Run(fmt.Sprintf("drawer=%d/scene=%d", drawer, scene), func(t *testing.T) {
				for _, zoom := range []bool{false, true} {
					for _, entry := range []string{"slash", "one", "top", "footer"} {
						t.Run(fmt.Sprintf("zoom=%t/%s", zoom, entry), func(t *testing.T) {
							p := appleNavigationPage(t)
							p.drawer, p.scene, p.zoomed = drawer, scene, zoom
							p.sceneMotion = appleTween{from: float64(scene), to: float64(scene)}
							if zoom {
								p.camera = appleTween{from: 1, to: 1}
							}
							p.draw(80, 30, time.Now(), 2)
							appleSearchEntry(t, p, entry)
							if p.scene != appleBrowse || p.drawer != appleSearch || !p.editing || !p.input.Focused() || p.zoomed || p.restoreZoom || p.autoZoomAfterFold {
								t.Fatalf("search intent: scene=%d drawer=%d editing=%t focused=%t zoom=%t restore=%t auto=%t", p.scene, p.drawer, p.editing, p.input.Focused(), p.zoomed, p.restoreZoom, p.autoZoomAfterFold)
							}
							c := p.client.(*appleMouseClient)
							if p.searching || p.library.searching || p.busy || len(p.operations) != 0 || c.searches != 0 || len(c.actions) != 0 {
								t.Fatal("navigation initiated a catalog or playback operation")
							}
						})
					}
				}
			})
		}
	}
}

func TestAppleMusicSearchNavigationShowsInput(t *testing.T) {
	for _, width := range []int{80, 99, 100} {
		for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
			t.Run(fmt.Sprintf("width=%d/level=%d", width, level), func(t *testing.T) {
				for _, scene := range []appleScene{appleBrowse, appleListen} {
					for _, entry := range []string{"slash", "one", "top", "footer"} {
						t.Run(fmt.Sprintf("scene=%d/%s", scene, entry), func(t *testing.T) {
							p := appleNavigationPage(t)
							p.level, p.drawer, p.scene, p.zoomed = level, appleLibrary, scene, true
							p.camera = appleTween{from: 1, to: 1}
							p.sceneMotion = appleTween{from: float64(scene), to: float64(scene)}
							p.draw(width, 30, time.Now(), 2)
							appleSearchEntry(t, p, entry)
							now := time.Now()
							for _, elapsed := range []time.Duration{0, 80 * time.Millisecond} {
								frame := ansi.Strip(p.draw(width, 30, now.Add(elapsed), 2))
								if !p.editing || !p.input.Focused() || !strings.Contains(frame, "navigation-query") {
									t.Fatalf("search input hidden at %v: editing=%t focused=%t\n%s", elapsed, p.editing, p.input.Focused(), frame)
								}
								appleNavigationHit(t, p, "search")
							}
						})
					}
				}
			})
		}
	}
}

func TestAppleMusicInternalDrawerSwitchDoesNotFocusSearch(t *testing.T) {
	p := appleNavigationPage(t)
	p.drawer = appleQueue
	p.switchDrawer(appleSearch, time.Now())
	if p.editing || p.input.Focused() {
		t.Fatal("internal drawer switch unexpectedly focused the search input")
	}
	p.drawer = appleQueue
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyTab})
	if p.drawer != appleSearch || p.editing || p.input.Focused() {
		t.Fatal("Tab did not change drawers without entering search editing")
	}
}

func TestAppleMusicMouseBackExitsEditing(t *testing.T) {
	p := appleNavigationPage(t)
	p.zoomed = true
	p.camera = appleTween{from: 1, to: 1}
	p.editing = true
	p.input.Focus()
	p.draw(100, 30, time.Now(), 2)
	h := appleNavigationHit(t, p, "esc")
	p.handleClick(h.x, h.y)
	if p.editing || p.input.Focused() || !p.zoomed {
		t.Fatalf("mouse Esc skipped the editing layer: editing=%t focused=%t zoom=%t", p.editing, p.input.Focused(), p.zoomed)
	}
}

func TestAppleMusicPanelMouseLanguageSwitch(t *testing.T) {
	lang := currentLang()
	t.Cleanup(func() { SetLanguage(lang) })
	SetLanguage("en")
	p := appleNavigationPage(t)
	p.handleKey(tea.KeyPressMsg{Code: 's'})
	p.draw(100, 30, time.Now(), 2)
	h := appleNavigationHit(t, p, "l")
	p.handleClick(h.x, h.y)
	if currentLang() != "zh" || p.panel != T("panel.title") {
		t.Fatalf("panel language button did not toggle: lang=%s panel=%q", currentLang(), p.panel)
	}
}

func TestAppleMusicPanelPreservesEditing(t *testing.T) {
	lang := currentLang()
	t.Cleanup(func() { SetLanguage(lang) })
	p := appleNavigationPage(t)
	p.editing = true
	p.input.Focus()
	p.handleKey(tea.KeyPressMsg{Code: 's', Text: "s"})
	if p.panel != "" || p.input.Value() != "navigation-querys" {
		t.Fatal("typing s while editing opened settings instead of entering text")
	}
	p.draw(100, 30, time.Now(), 2)
	h := appleNavigationHit(t, p, "s")
	p.handleClick(h.x, h.y)
	if p.panel == "" || !p.editing || !p.input.Focused() {
		t.Fatal("opening settings discarded search editing or focus")
	}
	p.draw(100, 30, time.Now(), 2)
	h = appleNavigationHit(t, p, "l")
	p.handleClick(h.x, h.y)
	if !p.editing || !p.input.Focused() {
		t.Fatal("changing panel language discarded input focus")
	}
	p.draw(100, 30, time.Now(), 2)
	h = appleNavigationHit(t, p, "esc")
	p.handleClick(h.x, h.y)
	if p.panel != "" || !p.editing || !p.input.Focused() || p.input.Value() != "navigation-querys" {
		t.Fatal("closing settings did not restore editing context")
	}
}

func TestAppleMusicPanelRejectsStaleHits(t *testing.T) {
	lang := currentLang()
	t.Cleanup(func() { SetLanguage(lang) })
	p := appleNavigationPage(t)
	p.catalog = appleCatalogView{request: applemusic.CatalogRequest{Kind: applemusic.CatalogSongs}, next: "next-page"}
	p.draw(100, 30, time.Now(), 2)
	h := appleNavigationHit(t, p, "l")
	p.handleKey(tea.KeyPressMsg{Code: 's'})
	if len(p.hits) != 0 {
		t.Fatal("opening settings retained background hits before the next frame")
	}
	p.handleClick(h.x, h.y)
	if currentLang() != lang || p.searching {
		t.Fatal("stale background load-more hit was handled as a panel action")
	}
}

func TestAppleMusicPanelCloseInvalidatesHits(t *testing.T) {
	for _, mouse := range []bool{false, true} {
		t.Run(fmt.Sprintf("mouse=%t", mouse), func(t *testing.T) {
			p := appleNavigationPage(t)
			p.editing = true
			p.input.Focus()
			p.draw(100, 30, time.Now(), 2)
			h := appleNavigationHit(t, p, "s")
			p.handleClick(h.x, h.y)
			p.draw(100, 30, time.Now(), 2)
			h = appleNavigationHit(t, p, "esc")
			if mouse {
				p.handleClick(h.x, h.y)
			} else {
				p.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
			}
			if len(p.hits) != 0 {
				t.Fatal("closing settings retained panel hits before the next frame")
			}
			p.handleClick(h.x, h.y)
			if p.panel != "" || !p.editing || !p.input.Focused() {
				t.Fatal("stale panel close hit escaped the search editing layer")
			}
		})
	}
}

func TestAppleMusicPlainLyricsReturnToTop(t *testing.T) {
	for _, synced := range []bool{false, true} {
		t.Run(fmt.Sprintf("synced=%t", synced), func(t *testing.T) {
			p := appleNavigationPage(t)
			now := time.Unix(100, 0)
			p.scene, p.sceneMotion.to = appleListen, 1
			p.state.Full, p.state.Paused = true, true
			p.state.Track.ID, p.state.Position = "current", 20
			p.stateAt, p.lastFrame = now, now
			p.lyrics.status = "unsynced"
			if synced {
				p.lyrics.status = "available"
			}
			p.lyrics.data = applemusic.Lyrics{TrackID: "current", Storefront: "us", Synced: synced}
			for i := range 30 {
				p.lyrics.data.Lines = append(p.lyrics.data.Lines, applemusic.LyricLine{Text: fmt.Sprintf("Lyric %d", i), Start: time.Duration(i) * time.Second, End: time.Duration(i+1) * time.Second})
			}
			p.lyrics.manual, p.lyrics.top = true, 10
			p.screen.reset(80, 30)
			area := appleBox(2, 3, 76, 10)
			p.drawLyrics(area, now, 1)
			p.handleKey(tea.KeyPressMsg{Code: 'f'})
			p.drawLyrics(area, now, 1)
			if p.lyrics.manual {
				t.Fatal("f did not restore automatic lyric following")
			}
			if !synced {
				if p.lyrics.top != 0 {
					t.Fatalf("plain lyrics: want top=0, got top=%d", p.lyrics.top)
				}
				return
			}
			focus := p.lyrics.rows[p.lyrics.top+(area.Dy()-2)/2]
			line := p.lyrics.data.Lines[focus.line]
			position := appleDuration(p.state.Position)
			if position < line.Start || position >= line.End {
				t.Fatalf("follow target is not the real current line: position=%v line=%d [%v,%v)", position, focus.line, line.Start, line.End)
			}
		})
	}
}
