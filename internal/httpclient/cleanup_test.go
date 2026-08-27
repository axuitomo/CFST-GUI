package httpclient

import (
	"testing"
	"time"
)

func TestCleanupExpiredH3FailureCacheKeepsValidEntries(t *testing.T) {
	cache := h3FailureCache{until: map[string]time.Time{
		"https://expired.example": time.Now().Add(-time.Minute),
		"https://valid.example":   time.Now().Add(time.Minute),
	}}
	transport := &fallbackRoundTripper{cache: cache}

	if removed := transport.cleanupExpiredH3FailureCache(time.Now()); removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	if _, exists := transport.cache.until["https://expired.example"]; exists {
		t.Fatal("expired entry still exists")
	}
	if _, exists := transport.cache.until["https://valid.example"]; !exists {
		t.Fatal("valid entry was removed")
	}
}
