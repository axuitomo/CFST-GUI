package mobileapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/appcore"
	"github.com/axuitomo/CFST-GUI/internal/colodict"
	"github.com/axuitomo/CFST-GUI/internal/utils"
)

func TestMobileSchedulerDailyTimesAcceptFullWidthSeparators(t *testing.T) {
	location := time.FixedZone("test", 8*60*60)
	now := time.Date(2026, 5, 9, 10, 30, 0, 0, location)
	cfg := mobileSchedulerConfigFromSnapshot(map[string]any{
		"scheduler": map[string]any{
			"dailyTimes": []any{"09：00，10：45；21：30、23:00"},
			"enabled":    true,
		},
	})

	if want := []string{"09：00", "10：45", "21：30", "23:00"}; !reflect.DeepEqual(cfg.DailyTimes, want) {
		t.Fatalf("DailyTimes = %#v, want %#v", cfg.DailyTimes, want)
	}
	next := mobileNextSchedulerRun(now, time.Time{}, cfg)
	if want := time.Date(2026, 5, 9, 10, 45, 0, 0, location); !next.Equal(want) {
		t.Fatalf("mobileNextSchedulerRun() = %v, want %v", next, want)
	}
}

func TestRunScheduledProbeFailsWhenUploadSelectionFails(t *testing.T) {
	oldTCP := mobileTCPProbeRunner
	oldTrace := mobileTraceProbeRunner
	oldDownload := mobileDownloadProbeRunner
	t.Cleanup(func() {
		mobileTCPProbeRunner = oldTCP
		mobileTraceProbeRunner = oldTrace
		mobileDownloadProbeRunner = oldDownload
	})

	mobileTCPProbeRunner = func() (utils.PingDelaySet, error) {
		return utils.PingDelaySet{{
			PingData: &utils.PingData{
				IP:       parseMobileTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed: 10 * 1024 * 1024,
		}}, nil
	}
	mobileTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet { return input }
	mobileDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	snapshot := defaultConfigSnapshot()
	probe := mapValue(snapshot["probe"])
	probe["disable_download"] = true
	probe["print_num"] = 0
	scheduler := mapValue(snapshot["scheduler"])
	scheduler["auto_dns_push"] = true
	scheduler["auto_github_export"] = true
	scheduler["enabled"] = true
	scheduler["interval_minutes"] = 15
	sharedFilter := mapValue(mapValue(snapshot["upload"])["shared_filter"])
	sharedFilter["colo_allow"] = "JP"
	sharedFilter["enabled"] = true
	sources := []map[string]any{{
		"content":  "1.1.1.1",
		"enabled":  true,
		"ip_limit": 10,
		"ip_mode":  "traverse",
		"kind":     "inline",
		"name":     "valid-source",
	}}
	snapshot["sources"] = sources
	if err := service.writeConfigSnapshot(snapshot); err != nil {
		t.Fatalf("writeConfigSnapshot: %v", err)
	}

	result := decodeCommandForTest(t, service.RunScheduledProbe("{}"))
	if boolValue(result["ok"], true) {
		t.Fatalf("RunScheduledProbe unexpectedly succeeded: %#v", result)
	}
	if got := stringValue(result["code"], ""); got != "SCHEDULER_RUN_FAILED" {
		t.Fatalf("code = %q, want SCHEDULER_RUN_FAILED", got)
	}
	data := mapValue(result["data"])
	if got := stringValue(data["workflow_stage"], ""); got != "upload_selection_failed" {
		t.Fatalf("workflow_stage = %q, want upload_selection_failed", got)
	}
	if got := stringValue(data["last_probe_status"], ""); got != "failed" {
		t.Fatalf("last_probe_status = %q, want failed", got)
	}
	if got := stringValue(data["last_dns_status"], ""); got != "failed" {
		t.Fatalf("last_dns_status = %q, want failed", got)
	}
	if got := stringValue(data["last_github_status"], ""); got != "failed" {
		t.Fatalf("last_github_status = %q, want failed", got)
	}
	notification := mapValue(data["upload_notification"])
	if got := stringValue(notification["status"], ""); got != appcore.UploadNotificationStatusFailed {
		t.Fatalf("notification status = %q, want failed; notification=%#v", got, notification)
	}
	if got := stringValue(data["next_run_at"], ""); got == "" {
		t.Fatal("next_run_at is empty, want scheduler to rearm after failure")
	}
	if message := stringValue(result["message"], ""); !strings.Contains(message, "COLO 文件不存在") {
		t.Fatalf("message = %q, want missing COLO dictionary error", message)
	}
}

func TestRunScheduledProbeReturnsUploadNotificationWhenNoRows(t *testing.T) {
	oldTCP := mobileTCPProbeRunner
	oldTrace := mobileTraceProbeRunner
	oldDownload := mobileDownloadProbeRunner
	t.Cleanup(func() {
		mobileTCPProbeRunner = oldTCP
		mobileTraceProbeRunner = oldTrace
		mobileDownloadProbeRunner = oldDownload
	})

	mobileTCPProbeRunner = func() (utils.PingDelaySet, error) {
		return utils.PingDelaySet{}, nil
	}
	mobileTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet { return input }
	mobileDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	service := NewService()
	sink := &probeEventSinkForTest{}
	service.SetEventSink(sink)
	decodeCommandForTest(t, service.Init(t.TempDir()))
	snapshot := defaultConfigSnapshot()
	probe := mapValue(snapshot["probe"])
	probe["disable_download"] = true
	scheduler := mapValue(snapshot["scheduler"])
	scheduler["auto_dns_push"] = true
	scheduler["auto_github_export"] = true
	scheduler["enabled"] = true
	scheduler["interval_minutes"] = 15
	snapshot["sources"] = []map[string]any{{
		"content":  "1.1.1.1",
		"enabled":  true,
		"ip_limit": 10,
		"ip_mode":  "traverse",
		"kind":     "inline",
		"name":     "valid-source",
	}}
	if err := service.writeConfigSnapshot(snapshot); err != nil {
		t.Fatalf("writeConfigSnapshot: %v", err)
	}

	result := decodeCommandForTest(t, service.RunScheduledProbe("{}"))
	if !boolValue(result["ok"], false) {
		t.Fatalf("RunScheduledProbe failed: %#v", result)
	}
	data := mapValue(result["data"])
	notification := mapValue(data["upload_notification"])
	if got := stringValue(notification["source"], ""); got != appcore.UploadNotificationSourceScheduledProbe {
		t.Fatalf("source = %q, want scheduled probe upload notification", got)
	}
	if got := stringValue(notification["status"], ""); got != appcore.UploadNotificationStatusSkipped {
		t.Fatalf("status = %q, want skipped; notification=%#v data=%#v", got, notification, data)
	}
	if !strings.Contains(strings.Join(sink.events, "\n"), `"event":"upload.notification"`) {
		t.Fatalf("events missing upload.notification: %#v", sink.events)
	}
}

func TestRunScheduledProbeSkipsGitHubWhenProviderDisabled(t *testing.T) {
	oldTCP := mobileTCPProbeRunner
	oldTrace := mobileTraceProbeRunner
	oldDownload := mobileDownloadProbeRunner
	t.Cleanup(func() {
		mobileTCPProbeRunner = oldTCP
		mobileTraceProbeRunner = oldTrace
		mobileDownloadProbeRunner = oldDownload
	})

	mobileTCPProbeRunner = func() (utils.PingDelaySet, error) {
		return utils.PingDelaySet{{
			PingData: &utils.PingData{
				IP:       parseMobileTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed: 10 * 1024 * 1024,
		}}, nil
	}
	mobileTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet { return input }
	mobileDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	snapshot := defaultConfigSnapshot()
	mapValue(snapshot["probe"])["disable_download"] = true
	scheduler := mapValue(snapshot["scheduler"])
	scheduler["auto_dns_push"] = false
	scheduler["auto_github_export"] = true
	scheduler["enabled"] = true
	scheduler["interval_minutes"] = 15
	snapshot["sources"] = []map[string]any{{
		"content":  "1.1.1.1",
		"enabled":  true,
		"ip_limit": 10,
		"ip_mode":  "traverse",
		"kind":     "inline",
		"name":     "valid-source",
	}}
	if err := service.writeConfigSnapshot(snapshot); err != nil {
		t.Fatalf("writeConfigSnapshot: %v", err)
	}

	result := decodeCommandForTest(t, service.RunScheduledProbe("{}"))
	if !boolValue(result["ok"], false) {
		t.Fatalf("RunScheduledProbe failed: %#v", result)
	}
	data := mapValue(result["data"])
	if got := stringValue(data["last_github_status"], ""); got != "skipped" {
		t.Fatalf("last_github_status = %q, want skipped", got)
	}
	if got := stringValue(data["last_probe_status"], ""); got != "completed" {
		t.Fatalf("last_probe_status = %q, want completed", got)
	}
	if message := stringValue(data["last_message"], ""); !strings.Contains(message, "GitHub 导出已跳过") {
		t.Fatalf("last_message = %q, want disabled GitHub skip message", message)
	}
	if got := stringValue(data["next_run_at"], ""); got == "" {
		t.Fatal("next_run_at is empty, want scheduler rearmed after GitHub skip")
	}
}

func TestRunScheduledProbeUsesCloudflareRoutingRules(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	oldTCP := mobileTCPProbeRunner
	oldTrace := mobileTraceProbeRunner
	oldDownload := mobileDownloadProbeRunner
	t.Cleanup(func() {
		cloudflareAPIBaseURL = oldBaseURL
		mobileTCPProbeRunner = oldTCP
		mobileTraceProbeRunner = oldTrace
		mobileDownloadProbeRunner = oldDownload
	})
	records := map[string][]CloudflareDNSRecord{
		"A": {
			{ID: "a-1", Type: "A", Name: "us.example.com", Content: "1.1.1.1", TTL: 300},
		},
		"AAAA":  {},
		"CNAME": {},
	}
	queriedNames := make([]string, 0)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			recordName := r.URL.Query().Get("name")
			queriedNames = append(queriedNames, recordName)
			if recordName != "us.example.com" {
				t.Fatalf("unexpected Cloudflare query: %s", r.URL.RawQuery)
			}
			recordType := r.URL.Query().Get("type")
			if recordType == "" {
				writeCloudflareTestResponse(w, map[string]any{
					"success":     true,
					"result":      allCloudflareRecordsForTest(records),
					"result_info": map[string]any{"page": 1, "total_pages": 1},
				})
				return
			}
			writeCloudflareTestResponse(w, map[string]any{
				"success":     true,
				"result":      records[recordType],
				"result_info": map[string]any{"page": 1, "total_pages": 1},
			})
		case http.MethodDelete:
			deleteCloudflareRecordForTest(records, pathBaseForTest(r.URL.Path))
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": map[string]string{"id": pathBaseForTest(r.URL.Path)}})
		case http.MethodPost:
			var record CloudflareDNSRecord
			if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
				t.Fatalf("decode post: %v", err)
			}
			record.ID = strings.ToLower(record.Type) + "-created"
			records[record.Type] = append(records[record.Type], record)
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": record})
		case http.MethodPatch:
			var record CloudflareDNSRecord
			if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
				t.Fatalf("decode patch: %v", err)
			}
			record.ID = pathBaseForTest(r.URL.Path)
			for index := range records[record.Type] {
				if records[record.Type][index].ID == record.ID {
					records[record.Type][index] = record
					writeCloudflareTestResponse(w, map[string]any{"success": true, "result": record})
					return
				}
			}
			t.Fatalf("patch target %q not found", record.ID)
		default:
			t.Fatalf("unexpected Cloudflare method %s", r.Method)
		}
	}))
	defer server.Close()
	cloudflareAPIBaseURL = server.URL

	mobileTCPProbeRunner = func() (utils.PingDelaySet, error) {
		return utils.PingDelaySet{
			{
				PingData: &utils.PingData{
					IP:       parseMobileTestIP("104.16.0.1"),
					Sended:   4,
					Received: 4,
					Delay:    20 * time.Millisecond,
				},
				DownloadSpeed: 5 * 1024 * 1024,
			},
			{
				PingData: &utils.PingData{
					IP:       parseMobileTestIP("104.20.0.1"),
					Sended:   4,
					Received: 4,
					Delay:    10 * time.Millisecond,
				},
				DownloadSpeed: 15 * 1024 * 1024,
			},
		}, nil
	}
	mobileTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet { return input }
	mobileDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	service := newServiceWithMobileColoDictionaryForTest(t)
	snapshot := defaultConfigSnapshot()
	mapValue(snapshot["probe"])["disable_download"] = true
	mapValue(snapshot["probe"])["print_num"] = 2
	scheduler := mapValue(snapshot["scheduler"])
	scheduler["auto_dns_push"] = true
	scheduler["auto_github_export"] = false
	scheduler["enabled"] = true
	scheduler["interval_minutes"] = 15
	snapshot["cloudflare"] = map[string]any{
		"api_token":       "test-token",
		"record_name":     "",
		"record_type":     "A",
		"routing_enabled": true,
		"routing_rules": []map[string]any{
			{"enabled": true, "filter_tokens": "US", "name": "us", "record_name": "us.example.com", "record_type": "A", "top_n": 1},
		},
		"ttl":     300,
		"zone_id": "zone-123",
	}
	snapshot["sources"] = []map[string]any{{
		"content":  "104.16.0.1\n104.20.0.1",
		"enabled":  true,
		"ip_limit": 10,
		"ip_mode":  "traverse",
		"kind":     "inline",
		"name":     "routing-source",
	}}
	if err := service.writeConfigSnapshot(snapshot); err != nil {
		t.Fatalf("writeConfigSnapshot: %v", err)
	}

	result := decodeCommandForTest(t, service.RunScheduledProbe("{}"))
	if !boolValue(result["ok"], false) {
		t.Fatalf("RunScheduledProbe failed: %#v", result)
	}
	data := mapValue(result["data"])
	if got := stringValue(data["last_dns_status"], ""); got != "completed" {
		t.Fatalf("last_dns_status = %q, want completed; data=%#v", got, data)
	}
	if got := intValue(data["cloudflare_upload_count"], 0); got != 1 {
		t.Fatalf("cloudflare_upload_count = %d, want routed upload count 1", got)
	}
	if !reflect.DeepEqual(recordContentsForTest(records["A"]), []string{"104.20.0.1"}) {
		t.Fatalf("A records = %#v", records["A"])
	}
	if len(queriedNames) == 0 {
		t.Fatal("queriedNames is empty, want route target queries")
	}
	for _, name := range queriedNames {
		if name != "us.example.com" {
			t.Fatalf("queriedNames = %#v, want only route target", queriedNames)
		}
	}
}

