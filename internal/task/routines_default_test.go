package task

import "testing"

func TestDefaultRoutinesForPlatform(t *testing.T) {
	cases := []struct {
		goos string
		want int
	}{
		{"android", 64},
		{"windows", 200},
		{"linux", 200},
		{"darwin", 200},
	}
	for _, tc := range cases {
		if got := defaultRoutinesForPlatform(tc.goos); got != tc.want {
			t.Fatalf("defaultRoutinesForPlatform(%q) = %d, want %d", tc.goos, got, tc.want)
		}
	}
}
