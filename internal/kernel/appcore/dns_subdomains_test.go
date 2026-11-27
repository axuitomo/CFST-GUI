package appcore

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInvokeDNSHEListSubdomainsReturnsVisualizationData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("endpoint")+"."+r.URL.Query().Get("action") != "subdomains.list" {
			t.Errorf("unexpected request %s", r.URL.RawQuery)
		}
		if r.Header.Get("X-API-Key") != "key-1" || r.Header.Get("X-API-Secret") != "secret-1" {
			t.Errorf("missing DNSHE credentials: %#v", r.Header)
		}
		fmt.Fprint(w, `{"success":true,"subdomains":[{"id":9,"subdomain":"edge","rootdomain":"l.cd","full_domain":"edge.l.cd","status":"Registered"},{"id":7,"subdomain":"api","rootdomain":"l.cd","full_domain":"api.l.cd","status":"Registered"}]}`)
	}))
	defer server.Close()

	service := NewService(ServiceOptions{DNSHEAPIBaseURL: server.URL, Storage: StorageLayout{Root: t.TempDir()}})
	payload, err := json.Marshal(map[string]any{
		"config": map[string]any{"dns_providers": []any{map[string]any{
			"enabled": true, "id": "he", "provider": "dnshe", "api_key": "key-1", "api_secret": "secret-1",
		}}},
		"provider_id": "he",
	})
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	result := service.Invoke("dns.subdomains", string(payload))
	if !result.OK || result.Code != "DNS_SUBDOMAINS_LISTED" {
		t.Fatalf("result = %+v, want DNS_SUBDOMAINS_LISTED", result)
	}
	data := mapValue(result.Data)
	if data["count"] != 2 || data["provider"] != "dnshe" {
		t.Fatalf("data = %#v", data)
	}
	items := configItemMaps(data["subdomains"])
	if len(items) != 2 || items[0]["full_domain"] != "api.l.cd" || items[0]["subdomain"] != "api" || items[1]["id"] != 9 {
		t.Fatalf("subdomains = %#v, want sorted full domains", items)
	}
}

func TestInvokeDNSHEListSubdomainsRejectsMissingCredentials(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir()}})
	result := service.Invoke("dns.subdomains", `{"config":{"dns_providers":[{"enabled":true,"id":"he","provider":"dnshe","api_key":"******","api_secret":"******","record_name":"edge.l.cd"}]},"provider_id":"he"}`)
	if result.OK || result.Code != "DNS_CONFIG_INVALID" {
		t.Fatalf("result = %+v, want DNS_CONFIG_INVALID", result)
	}
}
