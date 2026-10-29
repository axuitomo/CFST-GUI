package app

import (
	"fmt"
	"os"
	"runtime/pprof"
	"strings"
)

// startCPUProfileFromEnv 在 CFST_CPU_PROFILE 指向文件时开启 CPU profile，返回停止函数。
// 用途只有两个：给 PGO 采样（`task pgo`），以及排查用户反馈的卡顿。
// 没有设置环境变量时不做任何事、不分配任何东西，正常路径零开销。
func startCPUProfileFromEnv() func() {
	path := strings.TrimSpace(os.Getenv("CFST_CPU_PROFILE"))
	if path == "" {
		return func() {}
	}
	file, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法创建 CPU profile 文件 %s：%v\n", path, err)
		return func() {}
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		fmt.Fprintf(os.Stderr, "无法启动 CPU profile：%v\n", err)
		_ = file.Close()
		return func() {}
	}
	// 必须在进程正常退出前 stop：os.Exit 会跳过 defer，采集 CLI 路径时要确保走完整个流程。
	return func() {
		pprof.StopCPUProfile()
		_ = file.Close()
	}
}
