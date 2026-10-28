//go:build windows

package applemusic

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func secureCookieTestFile(t *testing.T, path string) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("O:" + user.User.Sid.String() + "D:P(A;;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err = windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, user.User.Sid, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCookieFileRejectsSharedWindowsACL(t *testing.T) {
	file := filepath.Join(t.TempDir(), "synthetic.json")
	if err := os.WriteFile(file, []byte(`[{"domain":"music.apple.com","name":"synthetic","value":"fixture"}]`), 0600); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err = windows.SetNamedSecurityInfo(file, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	if _, err = ReadCookieFile(file); err == nil {
		t.Fatal("shared cookie file accepted")
	}
	secureCookieTestFile(t, file)
	if _, err = ReadCookieFile(file); err != nil {
		t.Fatal(err)
	}
}
