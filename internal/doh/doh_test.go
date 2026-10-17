package doh_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/axuitomo/CFST-GUI/internal/doh"
	"golang.org/x/net/dns/dnsmessage"
)

// answerServer replies to every DoH query with the answers produced by respond,
// which receives the parsed query.
func answerServer(t *testing.T, respond func(query dnsmessage.Message) dnsmessage.Message) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if got := request.Header.Get("Content-Type"); got != "application/dns-message" {
			t.Errorf("Content-Type = %q", got)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		var parser dnsmessage.Parser
		header, err := parser.Start(body)
		if err != nil {
			t.Errorf("parse query: %v", err)
		}
		questions, err := parser.AllQuestions()
		if err != nil || len(questions) != 1 {
			t.Errorf("questions = %v, %v", questions, err)
		}
		answer := respond(dnsmessage.Message{Header: header, Questions: questions})
		packed, err := answer.Pack()
		if err != nil {
			t.Errorf("pack answer: %v", err)
		}
		writer.Header().Set("Content-Type", "application/dns-message")
		_, _ = writer.Write(packed)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestNewRejectsNonHTTPS(t *testing.T) {
	for _, endpoint := range []string{"http://dns.example/dns-query", "dns.example/dns-query", "https:///dns-query"} {
		if _, err := doh.New(endpoint); err == nil {
			t.Fatalf("New(%q) error = nil, want rejection", endpoint)
		}
	}
	if _, err := doh.New("https://dns.example/dns-query"); err != nil {
		t.Fatalf("New(https endpoint) error = %v", err)
	}
}

func TestLookupIPAddrAnswersAAndAAAA(t *testing.T) {
	server := answerServer(t, func(query dnsmessage.Message) dnsmessage.Message {
		question := query.Questions[0]
		answer := dnsmessage.Message{Header: dnsmessage.Header{ID: query.ID, Response: true}, Questions: query.Questions}
		switch question.Type {
		case dnsmessage.TypeA:
			answer.Answers = []dnsmessage.Resource{{
				Header: dnsmessage.ResourceHeader{Name: question.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 60},
				Body:   &dnsmessage.AResource{A: [4]byte{203, 0, 113, 7}},
			}}
		case dnsmessage.TypeAAAA:
			answer.Answers = []dnsmessage.Resource{{
				Header: dnsmessage.ResourceHeader{Name: question.Name, Type: dnsmessage.TypeAAAA, Class: dnsmessage.ClassINET, TTL: 60},
				Body:   &dnsmessage.AAAAResource{AAAA: [16]byte{0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1}},
			}}
		}
		return answer
	})

	resolver := &doh.Resolver{Endpoint: server.URL, Fallback: failingFallback(t)}
	addrs, err := resolver.LookupIPAddr(context.Background(), "bestcf.pages.dev")
	if err != nil {
		t.Fatalf("LookupIPAddr() error = %v", err)
	}
	if len(addrs) != 2 {
		t.Fatalf("addrs = %v, want one A and one AAAA", addrs)
	}
	if addrs[0].IP.To4() == nil || !addrs[0].IP.Equal(net.ParseIP("203.0.113.7")) {
		t.Fatalf("addrs[0] = %v, want IPv4 first", addrs[0].IP)
	}
	if addrs[1].IP.To4() != nil || !addrs[1].IP.Equal(net.ParseIP("2001:db8::1")) {
		t.Fatalf("addrs[1] = %v, want IPv6", addrs[1].IP)
	}
}

func TestLookupIPAddrDoesNotFallBackOnNXDOMAIN(t *testing.T) {
	server := answerServer(t, func(query dnsmessage.Message) dnsmessage.Message {
		return dnsmessage.Message{
			Header:    dnsmessage.Header{ID: query.ID, Response: true, RCode: dnsmessage.RCodeNameError},
			Questions: query.Questions,
		}
	})
	called := false
	resolver := &doh.Resolver{Endpoint: server.URL, Fallback: func(context.Context, string) ([]net.IPAddr, error) {
		called = true
		return []net.IPAddr{{IP: net.ParseIP("198.51.100.9")}}, nil
	}}
	_, err := resolver.LookupIPAddr(context.Background(), "missing.example")
	if err == nil {
		t.Fatal("LookupIPAddr() error = nil, want NXDOMAIN")
	}
	if called {
		t.Fatal("fallback was called for a valid NXDOMAIN answer")
	}
}

func TestLookupIPAddrFallsBackWhenEndpointFails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "busy", http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	resolver := &doh.Resolver{Endpoint: server.URL, Fallback: func(context.Context, string) ([]net.IPAddr, error) {
		return []net.IPAddr{{IP: net.ParseIP("198.51.100.9")}}, nil
	}}
	addrs, err := resolver.LookupIPAddr(context.Background(), "fallback.example")
	if err != nil {
		t.Fatalf("LookupIPAddr() error = %v", err)
	}
	if len(addrs) != 1 || !addrs[0].IP.Equal(net.ParseIP("198.51.100.9")) {
		t.Fatalf("addrs = %v, want the fallback address", addrs)
	}
}

func TestLookupIPAddrHonorsTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		select {
		case <-request.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	t.Cleanup(server.Close)

	resolver := &doh.Resolver{Endpoint: server.URL, Timeout: 50 * time.Millisecond, Fallback: func(context.Context, string) ([]net.IPAddr, error) {
		return nil, errors.New("fallback unavailable")
	}}
	start := time.Now()
	if _, err := resolver.LookupIPAddr(context.Background(), "slow.example"); err == nil {
		t.Fatal("LookupIPAddr() error = nil, want timeout")
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("lookup took %s, want the 50ms budget", elapsed)
	}
}

func TestLookupIPAddrKeepsLiteralIPs(t *testing.T) {
	resolver := &doh.Resolver{Endpoint: "https://unused.example/dns-query", Fallback: failingFallback(t)}
	addrs, err := resolver.LookupIPAddr(context.Background(), "203.0.113.7")
	if err != nil || len(addrs) != 1 || !addrs[0].IP.Equal(net.ParseIP("203.0.113.7")) {
		t.Fatalf("addrs = %v, %v, want the literal IP without a query", addrs, err)
	}
}

func failingFallback(t *testing.T) func(context.Context, string) ([]net.IPAddr, error) {
	t.Helper()
	return func(context.Context, string) ([]net.IPAddr, error) {
		t.Error("fallback must not be called")
		return nil, &net.DNSError{Err: "unexpected fallback", IsNotFound: true}
	}
}
