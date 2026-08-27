package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestShouldRunWebUIHealthcheck(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{args: []string{"--healthcheck"}, want: true},
		{args: []string{"--cli", "--healthcheck"}, want: false},
		{args: []string{"--healthcheck", "--verbose"}, want: false},
		{args: nil, want: false},
	}
	for _, tc := range tests {
		if got := shouldRunWebUIHealthcheck(tc.args); got != tc.want {
			t.Fatalf("shouldRunWebUIHealthcheck(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}

func TestWebUIHealthcheckURL(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{name: "default", addr: "", want: "http://127.0.0.1:34115/api/health"},
		{name: "wildcard", addr: "0.0.0.0:34115", want: "http://127.0.0.1:34115/api/health"},
		{name: "wildcard with scheme", addr: "http://0.0.0.0:34115", want: "http://127.0.0.1:34115/api/health"},
		{name: "port only", addr: ":34115", want: "http://127.0.0.1:34115/api/health"},
		{name: "loopback", addr: "127.0.0.1:34116", want: "http://127.0.0.1:34116/api/health"},
		{name: "hostname", addr: "localhost:34115", want: "http://localhost:34115/api/health"},
		{name: "ipv6 wildcard", addr: "[::]:34115", want: "http://127.0.0.1:34115/api/health"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := webUIHealthcheckURL(tc.addr); got != tc.want {
				t.Fatalf("webUIHealthcheckURL(%q) = %q, want %q", tc.addr, got, tc.want)
			}
		})
	}
}

func TestRunWebUIHealthcheck(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	if got := runWebUIHealthcheck(context.Background(), addr, server.Client()); got != 0 {
		t.Fatalf("runWebUIHealthcheck() = %d, want 0", got)
	}
}

func TestNormalizeCLIDownloadSettingsAppliesSharedFallback(t *testing.T) {
	protocol, warnings := normalizeCLIDownloadSettings("auto", "https://speed.example.test/500m")
	if protocol == "" {
		t.Fatal("expected resolved download protocol")
	}
	if runtime.GOOS == "android" && protocol != "tcp" {
		t.Fatalf("android auto protocol = %q, want tcp", protocol)
	}
	if runtime.GOOS == "linux" && (runtime.GOARCH == "arm" || runtime.GOARCH == "arm64") && protocol != "tcp" {
		t.Fatalf("linux arm auto protocol = %q, want tcp", protocol)
	}
	if runtime.GOOS == "android" && !containsWarning(warnings, "当前平台") {
		t.Fatalf("warnings = %#v, missing android protocol fallback", warnings)
	}

	_, warnings = normalizeCLIDownloadSettings("auto", "http://speed.example.test/500m")
	if runtime.GOOS == "android" && !containsWarning(warnings, "明文 HTTP") {
		t.Fatalf("warnings = %#v, missing android cleartext warning", warnings)
	}
	if runtime.GOOS != "android" && containsWarning(warnings, "明文 HTTP") {
		t.Fatalf("warnings = %#v, desktop should not warn about Android cleartext", warnings)
	}
}

func TestCLIProbePayloadMapsFlagsToSharedSnapshot(t *testing.T) {
	payload, printNum, outputFile, warnings := cliProbePayload(cliProbeFlags{
		Routines:             321,
		PingTimes:            5,
		TestCount:            7,
		DownloadTimeSeconds:  6,
		TCPPort:              2053,
		URL:                  "https://speed.example.test/500m",
		UserAgent:            "cli-agent",
		HostHeader:           "cf.example.com",
		SNI:                  "sni.example.com",
		CaptureAddress:       "127.0.0.1:8080",
		InsecureSkipVerify:   true,
		DownloadHTTPProtocol: "h2",
		Httping:              true,
		HttpingStatusCode:    301,
		HttpingCFColo:        "HKG,NRT",
		MaxDelayMS:           200,
		MinDelayMS:           10,
		MaxLossRate:          0.2,
		MinSpeed:             5,
		PrintNum:             3,
		IPText:               "1.1.1.1,2.2.2.0/24",
		OutputFile:           "out.csv",
		DisableDownload:      true,
		TestAll:              true,
		Debug:                true,
	})
	if printNum != 3 || outputFile != "out.csv" {
		t.Fatalf("print/output = %d/%q", printNum, outputFile)
	}
	if !payload.DisablePostProbePush || payload.ConfigSource != "cli" {
		t.Fatalf("payload flags = %#v", payload)
	}
	if len(payload.Sources) != 1 || payload.Sources[0].Kind != "inline" || payload.Sources[0].IPLimit != -1 {
		t.Fatalf("source = %#v", payload.Sources)
	}
	if !strings.Contains(payload.Sources[0].Content, "2.2.2.0/24") {
		t.Fatalf("inline content = %q", payload.Sources[0].Content)
	}
	probe := payload.Config["probe"].(map[string]any)
	if probe["strategy"] != "fast" || probe["test_all"] != true || probe["httping"] != true {
		t.Fatalf("probe = %#v", probe)
	}
	if probe["print_num"] != 0 {
		t.Fatalf("print_num should stay 0 so CSV is not trimmed, got %#v", probe["print_num"])
	}
	exportCfg := payload.Config["export"].(map[string]any)
	if exportCfg["file_name"] != "out.csv" || exportCfg["target_dir"] != "." {
		t.Fatalf("export = %#v", exportCfg)
	}
	if containsWarning(warnings, "明文 HTTP") {
		t.Fatalf("https URL should not warn about cleartext: %#v", warnings)
	}

	emptyExport, _, emptyPath, _ := cliProbePayload(cliProbeFlags{
		URL:        defaultFileTestURL,
		OutputFile: "",
		IPFile:     "ip.txt",
	})
	if emptyPath != "" {
		t.Fatalf("empty output path = %q", emptyPath)
	}
	if emptyExport.Config["export"].(map[string]any)["file_name"] != "" {
		t.Fatalf("empty export file_name = %#v", emptyExport.Config["export"])
	}
}

func containsWarning(warnings []string, needle string) bool {
	for _, warning := range warnings {
		if strings.Contains(warning, needle) {
			return true
		}
	}
	return false
}

func TestRunWebUIHealthcheckFailsOnNonOK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not ready", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	addr := strings.TrimPrefix(server.URL, "http://")
	if got := runWebUIHealthcheck(context.Background(), addr, server.Client()); got == 0 {
		t.Fatalf("runWebUIHealthcheck() = %d, want non-zero", got)
	}
}
