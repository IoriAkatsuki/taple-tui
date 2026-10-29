package ui

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/anhoder/foxful-cli/motion"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

func TestAppleMusicRackStacksSpinesInsteadOfThumbnails(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	for i := range 9 {
		p.tracks = append(p.tracks, applemusic.Track{ID: fmt.Sprint(i), Name: fmt.Sprintf("Tape %02d", i+1)})
	}
	p.draw(140, 45, time.Unix(100, 0), 2)
	count := 0
	for _, h := range p.hits {
		if h.kind == appleHitTrack {
			count++
		}
	}
	if count < 8 {
		t.Fatalf("got %d tracks: rack still uses separate face-up thumbnail cards", count)
	}
}

func TestAppleMusicPopupAndFlightKeepWideTextWidths(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	for i := range 10 {
		p.tracks = append(p.tracks, applemusic.Track{ID: fmt.Sprint(i), Name: "很长的中文歌曲标题覆盖位置验证❤️", Artist: "中文演唱者与组合"})
	}
	p.selected = 3
	now := time.Unix(100, 0)
	check := func(at time.Time) {
		t.Helper()
		out := p.draw(140, 45, at, 2)
		for _, line := range strings.Split(out, "\n") {
			if ansi.GraphemeWidth.StringWidth(line) != 140 || ansi.WcWidth.StringWidth(line) != 140 {
				t.Fatal("overlapping cassette broke terminal cell width")
			}
		}
	}
	for _, ms := range []int{0, 45, 90, 180} {
		check(now.Add(time.Duration(ms) * time.Millisecond))
	}
	p.beginInsert(p.tracks[p.selected], now.Add(200*time.Millisecond))
	for _, ms := range []int{0, 80, 150, 300, 380, 660} {
		check(p.insertStart.Add(time.Duration(ms) * time.Millisecond))
	}
}

func TestAppleMusicFastFirstLoadDoesNotFillBayBeforeFlight(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	p.tracks = []applemusic.Track{{ID: "new"}}
	now := time.Unix(100, 0)
	p.draw(140, 45, now, 2)
	p.beginInsert(p.tracks[0], now)
	if !p.insertStart.After(now) {
		t.Fatal("fixture did not enter popup preparation")
	}
	p.acceptState(applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Track: p.tracks[0]}, now.Add(10*time.Millisecond))
	if p.mechanics(now.Add(20*time.Millisecond)).Cassette >= 0 {
		t.Fatal("fast backend success put the incoming cassette into the bay twice")
	}
}

func TestAppleMusicRepeatedPickDoesNotRewindOldEjection(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	p.state.Track.ID = "old"
	now := time.Unix(100, 0)
	p.beginInsert(applemusic.Track{ID: "first"}, now)
	at := now.Add(220 * time.Millisecond)
	oldTime, oldFrame := p.insertEjectAt, p.mechanics(at)
	p.beginInsert(applemusic.Track{ID: "second"}, at)
	if p.insertEjectAt != oldTime || p.mechanics(at) != oldFrame {
		t.Fatal("new target teleported the outgoing cassette back into the bay")
	}
}

func TestAppleMusicIncomingCassetteCanBeReplacedWithoutSnapping(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	now := time.Unix(100, 0)
	p.beginInsert(applemusic.Track{ID: "first"}, now)
	p.state.Track.ID = "first"
	at := p.insertStart.Add(450 * time.Millisecond)
	before := p.mechanics(at)
	if before.Cassette != 1 {
		t.Fatal("fixture did not reach the raised bay handoff")
	}
	p.beginInsert(applemusic.Track{ID: "second"}, at)
	after := p.mechanics(at)
	if after.Cassette != before.Cassette || after.Door != before.Door || after.State != before.State {
		t.Fatal("new selection snapped an incoming cassette into the bay before removing it")
	}
}

func TestAppleMusicFlyingCassetteConsumesCoveredTrackClicks(t *testing.T) {
	p := NewAppleMusicPage(&appleMouseClient{})
	defer p.Close()
	p.level = motion.Full
	for i := range 8 {
		p.tracks = append(p.tracks, applemusic.Track{ID: fmt.Sprint(i), Name: "Tape"})
	}
	p.selected = 1
	now := time.Unix(100, 0)
	p.draw(140, 45, now, 2)
	p.draw(140, 45, now.Add(200*time.Millisecond), 2)
	p.beginInsert(p.tracks[p.selected], now.Add(200*time.Millisecond))
	p.draw(140, 45, p.insertStart.Add(-time.Nanosecond), 2)
	point := image.Pt(-1, -1)
	for _, surface := range p.hits {
		if surface.kind != appleHitCassette || surface.index != p.selected {
			continue
		}
		for _, base := range p.hits {
			if base.kind == appleHitTrack && base.index != p.selected && image.Pt(surface.x, surface.y).In(base.slot) {
				point = image.Pt(surface.x, surface.y)
				break
			}
		}
		if point.X >= 0 {
			break
		}
	}
	if point.X < 0 {
		t.Fatal("fixture has no popup occlusion")
	}
	p.draw(140, 45, p.insertStart, 2)
	if cmd := p.handleClick(point.X, point.Y); cmd != nil {
		t.Fatal("flying cassette click played a covered neighbour")
	}
}

