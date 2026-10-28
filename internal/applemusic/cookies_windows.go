//go:build windows

package applemusic

import (
	"errors"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

func openCookieFile(path string) (*os.File, error) {
	if !localCookiePath(path) {
		return nil, errors.New("cookie input must be a local file, not a network or device path")
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.READ_CONTROL, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(h), "cookie input")
	var info windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(h, &info); err != nil || info.FileAttributes&(windows.FILE_ATTRIBUTE_DIRECTORY|windows.FILE_ATTRIBUTE_REPARSE_POINT) != 0 {
		file.Close()
		return nil, errors.New("cookie input must be a regular file")
	}
	if err = checkPrivateCookieFile(h); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func checkPrivateCookieFile(handle windows.Handle) error {
	invalid := errors.New("cookie file must have a current-user-only protected ACL")
	sd, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || sd == nil {
		return invalid
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return invalid
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil || !owner.Equals(user.User.Sid) {
		return invalid
	}
	control, _, err := sd.Control()
	if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
		return invalid
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil || dacl.AceCount == 0 {
		return invalid
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if windows.GetAce(dacl, i, &ace) != nil {
			return invalid
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 || !sid.Equals(user.User.Sid) {
			return invalid
		}
	}
	return nil
}
