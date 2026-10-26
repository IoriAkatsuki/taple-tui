package commands

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/anhoder/foxful-cli/model"
	"github.com/anhoder/foxful-cli/motion"
	"github.com/go-musicfox/go-musicfox/internal/applemusic"
	"github.com/go-musicfox/go-musicfox/internal/configs"
	"github.com/go-musicfox/go-musicfox/internal/ui"
	"github.com/go-musicfox/go-musicfox/utils/slogx"
	"github.com/gookit/gcli/v2"
)

func NewAppleMusicCommand() *gcli.Command {
	return &gcli.Command{
		Name:   "apple-music",
		UseFor: "Linux Apple Music player using a dedicated Chrome session",
		Func: func(_ *gcli.Command, _ []string) error {
			if GlobalOptions.DebugMode {
				slogx.LevelVar().Set(slog.LevelDebug)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var profile string
			if GlobalOptions.PureMode {
				var err error
				profile, err = os.MkdirTemp("", "tapefox-apple-music-*")
				if err != nil {
					return err
				}
				defer os.RemoveAll(profile)
			}
			browser, err := applemusic.Open(ctx, profile)
			if err != nil {
				return err
			}
			defer browser.Close()
			page := ui.NewAppleMusicPage(browser)
			defer page.Close()
			opts := model.DefaultOptions()
			opts.AppName = "azichrome-tui / Apple Music"
			opts.EnableStartup = false
			opts.InitPage = page
			opts.Motion.Level = motion.ParseLevel(configs.AppConfig.Main.Motion)
			opts.Motion.FrameInterval = time.Second / time.Duration(configs.AppConfig.Main.FrameRate)
			return model.NewApp(opts).Run()
		},
	}
}
