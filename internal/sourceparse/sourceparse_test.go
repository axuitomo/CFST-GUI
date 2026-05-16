package sourceparse

import (
	"context"
	"errors"
	"net"
	"reflect"
	"strings"
	"testing"
)

type resolverFunc func(context.Context, string) ([]net.IPAddr, error)

func (fn resolverFunc) LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error) {
	return fn(ctx, host)
}

func TestParseCleansCommentsAndExtractsComplexFormats(t *testing.T) {
	resolver := resolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host != "edge.example.com" {
			return nil, errors.New("unexpected host")
		}
		return []net.IPAddr{
			{IP: net.ParseIP("203.0.113.10")},
			{IP: net.ParseIP("2001:db8::10")},
		}, nil
	})

	result := Parse(`
# pure comment

1.1.1.1 # inline comment
104.16.0.0/16; 1.0.0.1
address=/cf.example.com/8.8.8.8 # keep the IP, skip the host name on this line
https://edge.example.com/path/file.txt
bad-token
`, Options{Resolver: resolver})

	wantValid := []string{"1.1.1.1", "104.16.0.0/16", "1.0.0.1", "8.8.8.8", "203.0.113.10", "2001:db8::10"}
	if !reflect.DeepEqual(result.Valid, wantValid) {
		t.Fatalf("valid = %#v, want %#v", result.Valid, wantValid)
	}
	if !reflect.DeepEqual(result.Invalid, []string{"bad-token"}) {
		t.Fatalf("invalid = %#v, want bad-token", result.Invalid)
	}
}

func TestParseBestCFXinyitangStyleSource(t *testing.T) {
	resolver := resolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
		switch host {
		case "saas.sin.fan":
			return []net.IPAddr{{IP: net.ParseIP("203.0.113.10")}}, nil
		case "bestcf.030101.xyz":
			return []net.IPAddr{{IP: net.ParseIP("203.0.113.20")}}, nil
		default:
			return nil, errors.New("unexpected host " + host)
		}
	})

	result := Parse(`saas.sin.fan:443#▼ 优选IP | 05-10 22:38 | YouTube@真香定律
103.44.255.30:443#HK | 103.44.255.30:443
103.44.255.88:443#HK | 103.44.255.88:443
bestcf.030101.xyz:443#▲ 优选IP | 分享优选网 BestCF.pages.dev`, Options{Resolver: resolver})

	wantValid := []string{"203.0.113.10", "103.44.255.30", "103.44.255.88", "203.0.113.20"}
	if !reflect.DeepEqual(result.Valid, wantValid) {
		t.Fatalf("valid = %#v, want %#v", result.Valid, wantValid)
	}
	if len(result.Invalid) != 0 {
		t.Fatalf("invalid = %#v, want none", result.Invalid)
	}
}

func TestParseExtractsSourcePorts(t *testing.T) {
	resolver := resolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host != "example.com" {
			return nil, errors.New("unexpected host " + host)
		}
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.80")}}, nil
	})

	result := Parse(strings.Join([]string{
		"1.1.1.1:2053",
		"example.com:8443",
		"[2606:4700::1]:443",
		"1.0.0.1",
	}, "\n"), Options{Resolver: resolver})

	wantValid := []string{"1.1.1.1", "203.0.113.80", "2606:4700::1", "1.0.0.1"}
	if !reflect.DeepEqual(result.Valid, wantValid) {
		t.Fatalf("valid = %#v, want %#v", result.Valid, wantValid)
	}
	wantPorts := map[string]int{
		"1.1.1.1":      2053,
		"203.0.113.80": 8443,
		"2606:4700::1": 443,
	}
	if !reflect.DeepEqual(result.Ports, wantPorts) {
		t.Fatalf("ports = %#v, want %#v", result.Ports, wantPorts)
	}
}

func TestParseCIDRPortWarnsAndFallsBack(t *testing.T) {
	result := Parse("104.16.0.0/16:443", Options{})

	if !reflect.DeepEqual(result.Valid, []string{"104.16.0.0/16"}) {
		t.Fatalf("valid = %#v, want CIDR without port", result.Valid)
	}
	if len(result.Ports) != 0 {
		t.Fatalf("ports = %#v, want no source ports for CIDR", result.Ports)
	}
	if len(result.Warnings) == 0 || !strings.Contains(result.Warnings[0], "CIDR 输入暂不支持携带端口") {
		t.Fatalf("warnings = %#v, want CIDR port fallback warning", result.Warnings)
	}
}

func TestParseHashPortFormatsAndTrailingCommentCompatibility(t *testing.T) {
	resolver := resolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host != "example.com" {
			return nil, errors.New("unexpected host " + host)
		}
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.80")}}, nil
	})

	result := Parse(strings.Join([]string{
		"1.1.1.1#2053",
		"example.com#8443",
		"host:443#备注",
		"104.16.0.0/16#2053",
	}, "\n"), Options{Resolver: resolver})

	wantValid := []string{"1.1.1.1", "203.0.113.80", "104.16.0.0/16"}
	if !reflect.DeepEqual(result.Valid, wantValid) {
		t.Fatalf("valid = %#v, want %#v", result.Valid, wantValid)
	}
	wantPorts := map[string]int{
		"1.1.1.1":      2053,
		"203.0.113.80": 8443,
	}
	if !reflect.DeepEqual(result.Ports, wantPorts) {
		t.Fatalf("ports = %#v, want %#v", result.Ports, wantPorts)
	}
	if len(result.Warnings) == 0 || !strings.Contains(strings.Join(result.Warnings, " "), "CIDR 输入暂不支持携带端口") {
		t.Fatalf("warnings = %#v, want CIDR hash-port fallback warning", result.Warnings)
	}
}

