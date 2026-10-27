package ui

import (
	"context"
	"errors"
	"image"
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/motion"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
	"github.com/go-musicfox/go-musicfox/internal/ui/deck"
)

type appleMusicClient interface {
	Snapshot(context.Context) (applemusic.State, error)
	Catalog(context.Context, applemusic.CatalogRequest) (applemusic.CatalogPage, error)
	Lyrics(context.Context, applemusic.LyricsRequest) (applemusic.Lyrics, error)
	Do(context.Context, applemusic.Action) error
	Show(context.Context) error
}

type applePollMsg struct{}
type appleStateMsg struct {
	state      applemusic.State
	err        error
	generation uint64
	pollEpoch  uint64
}
type appleScene uint8

const (
	appleBrowse appleScene = iota
	appleListen
)

type appleDrawer uint8

const (
	appleSearch appleDrawer = iota
	appleLibrary
	appleQueue
)

type AppleMusicPage struct {
	appleCatalogState
	library                               appleCatalogState
	libraryKind                           string
	client                                appleMusicClient
	meter                                 *applemusic.Meter
	meterOpening                          bool
	meterError                            string
	ctx                                   context.Context
	cancel                                context.CancelFunc
	state                                 applemusic.State
	stateAt                               time.Time
	lyrics                                appleLyricsState
	stageRect                             image.Rectangle
	selected                              int
	selections                            [3]int
	drawer                                appleDrawer
	scene                                 appleScene
	panel                                 string
	input                                 textinput.Model
	editing, busy                         bool
	loggingOut                            bool
	searchKind                            string
	status                                string
	level                                 motion.Level
	tier                                  deck.Tier
	cellAspect                            float64
	rackPickup                            appleRackPickup
	vinylPickup                           appleVinylPickup
	rackFocused, rackGraphical            bool
	rackView                              deck.View
	rackHover                             int
	rackStart                             int
	rackStarts                            [3]int
	styleRNG                              *rand.Rand
	styles                                map[string]uint8
	lastCassetteStyle                     uint8
	operations                            []appleOperation
	generation                            uint64
	playGeneration, settledPlayGeneration uint64
	skipAcknowledged, pollEpoch           uint64
	pending                               bool
	pendingID, previousID                 string
	pendingCatalogID                      string
	pendingIndex                          int
	lastAction                            applemusic.Action
	camera, sceneMotion                   appleTween
	drawerTween, panelTween               appleTween
	zoomed                                bool
	restoreZoom, autoZoomAfterFold        bool
	inserting                             bool
	insertStart                           time.Time
	insertStyle                           uint8
	insertNumber                          int
	insertID                              string
	bayID                                 string
	bayStyle                              uint8
	bayNumber                             int
	insertOrigin                          deck.Rect
	insertPose                            deck.Pose
	insertSceneFrom                       float64
	insertOld                             deck.WalkmanState
	insertOldPresent                      bool
	insertEjectAt                         time.Time
	insertSourceCaptured                  bool
	swapStart                             time.Time
	swap                                  appleSwapState
	lastFrame, glitchAt                   time.Time
	reelAt                                time.Time
	reels                                 deck.Reels
	renderer                              deck.WalkmanRenderer
	gpuMu                                 sync.Mutex
	gpu                                   *deck.GPU
	renderBackend                         string
	screen                                npScreen
	desk                                  appleDeskStage
	hits                                  []appleHit
	hoverX, hoverY                        int
	mousePointer                          string
	counter                               CounterState
}

func NewAppleMusicPage(client appleMusicClient) *AppleMusicPage {
	ctx, cancel := context.WithCancel(context.Background())
	input := textinput.New()
	input.CharLimit = 128
	input.Placeholder = "搜索歌曲 / 专辑 / 歌手"
	return &AppleMusicPage{client: client, ctx: ctx, cancel: cancel, input: input,
		hoverX: -1, hoverY: -1, pendingIndex: -1, status: "正在后台恢复会话；需要登录时按 b",
		styleRNG: rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())), styles: make(map[string]uint8),
		searchKind: applemusic.CatalogSongs, libraryKind: applemusic.LibraryPlaylists,
		rackPickup:  appleRackPickup{current: appleRackChoice{index: -1}, wanted: appleRackChoice{index: -1}},
		vinylPickup: appleVinylPickup{current: -1, wanted: -1}}
}

