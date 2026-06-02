package utils

import (
	"math"
	"time"
)

const metricDecimalFactor = 100

// RoundMetricToTwoDecimals rounds result metrics to two decimal places.
func RoundMetricToTwoDecimals(value float64) float64 {
	return math.Round(value*metricDecimalFactor) / metricDecimalFactor
}

// DurationMilliseconds converts a duration to milliseconds rounded to 0.01ms.
func DurationMilliseconds(duration time.Duration) float64 {
	return RoundMetricToTwoDecimals(float64(duration) / float64(time.Millisecond))
}

// DownloadSpeedMBPerSecond converts bytes/s to MB/s rounded to 0.01 MB/s.
func DownloadSpeedMBPerSecond(bytesPerSecond float64) float64 {
	return RoundMetricToTwoDecimals(bytesPerSecond / 1024 / 1024)
}
