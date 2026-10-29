//go:build !webui

package app

import (
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
)

func TestUploadNotificationBody(t *testing.T) {
	tests := []struct {
		event appcore.ProbeEvent
		name  string
		want  string
	}{
		{
			name:  "上传结论取 message",
			event: appcore.ProbeEvent{Event: appcore.UploadNotificationEvent, Payload: map[string]any{"message": "已推送 3 条"}},
			want:  "已推送 3 条",
		},
		{
			name:  "空白正文不发通知",
			event: appcore.ProbeEvent{Event: appcore.UploadNotificationEvent, Payload: map[string]any{"message": "   "}},
			want:  "",
		},
		{
			name:  "非字符串正文不发通知",
			event: appcore.ProbeEvent{Event: appcore.UploadNotificationEvent, Payload: map[string]any{"message": 3}},
			want:  "",
		},
		{
			name:  "空载荷不发通知",
			event: appcore.ProbeEvent{Event: appcore.UploadNotificationEvent},
			want:  "",
		},
		{
			name:  "其他事件不发通知",
			event: appcore.ProbeEvent{Event: "probe.completed", Payload: map[string]any{"message": "完成"}},
			want:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := uploadNotificationBody(tt.event); got != tt.want {
				t.Fatalf("uploadNotificationBody() = %q, want %q", got, tt.want)
			}
		})
	}
}
