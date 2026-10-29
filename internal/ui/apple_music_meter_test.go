package ui

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/charmbracelet/x/ansi"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

func TestAppleMusicUnsupportedVUExplainsPlatformWithoutLinuxAdvice(t *testing.T) {
	p := NewAppleMusicPage(nil)
	defer p.Close()
	p.Update(appleMeterMsg{err: fmt.Errorf("meter: %w", applemusic.ErrMeterUnsupported)}, model.NewApp(model.DefaultOptions()))
	if !strings.Contains(p.meterError, "Linux") || strings.Contains(p.meterError, "parec") {
		t.Fatalf("unsupported platform got misleading advice: %q", p.meterError)
	}
	if p.meter != nil || p.meterOpening || !strings.Contains(p.status, p.meterError) {
		t.Fatal("unsupported sampler must remain unavailable and visible")
	}
	out := ansi.Strip(p.draw(120, 38, time.Unix(100, 0), 2))
	if !strings.Contains(out, "VU ERR") || strings.Contains(out, "VU METER · dBFS") {
		t.Fatal("unsupported sampler was presented as active")
	}
	exportPresentation(t, p, "unsupported-vu")
}

func TestAppleMusicVUStartsByDefaultAndCanBeToggled(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("parec sampler is Linux-only; platform unavailability is tested separately")
	}
	dir := t.TempDir()
	data := make([]byte, 8192)
	for i := 0; i < len(data); i += 8 {
		binary.LittleEndian.PutUint32(data[i:], math.Float32bits(.5))
	}
	pcm := filepath.Join(dir, "sample")
	if err := os.WriteFile(pcm, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "parec"), []byte("#!/bin/sh\n/bin/cat \"$TAPLE_TEST_PCM\"\nexec /bin/sleep 30\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("TAPLE_TEST_PCM", pcm)
	p := NewAppleMusicPage(&appleTestClient{})
	defer p.Close()
	a := model.NewApp(model.DefaultOptions())
	cmd := p.Init(a)
	if cmd == nil || p.meter != nil {
		t.Fatal("default startup must open the meter asynchronously")
	}
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 3 {
		t.Fatal("default startup did not schedule metering alongside polling")
	}
	if p.handleKey(tea.KeyPressMsg{Code: 'v'}) != nil {
		t.Fatal("duplicate meter open")
	}
	// GPU initialization has its own integration test; keep this PCM timing test independent of driver startup.
	for _, command := range batch[:2] {
		p.Update(command(), a)
	}
	deadline := time.Now().Add(time.Second)
	for !p.meter.Frame().Ready && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	f := p.meter.Frame()
	if !f.Ready || f.Levels[0] < .8 || f.Levels[1] != 0 {
		t.Fatalf("real PCM fixture not measured: %+v", f)
	}
	p.meterError = "temporary interruption"
	p.advance(time.Now())
	if p.meterError != "" {
		t.Fatal("fresh samples did not clear the old meter error")
	}
	out := ansi.Strip(p.draw(120, 38, time.Unix(100, 0), 2))
	if strings.Contains(out, "VU OFF") || !strings.Contains(out, "VU METER") {
		t.Fatal("active system mix is mislabeled")
	}
	if dir := os.Getenv("TAPLE_CATALOG_PREVIEW_DIR"); dir != "" {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		p.status = "FRAMEBUFFER TEST · 合成 PCM 校准信号，非真实播放"
		p.draw(140, 45, time.Unix(100, 0), 2)
		writeAppleFrame(t, p, filepath.Join(dir, "meter-enabled.html"))
	}
	cmd = p.handleKey(tea.KeyPressMsg{Code: 'v'})
	if cmd == nil || p.meter != nil {
		t.Fatal("VU did not switch off immediately")
	}
	cmd()
	p.draw(120, 38, time.Unix(100, 0), 2)
	for _, h := range p.hits {
		if h.key != "v" {
			continue
		}
		cmd = p.handleClick(h.x, h.y)
		if cmd == nil {
			t.Fatal("meter cannot be re-enabled from the UI")
		}
		p.Update(cmd(), a)
		if p.meter == nil {
			t.Fatal("UI meter restart failed")
		}
		return
	}
	t.Fatal("visible meter toggle missing")
}

func TestAppleMusicVUFailureIsNotReportedAsOff(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	p := NewAppleMusicPage(nil)
	defer p.Close()
	cmd := p.handleKey(tea.KeyPressMsg{Code: 'v'})
	if cmd == nil {
		t.Fatal("missing meter enable command")
	}
	p.Update(cmd(), model.NewApp(model.DefaultOptions()))
	out := ansi.Strip(p.draw(120, 38, time.Unix(100, 0), 2))
	if strings.Contains(out, "VU OFF") || !strings.Contains(out, "VU ERR") || p.status != T("meter.error", p.meterError) {
		t.Fatal("sampler failure hidden as disabled")
	}
	if p.handleKey(tea.KeyPressMsg{Code: 'v'}) == nil {
		t.Fatal("failed meter cannot be retried explicitly")
	}
}

func TestAppleMusicDefaultVUStartupUsesVisibleErrorPath(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	p := NewAppleMusicPage(&appleTestClient{})
	defer p.Close()
	a := model.NewApp(model.DefaultOptions())
	cmd := p.Init(a)
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) != 3 {
		t.Fatal("default startup did not start polling and metering asynchronously")
	}
	for _, command := range batch[:2] {
		p.Update(command(), a)
	}
	out := ansi.Strip(p.draw(120, 38, time.Unix(100, 0), 2))
	if !strings.Contains(out, "VU ERR") {
		t.Fatal("default meter startup failure fell back to VU OFF")
	}
}
