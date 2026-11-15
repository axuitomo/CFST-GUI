package appcore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/probecore"
)

func TestPushDNSProvidersWritesAliyunAndDNSPodPlatforms(t *testing.T) {
	aliyunCreated := 0
	aliyunServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		query := r.URL.Query()
		if query.Get("Signature") == "" {
			t.Errorf("aliyun request without signature: %v", query)
		}
		switch query.Get("Action") {
		case "DescribeDomains":
			fmt.Fprint(w, `{"TotalCount":1,"Domains":{"Domain":[{"DomainName":"example.com"}]}}`)
		case "DescribeDomainRecords":
			fmt.Fprint(w, `{"TotalCount":0,"DomainRecords":{"Record":[]}}`)
		case "AddDomainRecord":
			aliyunCreated++
			fmt.Fprint(w, `{"RecordId":"71"}`)
		default:
			t.Errorf("unexpected aliyun action %q", query.Get("Action"))
		}
	}))
	defer aliyunServer.Close()

	dnspodCreated := 0
	dnspodServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Header.Get("X-TC-Action") {
		case "DescribeDomainList":
			fmt.Fprint(w, `{"Response":{"DomainCountInfo":{"AllTotal":1},"DomainList":[{"Domain":"example.com"}]}}`)
		case "DescribeRecordList":
			fmt.Fprint(w, `{"Response":{"RecordCountInfo":{"TotalCount":0},"RecordList":[]}}`)
		case "CreateRecord":
			dnspodCreated++
			fmt.Fprint(w, `{"Response":{"RecordId":81}}`)
		default:
			t.Errorf("unexpected dnspod action %q", r.Header.Get("X-TC-Action"))
		}
	}))
	defer dnspodServer.Close()

	service := NewService(ServiceOptions{
		AliyunAPIBaseURL: aliyunServer.URL,
		DNSPodAPIBaseURL: dnspodServer.URL,
		Storage:          StorageLayout{Root: t.TempDir()},
	})
	snapshot := map[string]any{
		"dns_providers": []any{
			map[string]any{"enabled": true, "id": "ali", "label": "阿里云主站", "provider": "aliyun", "api_key": "ak", "api_secret": "secret", "record_name": "api.example.com", "record_type": "A", "ttl": 600, "top_n": 5},
			map[string]any{"enabled": true, "id": "dp", "provider": "dnspod", "api_key": "secret-id", "api_secret": "secret-key", "record_name": "edge.example.com", "record_type": "A", "ttl": 600, "top_n": 5},
		},
	}
	result := service.PushDNSProviders(context.Background(), map[string]any{
		"config":  snapshot,
		"results": []probecore.ProbeRow{{IP: "1.1.1.1", Colo: "LAX", DownloadSpeedMB: 10}},
	})
	if !result.OK || result.Code != "DNS_PUSH_COMPLETED" {
		t.Fatalf("result = %+v, want completed push", result)
	}
	if aliyunCreated != 1 || dnspodCreated != 1 {
		t.Fatalf("created = aliyun %d dnspod %d, want 1 and 1", aliyunCreated, dnspodCreated)
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
	if providers[0]["provider"] != DNSProviderAliyun || providers[0]["label"] != "阿里云主站" {
		t.Fatalf("aliyun report = %#v", providers[0])
	}
	if providers[1]["label"] != "DNSPod (edge.example.com)" {
		t.Fatalf("dnspod report label = %v", providers[1]["label"])
	}
}

func TestPushDNSProvidersReportsMissingAliyunCredentials(t *testing.T) {
	service := NewService(ServiceOptions{Storage: StorageLayout{Root: t.TempDir()}})
	snapshot := map[string]any{
		"dns_providers": []any{
			map[string]any{"enabled": true, "id": "ali", "provider": "aliyun", "api_key": "ak", "record_name": "api.example.com", "record_type": "A", "ttl": 600, "top_n": 5},
			map[string]any{"enabled": true, "id": "dp", "provider": "dnspod", "api_key": "id", "api_secret": "key", "record_type": "A", "ttl": 600, "top_n": 5},
		},
	}
	result := service.PushDNSProviders(context.Background(), map[string]any{
		"config":  snapshot,
		"results": []probecore.ProbeRow{{IP: "1.1.1.1", DownloadSpeedMB: 10}},
	})
	if result.OK {
		t.Fatalf("result = %+v, want failure for incomplete providers", result)
	}
	providers := configItemMaps(mapValue(result.Data)["providers"])
	if len(providers) != 2 {
		t.Fatalf("providers = %#v", providers)
	}
	for _, provider := range providers {
		if provider["code"] != "DNS_CONFIG_INVALID" {
			t.Fatalf("provider report = %#v", provider)
		}
	}
}

func TestDNSProvidersFromSnapshotNormalizesNewPlatforms(t *testing.T) {
	snapshot := map[string]any{}
	raw := []map[string]any{
		{"provider": "alidns", "api_key": " ak ", "api_secret": " sk ", "record_name": "API.Example.com.", "ttl": 10},
		{"provider": "dndps", "api_key": " id ", "api_secret": " key ", "record_name": "edge.example.com", "ttl": 0},
		{"provider": "tencentcloud", "record_name": "www.example.com"},
		{"provider": "unknown-platform"},
	}
	items := make([]any, 0, len(raw))
	for _, item := range raw {
		items = append(items, item)
	}
	snapshot["dns_providers"] = items
	providers := DNSProvidersFromSnapshot(snapshot)
	if len(providers) != 3 {
		t.Fatalf("providers = %#v, want 3", providers)
	}
	if providers[0].Provider != DNSProviderAliyun || providers[0].APIKey != "ak" || providers[0].TTL != DefaultAliyunTTL {
		t.Fatalf("aliyun provider = %#v", providers[0])
	}
	if providers[1].Provider != DNSProviderDNSPod || providers[1].TTL != DefaultDNSPodTTL {
		t.Fatalf("dnspod provider = %#v", providers[1])
	}
	if providers[2].Provider != DNSProviderDNSPod || providers[2].TTL != DefaultDNSPodTTL {
		t.Fatalf("tencent alias provider = %#v", providers[2])
	}
	if problem := dnsProviderConfigProblem(providers[0]); problem != "" {
		t.Fatalf("aliyun config problem = %q", problem)
	}
	if problem := dnsProviderConfigProblem(providers[2]); problem == "" {
		t.Fatal("dnspod provider without credentials reports no problem")
	}
	encoded, err := json.Marshal(dnspodDNSConfig(providers[2]))
	if err != nil || len(encoded) == 0 {
		t.Fatalf("dnspod config marshal: %v", err)
	}
}
