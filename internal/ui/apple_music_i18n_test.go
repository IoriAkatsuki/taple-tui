package ui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

func TestAppleMusicSettingsLanguageSwitch(t *testing.T) {
	SetLanguage("en")
	defer SetLanguage("en")

	p := NewAppleMusicPage(&appleMouseClient{})
	defer p.Close()

	// Open settings panel via 's'
	p.handleKey(tea.KeyPressMsg{Code: 's'})
	if p.panel == "" {
		t.Fatal("expected settings panel to be open")
	}

	frameEn := ansi.Strip(p.draw(140, 45, time.Unix(100, 0), 2))
	if !strings.Contains(frameEn, "English [en]") {
		t.Fatalf("expected English [en] in settings panel, got: %s", frameEn)
	}

	// Press 'l' to toggle to Chinese
	p.handleKey(tea.KeyPressMsg{Code: 'l'})
	if currentLang() != "zh" {
		t.Fatalf("expected language to switch to zh, got %s", currentLang())
	}

	frameZh := ansi.Strip(p.draw(140, 45, time.Unix(100, 0), 2))
	if !strings.Contains(frameZh, "简体中文 [zh]") {
		t.Fatalf("expected 简体中文 [zh] in settings panel, got: %s", frameZh)
	}
	if !strings.Contains(frameZh, "操作与设置") {
		t.Fatalf("expected 操作与设置 title in zh, got: %s", frameZh)
	}

	// Press 'tab' to toggle back to English
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyTab})
	if currentLang() != "en" {
		t.Fatalf("expected language to switch back to en, got %s", currentLang())
	}

	// Close panel with esc
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if p.panel != "" {
		t.Fatal("expected settings panel to be closed")
	}
}

func TestAppleMusicSignedInStatusI18n(t *testing.T) {
	SetLanguage("zh")
	defer SetLanguage("en")

	p := NewAppleMusicPage(&appleMouseClient{})
	defer p.Close()

	now := time.Now()
	// Test transition from unauthorized to authorized in zh
	p.acceptState(applemusic.State{Ready: true, Authorized: true}, now)
	if p.status != "已登录；浏览器在后台播放" {
		t.Fatalf("expected zh status '已登录；浏览器在后台播放', got: %q", p.status)
	}

	// Switch to en and re-accept state transition
	SetLanguage("en")
	p.state.Authorized = false
	p.acceptState(applemusic.State{Ready: true, Authorized: true}, now)
	if p.status != "Signed in; playing in background" {
		t.Fatalf("expected en status 'Signed in; playing in background', got: %q", p.status)
	}
}
