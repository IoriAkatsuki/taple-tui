package commands

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCookieInputIsValidatedBeforeBrowserStartup(t *testing.T) {
	previous := GlobalOptions
	defer func() { GlobalOptions = previous }()
	GlobalOptions.CookieFile = filepath.Join(t.TempDir(), "not-present-synthetic.json")
	t.Setenv("TAPLE_BROWSER", filepath.Join(t.TempDir(), "missing-runtime"))
	err := NewAppleMusicCommand().Func(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "Cookie") || strings.Contains(err.Error(), "not-present") || strings.Contains(err.Error(), "missing-runtime") {
		t.Fatalf("input must fail safely before startup: %v", err)
	}
}
