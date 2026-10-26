//go:build windows

package applemusic

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

func startChrome(cmd *exec.Cmd, read, write *os.File) error {
	var handles [2]windows.Handle
	for i, file := range []*os.File{read, write} {
		if err := windows.DuplicateHandle(windows.CurrentProcess(), windows.Handle(file.Fd()), windows.CurrentProcess(), &handles[i], 0, true, windows.DUPLICATE_SAME_ACCESS); err != nil {
			return err
		}
		defer windows.CloseHandle(handles[i])
	}
	// Windows Chrome consumes native handles, not Unix fd 3/4 or a TCP port.
	cmd.Args = append(cmd.Args, fmt.Sprintf("--remote-debugging-io-pipes=%d,%d", handles[0], handles[1]))
	cmd.SysProcAttr = &syscall.SysProcAttr{AdditionalInheritedHandles: []syscall.Handle{syscall.Handle(handles[0]), syscall.Handle(handles[1])}}
	return cmd.Start()
}
