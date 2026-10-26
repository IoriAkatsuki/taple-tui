package ui

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/anhoder/foxful-cli/motion"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

func TestAppleMusicSeatedTapeSurvivesSlowBackend(t *testing.T) {
	for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
		p := NewAppleMusicPage(&appleMouseClient{})
		p.level = level
		now := time.Unix(100, 0)
		track := applemusic.Track{ID: "new", Name: "Delayed load"}
		p.beginInsert(track, now)
		p.dispatch(applemusic.Action{Kind: "play", ID: track.ID})
		for _, delay := range []time.Duration{1279 * time.Millisecond, 1280 * time.Millisecond, 5 * time.Second} {
			at := p.insertStart.Add(delay)
			p.advance(at)
			p.acceptState(applemusic.State{Ready: true, Authorized: true}, at)
			st := p.mechanics(at)
			if st.Cassette < 0 || st.Style != p.insertStyle {
				t.Fatalf("%v at %v: seated cassette vanished or changed: %+v", level, delay, st)
			}
			if p.ready() || st.Angles != [2]float64{} {
				t.Fatal("visual cassette fabricated backend readiness")
			}
		}
		p.settledPlayGeneration = p.playGeneration
		p.acceptState(applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Track: track}, now.Add(6*time.Second))
		if !p.ready() || p.mechanics(now.Add(6*time.Second)).Style != p.insertStyle {
			t.Fatal("late confirmation changed the installed cassette")
		}
		p.Close()
	}
}

func TestAppleMusicLyricLayoutReusesUnchangedText(t *testing.T) {
	p, now := lyricTestPage(t)
	p.lyrics.area = appleBox(0, 0, 60, 20)
	p.layoutLyricRows(60, 0, true)
	allocs := testing.AllocsPerRun(20, func() { p.layoutLyricRows(60, time.Second, true) })
	if allocs > 0 {
		t.Fatalf("unchanged lyrics were rewrapped: %.0f allocations per frame", allocs)
	}
	p.layoutLyricRows(12, 0, true)
	if len(p.lyrics.rows) <= len(p.lyrics.data.Lines) {
		t.Fatal("resize did not rebuild wrapped rows")
	}
	p.lyrics.data.Lines = []applemusic.LyricLine{{Text: "replacement"}}
	p.lyrics.scrollInit = false
	p.draw(140, 45, now, 2)
	if len(p.lyrics.rows) != 1 || p.lyrics.rows[0].text != "replacement" {
		t.Fatal("replacement lyrics reused stale layout")
	}
}

func TestAppleMusicCancelledPickupKeepsWholeBayIdentity(t *testing.T) {
	p := NewAppleMusicPage(&appleMouseClient{})
	defer p.Close()
	p.level = motion.Full
	p.state = applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Track: applemusic.Track{ID: "old"}}
	p.bayID, p.bayStyle, p.bayNumber = "old", 3, 1
	p.selected = 1
	now := time.Unix(100, 0)
	p.beginInsert(applemusic.Track{ID: "new"}, now)
	p.dispatch(applemusic.Action{Kind: "play", ID: "new"})
	p.browse(now.Add(100 * time.Millisecond))
	st := p.mechanics(now.Add(101 * time.Millisecond))
	if st.Style != 3 || st.TrackNum != 1 {
		t.Fatalf("cancelled pickup mixed old/new identities: %+v", st)
	}
	p.settledPlayGeneration = p.playGeneration
	s := p.state
	s.Track.ID = "new"
	p.acceptState(s, now.Add(2*time.Second))
	st = p.mechanics(now.Add(2 * time.Second))
	if st.Style != p.insertStyle || st.TrackNum != 2 {
		t.Fatalf("late load kept old track number: %+v", st)
	}
	p.styles["new"] = p.insertStyle%5 + 1
	p.acceptState(s, now.Add(3*time.Second))
	if p.mechanics(now.Add(3*time.Second)).Style != p.insertStyle {
		t.Fatal("hover preview recolored the installed cassette")
	}
}

func BenchmarkAppleMusicLyricsEmpty(b *testing.B) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.lyrics.status = "loading"
	p.screen.reset(80, 3)
	p.drawLyricsEmpty(0, 0, 80, 3, 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.drawLyricsEmpty(0, 0, 80, 3, 1)
	}
}

func BenchmarkAppleMusicFrame(b *testing.B) {
	benchmarkAppleMusicFrame(b, nil)
}

func BenchmarkAppleMusicFrameVulkan(b *testing.B) {
	g, err := deck.OpenVulkan()
	if err != nil {
		b.Skip(err)
	}
	defer g.Close()
	benchmarkAppleMusicFrame(b, g)
	if g.Err() != nil {
		b.Fatal(g.Err())
	}
}

func benchmarkAppleMusicFrame(b *testing.B, gpu *deck.Vulkan) {
	for _, kind := range []string{"playing", "pickup", "flight", "lyrics"} {
		b.Run(kind, func(b *testing.B) {
			p := NewAppleMusicPage(nil)
			defer p.Close()
			p.styleRNG = rand.New(rand.NewPCG(1, 2))
			p.renderer.SetVulkan(gpu)
			p.rackPickup.renderer.SetVulkan(gpu)
			p.tier, p.level = deck.Octant, motion.Full
			p.state = applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Storefront: "us", Track: applemusic.Track{ID: "a", Duration: 240}, Position: 10}
			p.tracks = []applemusic.Track{{ID: "b", Name: "Benchmark"}}
			now := time.Unix(100, 0)
			if kind == "playing" || kind == "lyrics" {
				p.scene, p.sceneMotion.to, p.camera.to = appleListen, 1, 1
			}
			if kind == "lyrics" {
				p.lyrics.data = applemusic.Lyrics{TrackID: "a", Storefront: "us", Synced: true}
				for i := range 300 {
					p.lyrics.data.Lines = append(p.lyrics.data.Lines, applemusic.LyricLine{Text: fmt.Sprintf("Line %d · 测试歌词与长句换行 performance", i), Start: time.Duration(i) * time.Second, End: time.Duration(i+1) * time.Second})
				}
			}
			p.rackFocused = kind == "pickup"
			p.draw(140, 45, now, 2)
			if kind == "flight" {
				p.beginInsert(p.tracks[0], now)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				at := now.Add(time.Duration(i+1) * (time.Second / 60))
				if kind == "pickup" {
					p.rackPickup.pop.at = at.Add(-time.Duration(50+i%100) * time.Millisecond)
				}
				if kind == "flight" {
					at = p.insertStart.Add(time.Duration(81+i%298) * time.Millisecond)
				}
				p.stateAt = at
				p.draw(140, 45, at, 2)
			}
		})
	}
}
