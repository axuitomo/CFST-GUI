package appcore

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/kernel/appcore/notify"
)

func TestDNSReadTargetScopes(t *testing.T) {
	provider := DNSProviderConfig{ID: "aliyun-1", Provider: DNSProviderAliyun, RecordName: "cdn.example.com"}

	payload := map[string]any{"scope": "zone"}
	target, err := resolveDNSReadTarget(payload, provider)
	if err != nil || len(target.Names) != 0 || len(target.Labels) != 0 || target.Scope != "zone" {
		t.Fatalf("zone scope = %#v/%v, want whole zone", target, err)
	}

	payload = map[string]any{"scope": "custom", "name": "Edge.Example.COM."}
	target, err = resolveDNSReadTarget(payload, provider)
	if err != nil || target.Scope != "custom" || len(target.Names) != 1 || target.Names[0] != "edge.example.com" {
		t.Fatalf("custom scope = %#v/%v, want normalized name", target, err)
	}

	payload = map[string]any{"scope": "custom", "name": "Test"}
	target, err = resolveDNSReadTarget(payload, provider)
	if err != nil || len(target.Labels) != 1 || target.Labels[0] != "test" {
		t.Fatalf("custom label scope = %#v/%v, want a first-level label", target, err)
	}

	payload = map[string]any{"scope": "configured"}
	target, err = resolveDNSReadTarget(payload, provider)
	if err != nil || target.Scope != "configured" || len(target.Names) != 1 || target.Names[0] != "cdn.example.com" {
		t.Fatalf("configured scope = %#v/%v, want provider record name", target, err)
	}

	if _, err := resolveDNSReadTarget(map[string]any{"scope": "custom"}, provider); err == nil {
		t.Fatalf("custom scope without a name must fail")
	}
	if _, err := resolveDNSReadTarget(map[string]any{"scope": "configured"}, DNSProviderConfig{ID: "dnshe-1", Provider: DNSProviderDNSHE}); err == nil {
		t.Fatalf("configured scope without a provider record name must fail")
	}
}

