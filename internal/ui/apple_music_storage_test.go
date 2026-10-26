package ui

import (
	tea "charm.land/bubbletea/v2"
	"fmt"
	"github.com/anhoder/foxful-cli/model"
	"math/rand/v2"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anhoder/foxful-cli/motion"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

func storageTestPage() *AppleMusicPage {
	p := NewAppleMusicPage(nil)
	p.level = motion.Full
	for i := range 8 {
		p.tracks = append(p.tracks, applemusic.Track{ID: fmt.Sprint(i), Name: "Track", Artist: "Artist"})
	}
	return p
}

func TestAppleMusicHoverDoesNotConsumeRandomStyles(t *testing.T) {
	p, q := storageTestPage(), storageTestPage()
	defer p.Close()
	defer q.Close()
	p.styleRNG = rand.New(rand.NewPCG(10, 20))
	q.styleRNG = rand.New(rand.NewPCG(10, 20))
	now := time.Unix(100, 0)
	// Before any slot is popped out, all slots stay unified and consume no RNG.
	p.draw(140, 45, now, 2)
	if len(p.styles) != 0 {
		t.Fatal("stowed storage assigned random styles before any slot was popped up")
	}
	// Popping a slot up to wait for pick-up chooses one random style and reveals it.
	storageHover(p, 1, now.Add(10*time.Millisecond))
	firstStyle := p.rackPickup.style
	wantFirst := q.nextCassetteStyle()
	if firstStyle != wantFirst || firstStyle == 0 {
		t.Fatalf("pop-up for pick-up got style %d, want %d", firstStyle, wantFirst)
	}
	// Redrawing or dwelling on the same half-popped slot never re-rolls the style.
	for ms := 50; ms <= 400; ms += 50 {
		storageHover(p, 1, now.Add(time.Duration(ms)*time.Millisecond))
		if p.rackPickup.style != firstStyle {
			t.Fatal("dwelling on half-popped slot re-rolled its random style")
		}
	}
	// Confirming pick-up carries the exact same popped-up style into flight and bay.
	p.selected = 1
	p.beginInsert(p.tracks[1], now.Add(450*time.Millisecond))
	if p.insertStyle != firstStyle || p.nextCassetteStyle() != q.nextCassetteStyle() {
		t.Fatal("confirming pick-up did not preserve the popped-up random style")
	}
}

func TestAppleMusicEnterPicksTheHoveredCassetteWithoutScrolling(t *testing.T) {
	c := &appleMouseClient{}
	p := NewAppleMusicPage(c)
	defer p.Close()
	p.level = motion.Full
	for i := range 25 {
		p.tracks = append(p.tracks, applemusic.Track{ID: fmt.Sprint(i), Name: "Track"})
	}
	p.selected, p.rackFocused = 20, true
	now := time.Now()
	p.draw(140, 45, now, 2)
	start := p.rackStart
	var target appleHit
	for _, h := range p.hits {
		if h.kind == appleHitTrack && h.index == start {
			target = h
			break
		}
	}
	p.Update(tea.MouseMotionMsg{X: target.slot.Min.X + 2, Y: target.slot.Min.Y}, model.NewApp(model.DefaultOptions()))
	p.draw(140, 45, now.Add(200*time.Millisecond), 2)
	cmd := p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("hovered cassette was not picked")
	}
	cmd()
	p.draw(140, 45, now.Add(210*time.Millisecond), 2)
	if len(c.actions) != 1 || c.actions[0].ID != target.id || p.rackStart != start {
		t.Fatal("Enter picked a different cursor target or jumped the viewport")
	}
}

func TestAppleMusicStaleHoveredEnterDoesNotPickReplacement(t *testing.T) {
	p := storageTestPage()
	defer p.Close()
	p.client = &appleMouseClient{}
	now := time.Now()
	p.draw(140, 45, now, 2)
	for _, h := range p.hits {
		if h.kind == appleHitTrack && h.index == 2 {
			p.Update(tea.MouseMotionMsg{X: h.slot.Min.X + 2, Y: h.slot.Min.Y}, model.NewApp(model.DefaultOptions()))
			break
		}
	}
	p.tracks[2].ID = "replacement"
	if cmd := p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Fatal("stale hovered identity picked a replacement song")
	}
	if len(p.styles) != 0 {
		t.Fatal("stale target assigned a new random style")
	}
}

func TestAppleMusicRepeatedConfirmationDoesNotRerollStyle(t *testing.T) {
	for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
		p := storageTestPage()
		p.client = &appleMouseClient{}
		p.level = level
		p.draw(140, 45, time.Now(), 2)
		if p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter}) == nil {
			t.Fatal("pickup missing")
		}
		chosen := p.insertStyle
		if p.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter}) != nil || p.insertStyle != chosen {
			t.Fatalf("%v rerolled an ongoing pickup", level)
		}
		p.Close()
	}
}

