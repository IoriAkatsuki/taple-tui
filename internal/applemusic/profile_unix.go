//go:build !windows

package applemusic

import (
	"errors"
	"os"
)

func prepareProfile(path string) (os.FileInfo, error) {
	if err := os.MkdirAll(path, 0700); err != nil {
		return nil, err
	}
	info, err := profileDirectoryInfo(path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0700); err != nil {
		return nil, err
	}
	return info, nil
}

func profileDirectoryInfo(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("浏览器配置路径必须是独立目录，不能是符号链接")
	}
	return info, nil
}

func protectProfileRoot(root *os.Root) error {
	return root.Chmod(".", 0700)
}
