package host

import (
	"os"
)

var version = "2.2.0"

// HandleRemovedCLI rejects deprecated CLI flags unless arguments are exclusively dropped source files.
// Dropped source paths are returned so the caller can forward them to Options.StartupSources.
func HandleRemovedCLI(args []string) []string {
	if !ShouldRefuseRemovedCLI(args) {
		return nil
	}
	if dropped := DroppedSourceFiles(args); len(dropped) > 0 {
		return dropped
	}
	refuseRemovedCLI("CFST-GUI 已移除 CLI 模式，请启动图形界面，或使用 Linux WebUI 与 Android 应用。")
	os.Exit(2)
	return nil
}

func ShouldRefuseRemovedCLI(args []string) bool {
	if len(args) == 0 {
		return false
	}
	// 保留 --gui 作为显式 GUI 启动方式，其余参数一律视为已移除的 CLI 调用。
	return args[0] != "--gui"
}
