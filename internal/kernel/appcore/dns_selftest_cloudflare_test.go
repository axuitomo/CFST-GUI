package appcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/kernel/domain/dns/cloudflarecore"
)

// TestRunDNSPlatformSelfTestAgainstCloudflareAPI runs the full command against a
// fake Cloudflare API. It pins the adapter wiring (list / create / patch / delete
// paths, CNAME blocking and cleanup) without touching a real zone.
func TestRunDNSPlatformSelfTestAgainstCloudflareAPI(t *testing.T) {
	records := map[string]cloudflarecore.Record{}
	nextID := 1

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/dns_records"):
			name := r.URL.Query().Get("name")
			recordType := r.URL.Query().Get("type")
			list := make([]cloudflarecore.Record, 0, len(records))
			for _, record := range records {
				if name != "" && !strings.EqualFold(record.Name, name) {
					continue
				}
				if recordType != "" && !strings.EqualFold(record.Type, recordType) {
					continue
				}
				list = append(list, record)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success":     true,
				"result":      list,
				"result_info": map[string]any{"page": 1, "total_pages": 1},
			})
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/dns_records"):
			var record cloudflarecore.Record
			if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			record.ID = fmtID(nextID)
			nextID++
			records[record.ID] = record
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": record})
		case r.Method == http.MethodPatch && strings.Contains(r.URL.Path, "/dns_records/"):
			id := lastPathSegment(r.URL.Path)
			var record cloudflarecore.Record
			if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if _, ok := records[id]; !ok {
				_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "errors": []any{map[string]any{"code": 81044, "message": "not found"}}})
				return
			}
			record.ID = id
			records[id] = record
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": record})
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/dns_records/"):
			delete(records, lastPathSegment(r.URL.Path))
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": map[string]any{}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewService(ServiceOptions{
		CloudflareAPIBaseURL: server.URL,
		Storage:              StorageLayout{Root: t.TempDir()},
	})
	snapshot := map[string]any{
		"dns_providers": []any{map[string]any{
			"allow_cname": true,
			"api_token":   "token",
			"enabled":     true,
			"id":          "cf",
			"provider":    "cloudflare",
			"record_name": "sub.example.com",
			"record_type": "ALL",
			"ttl":         300,
			"zone_id":     "zone",
		}},
	}
	result := service.RunDNSPlatformSelfTest(context.Background(), map[string]any{
		"config":      snapshot,
		"provider_id": "cf",
		"test_name":   "cfst-selftest.sub.example.com",
	})
	if result.Code != "DNS_SELFTEST_COMPLETED" || !result.OK {
		t.Fatalf("self-test failed: code=%s message=%s data=%+v", result.Code, result.Message, result.Data)
	}
	if len(records) != 0 {
		t.Fatalf("expected the fake zone to be clean, got %+v", records)
	}
}

func fmtID(id int) string {
	return "id-" + strconv.Itoa(id)
}

func lastPathSegment(path string) string {
	parts := strings.Split(strings.TrimRight(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}
