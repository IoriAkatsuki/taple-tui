package ui

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/motion"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

func TestAppleMusicDefaultVulkanStartupAndClose(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	a := model.NewApp(model.DefaultOptions())
	commands := p.Init(a)().(tea.BatchMsg)
	if len(commands) != 3 {
		t.Fatal("default startup omitted asynchronous renderer initialization")
	}
	m := commands[2]().(appleRendererMsg)
	p.Update(m, a)
	if m.err != nil {
		if !strings.Contains(p.renderBackend, "CPU") {
			t.Fatal("Vulkan failure was hidden")
		}
		if os.Getenv("TAPEFOX_REQUIRE_VULKAN") == "1" {
			t.Fatal(m.err)
		}
		return
	}
	if p.gpu == nil || !strings.Contains(p.renderBackend, m.gpu.Name()) {
		t.Fatal("default page did not own/display the GPU")
	}
	p.tier = deck.Octant
	p.draw(140, 45, time.Unix(100, 0), 2)
	if m.gpu.Err() != nil {
		t.Fatal(m.gpu.Err())
	}
	p.Close()
	if p.gpu != nil {
		t.Fatal("page close retained the GPU device")
	}
	if msg := p.openRenderer()(); msg != nil {
		t.Fatal("closed page reopened a GPU device")
	}
}

func TestAppleMusicActivePollingAndVulkanPreview(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.styleRNG = rand.New(rand.NewPCG(1, 2))
	if p.pollDelay() != 500*time.Millisecond {
		t.Fatal("idle polling is too frequent")
	}
	p.pending = true
	if p.pollDelay() != 100*time.Millisecond {
		t.Fatal("loading feedback still waits half a second")
	}
	p.pending = false
	p.state.Playing = true
	if p.pollDelay() != 100*time.Millisecond {
		t.Fatal("lyric clock polling is too slow")
	}
	dir := os.Getenv("TAPEFOX_GPU_PREVIEW_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	m := p.openRenderer()().(appleRendererMsg)
	if m.err != nil {
		t.Fatal(m.err)
	}
	p.Update(m, model.NewApp(model.DefaultOptions()))
	p.level, p.tier = motion.Full, deck.Octant
	p.state = applemusic.State{Ready: true, Authorized: true, Full: true, Playing: true, Track: applemusic.Track{ID: "old", Name: "GPU framebuffer test", Duration: 240}, Storefront: "us", Position: 12}
	p.tracks = []applemusic.Track{{ID: "new", Name: "Vulkan compute · 合成测试"}}
	now := time.Unix(100, 0)
	p.draw(140, 45, now, 2)
	p.beginInsert(p.tracks[0], now)
	p.pending, p.pendingID = true, "new"
	p.state.Track = applemusic.Track{}
	for _, ms := range []int{0, 80, 379, 380, 500, 660, 760, 1080, 1279, 1280, 1455, 1630, 5000} {
		at := p.insertStart.Add(time.Duration(ms) * time.Millisecond)
		p.status = "GPU FRAMEBUFFER TEST · SYNTHETIC · 非真实播放"
		p.draw(140, 45, at, 2)
		writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("gpu-%04d.html", ms)))
	}
	for _, size := range [][2]int{{120, 24}, {80, 30}, {50, 20}} {
		for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
			p.level = level
			p.draw(size[0], size[1], p.insertStart.Add(6*time.Second), 2)
			writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("gpu-%dx%d-%s.html", size[0], size[1], level)))
		}
	}
	if m.gpu.Err() != nil {
		t.Fatal(m.gpu.Err())
	}
}
