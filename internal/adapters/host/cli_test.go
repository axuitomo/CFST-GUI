package host

import "testing"

func TestShouldRefuseRemovedCLI(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{args: nil, want: false},
		{args: []string{"--gui"}, want: false},
		{args: []string{"--cli"}, want: true},
		{args: []string{"-f", "ip.txt"}, want: true},
		{args: []string{"--healthcheck", "--verbose"}, want: true},
	}
	for _, tc := range tests {
		if got := ShouldRefuseRemovedCLI(tc.args); got != tc.want {
			t.Fatalf("ShouldRefuseRemovedCLI(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}
