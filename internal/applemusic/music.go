package applemusic

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"
)

//go:embed bridge.js
var bridgeJS string

type Track struct {
	Type      string  `json:"type"`
	Playable  bool    `json:"playable"`
	CatalogID string  `json:"catalogId"`
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Artist    string  `json:"artist"`
	Album     string  `json:"album"`
	Duration  float64 `json:"duration"`
}

type State struct {
	Storefront        string  `json:"storefront"`
	Ready             bool    `json:"ready"`
	Authorized        bool    `json:"authorized"`
	Playing           bool    `json:"playing"`
	Paused            bool    `json:"paused"`
	AutoplayAvailable bool    `json:"autoplayAvailable"`
	Autoplay          bool    `json:"autoplay"`
	Preview           bool    `json:"preview"`
	Full              bool    `json:"full"`
	Position          float64 `json:"position"`
	Volume            float64 `json:"volume"`
	Track             Track   `json:"track"`
	Queue             []Track `json:"queue"`
	QueueIndex        int     `json:"queueIndex"`
	QueueTotal        int     `json:"queueTotal"`
}

type Action struct {
	ResourceType string  `json:"resourceType,omitempty"`
	Kind         string  `json:"op"`
	ID           string  `json:"id,omitempty"`
	StartID      string  `json:"startID,omitempty"`
	Value        float64 `json:"value"`
}

func scriptFor(request any) (string, error) {
	args, err := json.Marshal(request)
	source := bridgeJS
	if _, ok := request.(mediaCapabilityRequest); ok {
		source = mediaCapabilitiesJS
	}
	return "(" + source + ")(" + string(args) + ")", err
}

func (b *Browser) Snapshot(ctx context.Context) (State, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	b.lifecycle.RLock()
	if b.stopped {
		b.lifecycle.RUnlock()
		return State{}, errors.New("播放器已关闭，请重新启动")
	}
	var s State
	err := b.evaluate(ctx, map[string]string{"op": "snapshot"}, &s)
	visible := !b.background
	closed := false
	select {
	case <-b.exited:
		closed = true
	default:
	}
	b.lifecycle.RUnlock()
	if visible && (s.Authorized || closed) && ctx.Err() == nil {
		b.lifecycle.Lock()
		if !b.stopped && !b.background {
			err = b.restartLocked(true)
		}
		b.lifecycle.Unlock()
		return State{}, err
	}
	return s, err
}

func (b *Browser) Do(ctx context.Context, a Action) error {
	valid := false
	switch a.Kind {
	case "play", "enqueue":
		valid = a.ID != "" && len(a.ID) <= 256 && !strings.ContainsAny(a.ID, "\x00\n\r") && (a.ResourceType == "" || a.ResourceType == "songs" || a.ResourceType == "library-songs")
	case "play-station":
		valid = a.ResourceType == "stations" && a.ID != "" && len(a.ID) <= 256 && !strings.ContainsAny(a.ID, "\x00\n\r")
	case "play-collection":
		valid = (a.ResourceType == "albums" || a.ResourceType == "playlists" || a.ResourceType == "library-albums" || a.ResourceType == "library-playlists") && a.ID != "" && len(a.ID) <= 256 && !strings.ContainsAny(a.ID, "\x00\n\r")
	case "toggle", "next", "previous":
		valid = true
	case "logout":
		valid = a.ID == "" && a.StartID == "" && a.ResourceType == "" && a.Value == 0
	case "seek":
		valid = a.Value >= 0 && a.Value <= 86400 && a.ID != "" && len(a.ID) <= 256 && !strings.ContainsAny(a.ID, "\x00\n\r")
	case "volume":
		valid = a.Value >= 0 && a.Value <= 1
	case "jump":
		valid = a.Value >= 0 && a.Value < 50 && a.Value == math.Trunc(a.Value) && a.ID != "" && len(a.ID) <= 256
	}
	if a.StartID != "" && (a.Kind != "play-collection" || len(a.StartID) > 256 || strings.ContainsAny(a.StartID, "\x00\n\r")) {
		valid = false
	}
	if !valid || math.IsNaN(a.Value) || math.IsInf(a.Value, 0) {
		return errors.New("无效的 Apple Music 控制参数")
	}
	if a.Kind == "logout" {
		return b.logout(ctx)
	}
	b.lifecycle.RLock()
	defer b.lifecycle.RUnlock()
	if b.stopped {
		return errors.New("播放器已关闭")
	}
	if !b.background {
		return errors.New("正在完成登录交接，请等待后台恢复后再播放")
	}
	started := time.Now()
	if needsPlaybackCapabilities(a.Kind) {
		if err := b.checkPlaybackCapabilities(ctx, a); err != nil {
			return err
		}
	}
	err := b.evaluate(ctx, a, nil)
	slog.Info("apple_music_control", "op", a.Kind, "elapsed_ms", time.Since(started).Milliseconds(), "ok", err == nil)
	return err
}

func bridgeError(code string) error {
	switch code {
	case "media_unsupported":
		return fmt.Errorf("%w：浏览器拒绝播放，请检查 AAC/Widevine 与当前运行时兼容性", ErrPlaybackUnavailable)
	case "stale":
		return errors.New("歌曲已变化，未发送过期进度操作")
	case "origin":
		return errors.New("仅允许控制 https://music.apple.com/ 页面")
	case "loading":
		return errors.New("Apple Music 正在加载；若长时间无响应，请在浏览器刷新页面")
	case "login":
		return errors.New("请在浏览器中登录 Apple Music 并确认订阅有效")
	case "unplayable":
		return errors.New("此条目没有可用的歌曲或播放信息")
	case "timeout":
		return errors.New("Apple Music 请求超时，请稍后重试")
	default:
		return errors.New("Apple Music 拒绝了请求，请检查网络、订阅及浏览器播放提示")
	}
}
