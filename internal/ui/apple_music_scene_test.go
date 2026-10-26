package ui

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/motion"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

func TestAppleMusicDrawerHasStackedSlotsAndNarrowBrowseHasNoStage(t *testing.T) {
	for _, width := range []int{80, 120} {
		p := NewAppleMusicPage(nil)
		p.level = motion.Off
		p.tracks = []applemusic.Track{{ID: "a", Name: "Alpha"}, {ID: "b", Name: "Beta"}}
		p.draw(width, 38, time.Unix(100, 0), 2)
		var rows []appleHit
		for _, hit := range p.hits {
			if width < 100 && hit.kind == appleHitStage {
				t.Fatal("narrow browsing must prioritize drawer, not a permanent device")
			}
			if hit.kind == appleHitTrack {
				rows = append(rows, hit)
			}
		}
		if len(rows) != 2 {
			t.Fatalf("missing stacked slots: %+v", rows)
		}
		for _, hit := range rows {
			if hit.slot.Dy() != 2 {
				t.Fatal("rack slots must stay two rows tall")
			}
		}
		p.Close()
	}
}

type appleNextEndClient struct {
	appleMouseClient
	id string
}

func (c *appleNextEndClient) Snapshot(context.Context) (applemusic.State, error) {
	return applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Track: applemusic.Track{ID: c.id}}, nil
}
func (c *appleNextEndClient) Do(_ context.Context, a applemusic.Action) error {
	if c.id == "a" {
		c.id = "b"
		return nil
	}
	return errors.New("queue ended")
}
func TestAppleMusicFailedSecondNextDoesNotConfirmFirstResult(t *testing.T) {
	c := &appleNextEndClient{id: "a"}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.state, _ = c.Snapshot(context.Background())
	first := p.dispatch(applemusic.Action{Kind: "next"})
	p.dispatch(applemusic.Action{Kind: "next"})
	_, second := p.Update(first(), model.NewApp(model.DefaultOptions()))
	p.Update(second(), model.NewApp(model.DefaultOptions()))
	s, _ := c.Snapshot(context.Background())
	p.acceptState(s, time.Now())
	if !p.pending || !strings.Contains(p.status, "queue ended") {
		t.Fatal("failed second next was confirmed using first next result")
	}
}

func TestAppleMusicRepeatedSwapPreservesCloseupIntent(t *testing.T) {
	p, now := swapTestPage(t)
	p.level, p.scene, p.zoomed = motion.Full, appleListen, true
	p.camera.to, p.sceneMotion.to = 1, 1
	p.beginSwap(now)
	p.dispatch(applemusic.Action{Kind: "next"})
	p.beginSwap(now.Add(100 * time.Millisecond))
	p.dispatch(applemusic.Action{Kind: "next"})
	confirmSwap(p, "new", now.Add(500*time.Millisecond))
	p.advance(now.Add(2 * time.Second))
	if !p.zoomed {
		t.Fatal("repeated skip lost close-up intent without a user zoom command")
	}
}

func TestAppleMusicPausedLoadCanBeConfirmedWithoutInventingPlayback(t *testing.T) {
	for _, full := range []bool{false, true} {
		p := NewAppleMusicPage(&appleMouseClient{})
		cmd := p.dispatch(applemusic.Action{Kind: "play", ID: "new"})
		p.Update(cmd(), model.NewApp(model.DefaultOptions()))
		p.acceptState(applemusic.State{Ready: true, Authorized: true, Full: full, Paused: true, Track: applemusic.Track{ID: "new", Duration: 240}}, time.Now())
		if p.pending == full || p.ready() != full || p.state.Playing {
			t.Fatalf("paused full=%v pending=%v ready=%v", full, p.pending, p.ready())
		}
		p.Close()
	}
}

func TestAppleMusicQueuedSeekDoesNotAffectReplacementTrack(t *testing.T) {
	c := &appleNextEndClient{id: "new"}
	p := NewAppleMusicPage(c)
	defer p.Close()
	cmd := p.dispatch(applemusic.Action{Kind: "seek", ID: "old", Value: 30})
	msg := cmd().(appleActionMsg)
	if msg.err == nil || !msg.notSent {
		t.Fatal("stale seek was sent to the replacement track")
	}
}

