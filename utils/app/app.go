package app

import (
	"os"
	"path/filepath"

	"github.com/go-musicfox/go-musicfox/internal/types"
)

var pureRoot string

func SetupPureRoot(root string) { pureRoot = root }

func ConfigDir() string {
	if pureRoot != "" {
		return pureRoot
	}
	if root := os.Getenv("MUSICFOX_ROOT"); root != "" {
		return root
	}
	root, err := os.UserConfigDir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(root, types.AppLocalDataDir)
}

func ConfigFilePath() string {
	return filepath.Join(ConfigDir(), types.AppTomlFile)
}

func LogDir() string {
	var dir string
	if pureRoot != "" {
		dir = filepath.Join(pureRoot, "log")
	} else if root := os.Getenv("MUSICFOX_ROOT"); root != "" {
		dir = filepath.Join(root, "log")
	} else {
		root := os.Getenv("XDG_STATE_HOME")
		if root == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				panic(err)
			}
			root = filepath.Join(home, ".local", "state")
		}
		dir = filepath.Join(root, types.AppLocalDataDir, "log")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		panic(err)
	}
	return dir
}
