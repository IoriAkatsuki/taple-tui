package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/gookit/gcli/v2"
)

func TestAppleMusicIsTheOnlyPlayerAndDefaultCommand(t *testing.T) {
	app := newCLI()
	app.ExitOnEnd = false
	app.Args = []string{"tapefox"}
	if app.HasCommand("netease") {
		t.Fatal("the legacy player must not be registered")
	}
	called := false
	command := app.Commands()["apple-music"]
	if command == nil {
		t.Fatal("Apple Music command is missing")
	}
	command.Func = func(_ *gcli.Command, _ []string) error {
		called = true
		return nil
	}
	if code := app.Run(); code != 0 || !called {
		t.Fatalf("default command: exit=%d, Apple Music invoked=%t", code, called)
	}
}

func TestRemovedVUFlagDoesNotStartPlayer(t *testing.T) {
	for _, args := range [][]string{{"tapefox", "--vu"}, {"tapefox", "apple-music", "--vu"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app := newCLI()
			app.Args = args
			called := false
			app.Commands()["apple-music"].Func = func(_ *gcli.Command, _ []string) error {
				called = true
				return nil
			}
			if code := app.Run(); called {
				t.Fatalf("obsolete feature flag still accepted: exit=%d, invoked=%t", code, called)
			}
		})
	}
}

func TestCLIHasNoLegacyPlaybackDependencies(t *testing.T) {
	out, err := exec.Command("go", "list", "-deps", ".").CombinedOutput()
	if err != nil {
		t.Fatalf("list CLI dependencies: %v\n%s", err, out)
	}
	for _, dependency := range strings.Fields(string(out)) {
		for _, legacy := range []string{
			"github.com/go-musicfox/netease-music",
			"github.com/gopxl/beep",
			"github.com/go-musicfox/go-musicfox/internal/player",
			"github.com/go-musicfox/go-musicfox/internal/netease",
			"github.com/go-musicfox/go-musicfox/internal/playlist",
			"github.com/go-musicfox/go-musicfox/internal/track",
			"github.com/go-musicfox/go-musicfox/internal/storage",
			"github.com/go-musicfox/go-musicfox/internal/lyric",
			"github.com/go-musicfox/go-musicfox/internal/lastfm",
			"github.com/go-musicfox/go-musicfox/internal/remote_control",
		} {
			if dependency == legacy || strings.HasPrefix(dependency, legacy+"/") {
				t.Errorf("legacy playback dependency: %s", dependency)
			}
		}
	}
}

func TestCookieImportOptionRemainsAnAppleMusicOperation(t *testing.T) {
	for _, args := range [][]string{{"tapefox", "--import-cookies", "synthetic.json"}, {"tapefox", "apple-music", "--import-cookies", "synthetic.json"}} {
		app := newCLI()
		app.Args = args
		called := false
		app.Commands()["apple-music"].Func = func(_ *gcli.Command, _ []string) error {
			called = true
			return nil
		}
		if code := app.Run(); code != 0 || !called {
			t.Fatalf("cookie import must use the existing player: code=%d called=%v", code, called)
		}
	}
}
