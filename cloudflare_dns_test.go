package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestCloudflareDNSPushUpdatesCreatesAndDeletesByIPFamily(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	t.Cleanup(func() {
		cloudflareAPIBaseURL = oldBaseURL
	})

	records := map[string][]CloudflareDNSRecord{
		"A": {
			{ID: "a-1", Type: "A", Name: "edge.example.com", Content: "1.1.1.1", TTL: 60},
			{ID: "a-2", Type: "A", Name: "edge.example.com", Content: "1.0.0.1", TTL: 60},
		},
		"AAAA": {
			{ID: "aaaa-1", Type: "AAAA", Name: "edge.example.com", Content: "2606:4700:4700::1111", TTL: 60},
		},
	}
	var createdCount, updatedCount, deletedCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		switch r.Method {
		case http.MethodGet:
			recordType := assertCloudflareListQueryForTest(t, r)
			writeCloudflareTestResponse(w, map[string]any{
				"success": true,
				"result":  records[recordType],
				"result_info": map[string]any{
					"page":        1,
					"total_pages": 1,
				},
			})
		case http.MethodPatch:
			updatedCount++
			record := decodeCloudflareRecordForTest(t, r)
			updateCloudflareRecordForTest(t, records, pathBase(r.URL.Path), record)
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": record})
		case http.MethodPost:
			createdCount++
			record := decodeCloudflareRecordForTest(t, r)
			record.ID = strings.ToLower(record.Type) + "-created"
			records[record.Type] = append(records[record.Type], record)
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": record})
		case http.MethodDelete:
			deletedCount++
			id := pathBase(r.URL.Path)
			deleteCloudflareRecordForTest(records, id)
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": map[string]string{"id": id}})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	cloudflareAPIBaseURL = server.URL

	result := (&App{}).PushCloudflareDNSRecords(cloudflareTestPayload("2.2.2.2\n3.3.3.3\n2606:4700:4700::2222\nbad\n2.2.2.2", 300))
	if !result.OK {
		t.Fatalf("push failed: %s", result.Message)
	}
	summary := mapValue(mapValue(result.Data)["summary"])
	if intValue(summary["created"], 0) != 0 || intValue(summary["updated"], 0) != 3 || intValue(summary["deleted"], 0) != 0 || intValue(summary["ignored"], 0) != 2 {
		t.Fatalf("summary = %#v, want created 0 updated 3 deleted 0 ignored 2", summary)
	}
	if createdCount != 0 || updatedCount != 3 || deletedCount != 0 {
		t.Fatalf("operation counts = created %d updated %d deleted %d", createdCount, updatedCount, deletedCount)
	}
	if got := recordContentsForTest(records["A"]); !reflect.DeepEqual(got, []string{"2.2.2.2", "3.3.3.3"}) {
		t.Fatalf("A contents = %#v", got)
	}
	if got := recordContentsForTest(records["AAAA"]); !reflect.DeepEqual(got, []string{"2606:4700:4700::2222"}) {
		t.Fatalf("AAAA contents = %#v", got)
	}

	result = (&App{}).PushCloudflareDNSRecords(cloudflareTestPayload("5.5.5.5", 300))
	if !result.OK {
		t.Fatalf("second push failed: %s", result.Message)
	}
	summary = mapValue(mapValue(result.Data)["summary"])
	if intValue(summary["created"], 0) != 0 || intValue(summary["updated"], 0) != 1 || intValue(summary["deleted"], 0) != 1 {
		t.Fatalf("second summary = %#v, want created 0 updated 1 deleted 1", summary)
	}
	if got := recordContentsForTest(records["A"]); !reflect.DeepEqual(got, []string{"5.5.5.5"}) {
		t.Fatalf("A contents after second push = %#v", got)
	}
	if got := recordContentsForTest(records["AAAA"]); !reflect.DeepEqual(got, []string{"2606:4700:4700::2222"}) {
		t.Fatalf("AAAA contents should be untouched when no IPv6 input, got %#v", got)
	}
}

func TestCloudflareDNSListReadsAAndAAAARecords(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	t.Cleanup(func() {
		cloudflareAPIBaseURL = oldBaseURL
	})

	queriedTypes := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/zones/zone-123/dns_records") {
			t.Fatalf("path = %s", r.URL.Path)
		}
		recordType := assertCloudflareListQueryForTest(t, r)
		queriedTypes = append(queriedTypes, recordType)
		writeCloudflareTestResponse(w, map[string]any{
			"success": true,
			"result": []CloudflareDNSRecord{
				{ID: strings.ToLower(recordType) + "-1", Type: recordType, Name: "edge.example.com", Content: "content-" + recordType, TTL: 300},
			},
			"result_info": map[string]any{"page": 1, "total_pages": 1},
		})
	}))
	defer server.Close()
	cloudflareAPIBaseURL = server.URL

	result := (&App{}).ListCloudflareDNSRecords(cloudflareTestPayload("", 300))
	if !result.OK {
		t.Fatalf("list failed: %s", result.Message)
	}
	data := mapValue(result.Data)
	if intValue(data["count"], 0) != 2 {
		t.Fatalf("count = %v, want 2", data["count"])
	}
	if !reflect.DeepEqual(queriedTypes, []string{"A", "AAAA"}) {
		t.Fatalf("queried types = %#v, want A and AAAA", queriedTypes)
	}
}

