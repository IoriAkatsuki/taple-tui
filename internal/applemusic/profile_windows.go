//go:build windows

package applemusic

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

func prepareProfile(path string) (os.FileInfo, error) {
	if err := rejectProfileReparsePoints(path); err != nil {
		return nil, err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	sd, err := windows.SecurityDescriptorFromString("O:" + user.User.Sid.String() + "D:P(A;OICI;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	if err := rejectProfileReparsePoints(path); err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	attrs := windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	if err := windows.CreateDirectory(name, &attrs); err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return nil, err
	}
	file, err := openProfileDirectory(path, windows.WRITE_DAC|windows.WRITE_OWNER|windows.FILE_LIST_DIRECTORY)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if err := checkPrivateProfile(file); err != nil {
		// Tighten only empty temporary/new directories, never adopt existing data.
		if _, readErr := file.Readdirnames(1); readErr != io.EOF {
			return nil, errors.New("现有 Chrome 配置目录不是当前用户专用目录，拒绝使用")
		}
		dacl, _, err := sd.DACL()
		if err != nil || dacl == nil {
			return nil, errors.New("无法建立 Chrome 配置目录的私有 ACL")
		}
		if err := windows.SetSecurityInfo(windows.Handle(file.Fd()), windows.SE_FILE_OBJECT,
			windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
			user.User.Sid, nil, dacl, nil); err != nil {
			return nil, err
		}
		if err := checkPrivateProfile(file); err != nil {
			return nil, err
		}
	}
	// File.Stat freezes the Windows file ID; Lstat defers ID lookup until SameFile.
	return file.Stat()
}

func rejectProfileReparsePoints(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for {
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return err
		}
		attrs, err := windows.GetFileAttributes(name)
		if err != nil && !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) && !errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			return err
		}
		if err == nil && attrs&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return errors.New("浏览器配置路径不能包含符号链接、junction 或其他 reparse point")
		}
		parent := filepath.Dir(path)
		if parent == path {
			return nil
		}
		path = parent
	}
}

func openProfileDirectory(path string, access uint32) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(name, access|windows.READ_CONTROL|windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return nil, err
	}
	var info windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(h, &info)
	if err != nil || info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		windows.CloseHandle(h)
		return nil, errors.New("浏览器配置路径必须是独立目录，不能是 reparse point")
	}
	return os.NewFile(uintptr(h), path), nil
}

func checkPrivateProfile(file *os.File) error {
	invalid := errors.New("Chrome 配置目录必须具有当前用户专用的受保护 ACL")
	sd, err := windows.GetSecurityInfo(windows.Handle(file.Fd()), windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil || sd == nil {
		return invalid
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
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
	if err != nil || dacl == nil || dacl.AceCount != 1 {
		return invalid
	}
	var ace *windows.ACCESS_ALLOWED_ACE
	if err := windows.GetAce(dacl, 0, &ace); err != nil {
		return err
	}
	sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || !sid.Equals(user.User.Sid) ||
		ace.Header.AceFlags != windows.OBJECT_INHERIT_ACE|windows.CONTAINER_INHERIT_ACE || ace.Mask != 0x1f01ff {
		return invalid
	}
	return nil
}

func profileDirectoryInfo(path string) (os.FileInfo, error) {
	if err := rejectProfileReparsePoints(path); err != nil {
		return nil, err
	}
	file, err := openProfileDirectory(path, 0)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if err := checkPrivateProfile(file); err != nil {
		return nil, err
	}
	return file.Stat()
}

func protectProfileRoot(root *os.Root) error {
	file, err := root.Open(".")
	if err != nil {
		return err
	}
	defer file.Close()
	return checkPrivateProfile(file)
}