type appleRelativeClient struct {
	appleMouseClient
	volume   float64
	position float64
}

func (c *appleRelativeClient) Snapshot(context.Context) (applemusic.State, error) {
	return applemusic.State{Volume: c.volume, Position: c.position, Track: applemusic.Track{ID: "current", Duration: 240}}, nil
}
func (c *appleRelativeClient) Do(_ context.Context, a applemusic.Action) error {
	c.actions = append(c.actions, a)
	if a.Kind == "volume" {
		c.volume = a.Value
	}
	if a.Kind == "seek" {
		c.position = a.Value
	}
	return nil
}
func TestAppleMusicRelativeControlsAccumulateInExecutionOrder(t *testing.T) {
	c := &appleRelativeClient{volume: .5, position: 30}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.state = applemusic.State{Volume: .5, Position: 30, Track: applemusic.Track{ID: "current", Duration: 240}}
	first := p.handleKey(tea.KeyPressMsg{Code: '+'})
	p.handleKey(tea.KeyPressMsg{Code: '+'})
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyRight})
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyRight})
	for cmd := first; cmd != nil; {
		_, cmd = p.Update(cmd(), model.NewApp(model.DefaultOptions()))
	}
	if math.Abs(c.volume-.6) > 1e-9 || c.position != 50 {
		t.Fatalf("relative controls used stale UI snapshots: volume=%v position=%v", c.volume, c.position)
	}
}

func TestAppleMusicQueueShrinkKeepsVisibleSelection(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.drawer, p.selected = appleQueue, 40
	p.acceptState(applemusic.State{Queue: []applemusic.Track{{ID: "only"}}, QueueTotal: 1}, time.Now())
	if p.selected != 0 {
		t.Fatal("queue shrink left selection beyond the visible list")
	}
}

func TestAppleMusicVolumeDoesNotInvalidatePlaybackSnapshot(t *testing.T) {
	p := NewAppleMusicPage(&appleMouseClient{})
	defer p.Close()
	p.dispatch(applemusic.Action{Kind: "volume", Value: .7})
	s := applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Track: applemusic.Track{ID: "a"}}
	p.Update(appleStateMsg{state: s}, model.NewApp(model.DefaultOptions()))
	if !p.ready() {
		t.Fatal("unrelated volume input discarded playback evidence")
	}
}

func TestAppleMusicSceneFramesAndTargets(t *testing.T) {
	for _, size := range [][2]int{{140, 45}, {120, 24}, {100, 30}, {99, 30}, {80, 30}, {60, 20}, {59, 20}, {80, 19}, {12, 4}, {1, 1}} {
		for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
			for _, scene := range []appleScene{appleBrowse, appleListen} {
				p := NewAppleMusicPage(nil)
				p.level, p.scene, p.sceneMotion.to = level, scene, float64(scene)
				p.state = applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Track: applemusic.Track{ID: "a", Name: "宽字测试 磁带", Duration: 240}, Position: 72}
				p.tracks = []applemusic.Track{p.state.Track, {ID: "b", Name: "下一首"}}
				for _, zoom := range []float64{0, .5, 1} {
					p.zoomed, p.camera.to = zoom > 0, zoom
					out := p.draw(size[0], size[1], time.Unix(100, 0), 2)
					lines := strings.Split(out, "\n")
					if len(lines) != size[1] {
						t.Fatalf("%v: %d rows", size, len(lines))
					}
					for _, line := range lines {
						if ansi.WcWidth.StringWidth(line) != size[0] || ansi.GraphemeWidth.StringWidth(line) != size[0] {
							t.Fatalf("%v: broken width", size)
						}
					}
					for _, hit := range p.hits {
						if hit.x < 0 || hit.y < 0 || hit.x+hit.w > size[0] || hit.y+hit.h > size[1] {
							t.Fatalf("%v: escaped target %+v", size, hit)
						}
					}
				}
				p.Close()
			}
		}
	}
}

