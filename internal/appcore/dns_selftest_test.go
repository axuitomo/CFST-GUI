package appcore

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/dnspush"
)

func TestDNSRecordNameValidation(t *testing.T) {
	provider := DNSProviderConfig{
		Provider:   DNSProviderCloudflare,
		RecordName: "Sub.Example.com.",
	}
	cases := []struct {
		name    string
		raw     string
		want    string
		wantErr string
	}{
		{name: "default child", want: "cfst-selftest.sub.example.com"},
		{name: "explicit child", raw: "cfst-selftest.probe.sub.example.com", want: "cfst-selftest.probe.sub.example.com"},
		{name: "production name rejected", raw: "sub.example.com", wantErr: "不能等于"},
		{name: "sibling name rejected", raw: "other.example.com", wantErr: "cfst-selftest"},
		{name: "production child rejected", raw: "www.example.com", wantErr: "cfst-selftest"},
		{name: "wildcard rejected", raw: "*.sub.example.com", wantErr: "通配符"},
		{name: "malformed label rejected", raw: "cfst-selftest.a..b.sub.example.com", wantErr: "格式无效"},
		{name: "trailing empty label rejected", raw: "cfst-selftest..sub.example.com", wantErr: "格式无效"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := dnsSelfTestRecordName(provider, tc.raw)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}

	if _, err := dnsSelfTestRecordName(DNSProviderConfig{Provider: DNSProviderCloudflare}, ""); err == nil {
		t.Fatal("expected an error when the configured record name is empty")
	}
}

// fakeSelfTestAdapter implements the same reconcile semantics the real cores
// use, but in memory. It lets the staged flow be asserted without HTTP.
type fakeSelfTestAdapter struct {
	failCNAME    bool
	failPush     bool
	ignoreTTL    bool
	name         string
	nextID       int
	records      map[string]dnsSelfTestRecord
	respectCNAME bool
}

func newFakeSelfTestAdapter(name string) *fakeSelfTestAdapter {
	return &fakeSelfTestAdapter{name: name, nextID: 1, records: map[string]dnsSelfTestRecord{}}
}

func (f *fakeSelfTestAdapter) create(record dnsSelfTestRecord) dnsSelfTestRecord {
	record.ID = fmt.Sprintf("id-%d", f.nextID)
	record.Name = f.name
	f.nextID++
	f.records[record.ID] = record
	return record
}

func (f *fakeSelfTestAdapter) Read(context.Context) (dnsSelfTestReadResult, error) {
	all := make([]dnsSelfTestRecord, 0, len(f.records))
	for _, record := range f.records {
		all = append(all, record)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })
	return dnsSelfTestReadResult{
		All:  all,
		A:    recordsBySelfTestType(all, dnspush.RecordTypeA),
		AAAA: recordsBySelfTestType(all, dnspush.RecordTypeAAAA),
	}, nil
}

func (f *fakeSelfTestAdapter) Push(_ context.Context, ips string, ttl int) (dnspush.Summary, []string, error) {
	groups, ignored := dnspush.NormalizeIPs(ips)
	summary := dnspush.Summary{Ignored: len(ignored)}
	if f.failPush {
		return summary, nil, errors.New("fake push failure")
	}
	if f.respectCNAME {
		for _, record := range f.records {
			if record.Type == dnspush.RecordTypeCNAME {
				return summary, nil, dnspush.ErrCNAMEProtected
			}
		}
	}
	desired := make([]dnsSelfTestRecord, 0)
	for _, recordType := range []string{dnspush.RecordTypeA, dnspush.RecordTypeAAAA} {
		for _, content := range groups.ForType(recordType) {
			desired = append(desired, dnsSelfTestRecord{Content: content, Name: f.name, TTL: ttl, Type: recordType})
		}
	}

	managed := managedSelfTestRecords(mustReadAll(f))
	cnames := recordsBySelfTestType(managed, dnspush.RecordTypeCNAME)
	if len(cnames) > 0 {
		for _, record := range cnames {
			delete(f.records, record.ID)
			summary.Deleted++
		}
		for _, record := range desired {
			f.create(record)
			summary.Created++
		}
		return summary, nil, nil
	}

	used := map[string]bool{}
	for _, want := range desired {
		matches := make([]dnsSelfTestRecord, 0)
		for _, existing := range managed {
			if strings.EqualFold(existing.Type, want.Type) && existing.Content == want.Content {
				matches = append(matches, existing)
			}
		}
		if len(matches) == 0 {
			f.create(want)
			summary.Created++
			continue
		}
		keep := matches[0]
		if keep.TTL != want.TTL && !f.ignoreTTL {
			keep.TTL = want.TTL
			f.records[keep.ID] = keep
			summary.Updated++
		}
		used[keep.ID] = true
	}
	for _, existing := range managed {
		switch strings.ToUpper(existing.Type) {
		case dnspush.RecordTypeA, dnspush.RecordTypeAAAA:
			if !used[existing.ID] {
				delete(f.records, existing.ID)
				summary.Deleted++
			}
		}
	}
	return summary, nil, nil
}