func TestAppleMusicLateSearchCannotReplaceThePickedObject(t *testing.T) {
	p := storageTestPage()
	defer p.Close()
	now := time.Now()
	p.draw(140, 45, now, 2)
	p.selected = 2
	p.beginInsert(p.tracks[2], now)
	p.searching = true
	p.Update(appleCatalogMsg{request: applemusic.CatalogRequest{Kind: applemusic.CatalogSongs}, page: applemusic.CatalogPage{Items: []applemusic.CatalogItem{{Type: "songs", ID: "replacement", Name: "New result"}}}}, model.NewApp(model.DefaultOptions()))
	if len(p.items()) != 8 || p.items()[2].ID != p.insertID || p.selected != 2 {
		t.Fatal("late search replaced the object while it was being extracted")
	}
	p.insertSourceCaptured = true
	p.advance(time.Now())
	if len(p.items()) != 1 || p.items()[0].ID != "replacement" || p.searching {
		t.Fatal("deferred search was not applied after source capture")
	}
}

func TestAppleMusicRapidPickWaitsForThePreviousPreviewToStow(t *testing.T) {
	p := storageTestPage()
	defer p.Close()
	now := time.Unix(100, 0)
	p.draw(140, 45, now, 2)
	storageHover(p, 1, now.Add(10*time.Millisecond))
	p.draw(140, 45, now.Add(250*time.Millisecond), 2)
	p.selected = 3
	p.beginInsert(p.tracks[3], now.Add(260*time.Millisecond))
	if p.insertStart.Before(now.Add(560 * time.Millisecond)) {
		t.Fatal("pickup did not wait for stow then extraction")
	}
	for ms := 260; ms < 560; ms += 30 {
		p.draw(140, 45, now.Add(time.Duration(ms)*time.Millisecond), 2)
		ids := map[string]bool{}
		for _, h := range p.hits {
			if h.kind == appleHitCassette {
				ids[h.id] = true
			}
		}
		if len(ids) > 1 {
			t.Fatal("rapid pickup exposed two physical cassettes")
		}
	}
	p.draw(140, 45, p.insertStart, 2)
	if p.insertPose.Pitch > .43 {
		t.Fatal("new cassette started flying before full extraction")
	}
}

func TestAppleMusicNarrowPickupFinishesExtractionBeforeFlight(t *testing.T) {
	p := storageTestPage()
	defer p.Close()
	now := time.Unix(100, 0)
	p.draw(80, 30, now, 2)
	for _, hit := range p.hits {
		if hit.kind == appleHitTrack && hit.index == 1 {
			p.hoverX, p.hoverY = hit.slot.Min.X+2, hit.slot.Min.Y
			break
		}
	}
	p.draw(80, 30, now.Add(10*time.Millisecond), 2)
	p.draw(80, 30, now.Add(210*time.Millisecond), 2)
	p.selected = 1
	p.beginInsert(p.tracks[1], now.Add(220*time.Millisecond))
	p.draw(80, 30, now.Add(300*time.Millisecond), 2)
	visible := false
	for _, hit := range p.hits {
		if hit.kind == appleHitCassette && hit.id == p.insertID {
			visible = true
		}
	}
	if !visible {
		t.Fatal("narrow screen hid the half-picked cassette before extraction finished")
	}
	p.draw(80, 30, p.insertStart.Add(-time.Nanosecond), 2)
	signature := func(x, y int) string {
		c := p.screen.buf.CellAt(x, y)
		var fg, bg [4]uint32
		if c.Style.Fg != nil {
			fg[0], fg[1], fg[2], fg[3] = c.Style.Fg.RGBA()
		}
		if c.Style.Bg != nil {
			bg[0], bg[1], bg[2], bg[3] = c.Style.Bg.RGBA()
		}
		return fmt.Sprintf("%q/%d/%v/%v", c.Content, c.Width, fg, bg)
	}
	before := map[[2]int]string{}
	for _, h := range p.hits {
		if h.kind == appleHitCassette && h.y >= 13 {
			for x := h.x; x < h.x+h.w; x++ {
				before[[2]int{x, h.y}] = signature(x, h.y)
			}
		}
	}
	if len(before) < 20 {
		t.Fatalf("narrow source comparison contains only %d cells", len(before))
	}
	p.draw(80, 30, p.insertStart, 2)
	if !p.insertSourceCaptured || p.insertPose.Pitch > .43 {
		t.Fatal("narrow screen launched an uncaptured half-extracted source")
	}
	for _, hit := range p.hits {
		if hit.kind == appleHitTrack {
			t.Fatal("hidden narrow drawer retained click targets")
		}
	}
	for xy, want := range before {
		if got := signature(xy[0], xy[1]); got != want {
			t.Fatalf("narrow source changed at %v: got %s want %s", xy, got, want)
		}
	}
}