func (p *AppleMusicPage) Type() model.PageType { return "apple_music" }
func (p *AppleMusicPage) Msg() tea.Msg         { return nil }
func (p *AppleMusicPage) IgnoreQuitKeyMsg(msg tea.KeyMsg) bool {
	return msg.String() != "ctrl+c" && (p.editing || p.panel != "")
}
func (p *AppleMusicPage) Close() error { p.cancel(); p.meter.Close(); p.closeRenderer(); return nil }
func (p *AppleMusicPage) Init(a *model.App) tea.Cmd {
	p.level, p.tier = a.MotionLevel(), deckTier(a)
	p.renderBackend = "CPU · 正在检测 " + deck.GPUBackend()
	return tea.Batch(p.poll(), p.toggleMeter(), p.openRenderer())
}
func (p *AppleMusicPage) poll() tea.Cmd {
	generation := p.playGeneration
	epoch := p.pollEpoch
	return func() tea.Msg {
		s, err := p.client.Snapshot(p.ctx)
		return appleStateMsg{state: s, err: err, generation: generation, pollEpoch: epoch}
	}
}

func (p *AppleMusicPage) pollDelay() time.Duration {
	if p.pending || p.busy || p.state.Playing {
		return 100 * time.Millisecond
	}
	return 500 * time.Millisecond
}

func (p *AppleMusicPage) acceptState(s applemusic.State, now time.Time) {
	if !s.Ready || !s.Authorized || (p.state.Storefront != "" && p.state.Storefront != s.Storefront) {
		p.invalidateLibrary()
	}
	p.stateAt = now
	if p.pending && p.pendingCatalogID != "" && s.Track.ID == p.pendingCatalogID && p.insertID == p.pendingID && p.insertStyle > 0 {
		p.styles[s.Track.ID] = p.insertStyle
	}
	if s.Ready && s.Track.ID != "" {
		if _, ok := p.styles[s.Track.ID]; !ok {
			p.styles[s.Track.ID] = p.nextCassetteStyle()
		}
	}
	if s.Authorized && !p.state.Authorized {
		p.status = T("status.signed_in")
	} else if s.Ready && !s.Authorized {
		p.status = T("status.unauthorized")
	}
	if p.state.Track.ID != "" && s.Track.ID != "" && s.Track.ID != p.state.Track.ID && p.level == motion.Full && !p.inserting && p.swapStart.IsZero() && !p.swap.armed {
		p.glitchAt = now
	}
	unchangedSkip := p.pending && (p.lastAction.Kind == "next" || p.lastAction.Kind == "previous") &&
		p.skipAcknowledged == p.playGeneration && s.Ready && s.Authorized && s.Track.ID != "" &&
		s.Track.ID == p.previousID && s.QueueIndex == p.pendingIndex
	if unchangedSkip {
		p.pending = false
		p.status = T("status.track_unchanged")
	}
	if s.Ready && s.Authorized && s.Full && (s.Playing || s.Paused) && s.Track.ID != "" {
		matches := !p.pending || (p.settledPlayGeneration == p.playGeneration && (p.pendingID == s.Track.ID || (p.pendingCatalogID != "" && p.pendingCatalogID == s.Track.ID)))
		if p.pending && p.lastAction.Kind == "jump" {
			matches = matches && s.QueueIndex == int(p.lastAction.Value)
		}
		if p.pending && p.pendingID == "" {
			matches = p.settledPlayGeneration == p.playGeneration && (s.Track.ID != p.previousID || s.QueueIndex != p.pendingIndex)
		}
		if matches {
			if p.pending {
				p.pending = false
				p.status = "正在播放"
				if s.Paused {
					p.status = "已暂停"
				}
			}
		}
	}
	p.state = s
	p.confirmArmedSwap(now)
	if p.hasFullTrackState() {
		p.swap.awaitState = false
	}
	p.bindSwapTarget(now)
	if !p.inserting && p.swapStart.IsZero() && !p.swap.armed && !p.pending && s.Ready && s.Track.ID != "" && p.bayID != s.Track.ID {
		p.bayID, p.bayStyle = s.Track.ID, p.styles[s.Track.ID]
		if s.Track.ID == p.insertID || s.Track.ID == p.pendingCatalogID {
			p.bayNumber = p.insertNumber
		} else if s.QueueTotal > 0 && s.QueueIndex >= 0 && s.QueueIndex < s.QueueTotal {
			p.bayNumber = s.QueueIndex + 1
		} else {
			p.bayNumber = 0
		}
	}
	p.selections[appleQueue] = min(p.selections[appleQueue], max(0, len(s.Queue)-1))
	if p.drawer == appleQueue {
		p.selected = min(p.selected, max(0, len(s.Queue)-1))
	}
}

