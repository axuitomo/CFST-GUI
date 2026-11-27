package colodict

import (
	"net/netip"
	"testing"
)

func TestColoLookupMatchesLinearScan(t *testing.T) {
	entries := []ColoEntry{
		{Prefix: netip.MustParsePrefix("1.0.0.0/8"), Colo: "wide"},
		{Prefix: netip.MustParsePrefix("1.2.3.0/24"), Colo: "narrow"},
		{Prefix: netip.MustParsePrefix("104.16.0.0/13"), Colo: "cloudflare"},
		{Prefix: netip.MustParsePrefix("2400:cb00::/32"), Colo: "v6colo"},
		{Prefix: netip.MustParsePrefix("2000::/3"), Colo: "v6wide"},
		{Prefix: netip.MustParsePrefix("0.0.0.0/0"), Colo: "all4"},
	}
	lookup := NewColoLookup(entries)
	for _, ip := range []string{
		"1.2.3.4",
		"1.9.9.9",
		"104.16.1.1",
		"8.8.8.8",
		"2400:cb00::1",
		"2a00::1",
		"not-an-ip",
		"",
	} {
		if got, want := lookup.Lookup(ip), LookupColo(entries, ip); got != want {
			t.Fatalf("Lookup(%q) = %q, want %q from the linear scan", ip, got, want)
		}
	}
}

func TestColoLookupKeepsDictionaryOrder(t *testing.T) {
	narrowFirst := []ColoEntry{
		{Prefix: netip.MustParsePrefix("1.2.3.0/24"), Colo: "narrow"},
		{Prefix: netip.MustParsePrefix("1.0.0.0/8"), Colo: "wide"},
	}
	wideFirst := []ColoEntry{
		{Prefix: netip.MustParsePrefix("1.0.0.0/8"), Colo: "wide"},
		{Prefix: netip.MustParsePrefix("1.2.3.0/24"), Colo: "narrow"},
	}
	if got := NewColoLookup(narrowFirst).Lookup("1.2.3.4"); got != "narrow" {
		t.Fatalf("narrow-first Lookup = %q, want narrow", got)
	}
	if got := NewColoLookup(wideFirst).Lookup("1.2.3.4"); got != "wide" {
		t.Fatalf("wide-first Lookup = %q, want wide", got)
	}
}

func TestColoLookupSeparatesAddressFamilies(t *testing.T) {
	entries := []ColoEntry{
		{Prefix: netip.MustParsePrefix("0.0.0.0/0"), Colo: "v4only"},
	}
	lookup := NewColoLookup(entries)
	if got := lookup.Lookup("2400:cb00::1"); got != "" {
		t.Fatalf("Lookup(v6) = %q, want no v4 match", got)
	}
	if got := lookup.Lookup("104.16.1.1"); got != "v4only" {
		t.Fatalf("Lookup(v4) = %q, want v4only", got)
	}
}
