package appcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/probe/probecore"
)

// The push entry point must refuse a bare IP list: rows carry the record_type
// filtering and the upload selection, so accepting raw IPs would let a caller
// write the address family its configuration excluded.
func TestPushCloudflareRejectsRawIPsWithoutRows(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"success":true,"result":[],"result_info":{"page":1,"total_pages":1}}`)
	}))
	defer server.Close()

	service := NewService(ServiceOptions{
		CloudflareAPIBaseURL: server.URL,
		Storage:              StorageLayout{Root: t.TempDir()},
	})
	snapshot := map[string]any{
		"cloudflare": map[string]any{
			"api_token":   "token-1",
			"record_name": "edge.example.com",
			"record_type": "ALL",
			"ttl":         300,
			"zone_id":     "zone-1",
		},
	}

	direct := service.pushCloudflareDNSRecordsContext(context.Background(), map[string]any{
		"config": snapshot,
		"ipsRaw": "1.1.1.1\n2001:db8::1",
	})
	if direct.Code != "DNS_INPUT_EMPTY" || direct.OK {
		t.Fatalf("direct result = %+v, want DNS_INPUT_EMPTY", direct)
	}

	throughDispatch := service.PushDNSProviders(context.Background(), map[string]any{
		"config": map[string]any{
			"dns_providers": []any{
				map[string]any{"enabled": true, "id": "cf", "provider": "cloudflare", "api_token": "token-1", "zone_id": "zone-1", "record_name": "edge.example.com", "record_type": "ALL", "ttl": 300},
			},
		},
		"ipsRaw": "1.1.1.1\n2001:db8::1",
	})
	if throughDispatch.Code != "DNS_INPUT_EMPTY" {
		t.Fatalf("dispatch result = %+v, want DNS_INPUT_EMPTY", throughDispatch)
	}
	if requests != 0 {
		t.Fatalf("cloudflare requests = %d, want 0", requests)
	}
}

// record_type selects the address family before the push reaches the core; the
// core itself writes both families from whatever pool it receives.
func TestPushCloudflareFiltersRowsByRecordType(t *testing.T) {
	cases := []struct {
		name       string
		recordType string
		want       []string
	}{
		{name: "A", recordType: "A", want: []string{"A"}},
		{name: "AAAA", recordType: "AAAA", want: []string{"AAAA"}},
		{name: "ALL", recordType: "ALL", want: []string{"A", "AAAA"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			created := make([]string, 0, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.Method {
				case http.MethodGet:
					fmt.Fprint(w, `{"success":true,"result":[],"result_info":{"page":1,"total_pages":1}}`)
				case http.MethodPost:
					var record map[string]any
					if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
						t.Fatalf("decode created record: %v", err)
					}
					createdType := stringValue(record["type"], "")
					created = append(created, createdType)
					response, err := json.Marshal(map[string]any{
						"success": true,
						"result": map[string]any{
							"content": stringValue(record["content"], ""),
							"id":      fmt.Sprintf("new-%d", len(created)),
							"name":    "edge.example.com",
							"ttl":     300,
							"type":    createdType,
						},
					})
					if err != nil {
						t.Fatalf("encode created response: %v", err)
					}
					_, _ = w.Write(response)
				default:
					t.Fatalf("unexpected cloudflare method %s", r.Method)
				}
			}))
			defer server.Close()

			service := NewService(ServiceOptions{
				CloudflareAPIBaseURL: server.URL,
				Storage:              StorageLayout{Root: t.TempDir()},
			})
			snapshot := map[string]any{
				"cloudflare": map[string]any{
					"api_token":   "token-1",
					"record_name": "edge.example.com",
					"record_type": testCase.recordType,
					"ttl":         300,
					"zone_id":     "zone-1",
				},
			}
			result := service.pushCloudflareDNSRecordsContext(context.Background(), map[string]any{
				"config": snapshot,
				"results": []probecore.ProbeRow{
					{Colo: "LAX", DownloadSpeedMB: 10, IP: "1.1.1.1"},
					{Colo: "LAX", DownloadSpeedMB: 10, IP: "2001:db8::1"},
				},
			})
			if !result.OK || result.Code != "DNS_PUSH_COMPLETED" {
				t.Fatalf("result = %+v, want completed push", result)
			}
			if !slices.Equal(created, testCase.want) {
				t.Fatalf("created types = %#v, want %#v", created, testCase.want)
			}
			if uploadCount := intValue(mapValue(result.Data)["upload_count"], 0); uploadCount != len(testCase.want) {
				t.Fatalf("upload_count = %d, want %d", uploadCount, len(testCase.want))
			}
		})
	}
}