func TestParseRejectsMalformedSourcePorts(t *testing.T) {
	resolver := resolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
		if host != "example.com" {
			return nil, errors.New("unexpected host " + host)
		}
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.80")}}, nil
	})

	result := Parse(strings.Join([]string{
		"1.1.1.1:2053abc",
		"example.com:8443abc",
		"[2606:4700::1]:443abc",
	}, "\n"), Options{Resolver: resolver})

	wantValid := []string{"1.1.1.1", "203.0.113.80", "2606:4700::1"}
	if !reflect.DeepEqual(result.Valid, wantValid) {
		t.Fatalf("valid = %#v, want %#v", result.Valid, wantValid)
	}
	if len(result.Ports) != 0 {
		t.Fatalf("ports = %#v, want malformed ports ignored", result.Ports)
	}
}

func TestParseCountsUnresolvedDomainsAsInvalid(t *testing.T) {
	resolver := resolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
		return nil, errors.New(host + " not found")
	})

	result := Parse("missing.example.com\nexample.123\n例子.com", Options{Resolver: resolver})

	if len(result.Valid) != 0 {
		t.Fatalf("valid = %#v, want empty", result.Valid)
	}
	wantInvalid := []string{"missing.example.com", "example.123", "例子.com"}
	if !reflect.DeepEqual(result.Invalid, wantInvalid) {
		t.Fatalf("invalid = %#v, want %#v", result.Invalid, wantInvalid)
	}
}

func TestParseCountsMalformedIPLikeTokenOnce(t *testing.T) {
	result := Parse("999.999.999.999", Options{
		Resolver: resolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
			t.Fatalf("resolver called for malformed IP-like host %q", host)
			return nil, nil
		}),
	})

	if !reflect.DeepEqual(result.Invalid, []string{"999.999.999.999"}) {
		t.Fatalf("invalid = %#v, want one malformed IP token", result.Invalid)
	}
}

func TestParseDoesNotExtractIPv6SuffixFromText(t *testing.T) {
	var resolved []string
	resolver := resolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
		resolved = append(resolved, host)
		if host != "example.com" {
			return nil, errors.New("unexpected host")
		}
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.20")}}, nil
	})

	result := Parse("note:: example.com", Options{Resolver: resolver})

	if !reflect.DeepEqual(result.Valid, []string{"203.0.113.20"}) {
		t.Fatalf("valid = %#v, want resolved domain IP", result.Valid)
	}
	if !reflect.DeepEqual(resolved, []string{"example.com"}) {
		t.Fatalf("resolved = %#v, want example.com", resolved)
	}
}

func TestParseKeepsValidIPv6Formats(t *testing.T) {
	result := Parse("2001:db8::1\n[2001:db8::2]\n2001:db8::/32", Options{
		Resolver: resolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
			t.Fatalf("resolver called for IPv6 token %q", host)
			return nil, nil
		}),
	})

	wantValid := []string{"2001:db8::1", "2001:db8::2", "2001:db8::/32"}
	if !reflect.DeepEqual(result.Valid, wantValid) {
		t.Fatalf("valid = %#v, want %#v", result.Valid, wantValid)
	}
}

func TestParseStopsResolvingDomainsAtLimit(t *testing.T) {
	var resolved []string
	resolver := resolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
		resolved = append(resolved, host)
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.30")}}, nil
	})

	result := Parse("first.example.com\nsecond.example.com", Options{Limit: 1, Resolver: resolver})

	if !reflect.DeepEqual(result.Valid, []string{"203.0.113.30"}) {
		t.Fatalf("valid = %#v, want first resolved IP only", result.Valid)
	}
	if !reflect.DeepEqual(resolved, []string{"first.example.com"}) {
		t.Fatalf("resolved = %#v, want only first domain", resolved)
	}
}

func TestParseCachesResolvedAndFailedDomains(t *testing.T) {
	calls := map[string]int{}
	resolver := resolverFunc(func(_ context.Context, host string) ([]net.IPAddr, error) {
		calls[host]++
		if host == "missing.example.com" {
			return nil, errors.New("missing")
		}
		return []net.IPAddr{{IP: net.ParseIP("203.0.113.40")}}, nil
	})

	result := Parse("cache.example.com\ncache.example.com\nmissing.example.com\nmissing.example.com", Options{Resolver: resolver})

	if !reflect.DeepEqual(result.Valid, []string{"203.0.113.40", "203.0.113.40"}) {
		t.Fatalf("valid = %#v, want cached successful result reused", result.Valid)
	}
	if !reflect.DeepEqual(result.Invalid, []string{"missing.example.com"}) {
		t.Fatalf("invalid = %#v, want failed domain counted once", result.Invalid)
	}
	if calls["cache.example.com"] != 1 || calls["missing.example.com"] != 1 {
		t.Fatalf("calls = %#v, want one resolver call per domain", calls)
	}
}

func TestNormalizeDomainRejectsInvalidNames(t *testing.T) {
	invalid := []string{
		"localhost",
		"example.123",
		"-bad.example.com",
		"bad-.example.com",
		"bad_name.example.com",
		"例子.com",
	}
	for _, domain := range invalid {
		if got, ok := NormalizeDomain(domain); ok {
			t.Fatalf("NormalizeDomain(%q) = (%q, true), want false", domain, got)
		}
	}

	if got, ok := NormalizeDomain("Example.COM."); !ok || got != "example.com" {
		t.Fatalf("NormalizeDomain = (%q, %v), want example.com true", got, ok)
	}
}
