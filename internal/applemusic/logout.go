package applemusic

import (
	"context"
	"errors"
	"os"
)

// logout forgets this application's browser session, not Apple's server-side authorization.
func (b *Browser) logout(ctx context.Context) error {
	b.lifecycle.Lock()
	defer b.lifecycle.Unlock()
	if b.stopped {
		return errors.New("播放器已关闭，请重启后重试清空登录")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	b.stopProcess()
	b.stopped = true
	if err := clearOwnedProfile(b.profile, b.profileInfo); err != nil {
		return errors.New("本地登录清空未完成，播放器已停止；请重启后重试")
	}
	if ctx.Err() != nil || b.ctx.Err() != nil {
		return errors.New("本地登录已清空；请重新启动播放器后登录")
	}
	if err := b.startLocked(true); err != nil {
		return errors.New("本地登录已清空，但后台重启失败；请重新启动播放器后登录")
	}
	return nil
}

func clearOwnedProfile(path string, expected os.FileInfo) error {
	info, err := os.Lstat(path)
	if err != nil || expected == nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !os.SameFile(info, expected) {
		return errors.New("浏览器配置目录身份已变化，拒绝清空")
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return err
	}
	defer root.Close()
	info, err = root.Stat(".")
	if err != nil || !os.SameFile(info, expected) {
		return errors.New("浏览器配置目录身份已变化，拒绝清空")
	}
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	entries, err := dir.ReadDir(-1)
	dir.Close()
	if err != nil {
		return err
	}
	for _, entry := range entries {
		// Rooted removal cannot follow a profile symlink into another browser's data.
		if err := root.RemoveAll(entry.Name()); err != nil {
			return err
		}
	}
	if err := root.Chmod(".", 0700); err != nil {
		return err
	}
	info, err = os.Lstat(path)
	if err != nil || !info.IsDir() || !os.SameFile(info, expected) {
		return errors.New("清空后配置目录被替换，拒绝重新启动")
	}
	return nil
}
