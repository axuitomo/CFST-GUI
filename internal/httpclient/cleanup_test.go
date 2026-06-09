package httpclient

import (
	"testing"
	"time"
)

func TestCleanupExpiredH3FailureCacheKeepsValidEntries(t *testing.T) {
	ResetH3FailureCacheForTest()
	t.Cleanup(ResetH3FailureCacheForTest)

	h3FailureCache.Lock()
	h3FailureCache.until["https://expired.example"] = time.Now().Add(-time.Minute)
	h3FailureCache.until["https://valid.example"] = time.Now().Add(time.Minute)
	h3FailureCache.Unlock()

	if removed := CleanupExpiredH3FailureCache(); removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	h3FailureCache.Lock()
	_, expiredExists := h3FailureCache.until["https://expired.example"]
	_, validExists := h3FailureCache.until["https://valid.example"]
	h3FailureCache.Unlock()
	if expiredExists {
		t.Fatal("expired entry still exists")
	}
	if !validExists {
		t.Fatal("valid entry was removed")
	}
}
