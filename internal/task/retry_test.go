package task

import (
	"net/http"
	"testing"
	"time"
)

func TestRetryAfterDelayParsesSecondsAndHTTPDate(t *testing.T) {
	now := time.Date(2026, 5, 9, 12, 0, 0, 0, time.UTC)

	if got := retryAfterDelay("5", now); got != 5*time.Second {
		t.Fatalf("seconds Retry-After = %v, want 5s", got)
	}
	if got := retryAfterDelay(now.Add(10*time.Second).Format(http.TimeFormat), now); got != 10*time.Second {
		t.Fatalf("date Retry-After = %v, want 10s", got)
	}
	if got := retryAfterDelay(now.Add(45*time.Second).Format(http.TimeFormat), now); got != maxRetryAfterWait {
		t.Fatalf("capped Retry-After = %v, want %v", got, maxRetryAfterWait)
	}
	if got := retryAfterDelay(now.Add(-time.Second).Format(http.TimeFormat), now); got != 0 {
		t.Fatalf("expired Retry-After = %v, want 0", got)
	}
	if got := retryAfterDelay("invalid", now); got != 0 {
		t.Fatalf("invalid Retry-After = %v, want 0", got)
	}
}
