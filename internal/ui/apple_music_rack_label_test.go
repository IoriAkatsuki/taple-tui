package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/anhoder/foxful-cli/motion"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
)

func TestAppleMusicPoppedTrackKeepsLightTitle(t *testing.T) {
	for _, size := range [][2]int{{255, 68}, {140, 45}, {80, 30}, {50, 20}} {
		for _, hover := range []bool{false, true} {
			for _, index := range []int{0, 7} {
				for _, level := range []motion.Level{motion.Full, motion.Subtle, motion.Off} {
					t.Run(fmt.Sprintf("%dx%d/hover-%v/row-%d/%s", size[0], size[1], hover, index, level), func(t *testing.T) {
						p := NewAppleMusicPage(nil)
						defer p.Close()
						p.level, p.selected, p.rackFocused = level, index, !hover
						for i := range 8 {
							p.tracks = append(p.tracks, applemusic.Track{ID: fmt.Sprint(i), Name: fmt.Sprintf("曲目 %02d", i+1)})
						}
						p.tracks[index].Name = "夜航"
						p.state.Track = p.tracks[0]
						now := time.Unix(100, 0)
						p.draw(size[0], size[1], now, 2)
						var selected appleHit
						for _, hit := range p.hits {
							if hit.kind == appleHitTrack && hit.index == index {
								selected = hit
							}
						}
						if selected.slot.Empty() {
							t.Fatal("selected slot missing")
						}
						if hover {
							p.hoverX, p.hoverY = selected.slot.Min.X+1, selected.slot.Min.Y
						}
						p.draw(size[0], size[1], now.Add(10*time.Millisecond), 2)
						p.draw(size[0], size[1], now.Add(300*time.Millisecond), 2)
						var row strings.Builder
						left := selected.slot.Min.X
						if !p.rackGraphical {
							left-- // Text spines include a leading marker outside the inset hit box.
						}
						for x := left; x < selected.slot.Max.X; x++ {
							cell := p.screen.buf.CellAt(x, selected.slot.Min.Y)
							row.WriteString(cell.Content)
							if cell.Content == "夜" && (!npSameColor(cell.Style.Fg, appleNeon.Text) || cell.Style.Attrs&uv.AttrBold == 0) {
								t.Fatal("selected title is not light and bold")
							}
						}
						if !strings.Contains(row.String(), "夜航") {
							t.Fatalf("popped cassette hid its title: %q", row.String())
						}
						if index == 0 && !strings.Contains(row.String(), "▶") {
							t.Fatal("selected title lost the playing marker")
						}
						if dir := os.Getenv("TAPLE_LABEL_PREVIEW_DIR"); dir != "" && !hover && level == motion.Full {
							if err := os.MkdirAll(dir, 0700); err != nil {
								t.Fatal(err)
							}
							p.status = "FRAMEBUFFER TEST · 浅色选中歌名 · 非真实播放"
							p.draw(size[0], size[1], now.Add(320*time.Millisecond), 2)
							writeAppleFrame(t, p, filepath.Join(dir, fmt.Sprintf("label-%dx%d-row-%d.html", size[0], size[1], index)))
						}
					})
				}
			}
		}
	}
}
