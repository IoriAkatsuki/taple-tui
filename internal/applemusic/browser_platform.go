package applemusic

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func chromeCandidates() []string {
	switch runtime.GOOS {
	case "windows":
		paths := []string{"chrome.exe", "chromium.exe"}
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
		for _, root := range roots {
			for _, app := range []string{"Google Chrome", "Chromium"} {
				paths = append(paths, filepath.Join(root, app+".app", "Contents", "MacOS", app))
			}
		}
		return append(paths, "google-chrome", "chromium")
	default:
		return []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser"}
	}
}

func findChrome() (string, error) {
	for _, name := range chromeCandidates() {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	return "", errors.New("未找到 Chrome；请安装 Google Chrome 或具备 Widevine 的 Chromium")
}

func chromeArgs(profile string, background bool) []string {
	args := []string{"--remote-debugging-pipe", "--no-first-run", "--no-default-browser-check", "--user-data-dir=" + profile}
	if background {
		if runtime.GOOS == "linux" {
			// Ozone creates virtual native windows while retaining normal Chrome/DRM.
			args = append(args, "--ozone-platform=headless")
		} else {
			// Normal Chrome may still show a window; headless DRM is not a fallback.
			args = append(args, "--start-minimized")
		}
		args = append(args, "--disable-background-timer-throttling", "--disable-renderer-backgrounding")
	}
	return append(args, "--new-window", "https://music.apple.com/")
}
