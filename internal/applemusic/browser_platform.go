package applemusic

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func chromeCandidates() []string {
	switch runtime.GOOS {
	case "windows":
		paths := []string{"chromium.exe", "chrome.exe"}
		for _, env := range []string{"LOCALAPPDATA", "PROGRAMFILES", "PROGRAMFILES(X86)"} {
			if root := os.Getenv(env); root != "" {
				paths = append(paths, filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe"))
			}
		}
		return paths
	case "darwin":
		roots := []string{"/Applications"}
		if home, err := os.UserHomeDir(); err == nil {
			roots = append(roots, filepath.Join(home, "Applications"))
		}
		var paths []string
		for _, app := range []string{"Chromium", "Google Chrome"} {
			for _, root := range roots {
				paths = append(paths, filepath.Join(root, app+".app", "Contents", "MacOS", app))
			}
		}
		return append(paths, "chromium", "google-chrome")
	default:
		return []string{"chromium", "chromium-browser", "google-chrome", "google-chrome-stable"}
	}
}

func findChrome() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	// Resolve the player symlink, not the working directory: a runtime belongs
	// beside the installed executable, not an arbitrary directory we enter.
	if real, err := filepath.EvalSymlinks(executable); err == nil {
		executable = real
	}
	candidates := append(bundledBrowserCandidates(executable, runtime.GOOS), chromeCandidates()...)
	return resolveBrowser(os.Getenv("TAPLE_BROWSER"), candidates)
}

func bundledBrowserCandidates(executable, goos string) []string {
	root := filepath.Join(filepath.Dir(executable), "runtime", "chromium")
	switch goos {
	case "windows":
		return []string{filepath.Join(root, "chromium.exe"), filepath.Join(root, "chrome.exe")}
	case "darwin":
		return []string{filepath.Join(root, "Chromium.app", "Contents", "MacOS", "Chromium")}
	default:
		return []string{filepath.Join(root, "chromium"), filepath.Join(root, "chrome")}
	}
}

func resolveBrowser(override string, candidates []string) (string, error) {
	if override != "" {
		path, err := exec.LookPath(override)
		if err != nil {
			return "", fmt.Errorf("TAPLE_BROWSER 指定的 Chromium 运行时不可执行: %w", err)
		}
		// A bad explicit selection must never silently launch a different browser.
		return filepath.Abs(path)
	}
	for _, name := range candidates {
		if path, err := exec.LookPath(name); err == nil {
			return filepath.Abs(path)
		}
	}
	return "", errors.New("未找到 Chromium 运行时；请放入 runtime/chromium 或用 TAPLE_BROWSER 指定路径（完整播放需要 AAC 和 Widevine 支持）")
}

func chromeArgs(profile string, background bool) []string {
	args := []string{"--remote-debugging-pipe", "--no-first-run", "--no-default-browser-check", "--user-data-dir=" + profile}
	if background {
		// Unified Chromium headless: no native window on any supported OS.
		// Keep the sandbox, audio and GPU enabled; CDM support is runtime-specific.
		args = append(args, "--headless=new")
		args = append(args, "--disable-background-timer-throttling", "--disable-renderer-backgrounding")
	} else {
		args = append(args, "--new-window")
	}
	return append(args, "https://music.apple.com/")
}

func requireLoginDisplay(goos, display, wayland string) error {
	if goos == "linux" && display == "" && wayland == "" {
		return errors.New("首次登录需要图形会话；请在同一用户的桌面终端完成登录后再无头运行。后台会话保持不变")
	}
	return nil
}
