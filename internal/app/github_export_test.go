package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportCSVToGitHubCreatesAndUpdatesContent(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 3, 4, 0, time.UTC)
	body, err := encodeProbeRowsCSV([]ProbeRow{
		{IP: "1.1.1.1", Sended: 4, Received: 4, LossRate: 0, DelayMS: 12.34, DownloadSpeedMB: 56.78, MaxDownloadSpeedMB: 78.9, Colo: "HKG", TraceDelayMS: 8.9},
	})
	if err != nil {
		t.Fatalf("encodeProbeRowsCSV returned error: %v", err)
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
			var putRequest githubContentsPutRequest
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
					_ = json.NewEncoder(w).Encode(githubContentsResponse{SHA: tc.existingSHA})
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
			restoreGitHubAPIBaseURL(t, server.URL)

			result, err := exportCSVToGitHub(context.Background(), githubExportConfig{
				Branch:                "main",
				CommitMessageTemplate: "CFST results {date} {time}",
				Owner:                 "octo",
				PathTemplate:          "cfst-results/{date}/{time}-{task_id}.csv",
				Repo:                  "demo",
				Token:                 "test-token",
			}, "task-1", body, 1, now)
			if err != nil {
				t.Fatalf("exportCSVToGitHub returned error: %v", err)
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

func TestGitHubExportCSVFromRowsUsesBOMEncoding(t *testing.T) {
	body, rowCount, err := githubExportCSVFromPayload(map[string]any{
		"results": []ProbeRow{
			{IP: "1.1.1.1", Sended: 4, Received: 4, DelayMS: 12.34, DownloadSpeedMB: 56.78, Colo: "HKG"},
		},
	}, "utf-8-bom")
	if err != nil {
		t.Fatalf("githubExportCSVFromPayload returned error: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("rowCount = %d, want 1", rowCount)
	}
	if !strings.HasPrefix(string(body), "\xEF\xBB\xBF") {
		t.Fatalf("CSV body does not start with BOM: %q", string(body[:3]))
	}
}

func TestExportCSVToGitHubPropagatesWriteErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		case http.MethodPut:
			http.Error(w, `{"message":"branch protected"}`, http.StatusConflict)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	restoreGitHubAPIBaseURL(t, server.URL)

	_, err := exportCSVToGitHub(context.Background(), githubExportConfig{
		Branch:                "main",
		CommitMessageTemplate: defaultGitHubExportCommitMessageTemplate,
		Owner:                 "octo",
		PathTemplate:          defaultGitHubExportPathTemplate,
		Repo:                  "demo",
		Token:                 "test-token",
	}, "task-1", []byte("header\nrow\n"), 1, time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "409") {
		t.Fatalf("err = %v, want propagated 409 conflict", err)
	}
}

func TestExportResultsToGitHubUsesFrontendResultRows(t *testing.T) {
	var putRequest githubContentsPutRequest
	var sawGet bool
	var sawPut bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("Authorization = %q, want bearer token", r.Header.Get("Authorization"))
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/demo/contents/results/task-one.csv":
			sawGet = true
			http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
		case r.Method == http.MethodPut && r.URL.Path == "/repos/octo/demo/contents/results/task-one.csv":
			sawPut = true
			if err := json.NewDecoder(r.Body).Decode(&putRequest); err != nil {
				t.Fatalf("decode put request: %v", err)
			}
			_, _ = w.Write([]byte(`{"content":{"path":"results/task-one.csv","sha":"content-sha","html_url":"https://github.com/octo/demo/blob/main/results/task-one.csv"},"commit":{"sha":"commit-sha"}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	restoreGitHubAPIBaseURL(t, server.URL)

	result := (&App{}).ExportResultsToGitHub(map[string]any{
		"config": map[string]any{
			"export": map[string]any{
				"github": map[string]any{
					"branch":                  "main",
					"commit_message_template": "CFST results {task_id}",
					"owner":                   "octo",
					"path_template":           "results/{task_id}.csv",
					"repo":                    "demo",
					"token":                   "test-token",
				},
			},
		},
		"results": []map[string]any{
			{"address": "1.1.1.1", "colo": "HKG", "download_mbps": 12.34, "max_download_mbps": 23.45, "tcp_latency_ms": 8.9},
		},
		"task_id": "task/one",
	})
	if !result.OK || result.Code != "GITHUB_EXPORT_OK" {
		t.Fatalf("result = %#v, want GITHUB_EXPORT_OK", result)
	}
	if data, ok := result.Data.(GitHubExportResult); !ok || data.WrittenRows != 1 {
		t.Fatalf("result data = %#v, want one written row", result.Data)
	}
	if !sawGet || !sawPut {
		t.Fatalf("sawGet=%v sawPut=%v, want both requests", sawGet, sawPut)
	}
	decoded, err := base64.StdEncoding.DecodeString(putRequest.Content)
	if err != nil {
		t.Fatalf("decode content: %v", err)
	}
	content := string(decoded)
	if !strings.Contains(content, "平均速率(MB/s),最高速率(MB/s)") || !strings.Contains(content, "12.34,23.45,HKG") {
		t.Fatalf("csv content = %q, want frontend result row", content)
	}
}

func TestExportResultsToGitHubRejectsRowsWithoutIP(t *testing.T) {
	result := (&App{}).ExportResultsToGitHub(map[string]any{
		"config": map[string]any{
			"export": map[string]any{
				"github": map[string]any{
					"owner": "octo",
					"repo":  "demo",
					"token": "test-token",
				},
			},
		},
		"results": []map[string]any{
			{"download_mbps": 12.34},
		},
	})
	if result.OK || result.Code != "GITHUB_EXPORT_INPUT_INVALID" {
		t.Fatalf("result = %#v, want GITHUB_EXPORT_INPUT_INVALID", result)
	}
}

func TestExportResultsCSVWritesFrontendRowsToPath(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), "exports", "filtered.csv")
	result := (&App{}).ExportResultsCSV(map[string]any{
		"config": map[string]any{
			"export": map[string]any{
				"csv_encoding": "utf-8-bom",
			},
		},
		"results": []map[string]any{
			{"address": "1.1.1.1", "colo": "HKG", "download_mbps": 12.34, "max_download_mbps": 23.45, "tcp_latency_ms": 8.9, "trace_latency_ms": 9.8},
		},
		"target_path": targetPath,
		"task_id":     "csv-task",
	})
	if !result.OK || result.Code != "RESULTS_CSV_EXPORT_OK" {
		t.Fatalf("result = %#v, want RESULTS_CSV_EXPORT_OK", result)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("result data = %#v, want map", result.Data)
	}
	if got := data["path"]; got != targetPath {
		t.Fatalf("path = %#v, want %q", got, targetPath)
	}
	if got := data["file_name"]; got != "filtered.csv" {
		t.Fatalf("file_name = %#v, want filtered.csv", got)
	}
	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("read exported csv: %v", err)
	}
	if !strings.HasPrefix(string(raw), "\xEF\xBB\xBF") {
		t.Fatalf("csv body does not start with BOM: %q", string(raw[:3]))
	}
	content := string(raw)
	if !strings.Contains(content, "平均速率(MB/s),最高速率(MB/s),地区码,追踪延迟(ms)") || !strings.Contains(content, "12.34,23.45,HKG,9.80") {
		t.Fatalf("csv content = %q, want exported row", content)
	}
}

func TestExportResultsCSVReturnsBase64ForTargetURI(t *testing.T) {
	result := (&App{}).ExportResultsCSV(map[string]any{
		"config": map[string]any{
			"export": map[string]any{
				"csv_encoding": "utf-8",
			},
		},
		"file_name":  "manual-export.csv",
		"results":    []map[string]any{{"address": "1.1.1.1", "colo": "HKG", "download_mbps": 12.34, "max_download_mbps": 23.45}},
		"target_uri": "browser-download:manual-export.csv",
	})
	if !result.OK || result.Code != "RESULTS_CSV_EXPORT_OK" {
		t.Fatalf("result = %#v, want RESULTS_CSV_EXPORT_OK", result)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("result data = %#v, want map", result.Data)
	}
	if got := data["file_name"]; got != "manual-export.csv" {
		t.Fatalf("file_name = %#v, want manual-export.csv", got)
	}
	contentBase64, ok := data["content_base64"].(string)
	if !ok || strings.TrimSpace(contentBase64) == "" {
		t.Fatalf("content_base64 = %#v, want non-empty string", data["content_base64"])
	}
	decoded, err := base64.StdEncoding.DecodeString(contentBase64)
	if err != nil {
		t.Fatalf("decode content_base64: %v", err)
	}
	content := string(decoded)
	if !strings.Contains(content, "IP 地址,已发送,已接收") || !strings.Contains(content, "1.1.1.1") {
		t.Fatalf("csv content = %q, want exported row", content)
	}
}

func TestExportResultsCSVFailsWhenTargetDirectoryCannotBeCreated(t *testing.T) {
	blockingPath := filepath.Join(t.TempDir(), "blocked")
	if err := os.WriteFile(blockingPath, []byte("occupied"), 0o644); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}
	result := (&App{}).ExportResultsCSV(map[string]any{
		"results": []map[string]any{
			{"address": "1.1.1.1"},
		},
		"target_path": filepath.Join(blockingPath, "result.csv"),
	})
	if result.OK || result.Code != "RESULTS_CSV_EXPORT_WRITE_FAILED" {
		t.Fatalf("result = %#v, want RESULTS_CSV_EXPORT_WRITE_FAILED", result)
	}
}

func TestTestGitHubExportChecksRepositoryBranchAndContents(t *testing.T) {
	var sawRepo bool
	var sawBranch bool
	var sawContents bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/demo":
			sawRepo = true
			_, _ = w.Write([]byte(`{"full_name":"octo/demo"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/demo/branches/main":
			sawBranch = true
			_, _ = w.Write([]byte(`{"name":"main"}`))
		case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/demo/contents/results":
			sawContents = true
			if r.URL.Query().Get("ref") != "main" {
				t.Fatalf("contents ref = %q, want main", r.URL.Query().Get("ref"))
			}
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()
	restoreGitHubAPIBaseURL(t, server.URL)

	result := (&App{}).TestGitHubExport(map[string]any{
		"config": map[string]any{
			"export": map[string]any{
				"github": map[string]any{
					"branch":        "main",
					"owner":         "octo",
					"path_template": "results/{task_id}.csv",
					"repo":          "demo",
					"token":         "test-token",
				},
			},
		},
	})
	if !result.OK || result.Code != "GITHUB_EXPORT_TEST_OK" {
		t.Fatalf("result = %#v, want GITHUB_EXPORT_TEST_OK", result)
	}
	if !sawRepo || !sawBranch || !sawContents {
		t.Fatalf("sawRepo=%v sawBranch=%v sawContents=%v, want all", sawRepo, sawBranch, sawContents)
	}
}

func TestTestGitHubExportFailsForBranchAndContentsErrors(t *testing.T) {
	for _, tc := range []struct {
		name           string
		branchStatus   int
		contentsStatus int
	}{
		{name: "missing branch", branchStatus: http.StatusNotFound, contentsStatus: http.StatusOK},
		{name: "forbidden contents", branchStatus: http.StatusOK, contentsStatus: http.StatusForbidden},
		{name: "missing repo", branchStatus: http.StatusOK, contentsStatus: http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/demo":
					if tc.name == "missing repo" {
						http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
						return
					}
					_, _ = w.Write([]byte(`{"full_name":"octo/demo"}`))
				case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/demo/branches/main":
					w.WriteHeader(tc.branchStatus)
					_, _ = w.Write([]byte(`{"name":"main"}`))
				case r.Method == http.MethodGet && r.URL.Path == "/repos/octo/demo/contents/results":
					w.WriteHeader(tc.contentsStatus)
					_, _ = w.Write([]byte(`[]`))
				default:
					t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
				}
			}))
			defer server.Close()
			restoreGitHubAPIBaseURL(t, server.URL)

			result := (&App{}).TestGitHubExport(map[string]any{
				"config": map[string]any{
					"export": map[string]any{
						"github": map[string]any{
							"branch":        "main",
							"owner":         "octo",
							"path_template": "results/{task_id}.csv",
							"repo":          "demo",
							"token":         "test-token",
						},
					},
				},
			})
			if result.OK || result.Code != "GITHUB_EXPORT_TEST_FAILED" {
				t.Fatalf("result = %#v, want GITHUB_EXPORT_TEST_FAILED", result)
			}
		})
	}
}

func TestGitHubExportConfigRejectsMissingToken(t *testing.T) {
	_, _, err := githubExportConfigFromSnapshot(map[string]any{
		"export": map[string]any{
			"github": map[string]any{
				"owner": "octo",
				"repo":  "demo",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "GitHub PAT") {
		t.Fatalf("err = %v, want missing PAT error", err)
	}
}

func restoreGitHubAPIBaseURL(t *testing.T, value string) {
	t.Helper()
	old := githubAPIBaseURL
	githubAPIBaseURL = value
	t.Cleanup(func() { githubAPIBaseURL = old })
}