func TestAppleMusicLocalZoomDoesNotReflowOtherColumn(t *testing.T) {
	for _, scene := range []appleScene{appleBrowse, appleListen} {
		p := NewAppleMusicPage(nil)
		p.scene, p.sceneMotion.to = scene, float64(scene)
		p.level = motion.Full
		now := time.Unix(100, 0)
		p.draw(140, 45, now, 2)
		var before []string
		start, end := 2, 65
		if scene == appleListen {
			start, end = 74, 137
		}
		for y := 3; y < 36; y++ {
			for x := start; x < end; x++ {
				before = append(before, fmt.Sprint(p.screen.buf.CellAt(x, y)))
			}
		}
		p.setZoom(true, now)
		p.draw(140, 45, now.Add(175*time.Millisecond), 2)
		var after []string
		for y := 3; y < 36; y++ {
			for x := start; x < end; x++ {
				after = append(after, fmt.Sprint(p.screen.buf.CellAt(x, y)))
			}
		}
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("scene %v zoom changed the other column", scene)
		}
		p.Close()
	}
}

func TestAppleMusicScenePreview(t *testing.T) {
	dir := os.Getenv("TAPEFOX_SCENE_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set TAPEFOX_SCENE_PREVIEW_DIR for actual framebuffer fixtures")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	newPage := func() *AppleMusicPage {
		p := NewAppleMusicPage(nil)
		p.level = motion.Full
		p.state = applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Volume: .6, Position: 72, Track: applemusic.Track{ID: "0", Name: "Night Drive / 夜行磁带", Artist: "视觉测试数据", Duration: 245}}
		for i, name := range []string{"Night Drive / 夜行磁带", "磁带 B 面", "Electric Dreams", "霓虹街角", "After Midnight", "Low Fidelity", "深夜电台", "Blue Hour"} {
			p.tracks = append(p.tracks, applemusic.Track{ID: fmt.Sprint(i), Name: name, Artist: "测试曲目"})
		}
		p.status = "FRAMEBUFFER TEST · 非真实登录或完整播放证据"
		return p
	}
	now := time.Unix(100, 0)
	for _, size := range [][2]int{{140, 45}, {120, 24}, {80, 30}, {59, 19}} {
		for _, scene := range []appleScene{appleBrowse, appleListen} {
			for _, z := range []float64{0, .5, 1} {
				p := newPage()
				p.scene, p.sceneMotion.to = scene, float64(scene)
				p.zoomed, p.camera.to = z > 0, z
				p.draw(size[0], size[1], now, 2)
				writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("scene-%d-%dx%d-zoom-%.1f.html", scene, size[0], size[1], z)))
				p.Close()
			}
		}
	}
	p := newPage()
	defer p.Close()
	p.draw(140, 45, now, 2)
	p.beginInsert(p.tracks[0], now)
	for _, ms := range []int{0, 80, 379, 380, 500, 660, 760, 920, 1080, 1280} {
		p.draw(140, 45, p.insertStart.Add(time.Duration(ms)*time.Millisecond), 2)
		writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("insert-%04d.html", ms)))
	}
	p.panel = "操作与设置"
	p.draw(140, 45, now.Add(2*time.Second), 2)
	writeAppleFrame(t, p, filepath.Join(dir, "panel.html"))
}

func TestAppleMusicCameraReversesFromCurrentPosition(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	now := time.Unix(100, 0)
	p.setZoom(true, now)
	mid := now.Add(175 * time.Millisecond)
	before := p.camera.value(mid)
	p.setZoom(false, mid)
	if math.Abs(before-p.camera.value(mid)) > 1e-9 || before <= 0 || before >= 1 {
		t.Fatal("camera jumped on reversal")
	}
	if p.camera.value(mid.Add(350*time.Millisecond)) != 0 {
		t.Fatal("camera did not finish pulling out")
	}
}

