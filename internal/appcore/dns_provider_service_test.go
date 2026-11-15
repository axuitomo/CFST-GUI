package appcore

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func TestPushDNSProvidersWritesEveryEnabledPlatform(t *testing.T) {
	cloudflareCreated := 0
	cloudflareServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"success":true,"result":[],"result_info":{"page":1,"total_pages":1}}`)
		case http.MethodPost:
			cloudflareCreated++
			fmt.Fprint(w, `{"success":true,"result":{"id":"cf-new","type":"A","name":"edge.example.com","content":"1.1.1.1","ttl":300}}`)
		default:
			t.Errorf("unexpected cloudflare method %s", r.Method)
		}
	}))
	defer cloudflareServer.Close()

	dnsheCreated := 0
	dnsheServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("endpoint") + "." + r.URL.Query().Get("action") {
		case "subdomains.list":
			fmt.Fprint(w, `{"success":true,"subdomains":[{"id":9,"full_domain":"api.example.com"}]}`)
		case "dns_records.list":
			fmt.Fprint(w, `{"success":true,"records":[]}`)
		case "dns_records.create":
			dnsheCreated++
			fmt.Fprint(w, `{"success":true,"id":40}`)
		default:
			t.Errorf("unexpected dnshe request %s", r.URL.RawQuery)
		}
	}))
	defer dnsheServer.Close()

	service := NewService(ServiceOptions{
		CloudflareAPIBaseURL: cloudflareServer.URL,
		DNSHEAPIBaseURL:      dnsheServer.URL,
		Storage:              StorageLayout{Root: t.TempDir()},
	})
	snapshot := map[string]any{
		"dns_providers": []any{
			map[string]any{"enabled": true, "id": "cf", "name": "主站", "provider": "cloudflare", "api_token": "token-1", "zone_id": "zone-1", "record_name": "edge.example.com", "record_type": "A", "ttl": 300, "top_n": 5},
			map[string]any{"enabled": true, "id": "he", "name": "备用", "provider": "dnshe", "api_key": "key-1", "api_secret": "secret-1", "record_name": "api.example.com", "record_type": "A", "ttl": 600, "top_n": 5},
		},
	}
	result := service.PushDNSProviders(context.Background(), map[string]any{
		"config":  snapshot,
		"results": []probecore.ProbeRow{{IP: "1.1.1.1", Colo: "LAX", DownloadSpeedMB: 10}},
	})
	if !result.OK || result.Code != "DNS_PUSH_COMPLETED" {
		t.Fatalf("result = %+v, want completed push", result)
	}
	if cloudflareCreated != 1 || dnsheCreated != 1 {
		t.Fatalf("created = cloudflare %d dnshe %d, want 1 and 1", cloudflareCreated, dnsheCreated)
	}
	providers := configItemMaps(mapValue(result.Data)["providers"])
	if len(providers) != 2 {
		t.Fatalf("providers = %#v, want 2 entries", providers)
	}
	for _, provider := range providers {
		if provider["status"] != UploadNotificationStatusCompleted || provider["upload_count"] != 1 {
			t.Fatalf("provider report = %#v", provider)
		}
	}
	reports := DNSProviderReportsFromCommandData(result.Data)
	if len(reports) != 2 || reports[1].Provider != DNSProviderDNSHE || reports[1].Label != "备用" {
		t.Fatalf("notification reports = %#v", reports)
	}
}

func TestPushDNSProvidersKeepsGoingWhenOnePlatformIsMisconfigured(t *testing.T) {
	cloudflareServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `{"success":true,"result":[],"result_info":{"page":1,"total_pages":1}}`)
			return
		}
		fmt.Fprint(w, `{"success":true,"result":{"id":"cf-new","type":"A","name":"edge.example.com","content":"1.1.1.1","ttl":300}}`)
	}))
	defer cloudflareServer.Close()

	service := NewService(ServiceOptions{
		CloudflareAPIBaseURL: cloudflareServer.URL,
		DNSHEAPIBaseURL:      "http://127.0.0.1:1",
		Storage:              StorageLayout{Root: t.TempDir()},
	})
	snapshot := map[string]any{
		"dns_providers": []any{
			map[string]any{"enabled": true, "id": "cf", "provider": "cloudflare", "api_token": "token-1", "zone_id": "zone-1", "record_name": "edge.example.com", "record_type": "A", "ttl": 300},
			map[string]any{"enabled": true, "id": "he", "provider": "dnshe", "api_key": "key-1", "record_name": "api.example.com"},
		},
	}
	result := service.PushDNSProviders(context.Background(), map[string]any{
		"config":  snapshot,
		"results": []probecore.ProbeRow{{IP: "1.1.1.1"}},
	})
	if !result.OK || result.Code != "DNS_PUSH_PARTIAL" {
		t.Fatalf("result = %+v, want partial push", result)
	}
	providers := configItemMaps(mapValue(result.Data)["providers"])
	if len(providers) != 2 {
		t.Fatalf("providers = %#v, want 2 entries", providers)
	}
	if providers[1]["status"] != UploadNotificationStatusFailed {
		t.Fatalf("misconfigured dnshe report = %#v", providers[1])
	}
}

func TestPushDNSProvidersRespectsProviderSelection(t *testing.T) {
	cloudflareServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			fmt.Fprint(w, `{"success":true,"result":[],"result_info":{"page":1,"total_pages":1}}`)
			return
		}
		fmt.Fprint(w, `{"success":true,"result":{"id":"cf-new","type":"A","name":"edge.example.com","content":"1.1.1.1","ttl":300}}`)
	}))
	defer cloudflareServer.Close()

	service := NewService(ServiceOptions{
		CloudflareAPIBaseURL: cloudflareServer.URL,
		DNSHEAPIBaseURL:      "http://127.0.0.1:1",
		Storage:              StorageLayout{Root: t.TempDir()},
	})
	snapshot := map[string]any{
		"dns_providers": []any{
			map[string]any{"enabled": true, "id": "cf", "provider": "cloudflare", "api_token": "token-1", "zone_id": "zone-1", "record_name": "edge.example.com", "record_type": "A", "ttl": 300},
			map[string]any{"enabled": true, "id": "he", "provider": "dnshe", "api_key": "key-1", "api_secret": "secret-1", "record_name": "api.example.com", "record_type": "A", "ttl": 600},
		},
	}
	result := service.PushDNSProviders(context.Background(), map[string]any{
		"config":      snapshot,
		"provider_id": "cf",
		"results":     []probecore.ProbeRow{{IP: "1.1.1.1"}},
	})
	if !result.OK || result.Code != "DNS_PUSH_COMPLETED" {
		t.Fatalf("result = %+v, want completed push", result)
	}
	providers := configItemMaps(mapValue(result.Data)["providers"])
	if len(providers) != 1 || providers[0]["id"] != "cf" {
		t.Fatalf("providers = %#v, want only the selected platform", providers)
	}
}

func TestDNSProvidersFromSnapshotFallsBackToLegacyCloudflareBlock(t *testing.T) {
	snapshot := map[string]any{
		"cloudflare": map[string]any{"api_token": "token-1", "enabled": true, "record_name": "edge.example.com", "record_type": "AAAA", "ttl": 60, "zone_id": "zone-1"},
		"upload":     map[string]any{"cloudflare": map[string]any{"top_n": 3}},
	}
	providers := DNSProvidersFromSnapshot(snapshot)
	if len(providers) != 1 {
		t.Fatalf("providers = %#v, want one legacy provider", providers)
	}
	provider := providers[0]
	if provider.Provider != DNSProviderCloudflare || !provider.Enabled || provider.RecordType != "AAAA" || provider.TTL != 60 || provider.TopN != 3 {
		t.Fatalf("legacy provider = %#v", provider)
	}
}
