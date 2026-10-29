package applemusic

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestStereoMeterMeasuresSignalAndRejectsNonFiniteSamples(t *testing.T) {
	data := make([]byte, 8*16)
	for i := 0; i < len(data); i += 8 {
		binary.LittleEndian.PutUint32(data[i:], math.Float32bits(1))
		binary.LittleEndian.PutUint32(data[i+4:], math.Float32bits(0))
	}
	if levels := meterLevels(data); levels[0] != 1 || levels[1] != 0 {
		t.Fatalf("levels=%v", levels)
	}
	for i := range data {
		data[i] = 0xff
	}
	if levels := meterLevels(data); levels != [2]float64{} {
		t.Fatalf("invalid samples=%v", levels)
	}
}

func TestMeterRMSCalibration(t *testing.T) {
	for _, db := range []float64{-60, -48, -24, -12, -6, 0} {
		data := make([]byte, 8*32)
		for i := 0; i < len(data); i += 8 {
			binary.LittleEndian.PutUint32(data[i:], math.Float32bits(float32(math.Pow(10, db/20))))
		}
		levels := meterLevels(data)
		want := min(max((db+48)/48, 0), 1)
		if math.Abs(levels[0]-want) > 1e-6 || levels[1] != 0 {
			t.Fatalf("%.0f dBFS: got %v want %.3f", db, levels, want)
		}
	}
}

func TestMeterSamplerFailureAndStaleData(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("parec sampler is Linux-only; see TestMeterPlatformBoundary")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "parec"), []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	m, err := OpenMeter(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	select {
	case <-m.done:
	case <-time.After(time.Second):
		t.Fatal("failed sampler did not exit")
	}
	if frame := m.Frame(); frame.Ready || frame.Error == "" {
		t.Fatalf("sampler failure hidden: %+v", frame)
	}
	m.frame.Store(MeterFrame{Ready: true, Levels: [2]float64{1, 1}, captured: time.Now().Add(-time.Second)})
	if frame := m.Frame(); frame.Ready || frame.Levels != [2]float64{} || frame.Error == "" {
		t.Fatal("stale samples kept lighting the meter")
	}
	m.frame.Store(MeterFrame{Ready: true, captured: time.Now()})
	if frame := m.Frame(); !frame.Ready || frame.Error != "" {
		t.Fatal("fresh silent PCM was treated as failure")
	}
}

func TestMeterPlatformBoundary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	m, err := OpenMeter(context.Background())
	if m != nil {
		m.Close()
		t.Fatal("meter started without a sampler")
	}
	if runtime.GOOS != "linux" && !errors.Is(err, ErrMeterUnsupported) {
		t.Fatalf("unsupported platform did not report its limitation: %v", err)
	}
	if runtime.GOOS == "linux" && (err == nil || errors.Is(err, ErrMeterUnsupported)) {
		t.Fatalf("Linux must attempt the real sampler: %v", err)
	}
}

func TestSystemOutputMeterLive(t *testing.T) {
	if os.Getenv("TAPLE_METER_LIVE") != "1" {
		t.Skip("set TAPLE_METER_LIVE=1 for system-output sampling; includes other apps")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	m, err := OpenMeter(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer m.Close()
	for ctx.Err() == nil {
		f := m.Frame()
		if f.Ready {
			t.Logf("system-output PCM received; nonzero=%t", f.Levels[0] > 0 || f.Levels[1] > 0)
			for until := time.Now().Add(650 * time.Millisecond); time.Now().Before(until); {
				time.Sleep(25 * time.Millisecond)
				if !m.Frame().Ready {
					t.Fatal("system-output meter stalled between sample batches")
				}
			}
			return
		}
		if f.Error != "" {
			t.Fatal(f.Error)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("no PCM received")
}
