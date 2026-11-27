package main

import (
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows/registry"
)

const (
	webview2GUID    = `{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`
	webview2URL     = "https://developer.microsoft.com/microsoft-edge/webview2/consumer/"
	webview2Missing = "CFST-GUI 需要 Microsoft Edge WebView2 Runtime。\r\n\r\n" +
		"当前系统未检测到该运行时。点「确定」打开微软下载页，装好运行时后重新运行本安装程序。"
	mbOK              = 0x00000000
	mbIconExclamation = 0x00000030
	mbIconError       = 0x00000010
	mbIconQuestion    = 0x00000020
	mbYesNo           = 0x00000004
	idYes             = 6
)

// webView2Installed 与原先 NSIS 的判断保持一致：
// 依次查 HKLM、WOW6432Node、HKCU 的 EdgeUpdate 客户端项，pv 非空且非 0.0.0.0 即视为已安装。
func webView2Installed() bool {
	probes := []struct {
		root registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\EdgeUpdate\Clients\` + webview2GUID},
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\` + webview2GUID},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\EdgeUpdate\Clients\` + webview2GUID},
	}
	for _, probe := range probes {
		key, err := registry.OpenKey(probe.root, probe.path, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		pv, _, err := key.GetStringValue("pv")
		key.Close()
		if err == nil && pv != "" && pv != "0.0.0.0" {
			return true
		}
	}
	return false
}

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW = user32.NewProc("MessageBoxW")
)

// messageBox 在创建任何窗口之前用原生对话框提示。
// 安装器界面依赖 WebView2，所以缺运行时这条路径不能走 UI。
func messageBox(text, title string, flags uint32) int {
	textPtr, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return 0
	}
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return 0
	}
	ret, _, _ := procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(textPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(flags),
	)
	return int(ret)
}

func openURL(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
