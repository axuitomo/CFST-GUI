package appcore

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func TestServiceInvokeSourcePreviewUsesSharedSourcePipeline(t *testing.T) {
	service := newSourceServiceForTest(t, ServiceOptions{
		SourceResolver: resolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
			if host != "edge.example.com" {
				t.Fatalf("resolved host = %q", host)
			}
			return []net.IPAddr{{IP: net.ParseIP("203.0.113.20")}}, nil
		}),
	})
	payload := encodeSourceServicePayload(t, map[string]any{
		"preview_limit": 3,
		"source": map[string]any{
			"content":  "1.1.1.1\n1.1.1.1\nhttps://edge.example.com/ips.txt\nbad",
			"ip_limit": 8,
			"ip_mode":  "traverse",
			"kind":     "inline",
			"name":     "shared-preview",
		},
	})

	result := service.Invoke("source.preview", payload)
	if !result.OK || result.Code != "SOURCE_PREVIEW_READY" {
		t.Fatalf("preview result = %#v", result)
	}
	data := mapValue(result.Data)
	entries, _ := data["preview_entries"].([]string)
	if want := []string{"1.1.1.1", "203.0.113.20"}; !reflect.DeepEqual(entries, want) {
		t.Fatalf("preview entries = %#v, want %#v", entries, want)
	}
	summary := mapValue(data["summary"])
	if got := intValue(summary["invalid_count"], 0); got != 1 {
		t.Fatalf("invalid_count = %d, want 1", got)
	}
	if summary["action"] != "预览" || summary["name"] != "shared-preview" {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestServiceInvokeSourceFetchPersistsStatus(t *testing.T) {
	service := newSourceServiceForTest(t, ServiceOptions{})
	if _, err := service.SaveConfig(map[string]any{
		"probe": map[string]any{"tcp_port": 443},
		"sources": []any{map[string]any{
			"content": "1.1.1.1",
			"enabled": true,
			"id":      "source-1",
			"kind":    "inline",
			"name":    "saved-source",
		}},
	}); err != nil {
		t.Fatal(err)
	}

	result := service.Invoke("source.fetch", encodeSourceServicePayload(t, map[string]any{
		"source": map[string]any{
			"content": "1.1.1.1\n1.0.0.1",
			"enabled": true,
			"id":      "source-1",
			"kind":    "inline",
			"name":    "saved-source",
		},
	}))
	if !result.OK {
		t.Fatalf("fetch result = %#v", result)
	}
	loaded, err := service.LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	sources := SourcesFromAny(loaded.Snapshot["sources"])
	if len(sources) != 1 || sources[0].LastFetchedCount != 2 || sources[0].LastFetchedAt == "" || sources[0].StatusText == "" {
		t.Fatalf("persisted sources = %#v", sources)
	}
}

func TestServiceInvokeSourceRejectsInvalidPayloadAndEmptyInput(t *testing.T) {
	service := newSourceServiceForTest(t, ServiceOptions{})
	invalid := service.Invoke("source.preview", "{")
	if invalid.OK || invalid.Code != "SOURCE_PAYLOAD_INVALID" {
		t.Fatalf("invalid payload result = %#v", invalid)
	}
	empty := service.Invoke("source.preview", `{}`)
	if empty.OK || empty.Code != "SOURCE_INPUT_EMPTY" {
		t.Fatalf("empty source result = %#v", empty)
	}
}

func newSourceServiceForTest(t *testing.T, overrides ServiceOptions) *Service {
	t.Helper()
	root := t.TempDir()
	overrides.Storage = StorageLayout{Root: root, ConfigFileName: "config.json"}
	overrides.ConfigPolicy = ConfigPolicy{
		DefaultSnapshot:  func() map[string]any { return map[string]any{"probe": map[string]any{"tcp_port": 443}} },
		SanitizeSnapshot: func(snapshot map[string]any) map[string]any { return snapshot },
	}
	overrides.ProbeConfigOptions = probecore.ConfigSnapshotOptions{
		DefaultExportTargetDir: filepath.Join(root, "exports"),
		DefaultSourceIPLimit:   500,
		IncludePortPolicy:      true,
		PortPolicy:             probecore.PortPolicySourceOverrideGlobal,
	}
	return NewService(overrides)
}

func encodeSourceServicePayload(t *testing.T, payload map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