func TestCloudflareDNSConfigNormalizesTTLChoices(t *testing.T) {
	for _, tc := range []struct {
		name        string
		ttl         any
		wantTTL     int
		wantWarning bool
	}{
		{name: "missing", ttl: nil, wantTTL: 300},
		{name: "legacy-auto", ttl: 1, wantTTL: 300, wantWarning: true},
		{name: "invalid", ttl: 120, wantTTL: 300, wantWarning: true},
		{name: "one-minute", ttl: 60, wantTTL: 60},
		{name: "five-minutes", ttl: 300, wantTTL: 300},
		{name: "ten-minutes", ttl: 600, wantTTL: 600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := cloudflareTestPayload("", 300)
			cloudflare := mapValue(mapValue(payload["config"])["cloudflare"])
			if tc.ttl == nil {
				delete(cloudflare, "ttl")
			} else {
				cloudflare["ttl"] = tc.ttl
			}

			cfg, warnings, err := cloudflareDNSConfigFromPayload(payload)
			if err != nil {
				t.Fatalf("cloudflareDNSConfigFromPayload returned error: %v", err)
			}
			if cfg.TTL != tc.wantTTL {
				t.Fatalf("TTL = %d, want %d", cfg.TTL, tc.wantTTL)
			}
			hasWarning := warningsContain(warnings, "Cloudflare TTL 仅支持 60、300、600 秒")
			if hasWarning != tc.wantWarning {
				t.Fatalf("warnings = %#v, want warning %v", warnings, tc.wantWarning)
			}
		})
	}
}

func TestNormalizeDNSPushIPsGroupsByAddressFamily(t *testing.T) {
	groups, ignored := normalizeDNSPushIPs("1.1.1.1 2606:4700:4700::1111 bad 1.1.1.1")
	if !reflect.DeepEqual(groups.A, []string{"1.1.1.1"}) {
		t.Fatalf("A group = %#v", groups.A)
	}
	if !reflect.DeepEqual(groups.AAAA, []string{"2606:4700:4700::1111"}) {
		t.Fatalf("AAAA group = %#v", groups.AAAA)
	}
	if !reflect.DeepEqual(ignored, []string{"bad", "1.1.1.1"}) {
		t.Fatalf("ignored = %#v", ignored)
	}
}

func cloudflareTestPayload(ipsRaw string, ttl int) map[string]any {
	return map[string]any{
		"config": map[string]any{
			"cloudflare": map[string]any{
				"api_token":   "test-token",
				"record_name": "edge.example.com",
				"record_type": "A",
				"ttl":         ttl,
				"zone_id":     "zone-123",
			},
		},
		"ipsRaw": ipsRaw,
	}
}

func assertCloudflareListQueryForTest(t *testing.T, r *http.Request) string {
	t.Helper()
	recordType := r.URL.Query().Get("type")
	if r.URL.Query().Get("name") != "edge.example.com" || (recordType != "A" && recordType != "AAAA") {
		t.Fatalf("unexpected query: %s", r.URL.RawQuery)
	}
	return recordType
}

func decodeCloudflareRecordForTest(t *testing.T, r *http.Request) CloudflareDNSRecord {
	t.Helper()
	var record CloudflareDNSRecord
	if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
		t.Fatalf("decode record body: %v", err)
	}
	if record.Type != "A" && record.Type != "AAAA" {
		t.Fatalf("record type = %q, want A or AAAA", record.Type)
	}
	if record.TTL != 300 {
		t.Fatalf("record TTL = %d, want 300", record.TTL)
	}
	return record
}

func updateCloudflareRecordForTest(t *testing.T, records map[string][]CloudflareDNSRecord, id string, record CloudflareDNSRecord) {
	t.Helper()
	for recordType, items := range records {
		for index := range items {
			if items[index].ID == id {
				record.ID = id
				records[recordType][index] = record
				return
			}
		}
	}
	t.Fatalf("unknown record id %s", id)
}

func deleteCloudflareRecordForTest(records map[string][]CloudflareDNSRecord, id string) {
	for recordType, items := range records {
		next := items[:0]
		for _, record := range items {
			if record.ID != id {
				next = append(next, record)
			}
		}
		records[recordType] = next
	}
}

func recordContentsForTest(records []CloudflareDNSRecord) []string {
	contents := make([]string, 0, len(records))
	for _, record := range records {
		contents = append(contents, record.Content)
	}
	return contents
}

func writeCloudflareTestResponse(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		panic(err)
	}
}

func pathBase(value string) string {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	return parts[len(parts)-1]
}