func TestAppleMusicWaitingDoesNotConfirmAnUnexecutedNext(t *testing.T) {
	p := NewAppleMusicPage(&appleMouseClient{})
	defer p.Close()
	p.state = applemusic.State{Track: applemusic.Track{ID: "a"}}
	first := p.dispatch(applemusic.Action{Kind: "next"})
	p.dispatch(applemusic.Action{Kind: "next"})
	p.acceptState(applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Track: applemusic.Track{ID: "b"}}, time.Now())
	if !p.pending {
		t.Fatal("first next confirmed a queued second request")
	}
	_, second := p.Update(first(), model.NewApp(model.DefaultOptions()))
	if second == nil {
		t.Fatal("second next missing")
	}
	p.Update(second(), model.NewApp(model.DefaultOptions()))
	p.acceptState(applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Track: applemusic.Track{ID: "c"}}, time.Now())
	if p.pending || !p.ready() {
		t.Fatal("latest real playback not confirmed")
	}
}

func TestAppleMusicTimelineDoesNotRollBackAtAnimationTimeout(t *testing.T) {
	for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
		p := NewAppleMusicPage(&appleMouseClient{})
		p.level = level
		now := time.Unix(100, 0)
		p.beginInsert(applemusic.Track{ID: "new"}, now)
		p.dispatch(applemusic.Action{Kind: "play", ID: "new"})
		p.advance(now.Add(5 * time.Second))
		if !p.pending || p.ready() || p.scene != appleListen {
			t.Fatalf("motion %v fabricated a successful load or rollback", level)
		}
		p.Close()
	}
}

func TestAppleMusicEscAndDrawerRestoreContext(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Off
	p.scene, p.zoomed, p.panel = appleListen, true, "操作"
	p.camera.to, p.sceneMotion.to = 1, 1
	esc := tea.KeyPressMsg{Code: tea.KeyEscape}
	p.handleKey(esc)
	if p.panel != "" || !p.zoomed || p.scene != appleListen {
		t.Fatal("first Esc did more than close panel")
	}
	p.handleKey(esc)
	if p.zoomed || p.scene != appleListen {
		t.Fatal("second Esc did more than leave close-up")
	}
	p.handleKey(esc)
	if p.scene != appleBrowse {
		t.Fatal("third Esc did not return to browsing")
	}
	p.selected = 4
	p.switchDrawer(appleQueue, time.Now())
	p.selected = 2
	p.switchDrawer(appleSearch, time.Now())
	if p.selected != 4 {
		t.Fatal("drawer selection context lost")
	}
}

func TestAppleMusicOffHasNoWaitingAnimation(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Off
	now := time.Unix(100, 0)
	a := p.draw(140, 45, now, 2)
	b := p.draw(140, 45, now.Add(173*time.Millisecond), 2)
	if a != b {
		t.Fatal("motion off still animates a waiting counter")
	}
}

func TestAppleMusicCanChooseAgainDuringInsertion(t *testing.T) {
	p := NewAppleMusicPage(&appleMouseClient{})
	defer p.Close()
	p.level = motion.Full
	p.tracks = []applemusic.Track{{ID: "a"}, {ID: "b"}}
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if p.pendingID != "b" {
		t.Fatal("animation blocked a newer selection")
	}
}

func TestAppleMusicZoomDuringFlightKeepsHandoffContinuous(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Full
	now := time.Unix(100, 0)
	p.beginInsert(applemusic.Track{ID: "new"}, now)
	p.setZoom(true, now.Add(100*time.Millisecond))
	if p.camera.value(now.Add(380*time.Millisecond)) != 0 {
		t.Fatal("camera changed the flight handoff projection")
	}
	p.advance(now.Add(1280 * time.Millisecond))
	if !p.zoomed || p.camera.value(now.Add(1630*time.Millisecond)) != 1 {
		t.Fatal("latest zoom intent was lost")
	}
}

