package mobileapi

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMobileExportResultsToGitHubWritesRows(t *testing.T) {
	var putRequest mobileGitHubContentsPutRequest
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
		case r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/contents/cfst-results/"):
			sawGet = true
			http.NotFound(w, r)
		case r.Method == http.MethodPut && strings.Contains(r.URL.Path, "/contents/cfst-results/"):
			sawPut = true
			if err := json.NewDecoder(r.Body).Decode(&putRequest); err != nil {
				t.Fatalf("decode PUT request: %v", err)
			}
			_, _ = w.Write([]byte(`{"commit":{"sha":"commit-sha"},"content":{"sha":"content-sha","path":"cfst-results/2026-05-09/task.csv","html_url":"https://github.com/o/r/blob/main/cfst-results/2026-05-09/task.csv"}}`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	defer server.Close()

	previousBaseURL := mobileGitHubAPIBaseURL
	mobileGitHubAPIBaseURL = server.URL
	t.Cleanup(func() { mobileGitHubAPIBaseURL = previousBaseURL })

	service := NewService()
	payload := map[string]any{
		"config": map[string]any{
			"export": map[string]any{
				"github": map[string]any{
					"branch":                  "main",
					"commit_message_template": "CFST results {task_id}",
					"owner":                   "owner",
					"path_template":           "cfst-results/{date}/{task_id}.csv",
					"repo":                    "repo",
					"token":                   "test-token",
				},
			},
		},
		"results": []map[string]any{
			{"address": "1.1.1.1", "colo": "HKG", "download_mbps": 12.34, "max_download_mbps": 23.45, "tcp_latency_ms": 8.9},
		},
		"task_id": "task/one",
	}
	raw, _ := json.Marshal(payload)
	var result commandResult
	if err := json.Unmarshal([]byte(service.ExportResultsToGitHub(string(raw))), &result); err != nil {
		t.Fatalf("decode command result: %v", err)
	}
	if !result.OK || result.Code != "GITHUB_EXPORT_OK" {
		t.Fatalf("result = %#v, want GITHUB_EXPORT_OK", result)
	}
	if !sawGet || !sawPut {
		t.Fatalf("sawGet=%v sawPut=%v, want both", sawGet, sawPut)
	}
	if putRequest.SHA != "" {
		t.Fatalf("PUT sha = %q, want empty for create", putRequest.SHA)
	}
	decoded, err := base64.StdEncoding.DecodeString(putRequest.Content)
	if err != nil {
		t.Fatalf("decode content: %v", err)
	}
	content := string(decoded)
	if !strings.Contains(content, "平均速率(MB/s),最高速率(MB/s)") || !strings.Contains(content, "12.34,23.45,HKG") {
		t.Fatalf("csv content = %q, want exported row", content)
	}
}

func TestMobileGitHubExportCSVFromRowsUsesBOMEncoding(t *testing.T) {
	service := NewService()
	body, rowCount, err := service.mobileGitHubExportCSVFromPayload(map[string]any{
		"results": []probeRow{
			{IP: "1.1.1.1", Sended: 4, Received: 4, DelayMS: 12.34, DownloadSpeedMB: 56.78, Colo: "HKG"},
		},
	}, "utf-8-bom")
	if err != nil {
		t.Fatalf("mobileGitHubExportCSVFromPayload returned error: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("rowCount = %d, want 1", rowCount)
	}
	if !strings.HasPrefix(string(body), "\xEF\xBB\xBF") {
		t.Fatalf("CSV body does not start with BOM: %q", string(body[:3]))
	}
}

func TestMobileTestGitHubExportChecksRepositoryBranchAndContents(t *testing.T) {
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
			if r.URL.Query().Get("ref") != "main" {
				t.Fatalf("contents ref = %q, want main", r.URL.Query().Get("ref"))
			}
			_, _ = w.Write([]byte(`[]`))
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	previousBaseURL := mobileGitHubAPIBaseURL
	mobileGitHubAPIBaseURL = server.URL
	t.Cleanup(func() { mobileGitHubAPIBaseURL = previousBaseURL })

	payload := map[string]any{"config": map[string]any{"export": map[string]any{"github": map[string]any{
		"branch":        "main",
		"owner":         "owner",
		"path_template": "results/{task_id}.csv",
		"repo":          "repo",
		"token":         "test-token",
	}}}}
	raw, _ := json.Marshal(payload)
	var result commandResult
	if err := json.Unmarshal([]byte(NewService().TestGitHubExport(string(raw))), &result); err != nil {
		t.Fatalf("decode command result: %v", err)
	}
	if !result.OK || result.Code != "GITHUB_EXPORT_TEST_OK" || !sawRepoGet || !sawBranchGet || !sawContentsGet {
		t.Fatalf("result = %#v sawRepo=%v sawBranch=%v sawContents=%v, want success", result, sawRepoGet, sawBranchGet, sawContentsGet)
	}
}

func TestMobileTestGitHubExportFailsForBranchAndContentsErrors(t *testing.T) {
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
				case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo":
					if tc.name == "missing repo" {
						http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
						return
					}
					_, _ = w.Write([]byte(`{"full_name":"owner/repo"}`))
				case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/branches/main":
					w.WriteHeader(tc.branchStatus)
					_, _ = w.Write([]byte(`{"name":"main"}`))
				case r.Method == http.MethodGet && r.URL.Path == "/repos/owner/repo/contents/results":
					w.WriteHeader(tc.contentsStatus)
					_, _ = w.Write([]byte(`[]`))
				default:
					t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
				}
			}))
			defer server.Close()

			previousBaseURL := mobileGitHubAPIBaseURL
			mobileGitHubAPIBaseURL = server.URL
			t.Cleanup(func() { mobileGitHubAPIBaseURL = previousBaseURL })

			payload := map[string]any{"config": map[string]any{"export": map[string]any{"github": map[string]any{
				"branch":        "main",
				"owner":         "owner",
				"path_template": "results/{task_id}.csv",
				"repo":          "repo",
				"token":         "test-token",
			}}}}
			raw, _ := json.Marshal(payload)
			var result commandResult
			if err := json.Unmarshal([]byte(NewService().TestGitHubExport(string(raw))), &result); err != nil {
				t.Fatalf("decode command result: %v", err)
			}
			if result.OK || result.Code != "GITHUB_EXPORT_TEST_FAILED" {
				t.Fatalf("result = %#v, want GITHUB_EXPORT_TEST_FAILED", result)
			}
		})
	}
}

func TestMobileLoadSchedulerStatusUnsupported(t *testing.T) {
	var result commandResult
	if err := json.Unmarshal([]byte(NewService().LoadSchedulerStatus()), &result); err != nil {
		t.Fatalf("decode command result: %v", err)
	}
	if result.OK || result.Code != "SCHEDULER_UNSUPPORTED" {
		t.Fatalf("result = %#v, want unsupported failure", result)
	}
}