func TestRunScheduledProbeKeepsDNSFailureDetailWhenGitHubNotEnabled(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	oldTCP := mobileTCPProbeRunner
	oldTrace := mobileTraceProbeRunner
	oldDownload := mobileDownloadProbeRunner
	t.Cleanup(func() {
		cloudflareAPIBaseURL = oldBaseURL
		mobileTCPProbeRunner = oldTCP
		mobileTraceProbeRunner = oldTrace
		mobileDownloadProbeRunner = oldDownload
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeCloudflareTestResponse(w, map[string]any{
				"success":     true,
				"result":      []CloudflareDNSRecord{},
				"result_info": map[string]any{"page": 1, "total_pages": 1},
			})
		case http.MethodPost:
			http.Error(w, "cloudflare failed", http.StatusBadGateway)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	cloudflareAPIBaseURL = server.URL

	mobileTCPProbeRunner = func() (utils.PingDelaySet, error) {
		return utils.PingDelaySet{{
			PingData: &utils.PingData{
				IP:       parseMobileTestIP("1.1.1.1"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
			},
			DownloadSpeed: 10 * 1024 * 1024,
		}}, nil
	}
	mobileTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet { return input }
	mobileDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	snapshot := defaultConfigSnapshot()
	mapValue(snapshot["probe"])["disable_download"] = true
	scheduler := mapValue(snapshot["scheduler"])
	scheduler["auto_dns_push"] = true
	scheduler["auto_github_export"] = false
	scheduler["enabled"] = true
	scheduler["interval_minutes"] = 15
	snapshot["cloudflare"] = map[string]any{
		"api_token":   "test-token",
		"record_name": "a.example.com",
		"record_type": "A",
		"ttl":         300,
		"zone_id":     "zone-123",
	}
	snapshot["sources"] = []map[string]any{{
		"content":  "1.1.1.1",
		"enabled":  true,
		"ip_limit": 10,
		"ip_mode":  "traverse",
		"kind":     "inline",
		"name":     "valid-source",
	}}
	if err := service.writeConfigSnapshot(snapshot); err != nil {
		t.Fatalf("writeConfigSnapshot: %v", err)
	}

	result := decodeCommandForTest(t, service.RunScheduledProbe("{}"))
	if !boolValue(result["ok"], false) {
		t.Fatalf("RunScheduledProbe failed: %#v", result)
	}
	data := mapValue(result["data"])
	if got := stringValue(data["last_dns_status"], ""); got != "failed" {
		t.Fatalf("last_dns_status = %q, want failed", got)
	}
	if message := stringValue(data["last_message"], ""); !strings.Contains(message, "cloudflare failed") {
		t.Fatalf("last_message = %q, want cloudflare failure detail", message)
	}
}

func TestRunScheduledProbePreservesPartialDNSStatus(t *testing.T) {
	oldBaseURL := cloudflareAPIBaseURL
	oldTCP := mobileTCPProbeRunner
	oldTrace := mobileTraceProbeRunner
	oldDownload := mobileDownloadProbeRunner
	t.Cleanup(func() {
		cloudflareAPIBaseURL = oldBaseURL
		mobileTCPProbeRunner = oldTCP
		mobileTraceProbeRunner = oldTrace
		mobileDownloadProbeRunner = oldDownload
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": []CloudflareDNSRecord{}, "result_info": map[string]any{"page": 1, "total_pages": 1}})
		case http.MethodPost:
			var record CloudflareDNSRecord
			if err := json.NewDecoder(r.Body).Decode(&record); err != nil {
				t.Fatalf("decode post: %v", err)
			}
			if record.Name == "edge.example.com" {
				http.Error(w, `{"success":false,"errors":[{"message":"primary unavailable"}]}`, http.StatusBadGateway)
				return
			}
			record.ID = strings.ToLower(record.Type) + "-created"
			writeCloudflareTestResponse(w, map[string]any{"success": true, "result": record})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()
	cloudflareAPIBaseURL = server.URL

	mobileTCPProbeRunner = func() (utils.PingDelaySet, error) {
		return utils.PingDelaySet{{
			PingData: &utils.PingData{
				IP:       parseMobileTestIP("203.0.113.10"),
				Sended:   3,
				Received: 3,
				Delay:    10 * time.Millisecond,
				Colo:     "HKG",
			},
			DownloadSpeed: 10 * 1024 * 1024,
		}}, nil
	}
	mobileTraceProbeRunner = func(input utils.PingDelaySet) utils.PingDelaySet { return input }
	mobileDownloadProbeRunner = func(input utils.PingDelaySet) utils.DownloadSpeedSet {
		return utils.DownloadSpeedSet(input)
	}

	service := NewService()
	baseDir := t.TempDir()
	decodeCommandForTest(t, service.Init(baseDir))
	if err := os.WriteFile(filepath.Join(baseDir, colodict.ColoFileName), []byte("ip_prefix,colo,country,region,city\n203.0.113.0/24,HKG,HK,,Hong Kong\n"), 0o600); err != nil {
		t.Fatalf("write mobile colo file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, colodict.ColoIPv4FileName), []byte("ip_prefix,colo,country,region,city\n203.0.113.0/24,HKG,HK,,Hong Kong\n"), 0o600); err != nil {
		t.Fatalf("write mobile IPv4 colo file: %v", err)
	}
	emptyIPv6Raw, err := colodict.EncodeColoEntries(nil)
	if err != nil {
		t.Fatalf("EncodeColoEntries(empty): %v", err)
	}
	if err := os.WriteFile(filepath.Join(baseDir, colodict.ColoIPv6FileName), emptyIPv6Raw, 0o600); err != nil {
		t.Fatalf("write mobile IPv6 colo file: %v", err)
	}

	snapshot := defaultConfigSnapshot()
	mapValue(snapshot["probe"])["disable_download"] = true
	scheduler := mapValue(snapshot["scheduler"])
	scheduler["auto_dns_push"] = true
	scheduler["auto_github_export"] = false
	scheduler["enabled"] = true
	scheduler["interval_minutes"] = 15
	snapshot["cloudflare"] = map[string]any{
		"api_token":       "test-token",
		"record_name":     "edge.example.com",
		"record_type":     "A",
		"routing_enabled": true,
		"routing_rules": []map[string]any{
			{"enabled": true, "name": "hk", "record_name": "hk.example.com", "record_type": "A", "filter_tokens": "HKG", "top_n": 1},
		},
		"ttl":     300,
		"zone_id": "zone-123",
	}
	snapshot["sources"] = []map[string]any{{
		"content":  "203.0.113.10",
		"enabled":  true,
		"ip_limit": 10,
		"ip_mode":  "traverse",
		"kind":     "inline",
		"name":     "partial-dns-source",
	}}
	if err := service.writeConfigSnapshot(snapshot); err != nil {
		t.Fatalf("writeConfigSnapshot: %v", err)
	}

	result := decodeCommandForTest(t, service.RunScheduledProbe("{}"))
	if !boolValue(result["ok"], false) {
		t.Fatalf("RunScheduledProbe failed: %#v", result)
	}
	data := mapValue(result["data"])
	if got := stringValue(data["last_dns_status"], ""); got != appcore.UploadNotificationStatusPartial {
		t.Fatalf("last_dns_status = %q, want partial", got)
	}
	if message := stringValue(data["last_message"], ""); !strings.Contains(message, "部分完成") {
		t.Fatalf("last_message = %q, want partial DNS detail", message)
	}
	notification := mapValue(data["upload_notification"])
	if got := stringValue(notification["cloudflare_status"], ""); got != appcore.UploadNotificationStatusPartial {
		t.Fatalf("cloudflare_status = %q, want partial", got)
	}
	if got := stringValue(notification["status"], ""); got != appcore.UploadNotificationStatusPartial {
		t.Fatalf("notification status = %q, want partial", got)
	}
}

func TestMobileSchedulerSingleTaskCompletionMessagePreservesDownstreamStatuses(t *testing.T) {
	if got := mobileSchedulerSingleTaskCompletionMessage("failed", "skipped"); got != "Android 定时测速流程已完成，DNS 推送失败，GitHub 导出已跳过。" {
		t.Fatalf("message = %q, want DNS failure with GitHub skipped", got)
	}
	if got := mobileSchedulerSingleTaskCompletionMessage("completed", "failed"); got != "Android 定时测速与 DNS 推送流程已完成，GitHub 导出失败。" {
		t.Fatalf("message = %q, want DNS completed with GitHub failure", got)
	}
}

func TestRunScheduledProbeFailureMarksEnabledDownstreamFailed(t *testing.T) {
	oldTCP := mobileTCPProbeRunner
	t.Cleanup(func() {
		mobileTCPProbeRunner = oldTCP
	})
	mobileTCPProbeRunner = func() (utils.PingDelaySet, error) {
		return nil, errors.New("scheduled mobile tcp failed")
	}

	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	snapshot := defaultConfigSnapshot()
	scheduler := mapValue(snapshot["scheduler"])
	scheduler["auto_dns_push"] = true
	scheduler["auto_github_export"] = true
	scheduler["enabled"] = true
	scheduler["interval_minutes"] = 15
	snapshot["sources"] = []map[string]any{{
		"content":  "1.1.1.1",
		"enabled":  true,
		"ip_limit": 10,
		"ip_mode":  "traverse",
		"kind":     "inline",
		"name":     "valid-source",
	}}
	if err := service.writeConfigSnapshot(snapshot); err != nil {
		t.Fatalf("writeConfigSnapshot: %v", err)
	}

	result := decodeCommandForTest(t, service.RunScheduledProbe("{}"))
	if boolValue(result["ok"], true) {
		t.Fatalf("RunScheduledProbe unexpectedly succeeded: %#v", result)
	}
	data := mapValue(result["data"])
	if got := stringValue(data["last_probe_status"], ""); got != "failed" {
		t.Fatalf("last_probe_status = %q, want failed", got)
	}
	if got := stringValue(data["last_dns_status"], ""); got != "failed" {
		t.Fatalf("last_dns_status = %q, want failed", got)
	}
	if got := stringValue(data["last_github_status"], ""); got != "failed" {
		t.Fatalf("last_github_status = %q, want failed", got)
	}
	notification := mapValue(data["upload_notification"])
	if got := stringValue(notification["status"], ""); got != appcore.UploadNotificationStatusFailed {
		t.Fatalf("notification status = %q, want failed; notification=%#v", got, notification)
	}
}

func TestRunScheduledProbeSkipActiveRearmsFutureRun(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	snapshot := defaultConfigSnapshot()
	scheduler := mapValue(snapshot["scheduler"])
	scheduler["enabled"] = true
	scheduler["interval_minutes"] = 15
	scheduler["skip_if_active"] = true
	if err := service.writeConfigSnapshot(snapshot); err != nil {
		t.Fatalf("writeConfigSnapshot: %v", err)
	}
	if err := service.writeSchedulerStatus(mobileSchedulerStatus{
		Enabled:   true,
		NextRunAt: time.Now().Add(-time.Minute).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("writeSchedulerStatus: %v", err)
	}
	service.stateMu.Lock()
	service.currentTaskID = "manual-task"
	service.stateMu.Unlock()

	start := time.Now()
	result := decodeCommandForTest(t, service.RunScheduledProbe("{}"))
	if !boolValue(result["ok"], false) {
		t.Fatalf("RunScheduledProbe failed: %#v", result)
	}
	if got := stringValue(result["code"], ""); got != "SCHEDULER_RUN_SKIPPED" {
		t.Fatalf("code = %q, want SCHEDULER_RUN_SKIPPED", got)
	}
	data := mapValue(result["data"])
	next := parseMobileSchedulerTime(stringValue(data["next_run_at"], ""))
	if next.IsZero() || !next.After(start) {
		t.Fatalf("next_run_at = %q, want future rearmed time after skip", data["next_run_at"])
	}
	if got := stringValue(data["last_probe_status"], ""); got != "skipped" {
		t.Fatalf("last_probe_status = %q, want skipped", got)
	}
}

func TestRunScheduledProbeLoadConfigFailureClearsStaleNextRun(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	if err := service.writeSchedulerStatus(mobileSchedulerStatus{
		Enabled:   true,
		NextRunAt: time.Now().Add(-time.Minute).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("writeSchedulerStatus: %v", err)
	}
	if err := os.WriteFile(service.configPath(), []byte("{"), 0o600); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	result := decodeCommandForTest(t, service.RunScheduledProbe("{}"))
	if boolValue(result["ok"], true) {
		t.Fatalf("RunScheduledProbe unexpectedly succeeded: %#v", result)
	}
	if got := stringValue(result["code"], ""); got != "SCHEDULER_RUN_FAILED" {
		t.Fatalf("code = %q, want SCHEDULER_RUN_FAILED", got)
	}
	data := mapValue(result["data"])
	if got := stringValue(data["next_run_at"], ""); got != "" {
		t.Fatalf("next_run_at = %q, want cleared after config load failure", got)
	}
	if got := stringValue(data["workflow_stage"], ""); got != "load_config_failed" {
		t.Fatalf("workflow_stage = %q, want load_config_failed", got)
	}
}

func TestRunScheduledProbeDisabledClearsStaleNextRun(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))
	snapshot := defaultConfigSnapshot()
	scheduler := mapValue(snapshot["scheduler"])
	scheduler["enabled"] = false
	if err := service.writeConfigSnapshot(snapshot); err != nil {
		t.Fatalf("writeConfigSnapshot: %v", err)
	}
	if err := service.writeSchedulerStatus(mobileSchedulerStatus{
		Enabled:   true,
		NextRunAt: time.Now().Add(-time.Minute).Format(time.RFC3339),
	}); err != nil {
		t.Fatalf("writeSchedulerStatus: %v", err)
	}

	result := decodeCommandForTest(t, service.RunScheduledProbe("{}"))
	if !boolValue(result["ok"], false) {
		t.Fatalf("RunScheduledProbe failed: %#v", result)
	}
	if got := stringValue(result["code"], ""); got != "SCHEDULER_RUN_SKIPPED" {
		t.Fatalf("code = %q, want SCHEDULER_RUN_SKIPPED", got)
	}
	data := mapValue(result["data"])
	if got := stringValue(data["next_run_at"], ""); got != "" {
		t.Fatalf("next_run_at = %q, want cleared when scheduler is disabled", got)
	}
	if boolValue(data["enabled"], true) {
		t.Fatal("enabled = true, want disabled scheduler status")
	}
}

func TestPushCloudflareDNSRecordsInvalidInputIncludesUploadCount(t *testing.T) {
	service := NewService()
	decodeCommandForTest(t, service.Init(t.TempDir()))

	result := decodeCommandForTest(t, service.PushCloudflareDNSRecords(encodeJSON(cloudflarePayloadForTest("not-an-ip"))))
	if boolValue(result["ok"], true) {
		t.Fatalf("PushCloudflareDNSRecords unexpectedly succeeded: %#v", result)
	}
	data := mapValue(result["data"])
	if got := intValue(data["upload_count"], -1); got != 0 {
		t.Fatalf("upload_count = %d, want 0 for invalid input", got)
	}
}