func TestAppleMusicPopupInterpolatesWithoutReflowingSlots(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	for i := range 8 {
		p.tracks = append(p.tracks, applemusic.Track{ID: fmt.Sprint(i), Name: "Tape"})
	}
	p.selected = 2
	p.rackFocused = true
	now := time.Unix(100, 0)
	var pitches []float64
	var rows []int
	for _, dt := range []time.Duration{0, 90 * time.Millisecond, 180 * time.Millisecond} {
		p.draw(140, 45, now.Add(dt), 2)
		for _, hit := range p.hits {
			if hit.kind != appleHitTrack {
				continue
			}
			if hit.index == 2 {
				pitches = append(pitches, hit.pose.Pitch)
			}
			if dt == 0 {
				rows = append(rows, hit.slot.Min.Y)
			} else if hit.slot.Min.Y != rows[hit.index] {
				t.Fatal("popup moved a neighbouring slot")
			}
		}
	}
	if len(pitches) != 3 || !(pitches[0] > pitches[1] && pitches[1] > pitches[2]) {
		t.Fatalf("popup jumped instead of unfolding: %v", pitches)
	}
}

func TestAppleMusicReplacementRequestDoesNotWaitForAnimation(t *testing.T) {
	c := &appleMouseClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Full
	p.state = applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Track: applemusic.Track{ID: "old"}}
	p.tracks = []applemusic.Track{{ID: "new"}}
	p.draw(140, 45, time.Now(), 2)
	cmd := p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil || !p.inserting || len(c.actions) != 0 {
		t.Fatal("play request must remain an asynchronous command")
	}
	cmd()
	if len(c.actions) != 1 || c.actions[0].ID != "new" || !p.inserting {
		t.Fatal("playback waited for the mechanical performance")
	}
}

func TestAppleMusicStackReplacementPreview(t *testing.T) {
	dir := os.Getenv("TAPLE_STACK_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set TAPLE_STACK_PREVIEW_DIR to export stacked popup and old/new handoffs")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	for i, name := range []string{"Old Tape / 第一张", "磁带 B 面", "New Tape / 弹出这一张", "Electric Dreams", "霓虹街角", "After Midnight", "Low Fidelity", "深夜电台", "Blue Hour"} {
		p.tracks = append(p.tracks, applemusic.Track{ID: fmt.Sprint(i), Name: name, Artist: "视觉测试曲目", Duration: 245})
	}
	p.state = applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Volume: .6, Position: 72, Track: p.tracks[0]}
	p.selected = 2
	now := time.Unix(100, 0)
	for _, ms := range []int{0, 45, 90, 135, 180, 250} {
		p.status = "FRAMEBUFFER TEST · 层叠弹出，不是实际账户或播放"
		p.draw(140, 45, now.Add(time.Duration(ms)*time.Millisecond), 2)
		writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("popup-%04d.html", ms)))
	}
	click := now.Add(300 * time.Millisecond)
	p.beginInsert(p.tracks[2], click)
	p.pending, p.pendingID, p.generation, p.playGeneration, p.settledPlayGeneration = true, p.tracks[2].ID, 1, 1, 1
	keys := map[int]bool{}
	for _, ms := range []int{140, 260, 319, 699, 700, 820, 900, 980} {
		keys[ms] = true
	}
	for ms := 0; ms <= 1600; ms += 40 {
		keys[ms] = true
	}
	steps := make([]int, 0, len(keys))
	for ms := range keys {
		steps = append(steps, ms)
	}
	sort.Ints(steps)
	for _, ms := range steps {
		at := click.Add(time.Duration(ms) * time.Millisecond)
		if ms == 200 {
			s := p.state
			s.Track, s.Position = p.tracks[2], 0
			p.acceptState(s, at)
		}
		p.status = "FRAMEBUFFER TEST · 旧白磁带退场 / 新蓝磁带插入"
		p.draw(140, 45, at, 2)
		writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("replace-%04d.html", ms)))
	}
}

func TestAppleMusicReplacementEjectsCapturedOldCassette(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	p.state = applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Track: applemusic.Track{ID: "old", Duration: 200}, Position: 80}
	now := time.Unix(100, 0)
	old := p.mechanics(now)
	p.beginInsert(applemusic.Track{ID: "new"}, now)
	atStart := p.mechanics(now)
	if atStart.State != old.State || atStart.Cassette != old.Cassette {
		t.Fatal("old cassette vanished or changed material before ejection")
	}
	if p.insertStart.Sub(now) < 300*time.Millisecond {
		t.Fatal("replacement did not reserve time for removing the old cassette")
	}
	p.state.Track.ID = "new"
	if p.mechanics(now.Add(100*time.Millisecond)).Style != old.Style {
		t.Fatal("early backend success replaced the old visual cassette")
	}
}

func TestAppleMusicPopupDoesNotStealVisibleNeighbourLabels(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	for i := range 8 {
		p.tracks = append(p.tracks, applemusic.Track{ID: fmt.Sprint(i), Name: "Tape"})
	}
	p.selected = 1
	p.rackFocused = true
	now := time.Unix(100, 0)
	p.draw(140, 45, now, 2)
	p.draw(140, 45, now.Add(200*time.Millisecond), 2)
	for _, h := range p.hits {
		if h.kind != appleHitTrack {
			continue
		}
		at := p.hitAt(h.slot.Min.X+2, h.slot.Min.Y)
		if at == nil || at.index != h.index {
			t.Fatalf("popup stole visible label for row %d", h.index)
		}
	}
}
