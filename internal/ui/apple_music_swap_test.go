package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/motion"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

func swapTestPage(t *testing.T) (*AppleMusicPage, time.Time) {
	t.Helper()
	p := NewAppleMusicPage(&appleMouseClient{})
	t.Cleanup(func() { p.Close() })
	p.level, p.scene = motion.Full, appleListen
	p.sceneMotion = appleTween{from: 1, to: 1}
	p.bayID, p.bayStyle, p.bayNumber = "old", 1, 1
	p.styles["old"], p.styles["new"] = 1, 4
	p.state = applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Storefront: "us", Track: applemusic.Track{ID: "old", Name: "OLD TAPE - SYNTHETIC TEST", Duration: 200}, Position: 80, QueueTotal: 2}
	return p, time.Unix(100, 0)
}

func confirmSwap(p *AppleMusicPage, id string, now time.Time) {
	p.settledPlayGeneration = p.playGeneration
	s := p.state
	s.Track = applemusic.Track{ID: id, Name: "NEW TAPE - SYNTHETIC TEST", Duration: 240}
	s.Position, s.QueueIndex = 1, 1
	p.acceptState(s, now)
}

func TestAppleMusicSwapEjectsOldIdentityBeforeInsertingConfirmedNewTape(t *testing.T) {
	p, now := swapTestPage(t)
	old := p.mechanics(now)
	p.beginSwap(now)
	p.dispatch(applemusic.Action{Kind: "next"})
	confirmSwap(p, "new", now.Add(100*time.Millisecond))
	out := p.mechanics(now.Add(200 * time.Millisecond))
	if out.Style != old.Style || out.TrackNum != old.TrackNum || out.Radii != old.Radii {
		t.Fatal("early backend result repainted the outgoing old tape")
	}
	in := p.mechanics(now.Add(650 * time.Millisecond))
	if in.Style != 4 || in.Style == out.Style || in.Cassette <= 0 || in.Cassette >= 1 {
		t.Fatalf("new tape was not independently sliding into the bay: %+v", in)
	}
	p.advance(now.Add(900 * time.Millisecond))
	if p.bayID != "new" || p.bayStyle != 4 || !p.swapStart.IsZero() {
		t.Fatal("seated bay did not retain the new tape identity")
	}
}

func TestAppleMusicSwapWaitsForBackendBeforeReturningToCloseUp(t *testing.T) {
	p, now := swapTestPage(t)
	p.zoomed, p.camera.to = true, 1
	p.beginSwap(now)
	p.dispatch(applemusic.Action{Kind: "next"})
	p.advance(now.Add(2 * time.Second))
	if p.swapStart.IsZero() || p.camera.value(now.Add(2*time.Second)) != 0 || p.mechanics(now.Add(2*time.Second)).Cassette >= 0 {
		t.Fatal("loading reseated the old tape or restored close-up before the new tape arrived")
	}
	confirmed := now.Add(3 * time.Second)
	confirmSwap(p, "new", confirmed)
	if p.mechanics(confirmed.Add(250*time.Millisecond)).Style != 4 {
		t.Fatal("late new identity was not used by insertion")
	}
	p.advance(confirmed.Add(500 * time.Millisecond))
	before := p.mechanics(confirmed.Add(500 * time.Millisecond)).Style
	p.advance(confirmed.Add(850 * time.Millisecond))
	if before != 4 || p.mechanics(confirmed.Add(850*time.Millisecond)).Style != before || p.camera.value(confirmed.Add(850*time.Millisecond)) != 1 {
		t.Fatal("close-up changed cassette material instead of keeping the seated new tape")
	}
}

func TestAppleMusicSeekDoesNotReplaceCassette(t *testing.T) {
	p, now := swapTestPage(t)
	before := p.mechanics(now)
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyRight})
	p.advance(now.Add(time.Second))
	if !p.swapStart.IsZero() || p.bayID != "old" || p.mechanics(now.Add(time.Second)).Style != before.Style {
		t.Fatal("seeking within the same song replaced the cassette")
	}
}

func TestAppleMusicSwapCoalescesBeforeNewTapeBecomesVisible(t *testing.T) {
	p, now := swapTestPage(t)
	p.styles["latest"] = 5
	p.beginSwap(now)
	p.dispatch(applemusic.Action{Kind: "next"})
	confirmSwap(p, "new", now.Add(100*time.Millisecond))
	start := p.swapStart
	p.beginSwap(now.Add(250 * time.Millisecond))
	p.dispatch(applemusic.Action{Kind: "next"})
	confirmSwap(p, "latest", now.Add(300*time.Millisecond))
	if p.swapStart != start || p.mechanics(now.Add(350*time.Millisecond)).Style != 1 || p.mechanics(now.Add(500*time.Millisecond)).Style != 5 {
		t.Fatal("repeated skip restarted ejection or inserted the superseded tape")
	}
}

