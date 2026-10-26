package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

type appleMeterMsg struct {
	meter *applemusic.Meter
	err   error
}

func (p *AppleMusicPage) toggleMeter() tea.Cmd {
	if p.meterOpening {
		return nil
	}
	old := p.meter
	p.meter = nil
	if old != nil && p.meterError == "" && old.Frame().Error == "" {
		p.status = "已关闭 VU 电平表"
		return func() tea.Msg { old.Close(); return nil }
	}
	p.meterOpening, p.meterError = true, ""
	p.status = "正在开启 VU 电平表…"
	return func() tea.Msg {
		old.Close()
		meter, err := applemusic.OpenMeter(p.ctx)
		return appleMeterMsg{meter, err}
	}
}