func (p *AppleMusicPage) ready() bool {
	return p.hasFullTrackState() && (p.state.Playing || p.state.Paused)
}

// A stopped full track identifies the cassette, but does not confirm playback.
func (p *AppleMusicPage) hasFullTrackState() bool {
	return !p.pending && p.state.Ready && p.state.Authorized && p.state.Full && p.state.Track.ID != ""
}

func (p *AppleMusicPage) Update(msg tea.Msg, a *model.App) (model.Page, tea.Cmd) {
	var cmd tea.Cmd
	now := time.Now()
	switch m := msg.(type) {
	case appleRendererMsg:
		if m.err != nil {
			p.renderBackend = "CPU · " + m.err.Error()
		} else if m.gpu != nil {
			p.renderer.SetGPU(m.gpu)
			p.rackPickup.renderer.SetGPU(m.gpu)
			p.renderBackend = deck.GPUBackend() + " GPU · " + m.gpu.Name()
		}
	case appleMeterMsg:
		p.meterOpening, p.meter = false, m.meter
		if m.err != nil {
			p.meterError = T("meter.need_pulse")
			if errors.Is(m.err, applemusic.ErrMeterUnsupported) {
				p.meterError = T("meter.platform")
			}
			p.status = T("meter.error", p.meterError)
		} else {
			p.status = "已开启 VU 电平表"
		}
	case tea.WindowSizeMsg:
		p.hits = nil
		p.lyrics.area = appleBox(0, 0, 0, 0)
		p.stageRect = appleBox(0, 0, 0, 0)
		p.hoverX, p.hoverY = -1, -1
		p.mousePointer = "default"
		cmd = a.SetMousePointer("default")
	case applePollMsg:
		if p.loggingOut {
			cmd = tea.Tick(p.pollDelay(), func(time.Time) tea.Msg { return applePollMsg{} })
		} else {
			cmd = p.poll()
		}
	case appleStateMsg:
		if !p.loggingOut && m.generation == p.playGeneration && m.pollEpoch == p.pollEpoch {
			if m.err != nil {
				p.status = m.err.Error()
				p.state.Ready, p.state.Playing, p.state.Authorized, p.state.Full = false, false, false, false
				p.state.Paused = false
				p.invalidateLibrary()
			} else {
				p.acceptState(m.state, now)
			}
			cmd = tea.Batch(p.loadLyrics(false), p.ensureLibrary())
		}
		cmd = tea.Batch(cmd, tea.Tick(p.pollDelay(), func(time.Time) tea.Msg { return applePollMsg{} }))
	case appleLyricsMsg:
		p.acceptLyrics(m)
	case appleCatalogMsg:
		c := p.catalogState(m.drawer)
		if m.generation != c.searchGeneration {
			break
		}
		if m.err == nil && p.inserting && !p.insertSourceCaptured && m.drawer == p.drawer {
			c.waitingSearch = &m
			p.status = "搜索结果已就绪，取出后刷新列表"
		} else {
			p.acceptCatalog(m)
		}
	case appleActionMsg:
		cmd = p.acceptAction(m, now)
	case tea.KeyPressMsg:
		cmd = p.handleKey(m)
	case tea.MouseClickMsg:
		if m.Button == tea.MouseLeft {
			cmd = p.handleClick(m.X, m.Y)
		}
	case tea.MouseWheelMsg:
		if p.panel == "" && p.scene == appleListen && !p.inserting && m.X >= p.lyrics.area.Min.X && m.X < p.lyrics.area.Max.X && m.Y >= p.lyrics.area.Min.Y && m.Y < p.lyrics.area.Max.Y {
			if m.Button == tea.MouseWheelUp {
				p.scrollLyrics(-1)
			}
			if m.Button == tea.MouseWheelDown {
				p.scrollLyrics(1)
			}
		} else if p.panel == "" && (p.scene == appleBrowse || (p.inserting && now.Before(p.insertStart))) {
			if m.Button == tea.MouseWheelUp {
				cmd = p.handleKey(tea.KeyPressMsg{Code: tea.KeyUp})
			}
			if m.Button == tea.MouseWheelDown {
				cmd = p.handleKey(tea.KeyPressMsg{Code: tea.KeyDown})
			}
		}
	case tea.MouseMotionMsg:
		p.hoverX, p.hoverY = m.X, m.Y
		p.rackFocused, p.rackHover = false, -1
		pointer := "default"
		if hit := p.hitAt(m.X, m.Y); hit != nil && hit.kind != appleHitMoving {
			pointer = "pointer"
			if (hit.kind == appleHitTrack || hit.kind == appleHitCassette) && hit.index >= 0 && hit.index < len(p.items()) && p.items()[hit.index].ID == hit.id {
				p.rackHover = hit.index
			} else if hit.kind == appleHitCatalog && hit.index >= 0 && hit.index < len(p.catalogState(p.drawer).catalog.items) && p.catalogState(p.drawer).catalog.items[hit.index].ID == hit.id {
				p.rackHover = hit.index
			}
			if hit.key == "/" {
				pointer = "text"
			}
		}
		if pointer != p.mousePointer {
			p.mousePointer = pointer
			cmd = a.SetMousePointer(pointer)
		}
	default:
		if p.editing && p.panel == "" {
			p.input, cmd = p.input.Update(msg)
		}
	}
	p.advance(now)
	if p.meter != nil && p.meter.Frame().Ready {
		a.AnimateSlow(100 * time.Millisecond)
	}
	if p.level != motion.Off {
		if p.camera.active(now) || p.sceneMotion.active(now) || p.drawerTween.active(now) || p.panelTween.active(now) || p.inserting || !p.swapStart.IsZero() || p.rackAnimating(now) || (p.ready() && p.state.Playing) {
			a.Animate(50 * time.Millisecond)
		} else if p.state.Playing || p.reels.Moving() || (a.WindowWidth() >= 60 && a.WindowHeight() >= 20) {
			a.AnimateSlow(100 * time.Millisecond)
		}
	}
	return p, cmd
}

