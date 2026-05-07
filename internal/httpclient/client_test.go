package httpclient

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestFallbackRoundTripperCachesH3Failure(t *testing.T) {
	ResetH3FailureCacheForTest()
	var h3Calls atomic.Int32
	var tcpCalls atomic.Int32
	transport := &fallbackRoundTripper{
		h3: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			h3Calls.Add(1)
			return nil, errors.New("udp blocked")
		}),
		tcp: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			tcpCalls.Add(1)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}

	for i := 0; i < 2; i++ {
		req, err := http.NewRequest(http.MethodGet, "https://example.test/data", nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = res.Body.Close()
	}

	if h3Calls.Load() != 1 {
		t.Fatalf("h3 calls = %d, want cached after first failure", h3Calls.Load())
	}
	if tcpCalls.Load() != 2 {
		t.Fatalf("tcp calls = %d, want fallback for both requests", tcpCalls.Load())
	}
}

func TestFallbackRoundTripperSkipsH3ForUnsafeRequest(t *testing.T) {
	ResetH3FailureCacheForTest()
	var h3Calls atomic.Int32
	var tcpCalls atomic.Int32
	transport := &fallbackRoundTripper{
		h3: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			h3Calls.Add(1)
			return nil, errors.New("should not be called")
		}),
		tcp: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			tcpCalls.Add(1)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("ok")),
				Header:     make(http.Header),
				Request:    req,
			}, nil
		}),
	}
	req, err := http.NewRequest(http.MethodPost, "https://api.example.test/write", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	res, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = res.Body.Close()
	if h3Calls.Load() != 0 || tcpCalls.Load() != 1 {
		t.Fatalf("calls = h3 %d tcp %d, want only tcp", h3Calls.Load(), tcpCalls.Load())
	}
}

func TestApplyNoCache(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://example.test/file", nil)
	if err != nil {
		t.Fatal(err)
	}
	ApplyNoCache(req)
	if req.Header.Get("Cache-Control") != "no-store" || req.Header.Get("Pragma") != "no-cache" {
		t.Fatalf("headers = %#v, want no-store/no-cache", req.Header)
	}
}
