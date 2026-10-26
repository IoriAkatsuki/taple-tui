//go:build !windows

package applemusic

import (
	"os"
	"os/exec"
)

func startChrome(cmd *exec.Cmd, read, write *os.File) error {
	cmd.ExtraFiles = []*os.File{read, write}
	return cmd.Start()
}
