package applemusic

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogoutActionIsLocalAndRejectsArguments(t *testing.T) {
	b := &Browser{stopped: true}
	if err := b.Do(context.Background(), Action{Kind: "logout"}); err == nil || !strings.Contains(err.Error(), "已关闭") {
		t.Fatalf("logout did not reach the local lifecycle guard: %v", err)
	}
	for _, a := range []Action{
		{Kind: "logout", ID: "arbitrary-directory"}, {Kind: "logout", Value: 1},
		{Kind: "logout", ResourceType: "songs"}, {Kind: "logout", StartID: "song"},
	} {
		if err := b.Do(context.Background(), a); err == nil || !strings.Contains(err.Error(), "无效") {
			t.Fatalf("logout accepted arguments: %v", err)
		}
	}
}

func TestClearOwnedProfileDoesNotFollowLinksOrRemoveItsDirectory(t *testing.T) {
	parent := t.TempDir()
	profile := filepath.Join(parent, "chrome")
	info, err := prepareProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(profile, "Default", "nested"), 0700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(parent, "other-browser")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	keep := filepath.Join(outside, "session")
	if err := os.WriteFile(keep, []byte("synthetic outside fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profile, "Default", "nested", "session"), []byte("synthetic"), 0600); err != nil {
		t.Fatal(err)
	}
	makeProfileTestLink(t, outside, filepath.Join(profile, "external-link"))
	if err := clearOwnedProfile(profile, info); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(profile)
	if err != nil || len(entries) != 0 {
		t.Fatalf("profile not empty: %v %v", entries, err)
	}
	after, err := os.Lstat(profile)
	if err != nil || !os.SameFile(info, after) {
		t.Fatal("profile directory identity or privacy changed")
	}
	assertProfilePrivate(t, profile)
	if _, err := os.Stat(keep); err != nil {
		t.Fatal("cleanup escaped into another browser")
	}
	if err := clearOwnedProfile(profile, info); err != nil {
		t.Fatal("empty profile must be safe to retry:", err)
	}
}

func TestClearOwnedProfileRejectsReplacedOrUnknownDirectories(t *testing.T) {
	for _, replacement := range []string{"symlink", "directory", "missing", "unknown"} {
		t.Run(replacement, func(t *testing.T) {
			parent := t.TempDir()
			profile := filepath.Join(parent, "chrome")
			info, err := prepareProfile(profile)
			if err != nil {
				t.Fatal(err)
			}
			if replacement == "unknown" {
				info = nil
			} else {
				if err := os.Rename(profile, profile+".moved"); err != nil {
					t.Fatal(err)
				}
				switch replacement {
				case "symlink":
					makeProfileTestLink(t, profile+".moved", profile)
				case "directory":
					if err := os.Mkdir(profile, 0700); err != nil {
						t.Fatal(err)
					}
				}
			}
			if replacement != "missing" {
				if err := os.WriteFile(filepath.Join(profile, "keep"), []byte("synthetic"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := clearOwnedProfile(profile, info); err == nil {
				t.Fatal("unowned/replaced path was accepted")
			}
			if replacement != "missing" {
				if _, err := os.Stat(filepath.Join(profile, "keep")); err != nil {
					t.Fatal("rejected directory was modified")
				}
			}
		})
	}
}

func TestChromeLogoutReplacesOnlyItsOwnTemporarySession(t *testing.T) {
	if os.Getenv("TAPLE_CHROME_LOGOUT") != "1" {
		t.Skip("opt-in Chrome test with a disposable profile; never the user's account")
	}
	binary, err := findChrome()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	profile := t.TempDir()
	b, err := launchBrowser(ctx, binary, profile, true)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	marker := filepath.Join(profile, "taple-synthetic-session")
	if err := os.WriteFile(marker, []byte("synthetic; no credentials"), 0600); err != nil {
		t.Fatal(err)
	}
	pid, exited := b.cmd.Process.Pid, b.exited
	if err := b.Do(ctx, Action{Kind: "logout"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	default:
		t.Fatal("old browser was not stopped before cleanup")
	}
	if b.cmd.Process.Pid == pid || b.stopped || !b.background {
		t.Fatal("fresh background browser was not started")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatal("old session data survived")
	}
	if err := b.pipe.call(ctx, "", "Browser.getVersion", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("synthetic subsequent login"), 0600); err != nil {
		t.Fatal(err)
	}
	b.Close()
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("ordinary quit erased the new session")
	}
	t.Log("Synthetic profile cleared; old Chrome exited; fresh Chrome reachable. No real account was accessed.")
}