func (p *AppleMusicPage) handleKey(key tea.KeyPressMsg) tea.Cmd {
	k := key.String()
	if k == "ctrl+c" {
		return tea.Quit
	}
	if p.loggingOut {
		return nil
	}
	if p.panel == appleLogoutPanel {
		switch k {
		case "esc", "enter", "n", "N":
			p.panel, p.hits = "", nil
		case "y", "Y":
			if p.screen.w == 0 || (p.screen.w >= 40 && p.screen.h >= 12) {
				return p.confirmLogout()
			}
		}
		return nil
	}
	if p.panel != "" {
		switch k {
		case "esc":
			p.panel = ""
			p.hits = nil
			return nil
		case "l", "L", "tab":
			if currentLang() == "zh" {
				SetLanguage("en")
			} else {
				SetLanguage("zh")
			}
			p.panel = T("panel.title")
			return nil
		case "b", "B":
			return p.openAccount(k == "B")
		}
		return nil
	}
	if p.editing {
		switch k {
		case "esc":
			p.editing = false
			p.input.Blur()
			return nil
		case "enter":
			if p.searching {
				return nil
			}
			return p.searchCatalog(p.searchKind)
		default:
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(key)
			return cmd
		}
	}
	now := time.Now()
	canChoose := p.scene == appleBrowse || p.inserting || p.sceneMotion.active(now)
	var action applemusic.Action
	switch k {
	case "q":
		return tea.Quit
	case "esc":
		if p.zoomed {
			p.setZoom(false, now)
		} else if p.scene == appleListen || p.inserting {
			p.browse(now)
		} else {
			p.backCatalog()
		}
		return nil
	case "backspace":
		if p.scene == appleBrowse {
			p.backCatalog()
		}
		return nil
	case "4", "5", "6", "7", "8":
		if p.drawer == appleLibrary {
			return p.loadLibrary(libraryKinds[k[0]-'4'])
		}
		if k == "7" || k == "8" {
			return nil
		}
		p.switchDrawer(appleSearch, now)
		return p.searchCatalog([]string{applemusic.CatalogSongs, applemusic.CatalogAlbums, applemusic.CatalogArtists}[k[0]-'4'])
	case "l":
		return p.moreCatalog()
	case "r":
		return p.refreshCatalog()
	case "P":
		if canChoose {
			return p.playCatalogCollection()
		}
		return nil
	case "v":
		return p.toggleMeter()
	case "y":
		return p.loadLyrics(true)
	case "f":
		if p.scene == appleListen {
			p.lyrics.manual = false
			if !p.lyrics.data.Synced {
				p.lyrics.top = 0
			}
		}
		return nil
	case "pgup", "pgdown":
		delta := max(1, p.lyrics.area.Dy()-2)
		if k == "pgup" {
			delta = -delta
		}
		p.scrollLyrics(delta)
		return nil
	case "/", "1":
		return p.editSearch(now)
	case "tab":
		return p.switchDrawer((p.drawer+1)%3, now)
	case "2", "3":
		return p.switchDrawer(appleDrawer(k[0]-'1'), now)
	case "i":
		if p.scene == appleBrowse {
			p.enterDetail(now)
		} else {
			p.browse(now)
		}
		return nil
	case "up", "k", "down", "j":
		if p.scene == appleListen && !p.inserting && !p.sceneMotion.active(now) {
			delta := 1
			if k == "up" || k == "k" {
				delta = -1
			}
			p.scrollLyrics(delta)
			return nil
		}
		if canChoose {
			if !p.rackFocused && p.rackHover >= 0 {
				p.selected = p.rackHover
			}
			p.rackFocused = true
			if k == "up" || k == "k" {
				p.selected = max(0, p.selected-1)
			} else {
				p.selected = min(max(0, p.listCount()-1), p.selected+1)
			}
		}
		p.hoverX, p.hoverY, p.rackHover = -1, -1, -1
		return nil
	case "b", "B":
		return p.openAccount(k == "B")
	case "z", "Z":
		p.setZoom(!p.zoomed, now)
		return nil
	case "o", "s":
		p.openPanel(now)
		return nil
	case "enter", "a":
		if !canChoose {
			return nil
		}
		if p.catalogContainers() {
			items := p.catalogState(p.drawer).catalog.items
			if k == "enter" && !p.rackFocused {
				if hit := p.hitAt(p.hoverX, p.hoverY); hit != nil && hit.kind == appleHitCatalog {
					return p.openCatalog(hit.index, hit.id)
				}
			}
			if k == "enter" && p.selected >= 0 && p.selected < len(items) {
				return p.openCatalog(p.selected, items[p.selected].ID)
			}
			return nil
		}
		items := p.items()
		if k == "enter" && !p.rackFocused {
			if hit := p.hitAt(p.hoverX, p.hoverY); hit != nil && (hit.kind == appleHitTrack || hit.kind == appleHitCassette) {
				if hit.index < 0 || hit.index >= len(items) || items[hit.index].ID != hit.id {
					p.status = "列表已变化，请重新选择"
					return nil
				}
				p.selected = hit.index
			}
		}
		if p.selected < 0 || p.selected >= len(items) {
			return nil
		}
		item := items[p.selected]
		if p.drawer != appleQueue && item.Type == "library-songs" && !item.Playable {
			p.status = "此资料库歌曲未提供可播放信息"
			return nil
		}
		if k == "enter" && (p.inserting || (p.scene == appleListen && p.sceneMotion.active(now))) && p.insertID == item.ID && p.insertNumber == p.selected+1 {
			return nil
		}
		action = applemusic.Action{Kind: "play", ID: item.ID, ResourceType: item.Type}
		if p.drawer == appleQueue {
			if k == "a" {
				return nil
			}
			action.Kind, action.Value = "jump", float64(p.selected)
		} else if k == "a" {
			action.Kind = "enqueue"
		} else if resource := appleCollectionResource(p.catalogState(p.drawer).catalog.request.Kind); resource != "" {
			action = applemusic.Action{Kind: "play-collection", ID: p.catalogState(p.drawer).catalog.request.ID, ResourceType: resource, StartID: item.ID}
		}
		if k == "enter" {
			p.beginInsert(item, now)
		}
	case "space":
		if p.state.Track.ID == "" && p.state.QueueTotal == 0 {
			p.status = "先搜索并选择一首歌曲"
			return nil
		}
		action.Kind = "toggle"
	case "n", "p":
		action.Kind = "next"
		if k == "p" {
			action.Kind = "previous"
		}
		p.armSwap(now)
	case "left":
		action = applemusic.Action{Kind: "seek-relative", ID: p.state.Track.ID, Value: -10}
	case "right":
		action = applemusic.Action{Kind: "seek-relative", ID: p.state.Track.ID, Value: 10}
	case "-":
		action = applemusic.Action{Kind: "volume-relative", Value: -.05}
	case "=", "+":
		action = applemusic.Action{Kind: "volume-relative", Value: .05}
	default:
		return nil
	}
	return p.dispatch(action)
}

