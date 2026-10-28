//go:build !windows

package applemusic

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func openCookieFile(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "cookie input")
	var stat unix.Stat_t
	if err = unix.Fstat(fd, &stat); err != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0077 != 0 || stat.Uid != uint32(os.Geteuid()) {
		file.Close()
		return nil, errors.New("cookie input must be a private regular file owned by the current user")
	}
	return file, nil
}
