package appcore

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDNSReadTargetNameScopes(t *testing.T) {
	provider := DNSProviderConfig{ID: "aliyun-1", Provider: DNSProviderAliyun, RecordName: "cdn.example.com"}

	payload := map[string]any{"scope": "zone"}
	name, scope, err := dnsReadTargetName(payload, provider)
	if err != nil || name != "" || scope != "zone" {
		t.Fatalf("zone scope = %q/%q/%v, want whole zone", name, scope, err)
	}

	payload = map[string]any{"scope": "custom", "name": "Edge.Example.COM."}
	name, scope, err = dnsReadTargetName(payload, provider)
	if err != nil || name != "edge.example.com" || scope != "custom" {
		t.Fatalf("custom scope = %q/%q/%v, want normalized name", name, scope, err)
	}

	payload = map[string]any{"scope": "configured"}
	name, scope, err = dnsReadTargetName(payload, provider)
	if err != nil || name != "cdn.example.com" || scope != "configured" {
		t.Fatalf("configured scope = %q/%q/%v, want provider record name", name, scope, err)
	}

	if _, _, err := dnsReadTargetName(map[string]any{"scope": "custom"}, provider); err == nil {
		t.Fatalf("custom scope without a name must fail")
	}
	if _, _, err := dnsReadTargetName(map[string]any{"scope": "configured"}, DNSProviderConfig{ID: "dnshe-1", Provider: DNSProviderDNSHE}); err == nil {
		t.Fatalf("configured scope without a provider record name must fail")
	}
}

func TestDNSReadRecordTypeFilters(t *testing.T) {
	cases := map[string]string{
		"":      "",
		"all":   "",
		"a":     "A",
		"AAAA":  "AAAA",
		"CNAME": "",
	}
	for input, want := range cases {
		payload := map[string]any{"record_type": input}
		if got := dnsReadRecordType(payload); got != want {
			t.Fatalf("record type %q = %q, want %q", input, got, want)
		}
	}
}

func TestFilterDNSRecordSnapshots(t *testing.T) {
	records := []dnsRecordSnapshot{
		{Name: "edge.example.com", Type: "A", Content: "1.1.1.1"},
		{Name: "Edge.Example.com.", Type: "AAAA", Content: "2400::1"},
		{Name: "other.example.com", Type: "A", Content: "2.2.2.2"},
	}

	if got := filterDNSRecordSnapshots(records, "", ""); len(got) != 3 {
		t.Fatalf("no filter kept %d records, want 3", len(got))
	}
	if got := filterDNSRecordSnapshots(records, "edge.example.com", ""); len(got) != 2 {
		t.Fatalf("name filter kept %d records, want 2", len(got))
	}
	if got := filterDNSRecordSnapshots(records, "", "A"); len(got) != 2 {
		t.Fatalf("type filter kept %d records, want 2", len(got))
	}
	if got := filterDNSRecordSnapshots(records, "edge.example.com", "AAAA"); len(got) != 1 || got[0].Content != "2400::1" {
		t.Fatalf("combined filter = %#v, want only the AAAA row", got)
	}
}

func TestInvokeDNSRecordsListUnknownProvider(t *testing.T) {
	result := (&Service{}).invokeDNSRecordsList(mustJSON(t, map[string]any{
		"config": map[string]any{
			"dns_providers": []any{
				map[string]any{"id": "cf-1", "provider": "cloudflare", "api_token": "token", "zone_id": "zone", "record_name": "edge.example.com"},
			},
		},
		"provider_id": "missing",
		"scope":       "zone",
	}))

	if result.Code != "DNS_PROVIDER_NOT_FOUND" {
		t.Fatalf("code = %q, want DNS_PROVIDER_NOT_FOUND", result.Code)
	}
	if !strings.Contains(result.Message, "没有找到") {
		t.Fatalf("message = %q, want a saved-platform hint", result.Message)
	}
}

func TestInvokeDNSRecordsListIncompleteProvider(t *testing.T) {
	result := (&Service{}).invokeDNSRecordsList(mustJSON(t, map[string]any{
		"config": map[string]any{
			"dns_providers": []any{
				map[string]any{"id": "dnshe-2", "provider": "dnshe", "api_key": "key", "api_secret": "secret"},
			},
		},
		"provider_id": "dnshe-2",
		"scope":       "zone",
	}))

	if result.Code != "DNS_CONFIG_INVALID" {
		t.Fatalf("code = %q, want DNS_CONFIG_INVALID", result.Code)
	}
	if !strings.Contains(result.Message, "记录名称") {
		t.Fatalf("message = %q, want the missing record-name reason", result.Message)
	}
}

func TestInvokeDNSRecordsListCustomScopeNeedsName(t *testing.T) {
	result := (&Service{}).invokeDNSRecordsList(mustJSON(t, map[string]any{
		"config": map[string]any{
			"dns_providers": []any{
				map[string]any{"id": "cf-1", "provider": "cloudflare", "api_token": "token", "zone_id": "zone", "record_name": "edge.example.com"},
			},
		},
		"provider_id": "cf-1",
		"scope":       "custom",
	}))

	if result.Code != "DNS_CONFIG_INVALID" {
		t.Fatalf("code = %q, want DNS_CONFIG_INVALID", result.Code)
	}
}

func TestInvokeDNSRecordsListDNSHESubdomainOnlyReadsApex(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("endpoint") + "." + r.URL.Query().Get("action") {
		case "subdomains.list":
			fmt.Fprint(w, `{"success":true,"subdomains":[{"id":9,"subdomain":"edge","rootdomain":"example.com","full_domain":"edge.example.com"}]}`)
		case "dns_records.list":
			fmt.Fprint(w, `{"success":true,"records":[{"id":1,"name":"edge.example.com","type":"A","content":"1.1.1.1","ttl":600},{"id":2,"name":"www.edge.example.com","type":"A","content":"2.2.2.2","ttl":600}]}`)
		default:
			t.Errorf("unexpected DNSHE request %s", r.URL.RawQuery)
		}
	}))
	defer server.Close()

	service := NewService(ServiceOptions{DNSHEAPIBaseURL: server.URL, Storage: StorageLayout{Root: t.TempDir()}})
	result := service.invokeDNSRecordsList(mustJSON(t, map[string]any{
		"config": map[string]any{
			"dns_providers": []any{
				map[string]any{"id": "dnshe-1", "provider": "dnshe", "api_key": "key", "api_secret": "secret", "subdomain": "edge"},
			},
		},
		"provider_id": "dnshe-1",
		"scope":       "configured",
	}))
	if !result.OK || result.Code != "DNS_RECORDS_LISTED" {
		t.Fatalf("result = %+v, want listed records", result)
	}
	records, ok := mapValue(result.Data)["records"].([]dnsRecordSnapshot)
	if !ok || len(records) != 1 || records[0].Name != "edge.example.com" || records[0].Content != "1.1.1.1" {
		t.Fatalf("records = %#v, want only the subdomain apex", mapValue(result.Data)["records"])
	}
}

func mustJSON(t *testing.T, value map[string]any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return string(raw)
}