func TestAppleMusicStoredHalfPeekAndPickupPreview(t *testing.T) {
	dir := os.Getenv("TAPEFOX_STORAGE_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set TAPEFOX_STORAGE_PREVIEW_DIR to export closed, half-peek and random pickup frames")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	p := storageTestPage()
	defer p.Close()
	p.styleRNG = rand.New(rand.NewPCG(10, 20))
	p.styles["0"], p.lastCassetteStyle = 5, 5
	for i, name := range []string{"Night Drive / 夜行", "磁带 B 面", "Electric Dreams", "霓虹街角", "After Midnight", "Low Fidelity", "深夜电台", "Blue Hour"} {
		p.tracks[i].Name, p.tracks[i].Artist, p.tracks[i].Duration = name, "视觉测试曲目", 245
	}
	p.state = applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Volume: .6, Position: 72, Track: p.tracks[0]}
	now := time.Unix(100, 0)
	write := func(ms int) {
		p.status = "FRAMEBUFFER TEST · 统一收纳 / 单张半弹出 / 拿起后显色"
		p.draw(140, 45, now.Add(time.Duration(ms)*time.Millisecond), 2)
		writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("storage-%04d.html", ms)))
	}
	write(0)
	storageHover(p, 1, now.Add(200*time.Millisecond))
	for _, ms := range []int{200, 240, 300, 350, 395, 430, 500} {
		write(ms)
	}
	storageHover(p, 3, now.Add(520*time.Millisecond))
	for _, ms := range []int{520, 550, 600, 650, 700, 760, 800, 860} {
		write(ms)
	}
	p.selected = 3
	p.beginInsert(p.tracks[3], now.Add(900*time.Millisecond))
	p.pending, p.pendingID, p.generation, p.playGeneration, p.settledPlayGeneration = true, p.tracks[3].ID, 1, 1, 1
	for ms := 900; ms <= 2900; ms += 40 {
		if ms == 1100 {
			s := p.state
			s.Track, s.Position = p.tracks[3], 0
			p.acceptState(s, now.Add(time.Duration(ms)*time.Millisecond))
		}
		write(ms)
	}
	t.Logf("picked style %d", p.insertStyle)
}

