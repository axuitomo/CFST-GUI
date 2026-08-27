package task

import "testing"

func TestCurrentRequestProfileDefaultsHostAndSNIToTraceURL(t *testing.T) {
	config := DefaultConfig()
	config.HostHeader = ""
	config.SNI = ""
	config.TraceURL = "https://trace.example.com:8443/cdn-cgi/trace"

	profile := NewEngine(config, Hooks{}).currentRequestProfile()
	if profile.HostHeader != "trace.example.com:8443" {
		t.Fatalf("HostHeader = %q, want trace.example.com:8443", profile.HostHeader)
	}
	if profile.SNI != "trace.example.com" {
		t.Fatalf("SNI = %q, want trace.example.com", profile.SNI)
	}
}

func TestCurrentRequestProfilePreservesCustomHostAndSNI(t *testing.T) {
	config := DefaultConfig()
	config.HostHeader = "host.example.com"
	config.SNI = "sni.example.com"
	config.TraceURL = "https://trace.example.com/cdn-cgi/trace"

	profile := NewEngine(config, Hooks{}).currentRequestProfile()
	if profile.HostHeader != "host.example.com" {
		t.Fatalf("HostHeader = %q, want host.example.com", profile.HostHeader)
	}
	if profile.SNI != "sni.example.com" {
		t.Fatalf("SNI = %q, want sni.example.com", profile.SNI)
	}
}

func TestDownloadRequestProfilePreservesOverridesForSameAuthority(t *testing.T) {
	config := DefaultConfig()
	config.URL = "https://origin.example.com/download.bin"
	config.TraceURL = "https://origin.example.com/cdn-cgi/trace"
	config.HostHeader = "host.example.com"
	config.SNI = "sni.example.com"

	profile := NewEngine(config, Hooks{}).downloadRequestProfile()
	if profile.HostHeader != "host.example.com" {
		t.Fatalf("HostHeader = %q, want host.example.com", profile.HostHeader)
	}
	if profile.SNI != "sni.example.com" {
		t.Fatalf("SNI = %q, want sni.example.com", profile.SNI)
	}
}

func TestDownloadRequestProfileUsesDownloadAuthorityWhenTraceDiffers(t *testing.T) {
	config := DefaultConfig()
	config.URL = "https://download.example.com:8443/download.bin"
	config.TraceURL = "https://trace.example.com/cdn-cgi/trace"
	config.HostHeader = "trace.example.com"
	config.SNI = "trace.example.com"

	profile := NewEngine(config, Hooks{}).downloadRequestProfile()
	if profile.HostHeader != "download.example.com:8443" {
		t.Fatalf("HostHeader = %q, want download.example.com:8443", profile.HostHeader)
	}
	if profile.SNI != "download.example.com" {
		t.Fatalf("SNI = %q, want download.example.com", profile.SNI)
	}
}

func TestDownloadRequestProfileUsesExplicitDownloadOverrides(t *testing.T) {
	config := DefaultConfig()
	config.URL = "https://203.0.113.10/download.bin"
	config.TraceURL = "https://trace.example.com/cdn-cgi/trace"
	config.HostHeader = "trace.example.com"
	config.SNI = "trace.example.com"
	config.DownloadHostHeader = "download.example.com"
	config.DownloadSNI = "download-tls.example.com"

	profile := NewEngine(config, Hooks{}).downloadRequestProfile()
	if profile.HostHeader != "download.example.com" {
		t.Fatalf("HostHeader = %q, want download.example.com", profile.HostHeader)
	}
	if profile.SNI != "download-tls.example.com" {
		t.Fatalf("SNI = %q, want download-tls.example.com", profile.SNI)
	}
}

func TestRequestIdentityNormalizesDefaultPortsAndRejectsSchemeChanges(t *testing.T) {
	if !sameRequestAuthority("https://example.com/download", "https://example.com:443/trace") {
		t.Fatal("expected implicit and explicit HTTPS default ports to share an authority")
	}
	if sameRequestOrigin("http://example.com/download", "https://example.com/download") {
		t.Fatal("expected HTTP to HTTPS redirect to change origin")
	}
}
