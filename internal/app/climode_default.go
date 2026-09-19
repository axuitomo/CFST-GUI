//go:build !windows || !tray

package app

// 非 Windows 桌面发行构建：CLI 输出有终端或重定向可写，直接进入 CLI。
func runCLIOrRefuse(args []string) {
	runCLI(args)
}
