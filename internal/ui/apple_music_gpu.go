package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

type appleRendererMsg struct {
	gpu *deck.GPU
	err error
}

func (p *AppleMusicPage) openRenderer() tea.Cmd {
	return func() tea.Msg {
		if p.ctx.Err() != nil {
			return nil
		}
		g, err := deck.OpenGPU()
		// Close may race a slow driver initialization after the event loop exits.
		p.gpuMu.Lock()
		defer p.gpuMu.Unlock()
		if p.ctx.Err() != nil {
			if g != nil {
				g.Close()
			}
			return nil
		}
		p.gpu = g
		return appleRendererMsg{g, err}
	}
}

func (p *AppleMusicPage) closeRenderer() {
	p.gpuMu.Lock()
	defer p.gpuMu.Unlock()
	if p.gpu != nil {
		p.gpu.Close()
		p.gpu = nil
	}
}
