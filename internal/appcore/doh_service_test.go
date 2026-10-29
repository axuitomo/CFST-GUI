package appcore

import (
	"context"
	"errors"
	"net"
	"strings"
	"testing"
)

type fakeDoHResolver struct {
	addrs     []net.IPAddr
	lookupErr error
	ech       map[string][]byte
	queries   []string
}

func (f *fakeDoHResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return f.addrs, f.lookupErr
}

func (f *fakeDoHResolver) ECHConfig(_ context.Context, host string) ([]byte, error) {
	f.queries = append(f.queries, host)
	return f.ech[host], nil
}

func TestDoHTestReportsAddressesAndECHSource(t *testing.T) {
	resolver := &fakeDoHResolver{
		addrs: []net.IPAddr{{IP: net.ParseIP("104.16.0.1")}, {IP: net.ParseIP("2606:4700::1")}},
		ech:   map[string][]byte{"cloudflare-ech.com": {1, 2, 3}},
	}
	request := dohTestRequest{Endpoint: "https://dns.example/dns-query", Host: "example.com", ECH: true, ECHDomain: "cloudflare-ech.com"}

	data, code, message, ok := runDoHTest(context.Background(), resolver, request)
	if !ok || code != "DOH_TEST_OK" {
		t.Fatalf("runDoHTest = %s %v, want DOH_TEST_OK", code, ok)
	}
	if got := data["ipv4"]; len(got.([]string)) != 1 {
		t.Fatalf("ipv4 = %#v, want one address", got)
	}
	if got := data["ipv6"]; len(got.([]string)) != 1 {
		t.Fatalf("ipv6 = %#v, want one address", got)
	}
	if data["ech_source"] != "domain" || data["ech_config_bytes"] != 3 {
		t.Fatalf("ech = %v/%v, want domain/3", data["ech_source"], data["ech_config_bytes"])
	}
	if !strings.Contains(message, "cloudflare-ech.com") {
		t.Fatalf("message = %q, want ECH domain mentioned", message)
	}
}

func TestDoHTestPrefersManualECHConfig(t *testing.T) {
	resolver := &fakeDoHResolver{
		addrs: []net.IPAddr{{IP: net.ParseIP("104.16.0.1")}},
		ech:   map[string][]byte{"example.com": {9, 9}},
	}
	request := dohTestRequest{Host: "example.com", ECH: true, ECHDomain: "cloudflare-ech.com", ECHConfig: []byte{1, 2}}

	data, _, _, ok := runDoHTest(context.Background(), resolver, request)
	if !ok || data["ech_source"] != "manual" || data["ech_config_bytes"] != 2 {
		t.Fatalf("ech = %v/%v ok=%v, want manual/2", data["ech_source"], data["ech_config_bytes"], ok)
	}
	if len(resolver.queries) != 0 {
		t.Fatalf("queries = %v, want no HTTPS RR lookup with manual config", resolver.queries)
	}
}

func TestDoHTestUsesHostECHBeforeDomain(t *testing.T) {
	resolver := &fakeDoHResolver{
		addrs: []net.IPAddr{{IP: net.ParseIP("104.16.0.1")}},
		ech:   map[string][]byte{"example.com": {5}, "cloudflare-ech.com": {7, 7}},
	}
	request := dohTestRequest{Host: "example.com", ECH: true, ECHDomain: "cloudflare-ech.com"}

	data, _, _, _ := runDoHTest(context.Background(), resolver, request)
	if data["ech_source"] != "host" || data["ech_config_bytes"] != 1 {
		t.Fatalf("ech = %v/%v, want host/1", data["ech_source"], data["ech_config_bytes"])
	}
}

func TestDoHTestFailsWhenEndpointDoesNotAnswer(t *testing.T) {
	resolver := &fakeDoHResolver{lookupErr: errors.New("DoH 返回状态 500 Internal Server Error")}
	request := dohTestRequest{Host: "example.com", ECH: false, ECHDomain: "cloudflare-ech.com"}

	_, code, message, ok := runDoHTest(context.Background(), resolver, request)
	if ok || code != "DOH_TEST_FAILED" {
		t.Fatalf("runDoHTest = %s %v, want DOH_TEST_FAILED", code, ok)
	}
	if !strings.Contains(message, "DoH 端点测试失败") {
		t.Fatalf("message = %q", message)
	}
}

func TestDoHTestTreatsNXDomainAsReachableEndpoint(t *testing.T) {
	resolver := &fakeDoHResolver{lookupErr: &net.DNSError{Err: "NXDOMAIN", Name: "missing.example", IsNotFound: true}}
	request := dohTestRequest{Host: "missing.example", ECH: true, ECHDomain: "cloudflare-ech.com"}

	data, code, message, ok := runDoHTest(context.Background(), resolver, request)
	if !ok || code != "DOH_TEST_OK" {
		t.Fatalf("runDoHTest = %s %v, want DOH_TEST_OK", code, ok)
	}
	if len(data["ipv4"].([]string)) != 0 || !strings.Contains(message, "没有解析结果") {
		t.Fatalf("data = %#v message = %q", data, message)
	}
}

func TestDoHTestRequestFallsBackWithoutPayload(t *testing.T) {
	request, warnings := dohTestRequestFromPayload(map[string]any{})
	if request.Endpoint != "https://dns.alidns.com/dns-query" || request.Host != defaultDoHTestHost || !request.ECH || request.ECHDomain != "cloudflare-ech.com" || request.ECHConfig != nil {
		t.Fatalf("request = %#v, want default endpoint/host/ECH domain", request)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %v, want none", warnings)
	}
}

func TestDoHTestRequestWarnsOnInvalidECHValues(t *testing.T) {
	request, warnings := dohTestRequestFromPayload(map[string]any{
		"ech_config": "not-base64",
		"ech_domain": "https://cloudflare-ech.com/path",
	})
	if request.ECHConfig != nil {
		t.Fatalf("ECHConfig = %v, want nil for invalid base64", request.ECHConfig)
	}
	if request.ECHDomain != "cloudflare-ech.com" {
		t.Fatalf("ECHDomain = %q, want default", request.ECHDomain)
	}
	if len(warnings) != 2 {
		t.Fatalf("warnings = %v, want two warnings", warnings)
	}
}

func TestDoHTestRejectsNonHTTPSDoHEndpoint(t *testing.T) {
	service := &Service{}
	result := service.Invoke("doh.test", `{"doh_url":"http://dns.example/dns-query"}`)
	if result.OK || result.Code != "DOH_TEST_INVALID" {
		t.Fatalf("result = %#v, want DOH_TEST_INVALID", result)
	}
}
