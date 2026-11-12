package appcore

import (
	"net/netip"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/axuitomo/CFST-GUI/internal/colodict"
)

func writeColoDictionaryForTest(t *testing.T) colodict.Paths {
	t.Helper()
	paths := colodict.DefaultPaths(filepath.Join(t.TempDir(), "colo"))
	if err := os.MkdirAll(filepath.Dir(paths.Colo), 0o700); err != nil {
		t.Fatalf("create colo dir: %v", err)
	}
	raw, err := colodict.EncodeColoEntries([]colodict.ColoEntry{
		{Prefix: netip.MustParsePrefix("104.16.0.0/14"), Colo: "SJC", Country: "US", Region: "CA", City: "San Jose"},
		{Prefix: netip.MustParsePrefix("104.20.0.0/14"), Colo: "LAX", Country: "US", Region: "CA", City: "Los Angeles"},
		{Prefix: netip.MustParsePrefix("2400:cb00::/32"), Colo: "NRT", Country: "JP", Region: "", City: "Tokyo"},
	})
	if err != nil {
		t.Fatalf("EncodeColoEntries returned error: %v", err)
	}
	if err := os.WriteFile(paths.Colo, raw, 0o600); err != nil {
		t.Fatalf("write colo file: %v", err)
	}
	return paths
}

func TestResolveConfiguredColos(t *testing.T) {
	paths := writeColoDictionaryForTest(t)

	if got, err := ResolveConfiguredColos(paths, "   ", "测试筛选"); err != nil || got != nil {
		t.Fatalf("empty input = %#v, err = %v", got, err)
	}

	for _, tc := range []struct {
		name string
		raw  string
		want []string
	}{
		{name: "colo tokens are sorted", raw: "sjc,NRT", want: []string{"NRT", "SJC"}},
		{name: "country expands to its colos", raw: "JP", want: []string{"NRT"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveConfiguredColos(paths, tc.raw, "测试筛选")
			if err != nil {
				t.Fatalf("ResolveConfiguredColos(%q) returned error: %v", tc.raw, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("ResolveConfiguredColos(%q) = %#v, want %#v", tc.raw, got, tc.want)
			}
		})
	}

	_, err := ResolveConfiguredColos(paths, "SJC,TOOLONG", "测试筛选")
	if err == nil || !strings.Contains(err.Error(), "未匹配") {
		t.Fatalf("unmatched token error = %v, want 未匹配", err)
	}

	missing := colodict.DefaultPaths(filepath.Join(t.TempDir(), "missing"))
	_, err = ResolveConfiguredColos(missing, "JP", "测试筛选")
	if err == nil || !strings.Contains(err.Error(), "需要先更新/处理 COLO 词典") {
		t.Fatalf("missing dictionary error = %v, want 需要先更新/处理 COLO 词典", err)
	}
}
