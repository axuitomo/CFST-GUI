package appcore

import (
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
)

func TestCloudflareProviderEnabledFromSnapshotAllowsRoutingTarget(t *testing.T) {
	snapshot := map[string]any{
		"cloudflare": map[string]any{
			"api_token":       "test-token",
			"enabled":         false,
			"record_name":     "",
			"routing_enabled": true,
			"routing_rules": []map[string]any{
				{"enabled": true, "record_name": "hk.example.com"},
			},
			"zone_id": "zone-123",
		},
	}

	if !CloudflareProviderEnabledFromSnapshot(snapshot) {
		t.Fatal("CloudflareProviderEnabledFromSnapshot = false, want true for valid routing target")
	}
}

func TestCloudflareProviderEnabledFromSnapshotRequiresUsableRoutingConfig(t *testing.T) {
	tests := []struct {
		name     string
		mutation func(map[string]any)
	}{
		{
			name: "masked token",
			mutation: func(cloudflare map[string]any) {
				cloudflare["api_token"] = "sk-...masked"
			},
		},
		{
			name: "missing zone",
			mutation: func(cloudflare map[string]any) {
				cloudflare["zone_id"] = ""
			},
		},
		{
			name: "routing disabled",
			mutation: func(cloudflare map[string]any) {
				cloudflare["routing_enabled"] = false
			},
		},
		{
			name: "no enabled target",
			mutation: func(cloudflare map[string]any) {
				cloudflare["routing_rules"] = []map[string]any{
					{"enabled": false, "record_name": "hk.example.com"},
					{"enabled": true, "record_name": ""},
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cloudflare := map[string]any{
				"api_token":       "test-token",
				"enabled":         false,
				"record_name":     "",
				"routing_enabled": true,
				"routing_rules": []map[string]any{
					{"enabled": true, "record_name": "hk.example.com"},
				},
				"zone_id": "zone-123",
			}
			tc.mutation(cloudflare)

			if CloudflareProviderEnabledFromSnapshot(map[string]any{"cloudflare": cloudflare}) {
				t.Fatal("CloudflareProviderEnabledFromSnapshot = true, want false")
			}
		})
	}
}

func TestCloudflareProviderEnabledFromSnapshotKeepsExplicitProviderEnabled(t *testing.T) {
	snapshot := map[string]any{
		"cloudflare": map[string]any{
			"enabled": true,
		},
	}

	if !CloudflareProviderEnabledFromSnapshot(snapshot) {
		t.Fatal("CloudflareProviderEnabledFromSnapshot = false, want true for explicit provider enabled")
	}
}

// 推送被终止时补发的通知：状态必须是 aborted，且带上已完成的测速漏斗。
func TestBuildPostProbeAbortedNotificationReportsAborted(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir()}})
	snapshot := map[string]any{"post_probe_push": map[string]any{"cloudflare_enabled": true, "github_enabled": true}}
	result := ProbeRunResult{
		DurationMS: 90_000,
		Source:     probecore.SourceSummary{ValidCount: 120},
		Summary:    probecore.ProbeSummary{Passed: 7, Total: 120},
		TaskID:     "aborted-task",
	}

	notification := service.buildPostProbeAbortedNotification(snapshot, result)
	if notification == nil {
		t.Fatal("notification = nil, want aborted notification when a provider is enabled")
	}
	if notification.Status != UploadNotificationStatusAborted || notification.GitHubStatus != UploadNotificationStatusAborted {
		t.Fatalf("notification = %#v, want aborted status", notification)
	}
	if len(notification.Providers) == 0 || notification.Providers[0].Status != UploadNotificationStatusAborted {
		t.Fatalf("providers = %#v, want aborted DNS provider", notification.Providers)
	}
	if notification.Funnel == nil || notification.Funnel.Candidates != 120 || notification.Funnel.Passed != 7 {
		t.Fatalf("funnel = %#v", notification.Funnel)
	}
	if notification.DurationMS != 90_000 {
		t.Fatalf("duration = %d, want 90000", notification.DurationMS)
	}
	if text := UploadNotificationSummaryText(*notification); !strings.Contains(text, "状态：🚫 熔断/终止") {
		t.Fatalf("text = %q, want aborted status label", text)
	}
	if got := service.buildPostProbeAbortedNotification(map[string]any{}, result); got != nil {
		t.Fatalf("notification = %#v, want nil when no provider is enabled", got)
	}
}
