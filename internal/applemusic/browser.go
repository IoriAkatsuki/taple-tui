// Package applemusic controls an isolated official web player without exporting credentials.
package applemusic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

type Browser struct {
	pipe                *pipe
	cmd                 *exec.Cmd
	exited              chan struct{}
	close               sync.Once
	sessionOnce         sync.Once
	sessionGate         chan struct{}
	targetMu            sync.Mutex
	target              string
	session             string
	lifecycle           sync.RWMutex
	ctx                 context.Context
	binary, profile     string
	profileInfo         os.FileInfo
	background, stopped bool
	launchDone          chan struct{}
	launchCancel        context.CancelFunc
	launchErr           error
	queueMu             sync.Mutex
	lastQueueRev        int64
	lastQueue           []Track
	lastQueueTarget     string
	lastQueueStorefront string
	lastQueueAuth       bool
}

func preflight(profile string) (string, string, os.FileInfo, error) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		return "", "", nil, errors.New("apple-music 支持 Linux，Windows/macOS 为实验性支持")
	}
	binary, err := findChrome()
	if err != nil {
		return "", "", nil, err
	}
	if profile == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			return "", "", nil, err
		}
		profile = filepath.Join(dir, "tapefox", "apple-music", "chrome")
	}
	info, err := prepareProfile(profile)
	if err != nil {
		return "", "", nil, err
	}
	return binary, profile, info, nil
}

func Open(ctx context.Context, profile string) (*Browser, error) {
	binary, profile, _, err := preflight(profile)
	if err != nil {
		return nil, err
	}
	return launchBrowser(ctx, binary, profile, true)
}

// OpenAsync validates the environment and profile root synchronously, then
// launches the headless browser session in the background so the TUI renders immediately.
func OpenAsync(ctx context.Context, profile string) (*Browser, error) {
	binary, profile, info, err := preflight(profile)
	if err != nil {
		return nil, err
	}
	launchCtx, launchCancel := context.WithCancel(ctx)
	done := make(chan struct{})
	b := &Browser{
		ctx:          ctx,
		binary:       binary,
		profile:      profile,
		profileInfo:  info,
		background:   true,
		launchDone:   done,
		launchCancel: launchCancel,
	}
	go func() {
		defer close(done)
		next, err := launchBrowser(launchCtx, binary, profile, true)
		b.lifecycle.Lock()
		defer b.lifecycle.Unlock()
		if b.stopped {
			if err == nil && next != nil {
				next.stopProcess()
			}
			b.launchErr = errors.New("播放器已关闭")
			return
		}
		if err != nil {
			b.launchErr = err
			return
		}
		b.pipe, b.cmd, b.exited = next.pipe, next.cmd, next.exited
		b.profileInfo = next.profileInfo
		b.background = true
	}()
	return b, nil
}

