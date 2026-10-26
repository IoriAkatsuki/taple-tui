package applemusic

import (
	"context"
	"encoding/binary"
	"io"
	"math"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"
)

type MeterFrame struct {
	Levels   [2]float64
	Ready    bool
	Error    string
	captured time.Time
}

type Meter struct {
	frame  atomic.Value
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once
}

// OpenMeter measures system output, not the microphone, and
// never writes audio to disk. Other applications on the same sink are included.
func OpenMeter(ctx context.Context) (*Meter, error) {
	ctx, cancel := context.WithCancel(ctx)
	// The default capture buffer arrives in multi-second bursts, not live meter frames.
	cmd := exec.CommandContext(ctx, "parec", "--raw", "--device=@DEFAULT_MONITOR@", "--format=float32le", "--rate=24000", "--channels=2", "--latency-msec=50", "--client-name=tapefox", "--stream-name=Apple Music system-output VU")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		cancel()
		_ = stdout.Close()
		return nil, err
	}
	m := &Meter{cancel: cancel, done: make(chan struct{})}
	m.frame.Store(MeterFrame{})
	go func() {
		defer close(m.done)
		defer stdout.Close()
		data := make([]byte, 8192)
		var previous [2]float64
		for {
			if _, err := io.ReadFull(stdout, data); err != nil {
				break
			}
			levels := meterLevels(data)
			for i := range levels {
				levels[i] = max(levels[i], previous[i]*.82)
			}
			previous = levels
			m.frame.Store(MeterFrame{Levels: levels, Ready: true, captured: time.Now()})
		}
		_ = cmd.Wait()
		frame := MeterFrame{}
		if ctx.Err() == nil {
			frame.Error = "系统输出采样中断，请检查 PulseAudio/PipeWire"
		}
		m.frame.Store(frame)
	}()
	return m, nil
}

func (m *Meter) Frame() MeterFrame {
	if m == nil {
		return MeterFrame{}
	}
	frame := m.frame.Load().(MeterFrame)
	if frame.Ready && time.Since(frame.captured) > 300*time.Millisecond {
		return MeterFrame{Error: "系统输出采样无新数据"}
	}
	return frame
}

func (m *Meter) Close() {
	if m != nil {
		m.once.Do(func() { m.cancel(); <-m.done })
	}
}

func meterLevels(data []byte) (levels [2]float64) {
	n := len(data) / 8
	if n == 0 {
		return
	}
	for i := range n {
		for channel := range 2 {
			s := float64(math.Float32frombits(binary.LittleEndian.Uint32(data[i*8+channel*4:])))
			if !math.IsNaN(s) && !math.IsInf(s, 0) {
				levels[channel] += s * s
			}
		}
	}
	for i := range levels {
		if levels[i] > 0 {
			levels[i] = min(max((10*math.Log10(levels[i]/float64(n))+48)/48, 0), 1)
		}
	}
	return
}
