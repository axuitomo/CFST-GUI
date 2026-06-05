package cloudflarecore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestParseConfigFromPayloadNormalizesTTLChoices(t *testing.T) {
	for _, tc := range []struct {
		name        string
		ttl         any
		wantTTL     int
		wantWarning bool
	}{
		{name: "missing", ttl: nil, wantTTL: DefaultTTL},
		{name: "legacy-auto", ttl: 1, wantTTL: DefaultTTL, wantWarning: true},
		{name: "invalid", ttl: 120, wantTTL: DefaultTTL, wantWarning: true},
		{name: "one-minute", ttl: 60, wantTTL: 60},
		{name: "five-minutes", ttl: 300, wantTTL: 300},
		{name: "ten-minutes", ttl: 600, wantTTL: 600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := cloudflareCorePayload("", 300)
			cloudflare := payload["config"].(map[string]any)["cloudflare"].(map[string]any)
			if tc.ttl == nil {
				delete(cloudflare, "ttl")
			} else {
				cloudflare["ttl"] = tc.ttl
			}

			cfg, warnings, err := ParseConfigFromPayload(payload)
			if err != nil {
				t.Fatalf("ParseConfigFromPayload returned error: %v", err)
			}
			if cfg.TTL != tc.wantTTL {
				t.Fatalf("TTL = %d, want %d", cfg.TTL, tc.wantTTL)
			}
			if got := cloudflareCoreContains(warnings, "Cloudflare TTL 仅支持 60、300、600 秒"); got != tc.wantWarning {
				t.Fatalf("warnings = %#v, want warning %v", warnings, tc.wantWarning)
			}
		})
	}
}

func TestParseConfigFromPayloadRejectsMaskedToken(t *testing.T) {
	payload := cloudflareCorePayload("", 300)
	payload["config"].(map[string]any)["cloudflare"].(map[string]any)["api_token"] = "abc***xyz"

	_, _, err := ParseConfigFromPayload(payload)
	if err == nil || !strings.Contains(err.Error(), "API Token") {
		t.Fatalf("ParseConfigFromPayload error = %v, want token error", err)
	}
}

func TestNormalizePushIPsGroupsByAddressFamilyAndDedupes(t *testing.T) {
	groups, ignored := NormalizePushIPs("1.1.1.1,2606:4700:4700::1111 bad 1.1.1.1")
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

func TestClientListRecordsReadsAAndAAAARecords(t *testing.T) {
	queriedTypes := make([]string, 0, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		recordType := assertCloudflareCoreListQuery(t, r)
		queriedTypes = append(queriedTypes, recordType)
		writeCloudflareCoreResponse(w, map[string]any{
			"success": true,
			"result": []Record{
				{ID: strings.ToLower(recordType) + "-1", Type: recordType, Name: "edge.example.com", Content: "content-" + recordType, TTL: 300},
			},
			"result_info": map[string]any{"page": 1, "total_pages": 1},
		})
	}))
	defer server.Close()

	cfg, _, err := ParseConfigFromPayload(cloudflareCorePayload("", 300))
	if err != nil {
		t.Fatalf("ParseConfigFromPayload returned error: %v", err)
	}
	records, err := NewClientWithOptions(ClientOptions{BaseURL: server.URL, Token: cfg.APIToken}).ListRecords(context.Background(), cfg)
	if err != nil {
		t.Fatalf("ListRecords returned error: %v", err)
	}
	if len(records) != 2 {
		t.Fatalf("records len = %d, want 2", len(records))
	}
	if !reflect.DeepEqual(queriedTypes, []string{RecordTypeA, RecordTypeAAAA}) {
		t.Fatalf("queried types = %#v, want A and AAAA", queriedTypes)
	}
}

