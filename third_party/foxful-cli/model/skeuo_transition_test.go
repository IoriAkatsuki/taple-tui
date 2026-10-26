package model

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/motion"
	"github.com/charmbracelet/x/ansi"
)

func TestFrameTransitionFallbackAndFullFrame(t *testing.T) {
	a := newOverlayApp(t, motion.Full)
	from, to := &framePage{text: "FROM"}, &framePage{text: "TO"}
	a.page = from
	a.View()
	called := 0
	a.options.PageTransition = func(a *App, page Page, tx PageTransition) bool {
		called++
		if page != to || tx.From != from {
			t.Fatal("wrong fallback page")
		}
		a.StartFrameTransition(time.Second, func(old, next string, elapsed time.Duration, w, h int) string {
			if !strings.Contains(old, "FROM") || !strings.Contains(next, "TO") || !strings.Contains(next, "NOTIFICATION") || w != 60 || h != 16 {
				t.Fatal("custom transition did not receive complete frames")
			}
			return "CUSTOM"
		})
		return true
	}
	a.page = to
	a.handleShowNotification(NotificationSpec{Message: "NOTIFICATION"})
	a.notifications[0].createdAt = time.Now().Add(-time.Second)
	if got := a.View().Content; got != "CUSTOM" || called != 1 {
		t.Fatalf("fallback result = %q, calls=%d", got, called)
	}
	a.Update(tea.KeyPressMsg{Code: 'x'})
	if got := a.View().Content; strings.Contains(got, "CUSTOM") || !strings.Contains(got, "TO") {
		t.Fatalf("input did not finish transition: %q", got)
	}
}

func TestSkeuoTransitionsInterruptible(t *testing.T) {
	for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
		a, m, frame := newTransitionMain(t, level)
		m.options.ListStyle = ListCassetteRack
		m.Update(tea.WindowSizeMsg{Width: 80, Height: 24}, a)
		a.page = m
		start := time.Now()
		frame(start)
		m.EnterMenu(&mockMenu{key: "sub", items: []MenuItem{{Title: "Inner"}, {Title: "Next"}}}, &MenuItem{Title: "Sub"})
		start = start.Add(16 * time.Millisecond)
		frame(start)
		settled := column(frame(start.Add(time.Second)), "Inner")
		if level == motion.Full {
			m.menuTr.start, m.menuTr.level = start, level
			if got := column(frame(start.Add(61*time.Millisecond)), "Inner"); got != settled+6 {
				t.Fatalf("drawer entrance = %d, want %d", got, settled+6)
			}
			if got := column(frame(start.Add(175*time.Millisecond)), "Inner"); got != settled-1 {
				t.Fatalf("drawer overshoot = %d, want %d", got, settled-1)
			}
		}
		m.menuTr.start = start
		a.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		if !m.menuTr.start.IsZero() || m.selectedIndex != 1 {
			t.Fatalf("level %v: interrupted key was not applied: selected=%d", level, m.selectedIndex)
		}
		p, _ := NewPopup(PopupSpec{Title: "Saved", Content: "POPUP TEXT"})
		a.ShowPopup(p)
		modalStart := a.overlay.modalStart[p]
		first := overlayAt(a, modalStart, time.Millisecond, true)
		if level == motion.Full {
			if y, _ := find(first, "POPUP TEXT"); y >= 0 {
				t.Fatal("hinge displayed content before unfolding")
			}
		}
		a.Update(tea.KeyPressMsg{Code: 'x'})
		if len(a.overlay.modalStart) != 0 {
			t.Fatal("input did not finish the modal entrance")
		}
		a.DismissPopup()
		a.Update(tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
		if len(a.overlay.ghosts) != 0 {
			t.Fatal("click did not discard closing overlays")
		}
		a.handleShowNotification(NotificationSpec{Title: "LED", Message: "TOAST TEXT"})
		n := a.notifications[0]
		first = overlayAt(a, n.createdAt, 35*time.Millisecond, false)
		if level == motion.Full && (!strings.Contains(ansi.Strip(strings.Join(first, "\n")), "█") || strings.Contains(ansi.Strip(strings.Join(first, "\n")), "TOAST TEXT")) {
			t.Fatal("toast entrance did not begin with only LED lights")
		}
		a.Update(tea.KeyPressMsg{Code: 'x'})
		if y, _ := find(overlayAt(a, time.Now(), 0, false), "TOAST TEXT"); y < 0 {
			t.Fatal("input did not reveal the whole toast")
		}
	}
}

func TestFrameTransitionRespectsPageHandlerAndOff(t *testing.T) {
	for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
		a := newOverlayApp(t, level)
		a.page = &framePage{text: "FROM"}
		a.View()
		a.options.PageTransition = func(*App, Page, PageTransition) bool { t.Fatal("page handler must run first"); return false }
		a.page = &framePage{text: "TO", handles: true}
		a.View()
		a.StartFrameTransition(time.Second, func(string, string, time.Duration, int, int) string { return "CUSTOM" })
		got := a.View().Content
		if (got == "CUSTOM") != (level != motion.Off) {
			t.Fatalf("level %v custom frame = %q", level, got)
		}
		a.Update(tea.WindowSizeMsg{Width: 50, Height: 15})
		if a.View().Content == "CUSTOM" {
			t.Fatal("resize retained custom frame")
		}
	}
}

func TestFrameTransitionStartsAfterIdle(t *testing.T) {
	a := newOverlayApp(t, motion.Full)
	a.page = &framePage{text: "PAGE"}
	a.View()
	a.motion.frameNow = time.Now().Add(-time.Minute)
	a.StartFrameTransition(time.Second, func(string, string, time.Duration, int, int) string { return "CUSTOM" })
	if a.View().Content != "CUSTOM" {
		t.Fatal("an idle page made the new transition expire before its first frame")
	}
}
