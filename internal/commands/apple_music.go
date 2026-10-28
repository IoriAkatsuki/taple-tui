package commands

import (
	"context"
	"fmt"
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
	var cookieFile string
	return &gcli.Command{
		Name:   "apple-music",
		UseFor: "Apple Music TUI using a dedicated headless Chromium runtime",
		Config: func(c *gcli.Command) {
			c.StrOpt(&cookieFile, "import-cookies", "", "", "Import a private Apple Music cookie file before starting the player")
		},
		Func: func(_ *gcli.Command, _ []string) error {
			if GlobalOptions.DebugMode {
				slogx.LevelVar().Set(slog.LevelDebug)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var cookies applemusic.CookieJar
			input := cookieFile
			if input == "" {
				input = GlobalOptions.CookieFile
			} else if GlobalOptions.CookieFile != "" {
				return fmt.Errorf("Cookie 文件选项只能指定一次")
			}
			if input != "" {
				var err error
				cookies, err = applemusic.ReadCookieFile(input)
				if err != nil {
					return err
				}
			}
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
			if cookies.Count() > 0 {
				if err := browser.ImportCookies(ctx, cookies); err != nil {
					return err
				}
				fmt.Printf("已导入 %d 个 Cookie，跳过 %d 个过期条目；文件未修改。登录及订阅状态仍由官方页面确认。\n", cookies.Count(), cookies.Expired())
			}
			page := ui.NewAppleMusicPage(browser)
			defer page.Close()
			opts := model.DefaultOptions()
			opts.AppName = "taple-tui / Apple Music"
			opts.EnableStartup = false
			opts.InitPage = page
			opts.Motion.Level = motion.ParseLevel(configs.AppConfig.Main.Motion)
			opts.Motion.FrameInterval = time.Second / time.Duration(configs.AppConfig.Main.FrameRate)
			return model.NewApp(opts).Run()
		},
	}
}
