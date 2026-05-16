package githubcore

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func TestExportCSVCreatesAndUpdatesContent(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 3, 4, 0, time.UTC)
	body, err := EncodeProbeRowsCSV([]probecore.ProbeRow{
		{IP: "1.1.1.1", Sended: 4, Received: 4, LossRate: 0, DelayMS: 12.34, DownloadSpeedMB: 56.78, MaxDownloadSpeedMB: 78.9, Colo: "HKG", TraceDelayMS: 8.9},
	})
	if err != nil {
		t.Fatalf("EncodeProbeRowsCSV returned error: %v", err)
	}

	for _, tc := range []struct {
		name        string
		existingSHA string
		wantSHA     string
	}{
		{name: "create", wantSHA: ""},
		{name: "update", existingSHA: "old-sha", wantSHA: "old-sha"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var putRequest ContentsPutRequest
			var sawGet bool
			var sawPut bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "Bearer test-token" {
					t.Fatalf("Authorization = %q, want bearer token", r.Header.Get("Authorization"))
				}
				if r.Header.Get("X-GitHub-Api-Version") != "2022-11-28" {
					t.Fatalf("X-GitHub-Api-Version = %q", r.Header.Get("X-GitHub-Api-Version"))
				}
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/demo/contents/cfst-results/2026-05-09/120304-task-1.csv":
					sawGet = true
					if r.URL.Query().Get("ref") != "main" {
						t.Fatalf("ref = %q, want main", r.URL.Query().Get("ref"))
					}
					if tc.existingSHA == "" {
						http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
						return
					}
					_ = json.NewEncoder(w).Encode(ContentsResponse{SHA: tc.existingSHA})
				case r.Method == http.MethodPut && r.URL.Path == "/repos/octo/demo/contents/cfst-results/2026-05-09/120304-task-1.csv":
					sawPut = true
					if err := json.NewDecoder(r.Body).Decode(&putRequest); err != nil {
						t.Fatalf("decode put request: %v", err)
					}
					w.Header().Set("Content-Type", "application/json")
					_, _ = w.Write([]byte(`{"content":{"path":"cfst-results/2026-05-09/120304-task-1.csv","sha":"new-content-sha","html_url":"https://github.com/octo/demo/blob/main/cfst-results/2026-05-09/120304-task-1.csv"},"commit":{"sha":"new-commit-sha"}}`))
				default:
					t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
				}
			}))
			defer server.Close()

			result, err := ExportCSV(context.Background(), NewClientWithOptions(ClientOptions{BaseURL: server.URL, Token: "test-token"}), Config{
				Branch:                "main",
				CommitMessageTemplate: "CFST results {date} {time}",
				Owner:                 "octo",
				PathTemplate:          "cfst-results/{date}/{time}-{task_id}.csv",
				Repo:                  "demo",
				Token:                 "test-token",
			}, "task-1", body, 1, now)
			if err != nil {
				t.Fatalf("ExportCSV returned error: %v", err)
			}
			if !sawGet || !sawPut {
				t.Fatalf("sawGet=%v sawPut=%v, want both requests", sawGet, sawPut)
			}
			if putRequest.SHA != tc.wantSHA {
				t.Fatalf("put SHA = %q, want %q", putRequest.SHA, tc.wantSHA)
			}
			if putRequest.Branch != "main" {
				t.Fatalf("put branch = %q, want main", putRequest.Branch)
			}
			if putRequest.Message != "CFST results 2026-05-09 120304" {
				t.Fatalf("put message = %q", putRequest.Message)
			}
			decoded, err := base64.StdEncoding.DecodeString(putRequest.Content)
			if err != nil {
				t.Fatalf("content is not base64: %v", err)
			}
			csvText := string(decoded)
			if !strings.Contains(csvText, "平均速率(MB/s),最高速率(MB/s)") || !strings.Contains(csvText, "56.78,78.90,HKG") {
				t.Fatalf("decoded CSV = %q", csvText)
			}
			if result.Path != "cfst-results/2026-05-09/120304-task-1.csv" || result.CommitSHA != "new-commit-sha" || result.ContentSHA != "new-content-sha" {
				t.Fatalf("result = %#v", result)
			}
			if result.WrittenRows != 1 {
				t.Fatalf("WrittenRows = %d, want 1", result.WrittenRows)
			}
		})
	}
}

func TestCSVRowsFromAnyAndBOMEncoding(t *testing.T) {
	rows := ProbeRowsFromAny([]map[string]any{
		{"address": "1.1.1.1", "colo": "HKG", "download_mbps": 12.34, "max_download_mbps": 23.45, "tcp_latency_ms": 8.9, "source_port": 8443, "test_port": 2053},
		{"download_mbps": 1.23},
	})
	if len(rows) != 1 {
		t.Fatalf("rows len = %d, want 1", len(rows))
	}
	if rows[0].IP != "1.1.1.1" || rows[0].SourcePort != 8443 || rows[0].TestPort != 2053 {
		t.Fatalf("row = %#v, want frontend row fields", rows[0])
	}
	body, err := EncodeProbeRowsCSVWithEncoding(rows, "utf-8-bom")
	if err != nil {
		t.Fatalf("EncodeProbeRowsCSVWithEncoding returned error: %v", err)
	}
	if !strings.HasPrefix(string(body), "\xEF\xBB\xBF") {
		t.Fatalf("CSV body does not start with BOM: %q", string(body[:3]))
	}
	if got := CountCSVDataRows(body); got != 1 {
		t.Fatalf("CountCSVDataRows = %d, want 1", got)
	}
}

