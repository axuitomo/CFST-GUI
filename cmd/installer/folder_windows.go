package main

import (
	"errors"
	"syscall"
	"unsafe"
)

// SHBrowseForFolder 是老 API，但它是这里唯一不需要额外依赖、
// 也不引入 COM 接口定义的选择目录对话框。Windows 10/11 仍然正常工作。
const (
	bifReturnOnlyFSDirs = 0x0001
	bifEditBox          = 0x0010
	bifNewDialogStyle   = 0x0040
)

var (
	shell32               = syscall.NewLazyDLL("shell32.dll")
	ole32                 = syscall.NewLazyDLL("ole32.dll")
	procBrowseForFolder   = shell32.NewProc("SHBrowseForFolderW")
	procGetPathFromIDList = shell32.NewProc("SHGetPathFromIDListW")
	procCoTaskMemFree     = ole32.NewProc("CoTaskMemFree")
)

// BROWSEINFO 在 x64 上是 64 字节：hwndOwner/pidlRoot/pszDisplayName/lpszTitle/
// ulFlags(+padding)/lpfn/lParam/iImage(+padding)。这里按偏移直接填，
// 避免为一个对话框定义一整套 COM 结构。
const (
	biDisplayName = 16
	biTitle       = 24
	biFlags       = 32
	biSize        = 64
)

// pickFolder 返回用户选择的目录；取消时返回空串且无错误。
func pickFolder(string) (string, error) {
	title, err := syscall.UTF16PtrFromString("选择安装位置")
	if err != nil {
		return "", err
	}
	display := make([]uint16, syscall.MAX_PATH)

	buf := make([]byte, biSize)
	*(*uintptr)(unsafe.Pointer(&buf[biDisplayName])) = uintptr(unsafe.Pointer(&display[0]))
	*(*uintptr)(unsafe.Pointer(&buf[biTitle])) = uintptr(unsafe.Pointer(title))
	*(*uint32)(unsafe.Pointer(&buf[biFlags])) = bifReturnOnlyFSDirs | bifEditBox | bifNewDialogStyle

	pidl, _, _ := procBrowseForFolder.Call(uintptr(unsafe.Pointer(&buf[0])))
	if pidl == 0 {
		return "", nil
	}
	defer func() { _, _, _ = procCoTaskMemFree.Call(pidl) }()

	out := make([]uint16, syscall.MAX_PATH)
	result, _, _ := procGetPathFromIDList.Call(pidl, uintptr(unsafe.Pointer(&out[0])))
	if result == 0 {
		return "", errors.New("SHGetPathFromIDListW failed")
	}
	return syscall.UTF16ToString(out), nil
}
