package applemusic

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
)

//go:embed media_capabilities.js
var mediaCapabilitiesJS string

// ErrPlaybackUnavailable denotes an explicit unsupported media environment.
var ErrPlaybackUnavailable = errors.New("播放运行时不可用")

type mediaCapabilityRequest struct {
	SkipIfPlaying        bool `json:"skipIfPlaying"`
	RequireAuthorization bool `json:"requireAuthorization"`
}

type mediaCapabilities struct {
	AAC      bool   `json:"aac"`
	Widevine bool   `json:"widevine"`
	Playing  bool   `json:"playing"`
	Reason   string `json:"reason"`
}

func (b *Browser) checkPlaybackCapabilities(ctx context.Context, action Action) error {
	var caps mediaCapabilities
	if err := b.evaluate(ctx, mediaCapabilityRequest{SkipIfPlaying: action.Kind == "toggle", RequireAuthorization: true}, &caps); err != nil {
		return err
	}
	// Pausing must not require a new DRM negotiation.
	if caps.Playing {
		return nil
	}
	slog.Info("apple_music_media_capabilities", "runtime", filepath.Base(b.binary), "aac", caps.AAC, "widevine", caps.Widevine, "reason", caps.Reason)
	switch {
	case !caps.AAC:
		return fmt.Errorf("%w：当前 Chromium 构建未启用 AAC 解码；请更换兼容运行时", ErrPlaybackUnavailable)
	case caps.Reason == "timeout":
		return fmt.Errorf("%w：Widevine 检测超时，请稍后重试；未提交播放请求", ErrPlaybackUnavailable)
	case !caps.Widevine:
		return fmt.Errorf("%w：当前无头运行时不能使用 Widevine；歌词可读取，但加密歌曲无法播放。请检查 CDM 或 TAPLE_BROWSER", ErrPlaybackUnavailable)
	default:
		return nil
	}
}

func needsPlaybackCapabilities(kind string) bool {
	switch kind {
	case "play", "play-collection", "play-station", "jump", "next", "previous", "toggle":
		return true
	default:
		return false
	}
}
