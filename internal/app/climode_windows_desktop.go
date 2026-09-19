//go:build windows && tray

package app

import (
	"os"

	"golang.org/x/sys/windows"
)

// 桌面发行版以 -H windowsgui 链接，没有控制台可写；收到 CLI 请求时弹窗说明并退出，
// 引导用户改用命令行版 cfst-gui-cli.exe。
func runCLIOrRefuse(args []string) {
	text := "CFST-GUI 桌面版不带控制台窗口，无法显示命令行输出。\n\n" +
		"请在终端使用命令行版 cfst-gui-cli.exe（GitHub Release 提供），例如：\n" +
		"cfst-gui-cli.exe --cli -f ip.txt -o result.csv\n\n" +
		"开发调试可执行 go run . --cli ..."
	// 弹窗失败时 err 同样被忽略：windowsgui 进程没有控制台可写，唯一出路是退出。
	_, _ = windows.MessageBox(0, mustUTF16Ptr(text), mustUTF16Ptr("CFST-GUI"), windows.MB_OK|windows.MB_ICONINFORMATION)
	os.Exit(2)
}

func mustUTF16Ptr(value string) *uint16 {
	ptr, err := windows.UTF16PtrFromString(value)
	if err != nil {
		os.Exit(2)
	}
	return ptr
}
