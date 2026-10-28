package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-musicfox/go-musicfox/internal/commands"
	"github.com/go-musicfox/go-musicfox/internal/configs"
	"github.com/go-musicfox/go-musicfox/internal/types"
	mfoxapp "github.com/go-musicfox/go-musicfox/utils/app"
	"github.com/go-musicfox/go-musicfox/utils/errorx"
	"github.com/gookit/gcli/v2"
)

func main() {
	errorx.RunWrapped(func() {
		defer errorx.Recover(false)
		os.Exit(musicfox())
	})
}

func newCLI() *gcli.App {
	app := gcli.NewApp()
	app.ExitOnEnd = false
	app.Name = types.AppName
	app.Version = types.AppVersion
	app.Description = types.AppDescription
	app.GOptsBinder = func(gf *gcli.Flags) {
		gf.BoolOpt(&commands.GlobalOptions.DebugMode, "debug", "", false, "Enable debug logging")
		gf.BoolOpt(&commands.GlobalOptions.PureMode, "pure", "", false, "Use a temporary configuration and Chrome profile")
		gf.StrOpt(&commands.GlobalOptions.CookieFile, "import-cookies", "", "", "Import a private Apple Music cookie file, then start the player (not proof of authorization)")
	}
	app.Add(commands.NewAppleMusicCommand())
	app.DefaultCommand("apple-music")
	return app
}

func musicfox() int {
	if isFlagTrue("pure") {
		root, err := os.MkdirTemp("", "tapefox-pure-*")
		if err != nil {
			panic(fmt.Errorf("create temporary configuration: %w", err))
		}
		defer os.RemoveAll(root)
		mfoxapp.SetupPureRoot(root)
		commands.GlobalOptions.PureMode = true
	}
	cfg, err := configs.NewConfigFromTomlFile(mfoxapp.ConfigFilePath())
	if err != nil {
		panic(fmt.Errorf("load configuration: %w", err))
	}
	configs.AppConfig = cfg
	return newCLI().Run()
}

// The configuration path is selected before gcli parses global flags.
func isFlagTrue(name string) bool {
	prefix := "--" + name
	for _, arg := range os.Args {
		if arg == prefix {
			return true
		}
		if after, ok := strings.CutPrefix(arg, prefix+"="); ok {
			switch after {
			case "true", "1", "yes":
				return true
			}
		}
	}
	return false
}
