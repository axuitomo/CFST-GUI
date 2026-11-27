package utils

import (
	"testing"
	"time"
)

func TestRoundMetricToTwoDecimals(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  float64
	}{
		{name: "round down", value: 12.344, want: 12.34},
		{name: "round up", value: 12.345, want: 12.35},
		{name: "integer", value: 12, want: 12},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RoundMetricToTwoDecimals(tt.value); got != tt.want {
				t.Fatalf("RoundMetricToTwoDecimals(%v) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}

func TestDurationMillisecondsRoundsToTwoDecimals(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     float64
	}{
		{name: "round down", duration: 12*time.Millisecond + 344*time.Microsecond, want: 12.34},
		{name: "round up", duration: 12*time.Millisecond + 345*time.Microsecond, want: 12.35},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DurationMilliseconds(tt.duration); got != tt.want {
				t.Fatalf("DurationMilliseconds(%v) = %v, want %v", tt.duration, got, tt.want)
			}
		})
	}
}

func TestDownloadSpeedMBPerSecondRoundsToTwoDecimals(t *testing.T) {
	tests := []struct {
		name           string
		bytesPerSecond float64
		want           float64
	}{
		{name: "round down", bytesPerSecond: 12.344 * 1024 * 1024, want: 12.34},
		{name: "round up", bytesPerSecond: 12.345 * 1024 * 1024, want: 12.35},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DownloadSpeedMBPerSecond(tt.bytesPerSecond); got != tt.want {
				t.Fatalf("DownloadSpeedMBPerSecond(%v) = %v, want %v", tt.bytesPerSecond, got, tt.want)
			}
		})
	}
}