func TestDNSReadTargetUsesRoutingRules(t *testing.T) {
	provider := DNSProviderConfig{
		ID:       "cf-1",
		Provider: DNSProviderCloudflare,
		Routing: notify.UploadCloudflareRoutingConfig{Enabled: true, Rules: []notify.UploadCloudflareRoutingRule{
			{Enabled: false, RecordName: "skip.example.com"},
			{Enabled: true, RecordName: "JP.Example.com."},
			{Enabled: true, RecordName: "jp.example.com"},
			{Enabled: true, RecordName: "us.example.com"},
		}},
	}

	// 记录名称为空时按"记录名称 > 第一个分流域名"补位。
	target, err := resolveDNSReadTarget(map[string]any{"scope": "configured"}, provider)
	if err != nil || target.Scope != "configured" || len(target.Names) != 1 || target.Names[0] != "jp.example.com" {
		t.Fatalf("configured fallback = %#v/%v, want the first enabled rule", target, err)
	}

	// 填了记录名称时优先级更高，不去读分流规则。
	provider.RecordName = "edge.example.com"
	target, err = resolveDNSReadTarget(map[string]any{"scope": "configured"}, provider)
	if err != nil || len(target.Names) != 1 || target.Names[0] != "edge.example.com" {
		t.Fatalf("configured priority = %#v/%v, want the record name", target, err)
	}

	// routes 作用域一次返回所有启用规则的去重记录名。
	target, err = resolveDNSReadTarget(map[string]any{"scope": "routes"}, provider)
	if err != nil || target.Scope != "routes" || len(target.Names) != 2 || target.Names[0] != "jp.example.com" || target.Names[1] != "us.example.com" {
		t.Fatalf("routes scope = %#v/%v, want both enabled rules", target, err)
	}
	if _, err := resolveDNSReadTarget(map[string]any{"scope": "routes"}, DNSProviderConfig{ID: "cf-2", Provider: DNSProviderCloudflare}); err == nil {
		t.Fatalf("routes scope without enabled rules must fail")
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

	if got := filterDNSRecordSnapshots(records, dnsReadTarget{}, ""); len(got) != 3 {
		t.Fatalf("no filter kept %d records, want 3", len(got))
	}
	if got := filterDNSRecordSnapshots(records, dnsReadTarget{Names: []string{"edge.example.com"}}, ""); len(got) != 2 {
		t.Fatalf("name filter kept %d records, want 2", len(got))
	}
	if got := filterDNSRecordSnapshots(records, dnsReadTarget{}, "A"); len(got) != 2 {
		t.Fatalf("type filter kept %d records, want 2", len(got))
	}
	if got := filterDNSRecordSnapshots(records, dnsReadTarget{Names: []string{"edge.example.com"}}, "AAAA"); len(got) != 1 || got[0].Content != "2400::1" {
		t.Fatalf("combined filter = %#v, want only the AAAA row", got)
	}
	// 一级子域名标签模糊匹配：test 命中 test.<zone>，不会命中 test2.<zone>。
	if got := filterDNSRecordSnapshots(records, dnsReadTarget{Labels: []string{"edge"}}, ""); len(got) != 2 {
		t.Fatalf("label filter kept %d records, want 2", len(got))
	}
	if got := filterDNSRecordSnapshots(records, dnsReadTarget{Labels: []string{"ed"}}, ""); len(got) != 0 {
		t.Fatalf("partial label kept %d records, want 0", len(got))
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

func TestInvokeDNSRecordsListDNSHEZoneScopeReturnsAllRecords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("endpoint") + "." + r.URL.Query().Get("action") {
		case "subdomains.list":
			fmt.Fprint(w, `{"success":true,"subdomains":[{"id":9,"subdomain":"edge","rootdomain":"example.com","full_domain":"edge.example.com"}]}`)
		case "dns_records.list":
			if got := r.URL.Query().Get("subdomain_id"); got != "9" {
				t.Errorf("subdomain_id = %s, want 9", got)
			}
			fmt.Fprint(w, `{"success":true,"records":[{"id":1,"name":"@","type":"A","content":"1.1.1.1","ttl":600},{"id":2,"name":"www","type":"A","content":"2.2.2.2","ttl":600}]}`)
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
		"scope":       "zone",
	}))
	if !result.OK || result.Code != "DNS_RECORDS_LISTED" {
		t.Fatalf("result = %+v, want listed records", result)
	}
	records, ok := mapValue(result.Data)["records"].([]dnsRecordSnapshot)
	if !ok || len(records) != 2 {
		t.Fatalf("records = %#v, want every record in the zone", mapValue(result.Data)["records"])
	}
}

func TestInvokeDNSRecordsListDNSHECoosesBareSubdomainTarget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("endpoint") + "." + r.URL.Query().Get("action") {
		case "subdomains.list":
			fmt.Fprint(w, `{"success":true,"subdomains":[{"id":5,"subdomain":"axuitomo","rootdomain":"l.cd","full_domain":"axuitomo.l.cd"}]}`)
		case "dns_records.list":
			fmt.Fprint(w, `{"success":true,"records":[{"id":1,"name":"@","type":"A","content":"1.1.1.1","ttl":600},{"id":2,"name":"test","type":"A","content":"2.2.2.2","ttl":600}]}`)
		default:
			t.Errorf("unexpected DNSHE request %s", r.URL.RawQuery)
		}
	}))
	defer server.Close()

	service := NewService(ServiceOptions{DNSHEAPIBaseURL: server.URL, Storage: StorageLayout{Root: t.TempDir()}})
	result := service.invokeDNSRecordsList(mustJSON(t, map[string]any{
		"config": map[string]any{
			"dns_providers": []any{
				map[string]any{
					"api_key":     "key",
					"api_secret":  "secret",
					"id":          "dnshe-1",
					"provider":    "dnshe",
					"record_name": "axuitomo.l.cd",
					"subdomain":   "test",
				},
			},
		},
		"provider_id": "dnshe-1",
		"scope":       "configured",
	}))
	if !result.OK || result.Code != "DNS_RECORDS_LISTED" {
		t.Fatalf("result = %+v, want listed records", result)
	}
	records, ok := mapValue(result.Data)["records"].([]dnsRecordSnapshot)
	if !ok || len(records) != 1 || records[0].Name != "test.axuitomo.l.cd" || records[0].Content != "2.2.2.2" {
		t.Fatalf("records = %#v, want the composed target only", mapValue(result.Data)["records"])
	}
}