func TestEncodeProbeRowsForGitHubSupportsTXTAndCSVTemplates(t *testing.T) {
	rows := []probecore.ProbeRow{
		{
			IP:                 "1.1.1.1",
			Colo:               "HKG",
			Sended:             4,
			Received:           4,
			LossRate:           0,
			DelayMS:            12.34,
			TraceDelayMS:       8.9,
			DownloadSpeedMB:    56.78,
			MaxDownloadSpeedMB: 78.9,
			SourcePort:         2053,
			TestPort:           443,
		},
	}

	txtBody, rowCount, err := EncodeProbeRowsForGitHub(rows, Config{
		Format:         "txt",
		TXTRowTemplate: "{index}:{ip}#{source_port}->{test_port}",
	})
	if err != nil {
		t.Fatalf("EncodeProbeRowsForGitHub txt returned error: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("txt rowCount = %d, want 1", rowCount)
	}
	if got := string(txtBody); got != "1:1.1.1.1#2053->443" {
		t.Fatalf("txt body = %q", got)
	}

	csvBody, rowCount, err := EncodeProbeRowsForGitHub(rows, Config{
		Format:            "csv",
		CSVHeaderTemplate: "IP,COLO,SOURCE,TEST",
		CSVRowTemplate:    "{ip},{colo},{source_port},{test_port}",
	})
	if err != nil {
		t.Fatalf("EncodeProbeRowsForGitHub csv template returned error: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("csv rowCount = %d, want 1", rowCount)
	}
	if got := string(csvBody); got != "IP,COLO,SOURCE,TEST\n1.1.1.1,HKG,2053,443" {
		t.Fatalf("csv body = %q", got)
	}
}

func TestParseConfigFromSnapshotReadsGitHubFormatTemplates(t *testing.T) {
	cfg, _, err := ParseConfigFromSnapshot(map[string]any{
		"export": map[string]any{
			"csv_encoding": "utf-8-bom",
			"github": map[string]any{
				"owner":                 "octo",
				"repo":                  "demo",
				"token":                 "test-token",
				"format":                "txt",
				"csv_header_template":   "IP,PORT",
				"csv_row_template":      "{ip},{test_port}",
				"txt_row_template":      "{ip}:{test_port}",
				"path_template":         "results/{task_id}.txt",
				"commit_message_template": "demo {task_id}",
			},
		},
	}, ConfigDefaults{})
	if err != nil {
		t.Fatalf("ParseConfigFromSnapshot returned error: %v", err)
	}
	if cfg.Format != "txt" || cfg.CSVEncoding != "utf-8-bom" {
		t.Fatalf("config format/encoding = %#v", cfg)
	}
	if cfg.CSVHeaderTemplate != "IP,PORT" || cfg.CSVRowTemplate != "{ip},{test_port}" || cfg.TXTRowTemplate != "{ip}:{test_port}" {
		t.Fatalf("config templates = %#v", cfg)
	}
}

func TestRenderTemplateEscapesTraversalAndTaskID(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 3, 4, 0, time.UTC)
	if got := RenderTemplate("results/{task_id}.csv", "task/one", now); got != "results/task-one.csv" {
		t.Fatalf("RenderTemplate = %q, want sanitized task id", got)
	}
	if got := RenderTemplate("../../secret.csv", "task", now); got != DefaultPathTemplate {
		t.Fatalf("RenderTemplate traversal = %q, want default", got)
	}
	if got := EscapeContentPath("dir with space/result.csv"); got != "dir%20with%20space/result.csv" {
		t.Fatalf("EscapeContentPath = %q", got)
	}
}

func TestCheckExportAccessAllowsMissingContentDirectory(t *testing.T) {
	var sawRepoGet bool
	var sawBranchGet bool
	var sawContentsGet bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo":
			sawRepoGet = true
			_, _ = w.Write([]byte(`{"full_name":"owner/repo"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/branches/main":
			sawBranchGet = true
			_, _ = w.Write([]byte(`{"name":"main"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/contents/results":
			sawContentsGet = true
			http.NotFound(w, r)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	err := NewClientWithOptions(ClientOptions{BaseURL: server.URL, Token: "test-token"}).CheckExportAccess(context.Background(), Config{
		Branch:       "main",
		Owner:        "owner",
		PathTemplate: "results/{task_id}.csv",
		Repo:         "repo",
		Token:        "test-token",
	})
	if err != nil {
		t.Fatalf("CheckExportAccess returned error: %v", err)
	}
	if !sawRepoGet || !sawBranchGet || !sawContentsGet {
		t.Fatalf("sawRepo=%v sawBranch=%v sawContents=%v, want all", sawRepoGet, sawBranchGet, sawContentsGet)
	}
}

func TestParseConfigRejectsMissingOrMaskedToken(t *testing.T) {
	for _, token := range []string{"", "abc***xyz"} {
		_, _, err := ParseConfigFromSnapshot(map[string]any{
			"export": map[string]any{
				"github": map[string]any{
					"owner": "octo",
					"repo":  "demo",
					"token": token,
				},
			},
		}, ConfigDefaults{})
		if err == nil || !strings.Contains(err.Error(), "GitHub PAT") {
			t.Fatalf("token %q error = %v, want PAT error", token, err)
		}
	}
}

func TestExportCSVTargetFileNameSanitizesInputs(t *testing.T) {
	payload := map[string]any{"file_name": `bad/name?.csv`}
	if got := ExportCSVTargetFileName(payload, "", "result.csv"); got != "bad-name-.csv" {
		t.Fatalf("file name = %q", got)
	}
	if got := ExportCSVTargetFileName(map[string]any{}, "browser-download:manual.csv", "result.csv"); got != "manual.csv" {
		t.Fatalf("browser file name = %q", got)
	}
	if got := ExportCSVTargetFileName(map[string]any{}, "content://exports/path/result.csv", "fallback.csv"); got != "result.csv" {
		t.Fatalf("uri file name = %q", got)
	}
}