func (f *fakeSelfTestAdapter) CreateCNAME(_ context.Context, target string, ttl int) (dnsSelfTestRecord, error) {
	if f.failCNAME {
		return dnsSelfTestRecord{}, fmt.Errorf("CNAME not allowed")
	}
	record := f.create(dnsSelfTestRecord{Content: target, TTL: ttl, Type: dnspush.RecordTypeCNAME})
	return record, nil
}

func (f *fakeSelfTestAdapter) Delete(_ context.Context, record dnsSelfTestRecord) error {
	delete(f.records, record.ID)
	return nil
}

func mustReadAll(adapter dnsSelfTestAdapter) []dnsSelfTestRecord {
	result, err := adapter.Read(context.Background())
	if err != nil {
		panic(err)
	}
	return result.All
}

func TestDNSSelfTestCommandRejectsProductionName(t *testing.T) {
	snapshot := map[string]any{
		"dns_providers": []any{map[string]any{
			"api_token":   "token",
			"enabled":     true,
			"id":          "cf",
			"provider":    "cloudflare",
			"record_name": "sub.example.com",
			"ttl":         300,
			"zone_id":     "zone",
		}},
	}
	result := (&Service{}).RunDNSPlatformSelfTest(context.Background(), map[string]any{
		"config":      snapshot,
		"provider_id": "cf",
		"test_name":   "www.example.com",
	})
	if result.Code != "DNS_SELFTEST_CONFIG_INVALID" || result.OK {
		t.Fatalf("expected config-invalid, got code=%s ok=%v message=%s", result.Code, result.OK, result.Message)
	}
	data := mapValue(result.Data)
	if data["wrote_records"] != false {
		t.Fatalf("validation failure must not report writes: %+v", data)
	}
	if data["failed"].(int) != 1 || data["skipped"].(int) != len(dnsSelfTestStageSpecs)-1 {
		t.Fatalf("unexpected stage counts: %+v", data)
	}
}