func (p *AppleMusicPage) editSearch(now time.Time) tea.Cmd {
	p.switchDrawer(appleSearch, now)
	p.setZoom(false, now)
	p.editing = true
	return p.input.Focus()
}

func (p *AppleMusicPage) openPanel(now time.Time) {
	p.hits = nil
	p.panel = T("panel.title")
	p.panelTween = appleTween{from: .25, to: .25}
	p.panelTween.start(1, now, 160*time.Millisecond, p.level)
}

func (p *AppleMusicPage) switchDrawer(drawer appleDrawer, now time.Time) tea.Cmd {
	changed := p.drawer != drawer
	p.selections[p.drawer] = p.selected
	p.rackStarts[p.drawer] = p.rackStart
	p.drawer, p.selected = drawer, p.selections[drawer]
	p.rackStart = p.rackStarts[drawer]
	if p.scene != appleBrowse || p.inserting {
		p.browse(now)
	} else {
		p.rackFocused = false
		p.hoverX, p.hoverY = -1, -1
		p.resetRackPickup()
	}
	if changed {
		p.drawerTween = appleTween{from: 0, to: 0}
		p.drawerTween.start(1, now, 220*time.Millisecond, p.level)
	}
	return p.ensureLibrary()
}

func (p *AppleMusicPage) items() []applemusic.Track {
	switch p.drawer {
	case appleQueue:
		return p.state.Queue
	case appleLibrary:
		return p.library.tracks
	default:
		return p.tracks
	}
}