func (b *Browser) waitReady(ctx context.Context) error {
	if b.launchDone == nil {
		return nil
	}
	select {
	case <-b.launchDone:
		return b.launchErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *Browser) currentTarget() string {
	b.targetMu.Lock()
	defer b.targetMu.Unlock()
	return b.target
}

func (b *Browser) invalidateQueue() {
	b.queueMu.Lock()
	b.lastQueueRev = 0
	b.lastQueue = nil
	b.lastQueueTarget = ""
	b.lastQueueStorefront = ""
	b.lastQueueAuth = false
	b.queueMu.Unlock()
}

func launchBrowser(ctx context.Context, binary, profile string, background bool) (*Browser, error) {
	info, err := prepareProfile(profile)
	if err != nil {
		return nil, err
	}
	childRead, write, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	read, childWrite, err := os.Pipe()
	if err != nil {
		_ = childRead.Close()
		_ = write.Close()
		return nil, err
	}
	defer childRead.Close()
	defer childWrite.Close()
	cmd := exec.CommandContext(ctx, binary, chromeArgs(profile, background)...)
	if err := startChrome(cmd, childRead, childWrite); err != nil {
		_ = read.Close()
		_ = write.Close()
		return nil, fmt.Errorf("启动 Chrome 失败: %w", err)
	}
	b := &Browser{pipe: newPipe(read, write), cmd: cmd, exited: make(chan struct{}), ctx: ctx, binary: binary, profile: profile, profileInfo: info, background: background}
	go func() { _ = cmd.Wait(); b.pipe.close(); close(b.exited) }()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := b.pipe.call(startup, "", "Browser.getVersion", nil, nil); err != nil {
		b.Close()
		return nil, fmt.Errorf("Chrome 未能启动；请先退出占用同一配置目录的播放器: %w", err)
	}
	return b, nil
}

func (b *Browser) Close() {
	b.close.Do(func() {
		b.lifecycle.Lock()
		b.stopped = true
		if b.launchCancel != nil {
			b.launchCancel()
		}
		b.stopProcess()
		b.lifecycle.Unlock()
		if b.launchDone != nil {
			<-b.launchDone
		}
	})
}

func (b *Browser) stopProcess() {
	if b.pipe == nil || b.cmd == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = b.pipe.call(ctx, "", "Browser.close", nil, nil)
	select {
	case <-b.exited:
	case <-ctx.Done():
		_ = b.cmd.Process.Kill()
		<-b.exited
	}
	b.pipe.close()
}

// The exclusive lifecycle lock prevents RPCs from crossing profile handover.
func (b *Browser) restartLocked(background bool) error {
	if b.stopped || b.ctx.Err() != nil {
		return errors.New("播放器已关闭")
	}
	b.stopProcess()
	return b.startLocked(background)
}

func (b *Browser) startLocked(background bool) error {
	next, err := launchBrowser(b.ctx, b.binary, b.profile, background)
	if err != nil {
		b.stopped = true
		return err
	}
	b.pipe, b.cmd, b.exited = next.pipe, next.cmd, next.exited
	b.targetMu.Lock()
	b.target, b.session = "", ""
	b.targetMu.Unlock()
	b.invalidateQueue()
	b.profileInfo = next.profileInfo
	b.background, b.stopped = background, false
	return nil
}

func appleOrigin(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host == "music.apple.com" && u.User == nil
}

// sessionForPage rechecks the target URL before every evaluation; login popups
// and other tabs are never evaluated, even when they belong to this process.
func (b *Browser) sessionForPage(ctx context.Context) (string, error) {
	if b.pipe == nil {
		return "", errors.New("后台 Apple Music 页面正在加载或已关闭；请稍候，必要时重新启动播放器")
	}
	// Keep session discovery serial without extending a caller's deadline.
	b.sessionOnce.Do(func() { b.sessionGate = make(chan struct{}, 1) })
	select {
	case b.sessionGate <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-b.sessionGate }()
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var targets struct {
		TargetInfos []struct{ TargetID, Type, URL string }
	}
	if err := b.pipe.call(ctx, "", "Target.getTargets", nil, &targets); err != nil {
		return "", err
	}
	b.targetMu.Lock()
	curTarget, curSession := b.target, b.session
	b.targetMu.Unlock()
	for _, t := range targets.TargetInfos {
		if t.Type != "page" || !appleOrigin(t.URL) || (curTarget != "" && t.TargetID != curTarget) {
			continue
		}
		if t.TargetID == curTarget && curSession != "" {
			return curSession, nil
		}
		var attached struct{ SessionID string }
		if err := b.pipe.call(ctx, "", "Target.attachToTarget", map[string]any{"targetId": t.TargetID, "flatten": true}, &attached); err != nil {
			return "", err
		}
		b.targetMu.Lock()
		b.target, b.session = t.TargetID, attached.SessionID
		b.targetMu.Unlock()
		return attached.SessionID, nil
	}
	return "", errors.New("后台 Apple Music 页面正在加载或已关闭；请稍候，必要时重新启动播放器")
}

func (b *Browser) evaluate(ctx context.Context, request any, dest any) error {
	started := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	session, err := b.sessionForPage(ctx)
	if err != nil {
		return err
	}
	expression, err := scriptFor(request)
	if err != nil {
		return err
	}
	var response struct {
		Result           struct{ Value json.RawMessage }
		ExceptionDetails json.RawMessage
	}
	action, userAction := request.(Action)
	err = b.pipe.call(ctx, session, "Runtime.evaluate", map[string]any{
		"expression": expression, "returnByValue": true, "awaitPromise": true, "userGesture": userAction, "timeout": 9000,
	}, &response)
	if err != nil {
		return err
	}
	if len(response.ExceptionDetails) != 0 {
		return errors.New("Apple Music 网页接口暂不可用，请检查浏览器后重试")
	}
	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Error  string          `json:"error"`
		Timing *struct {
			ResourcesMs float64 `json:"resourcesMs"`
			SetQueueMs  float64 `json:"setQueueMs"`
			ChangeMs    float64 `json:"changeMs"`
			PlayMs      float64 `json:"playMs"`
			TotalMs     float64 `json:"totalMs"`
			ReusedQueue bool    `json:"reusedQueue"`
		} `json:"timing"`
	}
	if err := json.Unmarshal(response.Result.Value, &envelope); err != nil {
		return errors.New("Apple Music 返回了无法识别的状态")
	}
	if t := envelope.Timing; userAction && t != nil {
		slog.Info("apple_music_pipeline", "op", action.Kind,
			"resources_ms", t.ResourcesMs, "set_queue_ms", t.SetQueueMs, "change_ms", t.ChangeMs, "play_ms", t.PlayMs,
			"bridge_ms", t.TotalMs, "roundtrip_ms", time.Since(started).Milliseconds(), "reused_queue", t.ReusedQueue, "ok", envelope.Error == "")
	}
	if envelope.Error != "" {
		return bridgeError(envelope.Error)
	}
	if dest != nil {
		return json.Unmarshal(envelope.Data, dest)
	}
	return nil
}

func (b *Browser) Show(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := b.waitReady(ctx); err != nil {
		return err
	}
	b.lifecycle.Lock()
	defer b.lifecycle.Unlock()
	if b.stopped {
		return errors.New("播放器已关闭")
	}
	var s State
	if err := b.evaluate(ctx, map[string]string{"op": "snapshot"}, &s); err == nil && s.Authorized {
		return errors.New("登录状态已保存，后台播放不需要打开浏览器")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if b.background {
		if err := requireLoginDisplay(runtime.GOOS, os.Getenv("DISPLAY"), os.Getenv("WAYLAND_DISPLAY")); err != nil {
			return err
		}
		return b.restartLocked(false)
	}
	session, err := b.sessionForPage(ctx)
	if err != nil {
		return err
	}
	var window struct{ WindowID int }
	if b.pipe.call(ctx, "", "Browser.getWindowForTarget", map[string]string{"targetId": b.currentTarget()}, &window) == nil {
		_ = b.pipe.call(ctx, "", "Browser.setWindowBounds", map[string]any{"windowId": window.WindowID, "bounds": map[string]string{"windowState": "normal"}}, nil)
	}
	return b.pipe.call(ctx, session, "Page.bringToFront", nil, nil)
}