func TestPushRecordsUpdatesCreatesDeletesAndKeepsUntouchedFamilies(t *testing.T) {
	records := map[string][]Record{
		RecordTypeA: {
			{ID: "a-1", Type: RecordTypeA, Name: "edge.example.com", Content: "1.1.1.1", TTL: 60},
			{ID: "a-2", Type: RecordTypeA, Name: "edge.example.com", Content: "1.0.0.1", TTL: 60},
		},
		RecordTypeAAAA: {
			{ID: "aaaa-1", Type: RecordTypeAAAA, Name: "edge.example.com", Content: "2606:4700:4700::1111", TTL: 60},
		},
	}
	var createdCount, updatedCount, deletedCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			recordType := assertCloudflareCoreListQuery(t, r)
			writeCloudflareCoreResponse(w, map[string]any{
				"success":     true,
				"result":      records[recordType],
				"result_info": map[string]any{"page": 1, "total_pages": 1},
			})
		case http.MethodPatch:
			updatedCount++
			record := decodeCloudflareCoreRecord(t, r)
			updateCloudflareCoreRecord(t, records, pathBaseForCoreTest(r.URL.Path), record)
			writeCloudflareCoreResponse(w, map[string]any{"success": true, "result": record})
		case http.MethodPost:
			createdCount++
			record := decodeCloudflareCoreRecord(t, r)
			record.ID = strings.ToLower(record.Type) + "-created"
			records[record.Type] = append(records[record.Type], record)
			writeCloudflareCoreResponse(w, map[string]any{"success": true, "result": record})
		case http.MethodDelete:
			deletedCount++
			id := pathBaseForCoreTest(r.URL.Path)
			deleteCloudflareCoreRecord(records, id)
			writeCloudflareCoreResponse(w, map[string]any{"success": true, "result": map[string]string{"id": id}})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	cfg, _, err := ParseConfigFromPayload(cloudflareCorePayload("", 300))
	if err != nil {
		t.Fatalf("ParseConfigFromPayload returned error: %v", err)
	}
	client := NewClientWithOptions(ClientOptions{BaseURL: server.URL, Token: cfg.APIToken})
	result, err := PushRecords(context.Background(), client, cfg, "2.2.2.2\n3.3.3.3\n2606:4700:4700::2222\nbad\n2.2.2.2")
	if err != nil {
		t.Fatalf("PushRecords returned error: %v", err)
	}
	if !result.HasInputIPs {
		t.Fatal("HasInputIPs = false, want true")
	}
	if result.Summary.Created != 0 || result.Summary.Updated != 3 || result.Summary.Deleted != 0 || result.Summary.Ignored != 2 {
		t.Fatalf("summary = %#v, want created 0 updated 3 deleted 0 ignored 2", result.Summary)
	}
	if createdCount != 0 || updatedCount != 3 || deletedCount != 0 {
		t.Fatalf("operation counts = created %d updated %d deleted %d", createdCount, updatedCount, deletedCount)
	}
	if got := cloudflareCoreContents(records[RecordTypeA]); !reflect.DeepEqual(got, []string{"2.2.2.2", "3.3.3.3"}) {
		t.Fatalf("A contents = %#v", got)
	}
	if got := cloudflareCoreContents(records[RecordTypeAAAA]); !reflect.DeepEqual(got, []string{"2606:4700:4700::2222"}) {
		t.Fatalf("AAAA contents = %#v", got)
	}

	result, err = PushRecords(context.Background(), client, cfg, "5.5.5.5")
	if err != nil {
		t.Fatalf("second PushRecords returned error: %v", err)
	}
	if result.Summary.Created != 0 || result.Summary.Updated != 1 || result.Summary.Deleted != 1 {
		t.Fatalf("second summary = %#v, want created 0 updated 1 deleted 1", result.Summary)
	}
	if got := cloudflareCoreContents(records[RecordTypeA]); !reflect.DeepEqual(got, []string{"5.5.5.5"}) {
		t.Fatalf("A contents after second push = %#v", got)
	}
	if got := cloudflareCoreContents(records[RecordTypeAAAA]); !reflect.DeepEqual(got, []string{"2606:4700:4700::2222"}) {
		t.Fatalf("AAAA contents should be untouched when no IPv6 input, got %#v", got)
	}
}

func TestPushRecordsReturnsOperationErrorForAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeCloudflareCoreResponse(w, map[string]any{
			"success": false,
			"errors":  []map[string]any{{"code": 1000, "message": "zone denied"}},
		})
	}))
	defer server.Close()

	cfg, _, err := ParseConfigFromPayload(cloudflareCorePayload("", 300))
	if err != nil {
		t.Fatalf("ParseConfigFromPayload returned error: %v", err)
	}
	_, err = PushRecords(context.Background(), NewClientWithOptions(ClientOptions{BaseURL: server.URL, Token: cfg.APIToken}), cfg, "1.1.1.1")
	if err == nil {
		t.Fatal("PushRecords returned nil error, want API error")
	}
	if got := OperationFromError(err); got != OperationList {
		t.Fatalf("operation = %q, want %q", got, OperationList)
	}
	if !strings.Contains(err.Error(), "zone denied") {
		t.Fatalf("error = %v, want API message", err)
	}
}

