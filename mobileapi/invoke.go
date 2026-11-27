package mobileapi

import (
	"fmt"
	"runtime/debug"
	"strings"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore"
)

func (s *Service) Invoke(command string, payloadJSON string) (resultJSON string) {
	command = strings.ToLower(strings.TrimSpace(command))
	defer s.recoverServicePanic(&resultJSON, command)
	if command == "bridge.trace" {
		payload, err := decodeObject(payloadJSON)
		if err != nil {
			return encodeCommand(appcore.NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil))
		}
		return s.recordBridgeTrace(payload)
	}
	started := time.Now()
	var result appcore.CommandResult
	defer func() {
		fields := map[string]any{
			"command": command, "code": result.Code, "ok": result.OK,
			"duration_ms": time.Since(started).Milliseconds(),
		}
		if result.TaskID != nil {
			fields["task_id"] = *result.TaskID
		}
		s.trace("invoke.out", fields)
	}()
	if coreResult, handled := s.core.TryInvoke(command, payloadJSON); handled {
		result = coreResult
		return encodeCommand(result)
	}
	payload, err := decodeObject(payloadJSON)
	if err != nil {
		result = appcore.NewCommandResult("COMMAND_PAYLOAD_INVALID", nil, err.Error(), false, nil, nil)
		return encodeCommand(result)
	}
	switch command {
	case "probe.record_export":
		result = s.recordAndroidExportResult(payload)
	default:
		result = appcore.NewCommandResult("COMMAND_UNKNOWN", nil, fmt.Sprintf("unknown command: %s", command), false, nil, nil)
	}
	return encodeCommand(result)
}

// recoverServicePanic 把命令处理里的 panic 收回到普通失败信封。
// gomobile 边界上未捕获的 panic 会直接终止 Android 进程，用户看到的是任务和应用
// 一起消失；收回成 COMMAND_PANIC 后，前端还能显示原因并保留任务现场。
func (s *Service) recoverServicePanic(resultJSON *string, command string) {
	recovered := recover()
	if recovered == nil {
		return
	}
	message := fmt.Sprintf("native panic: %v", recovered)
	s.logInvokePanic(command, message, string(debug.Stack()))
	*resultJSON = encodeCommand(appcore.NewCommandResult("COMMAND_PANIC", nil, message, false, nil, nil))
}

func (s *Service) logInvokePanic(command, message, stack string) {
	// 日志本身也是可能失败的弱路径，不能再把 panic 甩回桥接层。
	defer func() {
		_ = recover()
	}()
	s.core.DebugLogger().Event("bridge.panic", map[string]any{
		"command": command,
		"error":   message,
		"stack":   stack,
	})
	s.trace("invoke.panic", map[string]any{"command": command, "error_type": "panic", "ok": false})
}
