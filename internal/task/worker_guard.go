package task

import (
	"context"
	"fmt"
	"runtime/pprof"
)

// recoverWorkerPanic 兜住单个探测 worker goroutine 的 panic。
// 父 goroutine 的 recover 只覆盖自己所在的栈，抓不到子 goroutine 的 panic，
// 不在这里兜住就是整个进程闪退；兜住后只记录并把该 IP 计为失败，
// 其余 IP 继续跑完（进度条会少计这一个，可接受）。
// 必须写成 `defer e.recoverWorkerPanic(...)`：recover() 只有直接被 defer 的函数调用才生效。
// 阶段 2 的 worker 还要额外推进进度，继续用它自己的内联 recover。
func (e *Engine) recoverWorkerPanic(stage, ip, reason string) {
	recovered := recover()
	if recovered == nil {
		return
	}
	e.noteStageProbeOutcome(stage, ip, false)
	e.workerPanics.Add(1)
	e.debugEvent("probe.failed", map[string]any{
		"ip":      ip,
		"level":   "error",
		"message": fmt.Sprintf("%s worker 异常退出：%v", stage, recovered),
		"reason":  reason,
		"stage":   stage,
	})
}

// WorkerPanicCount 返回本次探测中异常退出的 worker 数。panic 明细只进调试日志，
// 关掉 debug 时上层仍要靠这个计数告诉用户“结果少了是因为有 worker 崩了”。
func (e *Engine) WorkerPanicCount() int {
	if e == nil {
		return 0
	}
	return int(e.workerPanics.Load())
}

// setWorkerLabels 给 worker goroutine 打上阶段与 IP 标签。
// Go 1.27 起 goroutine label 会出现在崩溃栈里（GODEBUG=tracebacklabels 默认开启），
// 万一仍然崩在 worker 里，栈上就能直接看出是哪个阶段、哪个 IP。
func setWorkerLabels(stage, ip string) {
	pprof.SetGoroutineLabels(pprof.WithLabels(context.Background(), pprof.Labels("stage", stage, "ip", ip)))
}
