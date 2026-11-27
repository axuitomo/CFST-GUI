//go:build windows && tray

package host

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// 发行版以 -H windowsgui 链接，没有控制台；stderr 写不出去，只能改用系统弹窗。
var procGetConsoleWindow = windows.NewLazySystemDLL("kernel32.dll").NewProc("GetConsoleWindow")

func refuseRemovedCLI(message string) {
	if hasConsoleWindow() {
		fmt.Fprintln(os.Stderr, message)
		return
	}
	text, err := windows.UTF16PtrFromString(message)
	if err != nil {
		fmt.Fprintln(os.Stderr, message)
		return
	}
	title, err := windows.UTF16PtrFromString("CFST-GUI")
	if err != nil {
		fmt.Fprintln(os.Stderr, message)
		return
	}
	_, _ = windows.MessageBox(0, text, title, windows.MB_OK|windows.MB_ICONWARNING)
}

func hasConsoleWindow() bool {
	if err := procGetConsoleWindow.Find(); err != nil {
		return false
	}
	hwnd, _, _ := procGetConsoleWindow.Call()
	return hwnd != 0
}