func TestPushRecordsDeletesExistingCNAMEBeforeCreate(t *testing.T) {
	records := map[string][]Record{
		RecordTypeA:    {},
		RecordTypeAAAA: {},
		RecordTypeCNAME: {
			{ID: "cname-1", Type: RecordTypeCNAME, Name: "edge.example.com", Content: "origin.example.net", TTL: 300},
		},
	}
	var createdCount, deletedCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if !strings.Contains(r.URL.Path, "/zones/zone-123/dns_records") {
				t.Fatalf("path = %s", r.URL.Path)
			}
			recordType := r.URL.Query().Get("type")
			recordName := r.URL.Query().Get("name")
			if recordName != "edge.example.com" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			if recordType == "" {
				writeCloudflareCoreResponse(w, map[string]any{
					"success":     true,
					"result":      records[RecordTypeCNAME],
					"result_info": map[string]any{"page": 1, "total_pages": 1},
				})
				return
			}
			if recordType != RecordTypeA && recordType != RecordTypeAAAA {
				t.Fatalf("unexpected typed query: %s", r.URL.RawQuery)
			}
			writeCloudflareCoreResponse(w, map[string]any{
				"success":     true,
				"result":      records[recordType],
				"result_info": map[string]any{"page": 1, "total_pages": 1},
			})
		case http.MethodPost:
			createdCount++
			record := decodeCloudflareCoreRecord(t, r)
			record.ID = strings.ToLower(record.Type) + "-created"
			records[record.Type] = append(records[record.Type], record)
			writeCloudflareCoreResponse(w, map[string]any{"success": true, "result": record})
		case http.MethodDelete:
			deletedCount++
			deleteCloudflareCoreRecord(records, pathBaseForCoreTest(r.URL.Path))
			writeCloudflareCoreResponse(w, map[string]any{"success": true, "result": map[string]string{"id": "cname-1"}})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	cfg, _, err := ParseConfigFromPayload(cloudflareCorePayload("", 300))
	if err != nil {
		t.Fatalf("ParseConfigFromPayload returned error: %v", err)
	}
	result, err := PushRecords(context.Background(), NewClientWithOptions(ClientOptions{BaseURL: server.URL, Token: cfg.APIToken}), cfg, "2.2.2.2")
	if err != nil {
		t.Fatalf("PushRecords returned error: %v", err)
	}
	if result.Summary.Created != 1 || result.Summary.Deleted != 1 {
		t.Fatalf("summary = %#v, want created 1 deleted 1", result.Summary)
	}
	if createdCount != 1 || deletedCount != 1 {
		t.Fatalf("operation counts = created %d deleted %d, want 1 and 1", createdCount, deletedCount)
	}
	if len(records[RecordTypeCNAME]) != 0 {
		t.Fatalf("CNAME records = %#v, want empty after delete", records[RecordTypeCNAME])
	}
	if got := cloudflareCoreContents(records[RecordTypeA]); !reflect.DeepEqual(got, []string{"2.2.2.2"}) {
		t.Fatalf("A contents = %#v", got)
	}
}

func TestPushRecordsEmptyInputKeepsIgnoredSummary(t *testing.T) {
	cfg, _, err := ParseConfigFromPayload(cloudflareCorePayload("", 300))
	if err != nil {
		t.Fatalf("ParseConfigFromPayload returned error: %v", err)
	}
	result, err := PushRecords(context.Background(), nil, cfg, "bad-entry")
	if err != nil {
		t.Fatalf("PushRecords returned error: %v", err)
	}
	if result.HasInputIPs {
		t.Fatal("HasInputIPs = true, want false")
	}
	if result.Summary.Ignored != 1 || !reflect.DeepEqual(result.IgnoredEntries, []string{"bad-entry"}) {
		t.Fatalf("result = %#v, want one ignored entry", result)
	}
}

func cloudflareCorePayload(ipsRaw string, ttl int) map[string]any {
	return map[string]any{
		"config": map[string]any{
			"cloudflare": map[string]any{
				"api_token":   "test-token",
				"record_name": "edge.example.com",
				"record_type": RecordTypeA,
				"ttl":         ttl,
				"zone_id":     "zone-123",
			},
		},
		"ipsRaw": ipsRaw,
	}
}

func assertCloudflareCoreListQuery(t *testing.T, r *http.Request) string {
	t.Helper()
	if !strings.Contains(r.URL.Path, "/zones/zone-123/dns_records") {
		t.Fatalf("path = %s", r.URL.Path)
	}
	recordType := r.URL.Query().Get("type")
	if r.URL.Query().Get("name") != "edge.example.com" || (recordType != RecordTypeA && recordType != RecordTypeAAAA) {
		t.Fatalf("unexpected query: %s", r.URL.RawQuery)
	}
	return recordType
}

func decodeCloudflareCoreRecord(t *testing.T, r *http.Request) Record {
	t.Helper()
	var record Record
	if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
		t.Fatalf("decode record body: %v", err)
	}
	if record.Type != RecordTypeA && record.Type != RecordTypeAAAA {
		t.Fatalf("record type = %q, want A or AAAA", record.Type)
	}
	if record.TTL != 300 {
		t.Fatalf("record TTL = %d, want 300", record.TTL)
	}
	return record
}

func updateCloudflareCoreRecord(t *testing.T, records map[string][]Record, id string, record Record) {
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

func deleteCloudflareCoreRecord(records map[string][]Record, id string) {
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

func cloudflareCoreContents(records []Record) []string {
	contents := make([]string, 0, len(records))
	for _, record := range records {
		contents = append(contents, record.Content)
	}
	return contents
}

func writeCloudflareCoreResponse(w http.ResponseWriter, body any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(body); err != nil {
		panic(err)
	}
}

func pathBaseForCoreTest(value string) string {
	parts := strings.Split(strings.Trim(value, "/"), "/")
	return parts[len(parts)-1]
}

func cloudflareCoreContains(values []string, needle string) bool {
	for _, value := range values {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
