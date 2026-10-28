package applemusic

import (
	"context"
	"errors"
	"time"
)

// ImportCookies only writes explicitly supplied cookies to this browser's profile.
// CDP does not guarantee atomic writes; errors must not imply rollback.
func (b *Browser) ImportCookies(ctx context.Context, jar CookieJar) error {
	if jar.Count() == 0 || jar.Count() > maxCookieCount {
		return errors.New("没有可导入的 Cookie")
	}
	ctx, cancel := context.WithTimeout(ctx, 35*time.Second)
	defer cancel()
	b.lifecycle.Lock()
	defer b.lifecycle.Unlock()
	if b.stopped {
		return errors.New("播放器已关闭，未导入 Cookie")
	}
	if !b.background {
		return errors.New("正在进行可见登录，未导入 Cookie")
	}
	for {
		var state State
		if err := b.evaluate(ctx, map[string]string{"op": "snapshot"}, &state); err == nil && state.Ready {
			if state.Authorized {
				return errors.New("专用浏览器已登录，拒绝混入另一份 Cookie；如需更换账号，请先在账户面板明确清空本机登录")
			}
			break
		}
		select {
		case <-ctx.Done():
			return errors.New("未能确认官方页面的登录状态，尚未导入 Cookie")
		case <-time.After(250 * time.Millisecond):
		}
	}
	if err := ctx.Err(); err != nil {
		return errors.New("导入已取消，尚未写入 Cookie")
	}
	if err := b.pipe.call(ctx, "", "Storage.setCookies", map[string]any{"cookies": jar.cookies}, nil); err != nil {
		return errors.New("浏览器未确认 Cookie 写入；部分条目可能已写入，不会自动重试或假报回滚")
	}
	session, err := b.sessionForPage(ctx)
	if err != nil {
		return errors.New("Cookie 写入已被接受，但官方页面不可用；授权尚未确认")
	}
	if err := b.pipe.call(ctx, session, "Page.reload", map[string]bool{"ignoreCache": true}, nil); err != nil {
		return errors.New("Cookie 写入已被接受，但刷新失败；授权尚未确认")
	}
	return nil
}
