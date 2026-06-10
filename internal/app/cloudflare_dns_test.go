package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
)

func TestCloudflareDNSPushClearsAAndAAAARecordsBeforeCreating(t *testing.T) {
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
			recordType := r.URL.Query().Get("type")
			if recordType == "" {
				if r.URL.Query().Get("name") != "edge.example.com" {
					t.Fatalf("unexpected query: %s", r.URL.RawQuery)
				}
				writeCloudflareTestResponse(w, map[string]any{
					"success":     true,
					"result":      allCloudflareRecordsForTest(records),
					"result_info": map[string]any{"page": 1, "total_pages": 1},
				})
				return
			}
			assertCloudflareListQueryForTest(t, r)
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
			t.Fatalf("unexpected PATCH request")
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

	payload := cloudflareTestPayload("2.2.2.2\n3.3.3.3\n2606:4700:4700::2222\nbad\n2.2.2.2", 300)
	mapValue(mapValue(payload["config"])["cloudflare"])["proxied"] = true
	result := (&App{}).PushCloudflareDNSRecords(payload)
	if !result.OK {
		t.Fatalf("push failed: %s", result.Message)
	}
	summary := mapValue(mapValue(result.Data)["summary"])
	if intValue(summary["created"], 0) != 3 || intValue(summary["updated"], 0) != 0 || intValue(summary["deleted"], 0) != 3 || intValue(summary["ignored"], 0) != 2 {
		t.Fatalf("summary = %#v, want created 3 updated 0 deleted 3 ignored 2", summary)
	}
	if createdCount != 3 || updatedCount != 0 || deletedCount != 3 {
		t.Fatalf("operation counts = created %d updated %d deleted %d, want 3, 0, 3", createdCount, updatedCount, deletedCount)
	}
	if got := recordContentsForTest(records["A"]); !slices.Equal(got, []string{"2.2.2.2", "3.3.3.3"}) {
		t.Fatalf("A contents = %#v", got)
	}
	if got := recordContentsForTest(records["AAAA"]); !slices.Equal(got, []string{"2606:4700:4700::2222"}) {
		t.Fatalf("AAAA contents = %#v", got)
	}

	result = (&App{}).PushCloudflareDNSRecords(cloudflareTestPayload("5.5.5.5", 300))
	if !result.OK {
		t.Fatalf("second push failed: %s", result.Message)
	}
	summary = mapValue(mapValue(result.Data)["summary"])
	if intValue(summary["created"], 0) != 1 || intValue(summary["updated"], 0) != 0 || intValue(summary["deleted"], 0) != 3 {
		t.Fatalf("second summary = %#v, want created 1 updated 0 deleted 3", summary)
	}
	if got := recordContentsForTest(records["A"]); !slices.Equal(got, []string{"5.5.5.5"}) {
		t.Fatalf("A contents after second push = %#v", got)
	}
	if got := recordContentsForTest(records["AAAA"]); len(got) != 0 {
		t.Fatalf("AAAA contents after second push = %#v, want empty", got)
	}
}

