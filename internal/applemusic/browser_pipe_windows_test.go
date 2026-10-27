//go:build windows

package applemusic

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestWindowsChromeFailedStartClosesInheritedHandles(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()
	cmd := exec.Command(filepath.Join(t.TempDir(), "missing-chrome.exe"))
	if err := startChrome(cmd, read, write); err == nil {
		t.Fatal("missing Chrome unexpectedly started")
	}
	if cmd.SysProcAttr == nil || len(cmd.SysProcAttr.AdditionalInheritedHandles) != 2 {
		t.Fatal("did not exercise inherited handle setup")
	}
	for _, handle := range cmd.SysProcAttr.AdditionalInheritedHandles {
		if _, err := windows.GetFileType(windows.Handle(handle)); err != windows.ERROR_INVALID_HANDLE {
			t.Fatal("failed startup leaked an inheritable handle:", err)
		}
	}
	for _, file := range []*os.File{read, write} {
		if _, err := windows.GetFileType(windows.Handle(file.Fd())); err != nil {
			t.Fatal("startup took ownership of a caller pipe:", err)
		}
	}
}

func TestWindowsChromeDiscoveryUsesUserInstall(t *testing.T) {
	t.Setenv("TAPEFOX_BROWSER", "")
	root := t.TempDir()
	t.Setenv("PATH", root)
	t.Setenv("LOCALAPPDATA", root)
	t.Setenv("PROGRAMFILES", "")
	t.Setenv("PROGRAMFILES(X86)", "")
	binary := filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe")
	if err := os.MkdirAll(filepath.Dir(binary), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, nil, 0700); err != nil {
		t.Fatal(err)
	}
	found, err := findChrome()
	if err != nil || found != binary {
		t.Fatalf("user-local Chrome was not discovered: %q %v", found, err)
	}
}

func TestWindowsChromePipeHandles(t *testing.T) {
	childRead, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer childRead.Close()
	defer write.Close()
	read, childWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer childWrite.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWindowsChromePipeChild$", "--")
	cmd.Env = append(os.Environ(), "TAPEFOX_PIPE_CHILD=1")
	if err := startChrome(cmd, childRead, childWrite); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	childRead.Close()
	childWrite.Close()
	if len(cmd.ExtraFiles) != 0 || cmd.SysProcAttr == nil || len(cmd.SysProcAttr.AdditionalInheritedHandles) != 2 {
		t.Fatal("Windows must inherit exactly the two explicit private pipe handles")
	}
	if _, err := write.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	var response [4]byte
	if _, err := io.ReadFull(read, response[:]); err != nil || string(response[:]) != "pong" {
		t.Fatalf("inherited duplex pipes failed: %q %v", response, err)
	}
}

func TestWindowsChromePipeChild(t *testing.T) {
	if os.Getenv("TAPEFOX_PIPE_CHILD") != "1" {
		return
	}
	for _, arg := range os.Args {
		value, ok := strings.CutPrefix(arg, "--remote-debugging-io-pipes=")
		if !ok {
			continue
		}
		parts := strings.Split(value, ",")
		if len(parts) != 2 {
			t.Fatal("invalid pipe handle pair")
		}
		handles := make([]uintptr, 2)
		for i, part := range parts {
			n, err := strconv.ParseUint(part, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			handles[i] = uintptr(n)
		}
		read := os.NewFile(handles[0], "inherited-read")
		write := os.NewFile(handles[1], "inherited-write")
		defer read.Close()
		defer write.Close()
		var request [4]byte
		if _, err := io.ReadFull(read, request[:]); err != nil || string(request[:]) != "ping" {
			t.Fatal("child could not read inherited pipe:", err)
		}
		if _, err := fmt.Fprint(write, "pong"); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatal("missing explicit Chromium pipe handles")
}
