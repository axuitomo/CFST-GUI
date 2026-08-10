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
