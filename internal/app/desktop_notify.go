//go:build !webui

package app

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/utils"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// 桌面通知不通过 application.NewService 注册：Wails 把服务启动错误当成致命错误
// （pkg/application/application.go 的 Run 在 startupService 失败时直接返回），而 Windows
// 后端的 ServiceStartup 要向 HKCU\Software\Classes\CLSID 写 toast 激活器，MSIX 打包下这一
// 写未必成功。通知只是任务完成后的附加提示，初始化失败只能记日志，不能拦住窗口和托盘。
var (
	desktopNotifierOnce sync.Once
	desktopNotifier     *notifications.NotificationService
)

// startDesktopNotifier 首次发送前懒初始化通知服务，失败返回 nil 并留痕。
func startDesktopNotifier() *notifications.NotificationService {
	desktopNotifierOnce.Do(func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				logDesktopNotifyFailure("desktop.notify_init_failed", fmt.Sprintf("%v", recovered))
			}
		}()
		service := notifications.New()
		if err := service.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
			logDesktopNotifyFailure("desktop.notify_init_failed", err.Error())
			return
		}
		// Windows/Linux 恒为 true；macOS 未授权时系统会直接丢弃通知，因此第一次发送前补一次授权。
		if _, err := service.RequestNotificationAuthorization(); err != nil {
			logDesktopNotifyFailure("desktop.notify_authorization_failed", err.Error())
		}
		desktopNotifier = service
	})
	return desktopNotifier
}

// sendDesktopNotification 发送一条系统通知。调用方必须异步调用：macOS 首次授权会阻塞到
// 用户在系统弹窗上作答，不能占住探测事件的转发路径。
func sendDesktopNotification(title, body string) {
	body = strings.TrimSpace(body)
	if body == "" {
		return
	}
	service := startDesktopNotifier()
	if service == nil {
		return
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			logDesktopNotifyFailure("desktop.notify_send_failed", fmt.Sprintf("%v", recovered))
		}
	}()
	if err := service.SendNotification(notifications.NotificationOptions{
		Body:  body,
		ID:    fmt.Sprintf("cfst-%d", time.Now().UnixNano()),
		Title: title,
	}); err != nil {
		logDesktopNotifyFailure("desktop.notify_send_failed", err.Error())
	}
}

// uploadNotificationBody 挑出需要弹系统通知的事件（上传结论），返回其正文；其余事件返回空串。
func uploadNotificationBody(event appcore.ProbeEvent) string {
	if event.Event != appcore.UploadNotificationEvent {
		return ""
	}
	message, _ := event.Payload["message"].(string)
	return strings.TrimSpace(message)
}

func logDesktopNotifyFailure(event, message string) {
	_ = utils.AppendErrorLog(errorLogFilePath(), event, map[string]any{"message": message})
}