func TestInvokeDNSRecordsListCloudflareRoutesAndLabelScopes(t *testing.T) {
	pushedNames := make([]string, 0, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		pushedNames = append(pushedNames, r.URL.Query().Get("name"))
		fmt.Fprint(w, `{"success":true,"result":[
			{"id":"1","name":"jp.xyz.qzz.io","type":"A","content":"1.1.1.1","ttl":300,"zone_name":"xyz.qzz.io"},
			{"id":"2","name":"us.xyz.qzz.io","type":"A","content":"2.2.2.2","ttl":300,"zone_name":"xyz.qzz.io"},
			{"id":"3","name":"test.xyz.qzz.io","type":"A","content":"3.3.3.3","ttl":300,"zone_name":"xyz.qzz.io"},
			{"id":"4","name":"other.xyz.qzz.io","type":"A","content":"4.4.4.4","ttl":300,"zone_name":"xyz.qzz.io"}
		],"result_info":{"page":1,"total_pages":1}}`)
	}))
	defer server.Close()

	service := NewService(ServiceOptions{CloudflareAPIBaseURL: server.URL, Storage: StorageLayout{Root: t.TempDir()}})
	config := map[string]any{
		"dns_providers": []any{
			map[string]any{
				"id":              "cf-1",
				"provider":        "cloudflare",
				"api_token":       "token",
				"zone_id":         "zone",
				"record_name":     "",
				"routing_enabled": true,
				"routing_rules": []any{
					map[string]any{"id": "r1", "enabled": true, "record_name": "jp.xyz.qzz.io"},
					map[string]any{"id": "r2", "enabled": true, "record_name": "us.xyz.qzz.io"},
					map[string]any{"id": "r3", "enabled": false, "record_name": "other.xyz.qzz.io"},
				},
			},
		},
	}

	// routes：一次读取所有启用分流规则的记录名。
	result := service.invokeDNSRecordsList(mustJSON(t, map[string]any{
		"config":      config,
		"provider_id": "cf-1",
		"scope":       "routes",
	}))
	if !result.OK || result.Code != "DNS_RECORDS_LISTED" {
		t.Fatalf("routes result = %+v, want listed records", result)
	}
	data := mapValue(result.Data)
	records, ok := data["records"].([]dnsRecordSnapshot)
	if !ok || len(records) != 2 {
		t.Fatalf("routes records = %#v, want the two enabled rule targets", data["records"])
	}
	if data["scope"] != "routes" || data["zone_name"] != "xyz.qzz.io" {
		t.Fatalf("routes data = %#v, want routes scope and zone name", data)
	}

	// configured：记录名称为空时退回第一个启用规则。
	result = service.invokeDNSRecordsList(mustJSON(t, map[string]any{
		"config":      config,
		"provider_id": "cf-1",
		"scope":       "configured",
	}))
	data = mapValue(result.Data)
	if records, ok := data["records"].([]dnsRecordSnapshot); !ok || len(records) != 1 || records[0].Name != "jp.xyz.qzz.io" {
		t.Fatalf("configured records = %#v, want the first enabled rule", data["records"])
	}

	// custom：一级子域名模糊匹配，并回报 Zone 域名。
	result = service.invokeDNSRecordsList(mustJSON(t, map[string]any{
		"config":      config,
		"provider_id": "cf-1",
		"scope":       "custom",
		"name":        "test",
	}))
	data = mapValue(result.Data)
	if records, ok := data["records"].([]dnsRecordSnapshot); !ok || len(records) != 1 || records[0].Name != "test.xyz.qzz.io" {
		t.Fatalf("label records = %#v, want the first-level match", data["records"])
	}
	if data["zone_name"] != "xyz.qzz.io" {
		t.Fatalf("label data = %#v, want the zone name", data)
	}
	// configured 只查一个记录名时下推 name；routes 与一级子域名读取必须列整 Zone 再本地过滤。
	if len(pushedNames) != 3 || pushedNames[0] != "" || pushedNames[1] != "jp.xyz.qzz.io" || pushedNames[2] != "" {
		t.Fatalf("pushed names = %#v, want zone listings for routes and labels", pushedNames)
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