func TestCloudflareDNSListReadsAAndAAAARecords(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	t.Cleanup(func() {
		cloudflareAPIBaseURL = oldBaseURL
	})

	queriedNames := make([]string, 0, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Fatalf("method = %s, want GET", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/zones/zone-123/dns_records") {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.URL.Query().Get("type") != "" {
			t.Fatalf("unexpected type query: %s", r.URL.RawQuery)
		}
		recordName := r.URL.Query().Get("name")
		if recordName != "edge.example.com" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		queriedNames = append(queriedNames, recordName)
		writeCloudflareTestResponse(w, map[string]any{
			"success": true,
			"result": []CloudflareDNSRecord{
				{ID: "a-1", Type: "A", Name: "edge.example.com", Content: "content-A", TTL: 300},
				{ID: "aaaa-1", Type: "AAAA", Name: "edge.example.com", Content: "content-AAAA", TTL: 300},
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
	if !slices.Equal(queriedNames, []string{"edge.example.com"}) {
		t.Fatalf("queried names = %#v, want configured record name once", queriedNames)
	}
}

func TestCloudflareDNSListConfiguredScopeRequiresRecordName(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	t.Cleanup(func() {
		cloudflareAPIBaseURL = oldBaseURL
	})

	requested := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = true
		t.Fatalf("unexpected Cloudflare request: %s %s", r.Method, r.URL.String())
	}))
	defer server.Close()
	cloudflareAPIBaseURL = server.URL

	payload := cloudflareTestPayload("", 300)
	payload["scope"] = "configured"
	cloudflare := mapValue(mapValue(payload["config"])["cloudflare"])
	cloudflare["record_name"] = ""

	result := (&App{}).ListCloudflareDNSRecords(payload)
	if result.OK || result.Code != "DNS_CONFIG_INVALID" {
		t.Fatalf("result = %#v, want DNS_CONFIG_INVALID", result)
	}
	if !strings.Contains(result.Message, "DNS 记录名称") {
		t.Fatalf("message = %q, want record name error", result.Message)
	}
	if requested {
		t.Fatalf("configured scope without record name should not request Cloudflare")
	}
}

func TestCloudflareDNSPushDeletesExistingCNAMEBeforeCreate(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	t.Cleanup(func() {
		cloudflareAPIBaseURL = oldBaseURL
	})

	records := map[string][]CloudflareDNSRecord{
		"A":    {},
		"AAAA": {},
		"CNAME": {
			{ID: "cname-1", Type: "CNAME", Name: "edge.example.com", Content: "origin.example.net", TTL: 300},
		},
	}
	var createdCount, deletedCount int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			if !strings.Contains(r.URL.Path, "/zones/zone-123/dns_records") {
				t.Fatalf("path = %s", r.URL.Path)
			}
			recordName := r.URL.Query().Get("name")
			recordType := r.URL.Query().Get("type")
			if recordName != "edge.example.com" {
				t.Fatalf("unexpected query: %s", r.URL.RawQuery)
			}
			if recordType == "" {
				writeCloudflareTestResponse(w, map[string]any{
					"success":     true,
					"result":      records["CNAME"],
					"result_info": map[string]any{"page": 1, "total_pages": 1},
				})
				return
			}
			if recordType != "A" && recordType != "AAAA" {
				t.Fatalf("unexpected typed query: %s", r.URL.RawQuery)
			}
			writeCloudflareTestResponse(w, map[string]any{
				"success":     true,
				"result":      records[recordType],
				"result_info": map[string]any{"page": 1, "total_pages": 1},
			})
		case http.MethodPost:
			createdCount++
			record := decodeCloudflareRecordForTest(t, r)
			record.ID = strings.ToLower(record.Type) + "-created"
			records[record.Type] = append(records[record.Type], record)
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": record})
		case http.MethodDelete:
			deletedCount++
			deleteCloudflareRecordForTest(records, pathBase(r.URL.Path))
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": map[string]string{"id": "cname-1"}})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	cloudflareAPIBaseURL = server.URL

	result := (&App{}).PushCloudflareDNSRecords(cloudflareTestPayload("2.2.2.2", 300))
	if !result.OK {
		t.Fatalf("push failed: %#v", result)
	}
	summary := mapValue(mapValue(result.Data)["summary"])
	if intValue(summary["created"], 0) != 1 || intValue(summary["deleted"], 0) != 1 {
		t.Fatalf("summary = %#v, want created 1 deleted 1", summary)
	}
	if createdCount != 1 || deletedCount != 1 {
		t.Fatalf("operation counts = created %d deleted %d, want 1 and 1", createdCount, deletedCount)
	}
	if len(records["CNAME"]) != 0 {
		t.Fatalf("CNAME records = %#v, want empty after delete", records["CNAME"])
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
	if !slices.Equal(groups.A, []string{"1.1.1.1"}) {
		t.Fatalf("A group = %#v", groups.A)
	}
	if !slices.Equal(groups.AAAA, []string{"2606:4700:4700::1111"}) {
		t.Fatalf("AAAA group = %#v", groups.AAAA)
	}
	if !slices.Equal(ignored, []string{"bad", "1.1.1.1"}) {
		t.Fatalf("ignored = %#v", ignored)
	}
}

func TestCloudflareDNSPushRoutesRowsToMultipleRecordNames(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	t.Cleanup(func() {
		cloudflareAPIBaseURL = oldBaseURL
	})
	configDir := configureDesktopConfigDirForTest(t)
	if err := os.WriteFile(filepath.Join(configDir, colodict.ColoFileName), []byte("ip_prefix,colo,country,region,city\n203.0.113.0/24,HKG,HK,,Hong Kong\n2001:db8:1::/48,HKG,HK,,Hong Kong\n198.51.100.0/24,NRT,JP,,Tokyo\n192.0.2.0/24,LAX,US,CA,Los Angeles\n"), 0o600); err != nil {
		t.Fatalf("write colo file: %v", err)
	}

	createdByName := make(map[string][]string)
	createdByNameAndType := make(map[string][]string)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		switch r.Method {
		case http.MethodGet:
			writeCloudflareTestResponse(w, map[string]any{
				"success": true,
				"result":  []CloudflareDNSRecord{},
				"result_info": map[string]any{
					"page":        1,
					"total_pages": 1,
				},
			})
		case http.MethodPost:
			record := decodeCloudflareRecordForTest(t, r)
			createdByName[record.Name] = append(createdByName[record.Name], record.Content)
			createdByNameAndType[record.Name+"|"+record.Type] = append(createdByNameAndType[record.Name+"|"+record.Type], record.Content)
			record.ID = strings.ToLower(record.Type) + "-created"
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": record})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	cloudflareAPIBaseURL = server.URL

	payload := cloudflareTestPayload("", 300)
	config := mapValue(payload["config"])
	mapValue(config["cloudflare"])["record_name"] = ""
	config["upload"] = map[string]any{
		"cloudflare": map[string]any{
			"routing_enabled": true,
			"routing_rules": []map[string]any{
				{"enabled": true, "name": "hk", "record_name": "hk.example.com", "record_type": "A", "filter_tokens": "HKG", "top_n": 1},
				{"enabled": true, "name": "hk-all", "record_name": "hk-all.example.com", "record_type": "ALL", "filter_tokens": "HKG", "top_n": 0},
				{"enabled": true, "name": "jp", "record_name": "jp.example.com", "record_type": "A", "filter_tokens": "JP", "top_n": 1},
				{"enabled": true, "name": "empty", "record_name": "empty.example.com", "record_type": "A", "filter_tokens": "ZZZ", "top_n": 1},
			},
		},
	}
	payload["results"] = []ProbeRow{
		{Colo: "HKG", DownloadSpeedMB: 20, IP: "203.0.113.10"},
		{Colo: "HKG", DownloadSpeedMB: 18, IP: "2001:db8:1::10"},
		{Colo: "NRT", DownloadSpeedMB: 30, IP: "198.51.100.10"},
		{Colo: "LAX", DownloadSpeedMB: 40, IP: "192.0.2.10"},
	}

	result := (&App{}).PushCloudflareDNSRecords(payload)
	if !result.OK {
		t.Fatalf("routing push failed: %s warnings=%#v", result.Message, result.Warnings)
	}
	data := mapValue(result.Data)
	if !boolValue(data["routing_enabled"], false) {
		t.Fatalf("routing_enabled = %#v, want true", data["routing_enabled"])
	}
	if got := intValue(data["success_targets"], 0); got != 3 {
		t.Fatalf("success_targets = %d, want 3", got)
	}
	if got := intValue(data["skipped_targets"], 0); got != 1 {
		t.Fatalf("skipped_targets = %d, want 1", got)
	}
	if got := intValue(data["upload_count"], 0); got != 4 {
		t.Fatalf("upload_count = %d, want 4", got)
	}
	if !slices.Equal(createdByName["hk.example.com"], []string{"203.0.113.10"}) {
		t.Fatalf("hk uploads = %#v", createdByName["hk.example.com"])
	}
	if !slices.Equal(createdByName["jp.example.com"], []string{"198.51.100.10"}) {
		t.Fatalf("jp uploads = %#v", createdByName["jp.example.com"])
	}
	if !slices.Equal(createdByNameAndType["hk-all.example.com|A"], []string{"203.0.113.10"}) {
		t.Fatalf("hk-all A uploads = %#v", createdByNameAndType["hk-all.example.com|A"])
	}
	if !slices.Equal(createdByNameAndType["hk-all.example.com|AAAA"], []string{"2001:db8:1::10"}) {
		t.Fatalf("hk-all AAAA uploads = %#v", createdByNameAndType["hk-all.example.com|AAAA"])
	}
	if _, ok := createdByName["empty.example.com"]; ok {
		t.Fatalf("empty route should not upload, got %#v", createdByName["empty.example.com"])
	}
	if warningsContain(result.Warnings, "记录类型 A 无匹配 IP") {
		t.Fatalf("warnings = %#v, want no second empty-record-type warning for skipped route", result.Warnings)
	}
}

func TestCloudflareDNSPushPrimaryTargetBeforeRoutes(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	t.Cleanup(func() {
		cloudflareAPIBaseURL = oldBaseURL
	})
	configDir := configureDesktopConfigDirForTest(t)
	if err := os.WriteFile(filepath.Join(configDir, colodict.ColoFileName), []byte("ip_prefix,colo,country,region,city\n203.0.113.0/24,HKG,HK,,Hong Kong\n198.51.100.0/24,NRT,JP,,Tokyo\n"), 0o600); err != nil {
		t.Fatalf("write colo file: %v", err)
	}

	postNames := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": []CloudflareDNSRecord{}, "result_info": map[string]any{"page": 1, "total_pages": 1}})
		case http.MethodPost:
			record := decodeCloudflareRecordForTest(t, r)
			postNames = append(postNames, record.Name)
			record.ID = strings.ToLower(record.Type) + "-created"
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": record})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	cloudflareAPIBaseURL = server.URL

	payload := cloudflareTestPayload("", 300)
	config := mapValue(payload["config"])
	config["upload"] = map[string]any{
		"cloudflare": map[string]any{
			"routing_enabled": true,
			"routing_rules": []map[string]any{
				{"enabled": true, "name": "hk", "record_name": "hk.example.com", "record_type": "A", "filter_tokens": "HKG", "top_n": 1},
			},
		},
	}
	payload["results"] = []ProbeRow{
		{Colo: "HKG", DownloadSpeedMB: 20, IP: "203.0.113.10"},
		{Colo: "NRT", DownloadSpeedMB: 30, IP: "198.51.100.10"},
	}

	result := (&App{}).PushCloudflareDNSRecords(payload)
	if !result.OK {
		t.Fatalf("push failed: %s warnings=%#v", result.Message, result.Warnings)
	}
	if len(postNames) < 3 {
		t.Fatalf("postNames = %#v, want primary and route posts", postNames)
	}
	if postNames[0] != "edge.example.com" || postNames[1] != "edge.example.com" || postNames[2] != "hk.example.com" {
		t.Fatalf("post order = %#v, want primary target before route", postNames)
	}
	data := mapValue(result.Data)
	if got := intValue(data["success_targets"], 0); got != 2 {
		t.Fatalf("success_targets = %d, want 2", got)
	}
	if !warningsContain(result.Warnings, "主目标 edge.example.com") || !warningsContain(result.Warnings, "分流目标 hk") {
		t.Fatalf("warnings = %#v, want primary and route success summaries", result.Warnings)
	}
}

func TestCloudflareDNSPushRoutesContinueWhenPrimaryFails(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	t.Cleanup(func() {
		cloudflareAPIBaseURL = oldBaseURL
	})
	configDir := configureDesktopConfigDirForTest(t)
	if err := os.WriteFile(filepath.Join(configDir, colodict.ColoFileName), []byte("ip_prefix,colo,country,region,city\n203.0.113.0/24,HKG,HK,,Hong Kong\n"), 0o600); err != nil {
		t.Fatalf("write colo file: %v", err)
	}

	postedRoutes := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": []CloudflareDNSRecord{}, "result_info": map[string]any{"page": 1, "total_pages": 1}})
		case http.MethodPost:
			record := decodeCloudflareRecordForTest(t, r)
			if record.Name == "edge.example.com" {
				http.Error(w, `{"success":false,"errors":[{"message":"primary unavailable"}]}`, http.StatusBadGateway)
				return
			}
			postedRoutes = append(postedRoutes, record.Name)
			record.ID = strings.ToLower(record.Type) + "-created"
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": record})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	cloudflareAPIBaseURL = server.URL

	payload := cloudflareTestPayload("", 300)
	config := mapValue(payload["config"])
	config["upload"] = map[string]any{
		"cloudflare": map[string]any{
			"routing_enabled": true,
			"routing_rules": []map[string]any{
				{"enabled": true, "name": "hk", "record_name": "hk.example.com", "record_type": "A", "filter_tokens": "HKG", "top_n": 1},
			},
		},
	}
	payload["results"] = []ProbeRow{{Colo: "HKG", DownloadSpeedMB: 20, IP: "203.0.113.10"}}

	result := (&App{}).PushCloudflareDNSRecords(payload)
	if !result.OK || result.Code != "DNS_PUSH_PARTIAL" {
		t.Fatalf("result = %#v, want OK DNS_PUSH_PARTIAL", result)
	}
	if !slices.Equal(postedRoutes, []string{"hk.example.com"}) {
		t.Fatalf("postedRoutes = %#v, want route upload after primary failure", postedRoutes)
	}
	data := mapValue(result.Data)
	if got := intValue(data["success_targets"], 0); got != 1 {
		t.Fatalf("success_targets = %d, want 1", got)
	}
	if got := intValue(data["failed_targets"], 0); got != 1 {
		t.Fatalf("failed_targets = %d, want 1", got)
	}
	if !warningsContain(result.Warnings, "主目标 edge.example.com") || !warningsContain(result.Warnings, "推送失败") {
		t.Fatalf("warnings = %#v, want primary failure reason", result.Warnings)
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
	if record.Proxied {
		t.Fatalf("record proxied = true, want false for grey-cloud DNS")
	}
	return record
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

func allCloudflareRecordsForTest(records map[string][]CloudflareDNSRecord) []CloudflareDNSRecord {
	all := make([]CloudflareDNSRecord, 0)
	for _, recordType := range []string{"A", "AAAA", "CNAME"} {
		all = append(all, records[recordType]...)
	}
	return all
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
