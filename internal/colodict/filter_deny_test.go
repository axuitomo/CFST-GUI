package colodict

import (
	"math/big"
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func newDenyFilterForTest(t *testing.T, prefixes ...string) *Filter {
	t.Helper()
	entries := make([]ColoEntry, 0, len(prefixes))
	for _, prefix := range prefixes {
		entries = append(entries, mustColoEntryForTest(t, prefix, "DEN", "", "", ""))
	}
	return &Filter{entries: entries, mode: "deny"}
}

func TestDenyFilterTokenSubtractsMatchedPrefixes(t *testing.T) {
	filter := newDenyFilterForTest(t, "104.22.1.0/24", "2400:cb00::/32")
	for _, tc := range []struct {
		name  string
		token string
		want  []string
	}{
		{
			name:  "exact prefix removed",
			token: "104.22.1.0/24",
			want:  nil,
		},
		{
			name:  "token inside removed prefix",
			token: "104.22.1.0/25",
			want:  nil,
		},
		{
			name:  "host inside removed prefix",
			token: "104.22.1.9",
			want:  nil,
		},
		{
			name:  "host outside removed prefix",
			token: "104.22.2.9",
			want:  []string{"104.22.2.9"},
		},
		{
			name:  "wider prefix keeps both families apart",
			token: "2400:cb00::/31",
			want:  []string{"2400:cb01::/32"},
		},
		{
			name:  "prefix in the other family is untouched",
			token: "198.51.100.0/24",
			want:  []string{"198.51.100.0/24"},
		},
		{
			name:  "wider prefix is split around the hole",
			token: "104.22.0.0/16",
			want: []string{
				"104.22.0.0/24",
				"104.22.2.0/23",
				"104.22.4.0/22",
				"104.22.8.0/21",
				"104.22.16.0/20",
				"104.22.32.0/19",
				"104.22.64.0/18",
				"104.22.128.0/17",
			},
		},
		{
			name:  "non address token is rejected",
			token: "SJC",
			want:  nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := filter.FilterToken(tc.token)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("FilterToken(%q) = %#v, want %#v", tc.token, got, tc.want)
			}
		})
	}
}

// 减法必须精确到地址:整个 IPv4 空间减掉一个地址后应恰好剩 2^32-1 个地址。
func TestDenyFilterTokenSplitsFullIPv4RangeWithoutLosingAddresses(t *testing.T) {
	filter := newDenyFilterForTest(t, "8.8.8.8/32")
	got := filter.FilterToken("0.0.0.0/0")
	if len(got) != 32 {
		t.Fatalf("got %d prefixes, want 32: %#v", len(got), got)
	}
	total := new(big.Int)
	denied := netip.MustParseAddr("8.8.8.8")
	for _, raw := range got {
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			t.Fatalf("ParsePrefix(%q): %v", raw, err)
		}
		if prefix.Contains(denied) {
			t.Fatalf("prefix %q still contains the denied address", raw)
		}
		size := new(big.Int).Lsh(big.NewInt(1), uint(32-prefix.Bits()))
		total.Add(total, size)
	}
	want := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 32), big.NewInt(1))
	if total.Cmp(want) != 0 {
		t.Fatalf("total addresses = %s, want %s", total, want)
	}
}

func TestDenyFilterTokenWithoutMatchedEntriesKeepsToken(t *testing.T) {
	filter := &Filter{mode: "deny"}
	if got := filter.FilterToken("203.0.113.7"); !reflect.DeepEqual(got, []string{"203.0.113.7"}) {
		t.Fatalf("FilterToken = %#v, want the original token", got)
	}
	var nilFilter *Filter
	if got := nilFilter.FilterToken("203.0.113.7"); !reflect.DeepEqual(got, []string{"203.0.113.7"}) {
		t.Fatalf("nil FilterToken = %#v, want the original token", got)
	}
}

func TestNewModeFilterForColosDenyMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ColoFileName)
	raw, err := EncodeColoEntries([]ColoEntry{
		mustColoEntryForTest(t, "104.16.0.0/14", "SJC", "US", "CA", "San Jose"),
		mustColoEntryForTest(t, "104.20.0.0/14", "LAX", "US", "CA", "Los Angeles"),
	})
	if err != nil {
		t.Fatalf("EncodeColoEntries returned error: %v", err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatalf("write colo file: %v", err)
	}

	filter, err := NewModeFilterForColos(path, map[string]struct{}{"SJC": {}}, "SJC", "deny")
	if err != nil {
		t.Fatalf("deny filter returned error: %v", err)
	}
	if filter == nil {
		t.Fatal("deny filter = nil, want a filter")
	}
	if got := filter.FilterToken("104.16.1.1"); got != nil {
		t.Fatalf("denied COLO prefix = %#v, want nil", got)
	}
	if got := filter.FilterToken("104.20.1.1"); !reflect.DeepEqual(got, []string{"104.20.1.1"}) {
		t.Fatalf("unlisted COLO prefix = %#v, want it kept", got)
	}

	// 地区码在词典里没有行时 deny 仍需生效(全部保留),allow 则应报错。
	emptyDeny, err := NewModeFilterForColos(path, map[string]struct{}{"ZZZ": {}}, "ZZZ", "deny")
	if err != nil || emptyDeny == nil {
		t.Fatalf("deny filter without matched colo = %#v, err = %v", emptyDeny, err)
	}
	if got := emptyDeny.FilterToken("104.16.1.1"); !reflect.DeepEqual(got, []string{"104.16.1.1"}) {
		t.Fatalf("unmatched deny FilterToken = %#v, want the original token", got)
	}
	if _, err := NewModeFilterForColos(path, map[string]struct{}{"ZZZ": {}}, "ZZZ", "allow"); err == nil {
		t.Fatal("allow filter without matched colo should fail")
	}
}