func TestAppleMusicDetailAutoZoomsToCassetteCloseUpAfterDrawerFolds(t *testing.T) {
	c := &appleMouseClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Full
	p.state = applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Track: applemusic.Track{ID: "track-1", Name: "Night Drive"}}
	p.tracks = []applemusic.Track{p.state.Track}
	now := time.Now()
	p.draw(140, 45, now, 2)

	// Click the NOW PLAYING detail card ("i") while in Browse.
	var detailHit *appleHit
	for i := range p.hits {
		if p.hits[i].kind == appleHitKey && p.hits[i].key == "i" {
			detailHit = &p.hits[i]
			break
		}
	}
	if detailHit == nil {
		t.Fatal("missing detail entry hit")
	}
	p.handleClick(detailHit.x, detailHit.y)
	foldStart := p.sceneMotion.at
	if p.scene != appleListen {
		t.Fatal("clicking detail did not switch to listen detail scene")
	}
	// While the left playlist is still folding (320ms < 640ms), camera has not zoomed yet.
	p.draw(140, 45, foldStart.Add(320*time.Millisecond), 2)
	if p.camera.value(foldStart.Add(320*time.Millisecond)) != 0 {
		t.Fatal("Walkman zoomed before left playlist finished folding")
	}
	// Once the 640ms drawer fold finishes, Walkman automatically starts zooming to the cassette close-up.
	p.draw(140, 45, foldStart.Add(640*time.Millisecond), 2)
	if !p.zoomed {
		t.Fatal("Walkman did not trigger automatic close-up after drawer folded")
	}
	if p.camera.value(foldStart.Add(990*time.Millisecond)) != 1 {
		t.Fatal("Walkman did not reach full cassette close-up at 640ms+350ms")
	}
	// Returning to browse pulls out from close-up and unfolds the playlist.
	p.browse(foldStart.Add(1000 * time.Millisecond))
	if p.zoomed || p.scene != appleBrowse {
		t.Fatal("returning to browse did not reset close-up zoom")
	}
}

func TestAppleMusicWalkmanFacesLeftForPlaylistAndRightForLyrics(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Off
	now := time.Unix(100, 0)

	// Browse scene: playlist is on the left, Walkman is on the right and faces left (Yaw > 0).
	p.scene, p.sceneMotion = appleBrowse, appleTween{from: 0, to: 0}
	browsePose := p.sceneBasePose(now)
	if browsePose.Yaw <= 0 {
		t.Fatalf("expected Walkman to face left (Yaw > 0) when playlist is on the left, got %v", browsePose.Yaw)
	}

	// Listen scene: lyrics/details are on the right, Walkman is on the left and faces right (Yaw < 0).
	p.scene, p.sceneMotion = appleListen, appleTween{from: 1, to: 1}
	listenPose := p.sceneBasePose(now)
	if listenPose.Yaw >= 0 || math.Abs(listenPose.Yaw+browsePose.Yaw) > 1e-9 {
		t.Fatalf("expected Walkman to face right (Yaw < 0) when lyrics are on the right, got %v", listenPose.Yaw)
	}
}

func TestAppleMusicSubtleAllowsNewSelectionDuringTransition(t *testing.T) {
	p := NewAppleMusicPage(&appleMouseClient{})
	defer p.Close()
	p.level = motion.Subtle
	p.tracks = []applemusic.Track{{ID: "a"}, {ID: "b"}}
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
	p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if p.pendingID != "b" {
		t.Fatal("subtle transition blocked a newer selection")
	}
}

func TestAppleMusicUnconfirmedTimeIsUnknown(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.level = motion.Off
	p.state = applemusic.State{Ready: true, Authorized: true, Track: applemusic.Track{ID: "requested", Duration: 240}}
	frame := ansi.Strip(p.draw(120, 38, time.Unix(100, 0), 2))
	if !strings.Contains(frame, "--:--") {
		t.Fatal("unconfirmed load displayed a real clock")
	}
}

func TestAppleMusicBusyDoesNotDropTransport(t *testing.T) {
	c := &appleMouseClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.state.Track.ID = "current"
	first := p.handleKey(tea.KeyPressMsg{Code: 'n'})
	second := p.handleKey(tea.KeyPressMsg{Code: tea.KeySpace})
	if first == nil {
		t.Fatal("first action missing")
	}
	_ = second
	msg := first()
	_, next := p.Update(msg, model.NewApp(model.DefaultOptions()))
	if next == nil {
		t.Fatal("pause during a pending command was dropped")
	}
	next()
	if len(c.actions) != 2 || c.actions[1].Kind != "toggle" {
		t.Fatalf("actions: %+v", c.actions)
	}
}
