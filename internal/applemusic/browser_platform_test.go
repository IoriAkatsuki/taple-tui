package applemusic

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestChromeArgumentsKeepPrivateNormalBrowser(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "isolated profile")
	for _, background := range []bool{false, true} {
		args := chromeArgs(profile, background)
		for _, required := range []string{"--remote-debugging-pipe", "--user-data-dir=" + profile, "https://music.apple.com/"} {
			if !slices.Contains(args, required) {
				t.Fatalf("missing argument %q", required)
			}
		}
		for _, arg := range args {
			if strings.HasPrefix(arg, "--remote-debugging-port") || strings.HasPrefix(arg, "--headless") {
				t.Fatalf("browser must retain normal DRM identity and private pipes: %q", arg)
			}
		}
		if slices.Contains(args, "--ozone-platform=headless") != (background && runtime.GOOS == "linux") {
			t.Fatal("Ozone virtual windows must only be used on Linux")
		}
		if slices.Contains(args, "--start-minimized") != (background && runtime.GOOS != "linux") {
			t.Fatal("non-Linux background launch must request a normal minimized window")
		}
	}
}

func TestChromeDiscoveryIncludesNativeInstallations(t *testing.T) {
	candidates := chromeCandidates()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("LOCALAPPDATA", t.TempDir())
		t.Setenv("PROGRAMFILES", t.TempDir())
		t.Setenv("PROGRAMFILES(X86)", t.TempDir())
		candidates = chromeCandidates()
		for _, env := range []string{"LOCALAPPDATA", "PROGRAMFILES", "PROGRAMFILES(X86)"} {
			if !slices.Contains(candidates, filepath.Join(os.Getenv(env), "Google", "Chrome", "Application", "chrome.exe")) {
				t.Fatalf("missing installation root %s", env)
			}
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		for _, root := range []string{"/Applications", filepath.Join(home, "Applications")} {
			if !slices.Contains(candidates, filepath.Join(root, "Google Chrome.app", "Contents", "MacOS", "Google Chrome")) {
				t.Fatal("missing native Chrome app bundle")
			}
		}
	case "linux":
		if !slices.Contains(candidates, "google-chrome-stable") || !slices.Contains(candidates, "chromium") {
			t.Fatal("existing Linux discovery changed")
		}
	}
}

func TestPrepareProfileRejectsLinkedDirectory(t *testing.T) {
	parent := t.TempDir()
	target, link := filepath.Join(parent, "other-browser"), filepath.Join(parent, "chrome")
	if err := os.Mkdir(target, 0700); err != nil {
		t.Fatal(err)
	}
	makeProfileTestLink(t, target, link)
	if _, err := prepareProfile(link); err == nil {
		t.Fatal("linked profile was accepted")
	}
}

func TestChromeStartupFailureReturnsWithoutKeepingBrowser(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	// The Go test executable exits on Chrome's unrecognized switches.
	browser, err := launchBrowser(ctx, os.Args[0], filepath.Join(t.TempDir(), "chrome"), true)
	if err == nil || browser != nil || !strings.Contains(err.Error(), "Chrome 未能启动") {
		if browser != nil {
			browser.Close()
		}
		t.Fatal("startup did not exercise detection of an exited browser:", err)
	}
	if ctx.Err() != nil {
		t.Fatal("startup did not notice the child exit")
	}
}

func TestChromeFailedStartClosesParentPipes(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux fd accounting; Windows has a native inherited-handle cleanup test")
	}
	parent := t.TempDir()
	// Initialize Go's lazy poll descriptors before comparing pipe fd counts.
	_, _ = launchBrowser(context.Background(), filepath.Join(parent, "missing-chrome"), filepath.Join(parent, "profile"), true)
	before, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Fatal(err)
	}
	for range 5 {
		if _, err := launchBrowser(context.Background(), filepath.Join(parent, "missing-chrome"), filepath.Join(parent, "profile"), true); err == nil {
			t.Fatal("missing Chrome unexpectedly started")
		}
	}
	after, err := os.ReadDir("/proc/self/fd")
	if err != nil || len(after) > len(before) {
		t.Fatalf("startup leaked pipe fds: before=%d after=%d error=%v", len(before), len(after), err)
	}
}

// This test never opens Apple Music or an existing browser profile.
func TestChromePrivatePipeSmoke(t *testing.T) {
	if os.Getenv("TAPEFOX_CHROME_SMOKE") != "1" {
		t.Skip("set TAPEFOX_CHROME_SMOKE=1 for a disposable, account-free native Chrome smoke test")
	}
	binary, err := findChrome()
	if err != nil {
		t.Fatal(err)
	}
	profile := filepath.Join(t.TempDir(), "chrome")
	identity, err := prepareProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		info, err := prepareProfile(profile)
		if err != nil || !os.SameFile(identity, info) {
			t.Fatal("profile identity changed across restart:", err)
		}
		marker := filepath.Join(profile, "tapefox-synthetic-session")
		if err := os.WriteFile(marker, []byte("synthetic; no credentials"), 0600); err != nil {
			t.Fatal(err)
		}
		chromePrivatePipeSmokeRound(t, binary, profile)
		if _, err := os.Stat(marker); err != nil {
			t.Fatal("ordinary browser close removed session data:", err)
		}
		if err := clearOwnedProfile(profile, identity); err != nil {
			t.Fatal("closed Chrome retained profile locks or changed directory identity:", err)
		}
		entries, err := os.ReadDir(profile)
		if err != nil || len(entries) != 0 {
			t.Fatal("synthetic profile was not cleared:", err)
		}
		assertProfilePrivate(t, profile)
	}
	t.Log("Native Chrome private pipe, own-process exit, private profile cleanup and restart verified; no account or playback was tested")
}

func chromePrivatePipeSmokeRound(t *testing.T, binary, profile string) {
	t.Helper()
	childRead, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer childRead.Close()
	defer write.Close()
	read, childWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer childWrite.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	args := chromeArgs(profile, true)
	args[len(args)-1] = "about:blank"
	args = append(args, "--disable-background-networking", "--disable-component-update")
	cmd := exec.CommandContext(ctx, binary, args...)
	if err := startChrome(cmd, childRead, childWrite); err != nil {
		t.Fatal(err)
	}
	childRead.Close()
	childWrite.Close()
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()
	defer func() {
		select {
		case <-exited:
		default:
			_ = cmd.Process.Kill()
			<-exited
		}
	}()
	pipe := newPipe(read, write)
	defer pipe.close()
	var version struct{ Product string }
	if err := pipe.call(ctx, "", "Browser.getVersion", nil, &version); err != nil {
		t.Fatal(err)
	}
	if version.Product == "" {
		t.Fatal("Chrome did not report a version over the private pipe")
	}
	// Browser.close can close the pipe before its acknowledgement is delivered.
	_ = pipe.call(ctx, "", "Browser.close", nil, nil)
	select {
	case <-exited:
	case <-ctx.Done():
		t.Fatal("own Chrome did not exit after Browser.close")
	}
}