func TestDNSHESelfTestFullName(t *testing.T) {
	cases := []struct {
		name     string
		record   string
		expected string
	}{
		{name: "apex", record: "@", expected: "api.example.com"},
		{name: "bare label", record: "www", expected: "www.api.example.com"},
		{name: "multi label relative", record: "cfst-selftest.www", expected: "cfst-selftest.www.api.example.com"},
		{name: "already full", record: "cfst-selftest.api.example.com", expected: "cfst-selftest.api.example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dnsheSelfTestFullName("api.example.com", tc.record); got != tc.expected {
				t.Fatalf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestDNSSelfTestTTLPair(t *testing.T) {
	cases := []struct {
		name     string
		provider string
		ttl      int
		base     int
		alt      int
	}{
		{name: "cloudflare 300", provider: DNSProviderCloudflare, ttl: 300, base: 300, alt: 600},
		{name: "cloudflare 600", provider: DNSProviderCloudflare, ttl: 600, base: 600, alt: 300},
		{name: "aliyun 600", provider: DNSProviderAliyun, ttl: 600, base: 600, alt: 1200},
		{name: "aliyun low ttl clamped", provider: DNSProviderAliyun, ttl: 60, base: 600, alt: 1200},
		{name: "dnspod 1200", provider: DNSProviderDNSPod, ttl: 1200, base: 1200, alt: 600},
		{name: "dnshe default", provider: DNSProviderDNSHE, ttl: 0, base: 600, alt: 1200},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, alt := dnsSelfTestTTLPair(DNSProviderConfig{Provider: tc.provider, TTL: tc.ttl})
			if base != tc.base || alt != tc.alt {
				t.Fatalf("got %d/%d, want %d/%d", base, alt, tc.base, tc.alt)
			}
		})
	}
}

func TestDNSSelfTestFakeAdapterPasses(t *testing.T) {
	provider := DNSProviderConfig{
		APIToken:   "token",
		AllowCNAME: true,
		Enabled:    true,
		ID:         "cf",
		Provider:   DNSProviderCloudflare,
		RecordName: "sub.example.com",
		TTL:        300,
		ZoneID:     "zone",
	}
	testName := "cfst-selftest.sub.example.com"
	adapter := newFakeSelfTestAdapter(testName)
	result := (&Service{}).runDNSPlatformSelfTestWithAdapter(context.Background(), provider, testName, 300, 600, adapter)
	if result.Code != "DNS_SELFTEST_COMPLETED" || !result.OK {
		t.Fatalf("expected completed, got code=%s ok=%v message=%s", result.Code, result.OK, result.Message)
	}
	data := mapValue(result.Data)
	if data["failed"].(int) != 0 || data["skipped"].(int) != 0 {
		t.Fatalf("unexpected stage counts: %+v", data)
	}
	if len(adapter.records) != 0 {
		t.Fatalf("expected cleanup to remove all test records, got %+v", adapter.records)
	}
}

func TestDNSSelfTestFakeAdapterPreservesProtectedRecords(t *testing.T) {
	provider := DNSProviderConfig{
		APIToken:   "token",
		AllowCNAME: true,
		Enabled:    true,
		ID:         "cf",
		Provider:   DNSProviderCloudflare,
		RecordName: "sub.example.com",
		TTL:        300,
		ZoneID:     "zone",
	}
	testName := "cfst-selftest.sub.example.com"
	adapter := newFakeSelfTestAdapter(testName)
	protected := dnsSelfTestRecord{
		Content: "v=spf1 -all",
		ID:      "txt-1",
		Name:    testName,
		TTL:     300,
		Type:    "TXT",
	}
	adapter.records[protected.ID] = protected

	result := (&Service{}).runDNSPlatformSelfTestWithAdapter(context.Background(), provider, testName, 300, 600, adapter)
	if result.Code != "DNS_SELFTEST_COMPLETED" || !result.OK {
		t.Fatalf("expected completed, got code=%s ok=%v message=%s", result.Code, result.OK, result.Message)
	}
	if len(adapter.records) != 1 {
		t.Fatalf("expected only the protected TXT record to remain, got %+v", adapter.records)
	}
	if got := adapter.records[protected.ID]; got.Content != protected.Content || got.Type != protected.Type {
		t.Fatalf("protected TXT record changed: %+v", got)
	}
}

func TestDNSSelfTestFakeAdapterReportsTTLNotUpdated(t *testing.T) {
	provider := DNSProviderConfig{
		APIToken:   "token",
		AllowCNAME: true,
		Enabled:    true,
		ID:         "cf",
		Provider:   DNSProviderCloudflare,
		RecordName: "sub.example.com",
		TTL:        300,
		ZoneID:     "zone",
	}
	testName := "cfst-selftest.sub.example.com"
	adapter := newFakeSelfTestAdapter(testName)
	adapter.ignoreTTL = true
	result := (&Service{}).runDNSPlatformSelfTestWithAdapter(context.Background(), provider, testName, 300, 600, adapter)
	if result.Code != "DNS_SELFTEST_FAILED" || result.OK {
		t.Fatalf("expected failed, got code=%s ok=%v", result.Code, result.OK)
	}
	if !mapValue(result.Data)["cleanup_ok"].(bool) {
		t.Fatalf("expected cleanup to succeed after update failure: %+v", result.Data)
	}
	if len(adapter.records) != 0 {
		t.Fatalf("expected cleanup to remove all test records, got %+v", adapter.records)
	}
}

func TestDNSSelfTestFakeAdapterSkipsCNAME(t *testing.T) {
	provider := DNSProviderConfig{
		APIToken:   "token",
		AllowCNAME: true,
		Enabled:    true,
		ID:         "cf",
		Provider:   DNSProviderCloudflare,
		RecordName: "sub.example.com",
		TTL:        300,
		ZoneID:     "zone",
	}
	testName := "cfst-selftest.sub.example.com"
	adapter := newFakeSelfTestAdapter(testName)
	adapter.failCNAME = true
	result := (&Service{}).runDNSPlatformSelfTestWithAdapter(context.Background(), provider, testName, 300, 600, adapter)
	if result.Code != "DNS_SELFTEST_PARTIAL" || !result.OK {
		t.Fatalf("expected partial, got code=%s ok=%v message=%s", result.Code, result.OK, result.Message)
	}
	if mapValue(result.Data)["skipped"].(int) != 1 {
		t.Fatalf("expected exactly one skipped stage, got %+v", result.Data)
	}
	if len(adapter.records) != 0 {
		t.Fatalf("expected cleanup to remove all test records, got %+v", adapter.records)
	}
}

func TestDNSSelfTestFakeAdapterRefusesCNAMEWhenDisabled(t *testing.T) {
	provider := DNSProviderConfig{
		APIToken:   "token",
		AllowCNAME: false,
		Enabled:    true,
		ID:         "cf",
		Provider:   DNSProviderCloudflare,
		RecordName: "sub.example.com",
		TTL:        300,
		ZoneID:     "zone",
	}
	testName := "cfst-selftest.sub.example.com"
	adapter := newFakeSelfTestAdapter(testName)
	adapter.respectCNAME = true
	result := (&Service{}).runDNSPlatformSelfTestWithAdapter(context.Background(), provider, testName, 300, 600, adapter)
	if result.Code != "DNS_SELFTEST_COMPLETED" || !result.OK {
		t.Fatalf("expected completed, got code=%s ok=%v message=%s", result.Code, result.OK, result.Message)
	}
	if len(adapter.records) != 0 {
		t.Fatalf("expected cleanup to remove the test CNAME, got %+v", adapter.records)
	}
	data := mapValue(result.Data)
	var cnameStage dnsSelfTestStage
	for _, stage := range data["stages"].([]dnsSelfTestStage) {
		if stage.ID == "cname" {
			cnameStage = stage
			break
		}
	}
	if cnameStage.Status != dnsSelfTestStatusPassed {
		t.Fatalf("cname stage = %+v, want passed", cnameStage)
	}
	if preserved, _ := cnameStage.Details["cname_preserved"].(bool); !preserved {
		t.Fatalf("cname stage details = %+v, want cname_preserved=true", cnameStage.Details)
	}
}

func TestDNSSelfTestResetOnlyDoesNotReportWroteRecords(t *testing.T) {
	provider := DNSProviderConfig{
		APIToken:   "token",
		AllowCNAME: true,
		Enabled:    true,
		ID:         "cf",
		Provider:   DNSProviderCloudflare,
		RecordName: "sub.example.com",
		TTL:        300,
		ZoneID:     "zone",
	}
	testName := "cfst-selftest.sub.example.com"
	adapter := newFakeSelfTestAdapter(testName)
	adapter.records["residue"] = dnsSelfTestRecord{Content: "192.0.2.99", ID: "residue", Name: testName, TTL: 300, Type: dnspush.RecordTypeA}
	adapter.failPush = true
	result := (&Service{}).runDNSPlatformSelfTestWithAdapter(context.Background(), provider, testName, 300, 600, adapter)
	if result.Code != "DNS_SELFTEST_FAILED" || result.OK {
		t.Fatalf("expected failed, got code=%s ok=%v", result.Code, result.OK)
	}
	data := mapValue(result.Data)
	if wrote, _ := data["wrote_records"].(bool); wrote {
		t.Fatalf("reset-only run reported wrote_records=true: %+v", result.Data)
	}
	if cleanup, _ := data["cleanup_ok"].(bool); !cleanup {
		t.Fatalf("expected cleanup_ok=true, got %+v", result.Data)
	}
	if len(adapter.records) != 0 {
		t.Fatalf("expected cleanup to remove all records, got %+v", adapter.records)
	}
}

func TestRunDNSPlatformSelfTestBusy(t *testing.T) {
	service := &Service{}
	service.dnsSelfTestMu.Lock()
	defer service.dnsSelfTestMu.Unlock()
	result := service.RunDNSPlatformSelfTest(context.Background(), map[string]any{})
	if result.Code != "DNS_SELFTEST_BUSY" || result.OK {
		t.Fatalf("expected DNS_SELFTEST_BUSY, got code=%s ok=%v message=%s", result.Code, result.OK, result.Message)
	}
}
