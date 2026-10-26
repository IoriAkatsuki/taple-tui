//go:build windows

package applemusic

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Junction creation needs no symlink privilege, including on Windows CI.
func makeProfileTestLink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Mkdir(link, 0700); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(link)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	target, err = filepath.Abs(target)
	if err != nil {
		t.Fatal(err)
	}
	substitute, err := windows.UTF16FromString(`\??\` + target)
	if err != nil {
		t.Fatal(err)
	}
	printName, err := windows.UTF16FromString(target)
	if err != nil {
		t.Fatal(err)
	}
	data := make([]byte, 16+2*(len(substitute)+len(printName)))
	binary.LittleEndian.PutUint32(data[0:], windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(data[4:], uint16(len(data)-8))
	binary.LittleEndian.PutUint16(data[10:], uint16((len(substitute)-1)*2))
	binary.LittleEndian.PutUint16(data[12:], uint16(len(substitute)*2))
	binary.LittleEndian.PutUint16(data[14:], uint16((len(printName)-1)*2))
	for i, c := range append(substitute, printName...) {
		binary.LittleEndian.PutUint16(data[16+i*2:], c)
	}
	var returned uint32
	if err := windows.DeviceIoControl(h, windows.FSCTL_SET_REPARSE_POINT, &data[0], uint32(len(data)), nil, 0, &returned, nil); err != nil {
		t.Fatal(err)
	}
}

func assertProfilePrivate(t *testing.T, path string) {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || sd == nil {
		t.Fatal("cannot read profile ACL:", err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	owner, _, err := sd.Owner()
	if err != nil || !owner.Equals(user.User.Sid) {
		t.Fatal("profile is not owned by the current user")
	}
	control, _, err := sd.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatal("profile inherits a parent ACL")
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil || dacl.AceCount != 1 {
		t.Fatal("profile must grant only the current user")
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		t.Fatal(err)
	}
	sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || !sid.Equals(user.User.Sid) ||
		ace.Header.AceFlags != windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE || ace.Mask != 0x1f01ff {
		t.Fatal("profile does not have a user-only inheritable full-control ACL")
	}
}

func TestWindowsProfilePrivacyAndFrozenIdentity(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "chrome")
	before, err := prepareProfile(profile)
	if err != nil {
		assertProfilePrivate(t, profile)
		t.Fatal(err)
	}
	assertProfilePrivate(t, profile)
	if err := os.WriteFile(filepath.Join(profile, "synthetic"), []byte("not credentials"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareProfile(profile); err != nil {
		t.Fatal("private existing profile rejected:", err)
	}
	if err := os.Rename(profile, profile+".old"); err != nil {
		t.Fatal(err)
	}
	after, err := prepareProfile(profile)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(before, after) {
		t.Fatal("profile identity was lazily read from the replacement path")
	}
}

func TestWindowsRejectsPublicNonemptyProfileWithoutChangingIt(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "chrome")
	if err := os.Mkdir(profile, 0700); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;OICI;FA;;;WD)")
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(profile, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(profile, "synthetic")
	if err := os.WriteFile(marker, []byte("not credentials"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := prepareProfile(profile); err == nil {
		t.Fatal("existing non-private data was accepted")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("rejected profile was modified")
	}
}

func TestWindowsRejectsJunctionInProfileParent(t *testing.T) {
	parent := t.TempDir()
	outside := filepath.Join(parent, "outside")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "redirected")
	makeProfileTestLink(t, outside, link)
	if _, err := prepareProfile(filepath.Join(link, "chrome")); err == nil {
		t.Fatal("profile was created through a junction")
	}
	if _, err := os.Stat(filepath.Join(outside, "chrome")); !os.IsNotExist(err) {
		t.Fatal("rejected junction target was modified")
	}
}