func TestAppleMusicRepeatedSkipDoesNotRepaintVisibleIncomingTape(t *testing.T) {
	p, now := swapTestPage(t)
	p.styles["latest"] = 5
	p.beginSwap(now)
	p.dispatch(applemusic.Action{Kind: "next"})
	confirmSwap(p, "new", now.Add(100*time.Millisecond))
	p.beginSwap(now.Add(600 * time.Millisecond))
	p.dispatch(applemusic.Action{Kind: "next"})
	confirmSwap(p, "latest", now.Add(700*time.Millisecond))
	if p.mechanics(now.Add(850*time.Millisecond)).Style != 4 {
		t.Fatal("a later result repainted a cassette already entering the bay")
	}
	p.advance(now.Add(900 * time.Millisecond))
	if p.mechanics(now.Add(1050*time.Millisecond)).Style != 4 {
		t.Fatal("the next swap did not eject the cassette just seated")
	}
	p.advance(now.Add(1800 * time.Millisecond))
	if p.bayID != "latest" || p.bayStyle != 5 || !p.swapStart.IsZero() {
		t.Fatal("latest replacement failed to settle")
	}
}

func TestAppleMusicSwapKeepsTargetIdentityAcrossMotionLevels(t *testing.T) {
	for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
		p, now := swapTestPage(t)
		p.level = level
		p.beginSwap(now)
		p.dispatch(applemusic.Action{Kind: "previous"})
		confirmSwap(p, "new", now.Add(100*time.Millisecond))
		p.advance(now.Add(time.Second))
		if p.bayID != "new" || p.mechanics(now.Add(time.Second)).Style != 4 || p.bayNumber != 2 || !p.ready() {
			t.Fatalf("motion %v changed playback identity", level)
		}
	}
}

func TestAppleMusicAutomaticAdvanceDuringSwapDoesNotRepaintVisibleTape(t *testing.T) {
	p, now := swapTestPage(t)
	p.styles["auto"] = 5
	p.beginSwap(now)
	p.dispatch(applemusic.Action{Kind: "next"})
	confirmSwap(p, "new", now.Add(100*time.Millisecond))
	s := p.state
	s.Track.ID = "auto"
	p.acceptState(s, now.Add(650*time.Millisecond))
	if p.mechanics(now.Add(800*time.Millisecond)).Style != 4 {
		t.Fatal("automatic next repainted the incoming tape")
	}
	p.advance(now.Add(900 * time.Millisecond))
	p.advance(now.Add(1800 * time.Millisecond))
	if p.bayID != "auto" || p.bayStyle != 5 {
		t.Fatal("automatic next did not settle after the visible swap")
	}
}

func TestAppleMusicReverseSkipCanReturnToTheOriginalTape(t *testing.T) {
	p, now := swapTestPage(t)
	p.beginSwap(now)
	p.dispatch(applemusic.Action{Kind: "next"})
	confirmSwap(p, "new", now.Add(100*time.Millisecond))
	p.beginSwap(now.Add(250 * time.Millisecond))
	p.dispatch(applemusic.Action{Kind: "previous"})
	p.settledPlayGeneration = p.playGeneration
	s := p.state
	s.Track.ID, s.QueueIndex = "old", 0
	p.acceptState(s, now.Add(300*time.Millisecond))
	p.advance(now.Add(time.Second))
	if !p.swapStart.IsZero() || p.bayID != "old" || p.mechanics(now.Add(time.Second)).Cassette != 0 {
		t.Fatal("confirmed return to the original tape left the bay empty")
	}
}

func TestAppleMusicReverseSkipCancelsObsoleteSecondSwap(t *testing.T) {
	p, now := swapTestPage(t)
	p.zoomed, p.camera.to = true, 1
	p.styles["third"] = 5
	p.beginSwap(now)
	p.dispatch(applemusic.Action{Kind: "next"})
	confirmSwap(p, "new", now.Add(100*time.Millisecond))
	p.beginSwap(now.Add(1000 * time.Millisecond))
	p.dispatch(applemusic.Action{Kind: "next"})
	confirmSwap(p, "third", now.Add(1050*time.Millisecond))
	p.beginSwap(now.Add(1100 * time.Millisecond))
	p.dispatch(applemusic.Action{Kind: "previous"})
	confirmSwap(p, "new", now.Add(1150*time.Millisecond))
	p.advance(now.Add(2 * time.Second))
	if !p.swapStart.IsZero() || p.bayID != "new" || p.mechanics(now.Add(2*time.Second)).Cassette != 0 || !p.zoomed {
		t.Fatal("reversed skip kept an obsolete swap queued after its target was cancelled")
	}
}

type appleUnsentSwapClient struct{ appleMouseClient }

func (c *appleUnsentSwapClient) Snapshot(context.Context) (applemusic.State, error) {
	return applemusic.State{}, errors.New("preflight failed; no control sent")
}

