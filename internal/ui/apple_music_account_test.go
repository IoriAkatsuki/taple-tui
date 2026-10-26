package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/motion"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

type appleLogoutClient struct {
	appleMouseClient
	err error
}

func (c *appleLogoutClient) Do(_ context.Context, action applemusic.Action) error {
	c.actions = append(c.actions, action)
	return c.err
}

func TestAppleMusicAccountRequiresExplicitConfirmation(t *testing.T) {
	for _, cancelKey := range []rune{tea.KeyEscape, tea.KeyEnter, 'n'} {
		p, c := boundaryTestPage(t, 0, 2)
		if cmd := p.handleKey(tea.KeyPressMsg{Code: 'b'}); cmd != nil || p.panel == "" {
			t.Fatal("account click must open confirmation, not operate the browser")
		}
		if cmd := p.handleKey(tea.KeyPressMsg{Code: cancelKey}); cmd != nil || p.panel != "" {
			t.Fatal("cancel/default Enter must dismiss without clearing")
		}
		if len(c.actions) != 0 || !p.state.Authorized || p.bayID != "old" {
			t.Fatal("cancel changed the session or tape")
		}
	}
}

func TestAppleMusicLogoutInvalidatesOldDataAndRequiresLogin(t *testing.T) {
	p, c := boundaryTestPage(t, 0, 2)
	p.level = motion.Off
	p.library.lastRequest = &appleCatalogMsg{}
	p.library.tracks = []applemusic.Track{{ID: "private"}}
	p.lyrics.data.Lines = []applemusic.LyricLine{{Text: "private lyrics"}}
	p.beginInsert(applemusic.Track{ID: "old"}, time.Now())
	oldState := p.poll()()
	oldCatalog := appleCatalogMsg{drawer: appleLibrary, generation: p.library.searchGeneration, page: applemusic.CatalogPage{Items: []applemusic.CatalogItem{{ID: "private", Type: "library-songs"}}}}
	p.handleKey(tea.KeyPressMsg{Code: 'b'})
	cmd := p.handleKey(tea.KeyPressMsg{Code: 'y'})
	if cmd == nil || len(c.actions) != 0 {
		t.Fatal("logout must run asynchronously")
	}
	p.Update(oldState, nil)
	p.Update(oldCatalog, nil)
	if p.state.Authorized || len(p.library.tracks) != 0 || len(p.lyrics.data.Lines) != 0 || p.bayID != "" || p.inserting {
		t.Fatal("old account survived clearing intent")
	}
	if cmd := p.handleKey(tea.KeyPressMsg{Code: 'n'}); cmd != nil {
		t.Fatal("playback allowed while clearing")
	}
	p.Update(cmd(), nil)
	if len(c.actions) != 1 || c.actions[0].Kind != "logout" || p.panel != "" {
		t.Fatal("confirmed local logout was not performed")
	}
	p.Update(oldState, nil)
	if p.state.Authorized {
		t.Fatal("old snapshot restored a cleared account")
	}
	if cmd := p.handleKey(tea.KeyPressMsg{Code: 'b'}); cmd == nil {
		t.Fatal("sign-in entry disappeared")
	} else {
		cmd()
	}
	if c.shown != 1 {
		t.Fatal("re-login did not open the browser")
	}
	p.acceptState(applemusic.State{Ready: true, Authorized: true, Storefront: "us"}, time.Now())
	p.Update(oldCatalog, nil)
	if len(p.library.tracks) != 0 {
		t.Fatal("same-region re-login accepted a previous account's catalog")
	}
}

func TestAppleMusicLogoutDropsUnsentControlsButWaitsForTheActiveOne(t *testing.T) {
	p, c := boundaryTestPage(t, 0, 2)
	first := p.dispatch(applemusic.Action{Kind: "play", ID: "first"})
	p.dispatch(applemusic.Action{Kind: "play", ID: "unsent"})
	p.dispatch(applemusic.Action{Kind: "enqueue", ID: "unsent-too"})
	p.handleKey(tea.KeyPressMsg{Code: 'b'})
	if cmd := p.handleKey(tea.KeyPressMsg{Code: 'y'}); cmd != nil {
		t.Fatal("logout bypassed the active command")
	}
	_, next := p.Update(first(), model.NewApp(model.DefaultOptions()))
	if next == nil {
		t.Fatal("logout did not follow the active command")
	}
	p.Update(next(), model.NewApp(model.DefaultOptions()))
	if len(c.actions) != 2 || c.actions[1].Kind != "logout" {
		t.Fatalf("old controls escaped into the new session: %+v", c.actions)
	}
}

