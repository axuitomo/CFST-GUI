package appcore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/dnshecore"
)

func TestResolveDNSHEProviderSubdomainPrefersSubdomain(t *testing.T) {
	var getCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch query.Get("endpoint") + "." + query.Get("action") {
		case "subdomains.list":
			writeJSON(t, w, map[string]any{"success": true, "subdomains": []map[string]any{
				{"id": 8, "subdomain": "edge", "rootdomain": "example.com", "full_domain": "edge.example.com"},
				{"id": 99, "subdomain": "other", "rootdomain": "example.com", "full_domain": "other.example.com"},
			}})
		case "subdomains.get":
			getCalls++
			writeJSON(t, w, map[string]any{"success": true, "subdomain": map[string]any{
				"id": 99, "subdomain": "other", "rootdomain": "example.com", "full_domain": "other.example.com",
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := dnshecore.NewClientWithOptions(dnshecore.ClientOptions{APIKey: "key", APISecret: "secret", BaseURL: server.URL})
	provider := DNSProviderConfig{
		Provider:    DNSProviderDNSHE,
		RecordName:  "www.edge.example.com",
		Subdomain:   "edge",
		SubdomainID: 99,
	}
	subdomain, recordName, err := resolveDNSHETarget(context.Background(), client, provider)
	if err != nil {
		t.Fatalf("resolveDNSHETarget returned error: %v", err)
	}
	if subdomain.ID != 8 || subdomain.FullDomain != "edge.example.com" {
		t.Fatalf("subdomain = %#v, want edge.example.com (id 8)", subdomain)
	}
	if recordName != "www.edge.example.com" {
		t.Fatalf("recordName = %q, want www.edge.example.com", recordName)
	}
	if getCalls != 0 {
		t.Fatalf("subdomains.get was called %d times; subdomain name should win over subdomain_id", getCalls)
	}
}

func TestRunDNSSelfTestDNSHESubdomainOnlyDerivesTestName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		switch query.Get("endpoint") + "." + query.Get("action") {
		case "subdomains.list":
			writeJSON(t, w, map[string]any{"success": true, "subdomains": []map[string]any{
				{"id": 8, "subdomain": "edge", "rootdomain": "example.com", "full_domain": "edge.example.com"},
			}})
		case "dns_records.list":
			http.Error(w, `{"success":false,"message":"stop after preflight"}`, http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewService(ServiceOptions{
		DNSHEAPIBaseURL: server.URL,
		Storage:         StorageLayout{Root: t.TempDir()},
	})
	result := service.RunDNSPlatformSelfTest(context.Background(), map[string]any{
		"config": map[string]any{"dns_providers": []any{map[string]any{
			"api_key":    "key",
			"api_secret": "secret",
			"enabled":    true,
			"id":         "he",
			"provider":   "dnshe",
			"subdomain":  "edge",
		}}},
		"provider_id": "he",
	})
	if result.Code == "DNS_SELFTEST_CONFIG_INVALID" {
		t.Fatalf("subdomain-only provider was rejected before the write stages: %s", result.Message)
	}
	data := mapValue(result.Data)
	if data["test_name"] != "cfst-selftest.edge.example.com" {
		t.Fatalf("test_name = %v, want cfst-selftest.edge.example.com", data["test_name"])
	}
	if data["record_name"] != "edge.example.com" {
		t.Fatalf("record_name = %v, want resolved edge.example.com", data["record_name"])
	}
	if data["allow_cname"] != false {
		t.Fatalf("allow_cname = %v, want false by default", data["allow_cname"])
	}
}

func TestResolveDNSHETargetComposesBareLabelUnderRecordName(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("endpoint") + "." + r.URL.Query().Get("action") {
		case "subdomains.list":
			writeJSON(t, w, map[string]any{"success": true, "subdomains": []map[string]any{
				{"id": 5, "subdomain": "axuitomo", "rootdomain": "l.cd", "full_domain": "axuitomo.l.cd"},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := dnshecore.NewClientWithOptions(dnshecore.ClientOptions{APIKey: "key", APISecret: "secret", BaseURL: server.URL})
	subdomain, recordName, err := resolveDNSHETarget(context.Background(), client, DNSProviderConfig{
		Provider:   DNSProviderDNSHE,
		RecordName: "axuitomo.l.cd",
		Subdomain:  "test",
	})
	if err != nil {
		t.Fatalf("resolveDNSHETarget returned error: %v", err)
	}
	if subdomain.ID != 5 || subdomain.FullDomain != "axuitomo.l.cd" {
		t.Fatalf("subdomain = %#v, want axuitomo.l.cd (id 5)", subdomain)
	}
	if recordName != "test.axuitomo.l.cd" {
		t.Fatalf("recordName = %q, want test.axuitomo.l.cd", recordName)
	}
}

func TestRunDNSSelfTestDNSHEComposesBareSubdomainTestName(t *testing.T) {
	if testing.Short() {
		t.Skip("500 响应触发生产重试退避 ≈2s；全量语义由 CI/check.ps1 覆盖")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Query().Get("endpoint") + "." + r.URL.Query().Get("action") {
		case "subdomains.list":
			writeJSON(t, w, map[string]any{"success": true, "subdomains": []map[string]any{
				{"id": 5, "subdomain": "axuitomo", "rootdomain": "l.cd", "full_domain": "axuitomo.l.cd"},
			}})
		case "dns_records.list":
			http.Error(w, `{"success":false,"message":"stop after preflight"}`, http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	service := NewService(ServiceOptions{
		DNSHEAPIBaseURL: server.URL,
		Storage:         StorageLayout{Root: t.TempDir()},
	})
	result := service.RunDNSPlatformSelfTest(context.Background(), map[string]any{
		"config": map[string]any{"dns_providers": []any{map[string]any{
			"api_key":     "key",
			"api_secret":  "secret",
			"enabled":     true,
			"id":          "he",
			"provider":    "dnshe",
			"record_name": "axuitomo.l.cd",
			"subdomain":   "test",
		}}},
		"provider_id": "he",
	})
	if result.Code == "DNS_SELFTEST_CONFIG_INVALID" {
		t.Fatalf("bare subdomain was rejected at preflight: %s", result.Message)
	}
	data := mapValue(result.Data)
	if data["test_name"] != "cfst-selftest.axuitomo.l.cd" {
		t.Fatalf("test_name = %v, want cfst-selftest.axuitomo.l.cd (zone apex base)", data["test_name"])
	}
	if data["record_name"] != "test.axuitomo.l.cd" {
		t.Fatalf("record_name = %v, want composed target", data["record_name"])
	}
}

func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		t.Fatalf("encode response: %v", err)
	}
}