func (p *AppleMusicPage) View(a *model.App) string {
	return p.draw(a.WindowWidth(), a.WindowHeight(), a.FrameTime(), deckAspect(a))
}

func (p *AppleMusicPage) advance(now time.Time) {
	p.gpuMu.Lock()
	if p.gpu != nil && p.gpu.Err() != nil {
		p.renderBackend = "CPU · " + p.gpu.Err().Error()
	}
	p.gpuMu.Unlock()
	meterFrame := p.meter.Frame()
	if meterFrame.Ready && p.meterError != "" {
		p.meterError = ""
		p.status = "VU 电平表已恢复"
	}
	if err := meterFrame.Error; err != "" && err != p.meterError {
		p.meterError = err
		p.status = "VU 不可用：" + err + "；v 重试"
	}
	p.lastFrame = now
	if p.inserting && !now.Before(p.insertStart.Add(660*time.Millisecond)) {
		p.bayID, p.bayStyle, p.bayNumber = p.insertID, p.insertStyle, p.insertNumber
	}
	if p.inserting && !now.Before(p.insertStart.Add(1280*time.Millisecond)) {
		if p.swap.armed {
			p.swap.old = p.mechanics(now)
		}
		p.inserting = false
		p.resetRackPickup()
		p.sceneMotion = appleTween{from: 1, to: 1}
		if p.restoreZoom {
			p.setZoom(true, p.insertStart.Add(1280*time.Millisecond))
		}
	}
	if p.autoZoomAfterFold && !p.inserting && !p.sceneMotion.active(now) {
		p.autoZoomAfterFold = false
		if p.scene == appleListen {
			foldEnd := now
			if !p.sceneMotion.at.IsZero() && p.sceneMotion.duration > 0 {
				foldEnd = p.sceneMotion.at.Add(p.sceneMotion.duration)
			}
			p.setZoom(true, foldEnd)
		}
	}
	p.advanceSwap(now)
	for _, c := range []*appleCatalogState{&p.appleCatalogState, &p.library} {
		if c.waitingSearch != nil && (!p.inserting || p.insertSourceCaptured) {
			p.acceptCatalog(*c.waitingSearch)
		}
	}
	playing := p.ready() && p.state.Playing
	if p.inserting && now.Before(p.insertStart.Add(660*time.Millisecond)) {
		playing = false
	}
	if !p.swapStart.IsZero() && (p.swap.insertAt.IsZero() || now.Before(p.swap.insertAt.Add(340*time.Millisecond))) {
		playing = false
	}
	// Mouse events must not force a new ray-cast for every fractional reel step.
	if p.level != motion.Off && (p.reelAt.IsZero() || now.Sub(p.reelAt) >= time.Second/60) {
		dt := min(max(now.Sub(p.reelAt).Seconds(), 0), .1)
		p.reelAt = now
		p.reels.Advance(dt, playing, now, deck.PackRadii(p.progress(), false), 1)
	}
	p.counter.Update(now, appleDuration(p.state.Position))
}

func (p *AppleMusicPage) progress() float64 {
	if p.hasFullTrackState() && p.state.Track.Duration > 0 {
		return min(max(p.state.Position/p.state.Track.Duration, 0), 1)
	}
	return 0
}

func (p *AppleMusicPage) nextCassetteStyle() uint8 {
	n := len(deck.CassetteStyles)
	if p.lastCassetteStyle > 0 {
		n--
	}
	style := uint8(1 + p.styleRNG.IntN(n))
	if p.lastCassetteStyle > 0 && style >= p.lastCassetteStyle {
		style++
	}
	p.lastCassetteStyle = style
	return style
}

func appleDuration(seconds float64) time.Duration {
	if math.IsNaN(seconds) || math.IsInf(seconds, 0) {
		return 0
	}
	return time.Duration(min(max(seconds, 0), 86400) * float64(time.Second))
}