func TestAppleMusicFailedClearDoesNotClaimSuccessOrRetry(t *testing.T) {
	c := &appleLogoutClient{err: errors.New("synthetic cleanup failed")}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.state = applemusic.State{Ready: true, Authorized: true}
	p.handleKey(tea.KeyPressMsg{Code: 'b'})
	cmd := p.handleKey(tea.KeyPressMsg{Code: 'y'})
	if cmd == nil {
		t.Fatal("missing clear operation")
	}
	p.Update(cmd(), model.NewApp(model.DefaultOptions()))
	if !strings.Contains(p.status, "synthetic cleanup failed") || p.state.Authorized || len(c.actions) != 1 {
		t.Fatal("clear failure was hidden or retried")
	}
}

func TestAppleMusicLogoutConfirmationFramebuffer(t *testing.T) {
	defer SetLanguage(currentLang())
	for _, lang := range []string{"en", "zh"} {
		SetLanguage(lang)
		for _, size := range [][2]int{{140, 45}, {120, 24}, {80, 30}, {50, 20}, {40, 12}} {
			p, _ := boundaryTestPage(t, 0, 2)
			p.level = motion.Off
			p.handleKey(tea.KeyPressMsg{Code: 'b'})
			frame := p.draw(size[0], size[1], time.Now(), 2)
			if !strings.Contains(ansi.Strip(frame), T("account.clear_title")) {
				t.Fatal("missing clear-session warning")
			}
			for _, line := range strings.Split(frame, "\n") {
				if ansi.StringWidth(line) > size[0] {
					t.Fatal("confirmation overflows viewport")
				}
			}
			for _, hit := range p.hits {
				if hit.kind != appleHitKey || (hit.key != "y" && hit.key != "esc") {
					t.Fatal("confirmation leaks background targets")
				}
			}
			exportPresentation(t, p, fmt.Sprintf("account-clear-%s-%dx%d", lang, size[0], size[1]))
			h := appleNavigationHit(t, p, "esc")
			p.handleClick(h.x, h.y)
			if p.panel != "" {
				t.Fatal("mouse cancel failed")
			}
		}
	}
}

func TestAppleMusicLogoutMouseConfirmationAndSettings(t *testing.T) {
	p, c := boundaryTestPage(t, 0, 2)
	p.level = motion.Off
	p.handleKey(tea.KeyPressMsg{Code: 's'})
	p.draw(100, 30, time.Now(), 2)
	h := appleNavigationHit(t, p, "B")
	p.handleClick(h.x, h.y)
	p.draw(100, 30, time.Now(), 2)
	h = appleNavigationHit(t, p, "y")
	cmd := p.handleClick(h.x, h.y)
	if cmd == nil || len(c.actions) != 0 {
		t.Fatal("mouse confirmation did not create an async clear operation")
	}
	if again := p.handleClick(h.x, h.y); again != nil {
		t.Fatal("stale confirmation hit submitted a second clear")
	}
	p.draw(100, 30, time.Now(), 2)
	if len(p.hits) != 0 {
		t.Fatal("clearing panel exposes clickable controls")
	}
	exportPresentation(t, p, "account-clearing-100x30")
	p.Update(cmd(), nil)
	if len(c.actions) != 1 || c.actions[0].Kind != "logout" {
		t.Fatal("mouse clear did not execute exactly once")
	}
	p.draw(100, 30, time.Now(), 2)
	exportPresentation(t, p, "account-cleared-100x30")
}

func TestAppleMusicTinyLogoutPanelCannotConfirmInvisibleWarning(t *testing.T) {
	p, c := boundaryTestPage(t, 0, 2)
	p.level = motion.Off
	p.handleKey(tea.KeyPressMsg{Code: 'b'})
	p.draw(30, 8, time.Now(), 2)
	if cmd := p.handleKey(tea.KeyPressMsg{Code: 'y'}); cmd != nil {
		t.Fatal("tiny viewport accepted an invisible warning")
	}
	for _, hit := range p.hits {
		if hit.key == "y" {
			t.Fatal("tiny viewport retained destructive click target")
		}
	}
	if len(c.actions) != 0 {
		t.Fatal("tiny warning cleared the browser")
	}
}

func TestAppleMusicAccountMouseCancelPreservesSearchEditing(t *testing.T) {
	p, _ := boundaryTestPage(t, 0, 2)
	p.level = motion.Off
	p.editing = true
	p.input.SetValue("keep this query")
	p.input.Focus()
	p.draw(80, 30, time.Now(), 2)
	h := appleNavigationHit(t, p, "b")
	p.handleClick(h.x, h.y)
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !p.editing || !p.input.Focused() || p.input.Value() != "keep this query" {
		t.Fatal("cancel discarded the previous editor context")
	}
	p.openPanel(time.Now())
	p.draw(50, 20, time.Now(), 2)
	appleNavigationHit(t, p, "esc")
}
