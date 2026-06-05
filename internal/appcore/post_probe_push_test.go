package appcore

import "testing"

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