func TestAppleMusicNarrowStoragePreview(t *testing.T) {
	dir := os.Getenv("TAPEFOX_STORAGE_PREVIEW_DIR")
	if dir == "" {
		t.Skip("set TAPEFOX_STORAGE_PREVIEW_DIR to export narrow pickup frames")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	p := storageTestPage()
	defer p.Close()
	p.styleRNG = rand.New(rand.NewPCG(10, 20))
	p.selected, p.rackFocused = 1, true
	now := time.Unix(100, 0)
	write := func(name string, at time.Time) {
		p.status = "FRAMEBUFFER TEST · 窄屏取出接续"
		p.draw(80, 30, at, 2)
		writeAppleFrame(t, p, filepath.Join(dir, name+".html"))
	}
	write("narrow-start", now)
	write("narrow-half", now.Add(200*time.Millisecond))
	p.beginInsert(p.tracks[1], now.Add(220*time.Millisecond))
	write("narrow-picking", now.Add(320*time.Millisecond))
	write("narrow-extracted", p.insertStart.Add(-time.Nanosecond))
	write("narrow-flight", p.insertStart)
	write("narrow-seated", p.insertStart.Add(1280*time.Millisecond))
}

func storageHover(p *AppleMusicPage, index int, now time.Time) {
	for _, h := range p.hits {
		if h.kind == appleHitTrack && h.index == index {
			p.hoverX, p.hoverY = h.slot.Min.X+2, h.slot.Min.Y
			break
		}
	}
	p.draw(140, 45, now, 2)
}

func TestAppleMusicStorageStartsFullyStowed(t *testing.T) {
	p := storageTestPage()
	defer p.Close()
	now := time.Unix(100, 0)
	p.draw(140, 45, now, 2)
	p.draw(140, 45, now.Add(time.Second), 2)
	for _, h := range p.hits {
		if h.kind == appleHitCassette {
			t.Fatal("unfocused rack exposed a cassette instead of keeping everything stowed")
		}
	}
}

func TestAppleMusicUnfocusedStorageDoesNotSchedulePickupAnimation(t *testing.T) {
	p := storageTestPage()
	defer p.Close()
	now := time.Unix(100, 0)
	for _, ms := range []int{0, 16, 500} {
		at := now.Add(time.Duration(ms) * time.Millisecond)
		p.draw(140, 45, at, 2)
		if p.rackAnimating(at) {
			t.Fatal("fully stowed storage kept starting a hidden pickup animation")
		}
	}
}

func TestAppleMusicStorageUsesOnePaletteBeforePickup(t *testing.T) {
	p := storageTestPage()
	defer p.Close()
	p.draw(140, 45, time.Unix(100, 0), 2)
	var first *appleHit
	for _, h := range p.hits {
		if h.kind != appleHitTrack {
			continue
		}
		if first == nil {
			copy := h
			first = &copy
			continue
		}
		a := p.screen.buf.CellAt(first.slot.Min.X+4, first.slot.Min.Y)
		b := p.screen.buf.CellAt(h.slot.Min.X+4, h.slot.Min.Y)
		if !npSameColor(a.Style.Bg, b.Style.Bg) {
			t.Fatal("stored tracks leaked their individual style colours")
		}
	}
}

func TestAppleMusicCursorOnlyHalfEjectsOneCassette(t *testing.T) {
	p := storageTestPage()
	defer p.Close()
	now := time.Unix(100, 0)
	p.draw(140, 45, now, 2)
	storageHover(p, 1, now.Add(10*time.Millisecond))
	p.draw(140, 45, now.Add(250*time.Millisecond), 2)
	if p.rackPickup.pop.value(now.Add(250*time.Millisecond)) != .5 {
		t.Fatal("cursor did not stop at half extraction")
	}
	minY, maxY, area := 45, 0, 0
	for _, h := range p.hits {
		if h.kind == appleHitCassette {
			minY = min(minY, h.y)
			maxY = max(maxY, h.y+h.h)
			area += h.w * h.h
		}
	}
	var source appleHit
	for _, h := range p.hits {
		if h.kind == appleHitTrack && h.index == 1 {
			source = h
		}
	}
	from := source.cassette
	from.Y -= 3
	var renderer deck.WalkmanRenderer
	frame := renderer.RenderWalkman(p.rackView, deckPaletteOf(appleNeon), deck.RestPose(), deck.WalkmanState{State: deck.State{Radii: deck.PackRadii(0, false)}, Cassette: 1, OnlyCassette: true, FlightFrom: from, FlightPose: source.pose, StorageCover: 1})
	full := appleFrameBounds(frame, 0, 3)
	if area == 0 || maxY-minY > (full.Dy()+1)/2 {
		t.Fatalf("visible %d rows, complete cassette %d rows: not half-exposed", maxY-minY, full.Dy())
	}
}

func TestAppleMusicCursorSwitchNeverExposesTwoCassettes(t *testing.T) {
	p := storageTestPage()
	defer p.Close()
	now := time.Unix(100, 0)
	p.draw(140, 45, now, 2)
	storageHover(p, 1, now.Add(10*time.Millisecond))
	p.draw(140, 45, now.Add(250*time.Millisecond), 2)
	storageHover(p, 3, now.Add(260*time.Millisecond))
	for _, ms := range []int{280, 320, 400, 450, 600} {
		p.draw(140, 45, now.Add(time.Duration(ms)*time.Millisecond), 2)
		ids := map[string]bool{}
		for _, h := range p.hits {
			if h.kind == appleHitCassette {
				ids[h.id] = true
			}
		}
		if len(ids) > 1 {
			t.Fatalf("cursor switch exposed multiple cassettes: %v", ids)
		}
	}
}

func TestAppleMusicStyleIsChosenOncePerPickupNotFromTrackID(t *testing.T) {
	p := storageTestPage()
	defer p.Close()
	p.level = motion.Off
	now := time.Unix(100, 0)
	seen := map[uint8]bool{}
	var previous uint8
	for i := range 12 {
		p.beginInsert(p.tracks[0], now.Add(time.Duration(i)*time.Second))
		chosen := p.insertStyle
		if chosen < 1 || chosen > 5 {
			t.Fatal("pickup did not choose a real cassette style")
		}
		if i > 0 && chosen == previous {
			t.Fatal("consecutive pickups reused a fixed ID-derived style")
		}
		seen[chosen], previous = true, chosen
		p.state.Track.ID = p.tracks[0].ID
		for j := range 3 {
			if p.mechanics(now.Add(time.Duration(j)*time.Second)).Style != chosen {
				t.Fatal("chosen colour changed after pickup")
			}
		}
	}
	if len(seen) < 2 {
		t.Fatal("pickups did not vary their style")
	}
}