func TestAppleMusicUnsentSwapRecoversOnlyAfterFreshBackendState(t *testing.T) {
	p, _ := swapTestPage(t)
	now := time.Now()
	c := &appleUnsentSwapClient{}
	p.client = c
	p.zoomed, p.camera.to = true, 1
	old := p.state
	p.beginSwap(now)
	cmd := p.dispatch(applemusic.Action{Kind: "next"})
	p.Update(cmd(), model.NewApp(model.DefaultOptions()))
	if len(c.actions) != 0 {
		t.Fatal("preflight failure still sent playback control")
	}
	p.advance(now.Add(2 * time.Second))
	if p.mechanics(now.Add(2*time.Second)).Cassette >= 0 {
		t.Fatal("failure restored the old source without a fresh snapshot")
	}
	p.acceptState(old, now.Add(3*time.Second))
	p.advance(now.Add(4 * time.Second))
	if p.pending || !p.swapStart.IsZero() || p.bayID != "old" || p.mechanics(now.Add(4*time.Second)).Cassette != 0 || !p.zoomed {
		t.Fatal("a control never sent left the confirmed old tape permanently outside the bay")
	}
	if !strings.Contains(p.status, "preflight failed") {
		t.Fatal("visual recovery hid the control error")
	}
}

func TestAppleMusicOldUnsentSwapCannotCancelNewerTarget(t *testing.T) {
	p, _ := swapTestPage(t)
	p.client = &appleUnsentSwapClient{}
	now := time.Now()
	p.beginSwap(now)
	first := p.dispatch(applemusic.Action{Kind: "next"})
	p.beginSwap(now.Add(time.Millisecond))
	p.dispatch(applemusic.Action{Kind: "next"})
	p.Update(first(), model.NewApp(model.DefaultOptions()))
	if !p.pending || p.swap.nextID != "" {
		t.Fatal("older unsent failure cancelled the newer playback target")
	}
}

func TestAppleMusicRecoveredSwapRestartsCloseUpFromCurrentTime(t *testing.T) {
	p, _ := swapTestPage(t)
	now := time.Now().Add(-time.Second)
	p.zoomed, p.camera.to = true, 1
	p.beginSwap(now)
	p.dispatch(applemusic.Action{Kind: "next"})
	confirmSwap(p, "new", now.Add(100*time.Millisecond))
	p.busy = false
	p.client = &appleUnsentSwapClient{}
	p.beginSwap(now.Add(time.Second))
	cmd := p.dispatch(applemusic.Action{Kind: "next"})
	p.Update(cmd(), model.NewApp(model.DefaultOptions()))
	p.advance(now.Add(2 * time.Second))
	recovered := now.Add(3 * time.Second)
	p.acceptState(p.state, recovered)
	p.advance(recovered)
	if p.camera.value(recovered) != 0 {
		t.Fatal("late recovery backdated the camera and jumped straight into close-up")
	}
	if z := p.camera.value(recovered.Add(175 * time.Millisecond)); z <= 0 || z >= 1 {
		t.Fatal("recovery lost the 350 ms camera transition")
	}
	p.advance(recovered.Add(350 * time.Millisecond))
	if p.camera.value(recovered.Add(350*time.Millisecond)) != 1 || p.bayID != "new" {
		t.Fatal("recovery did not finish on the already seated new tape")
	}
}

func TestAppleMusicSwapFramebuffersAndResizes(t *testing.T) {
	for _, size := range [][2]int{{140, 45}, {120, 24}, {80, 30}, {50, 20}} {
		p, now := swapTestPage(t)
		p.tier = deck.Quadrant
		p.zoomed, p.camera.to = true, 1
		p.beginSwap(now)
		p.dispatch(applemusic.Action{Kind: "next"})
		for _, ms := range []int{0, 350, 500, 650, 750, 1800, 2000, 2090, 2180, 2260, 2340, 2500, 2675, 2850} {
			if ms == 2000 {
				confirmSwap(p, "new", now.Add(2*time.Second))
			}
			p.status = fmt.Sprintf("SYNTHETIC SWAP TEST - %d ms - NOT LIVE PLAYBACK", ms)
			frame := p.draw(size[0], size[1], now.Add(time.Duration(ms)*time.Millisecond), 2)
			for _, line := range strings.Split(frame, "\n") {
				if ansi.WcWidth.StringWidth(line) != size[0] {
					t.Fatalf("swap overflow at %v / %d", size, ms)
				}
			}
			if dir := os.Getenv("TAPEFOX_SWAP_PREVIEW_DIR"); dir != "" {
				if err := os.MkdirAll(dir, 0700); err != nil {
					t.Fatal(err)
				}
				writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("swap-%dx%d-%04d.html", size[0], size[1], ms)))
			}
		}
		if p.bayID != "new" || p.bayStyle != 4 {
			t.Fatal("window size changed the seated cassette")
		}
	}
}
