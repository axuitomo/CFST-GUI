package task

import (
	"net"
	"testing"
)

func TestSourceColoFilterAllowAndDenyModes(t *testing.T) {
	oldFilters := SourceColoFilters
	t.Cleanup(func() { SourceColoFilters = oldFilters })

	ip := &net.IPAddr{IP: net.ParseIP("203.0.113.10")}
	SourceColoFilters = SourceColoFilterMap{
		ip.String(): NewSourceColoFilterWithMode("HKG", ColoFilterModeAllow),
	}
	if !sourceAllowsColo(ip, "HKG") {
		t.Fatal("allow mode should accept listed COLO")
	}
	if sourceAllowsColo(ip, "LAX") {
		t.Fatal("allow mode should reject unlisted COLO")
	}
	if sourceAllowsColo(ip, "") {
		t.Fatal("allow mode should reject empty COLO when list is non-empty")
	}

	SourceColoFilters = SourceColoFilterMap{
		ip.String(): NewSourceColoFilterWithMode("HKG", ColoFilterModeDeny),
	}
	if sourceAllowsColo(ip, "HKG") {
		t.Fatal("deny mode should reject listed COLO")
	}
	if !sourceAllowsColo(ip, "LAX") {
		t.Fatal("deny mode should accept unlisted COLO")
	}
	if !sourceAllowsColo(ip, "") {
		t.Fatal("deny mode should accept empty COLO")
	}
}

func TestConfiguredFinalColoAllowAndDenyModes(t *testing.T) {
	oldRaw := HttpingCFColo
	oldMode := HttpingCFColoMode
	oldMap := HttpingCFColomap
	t.Cleanup(func() {
		HttpingCFColo = oldRaw
		HttpingCFColoMode = oldMode
		HttpingCFColomap = oldMap
	})

	HttpingCFColo = "HKG"
	HttpingCFColomap = MapColoMap()
	HttpingCFColoMode = ColoFilterModeAllow
	if _, ok := configuredColoAllowed("HKG"); !ok {
		t.Fatal("allow mode should accept listed final COLO")
	}
	if _, ok := configuredColoAllowed("LAX"); ok {
		t.Fatal("allow mode should reject unlisted final COLO")
	}
	if _, ok := configuredColoAllowed(""); ok {
		t.Fatal("allow mode should reject empty final COLO")
	}

	HttpingCFColoMode = ColoFilterModeDeny
	if _, ok := configuredColoAllowed("HKG"); ok {
		t.Fatal("deny mode should reject listed final COLO")
	}
	if _, ok := configuredColoAllowed("LAX"); !ok {
		t.Fatal("deny mode should accept unlisted final COLO")
	}
	if _, ok := configuredColoAllowed(""); !ok {
		t.Fatal("deny mode should accept empty final COLO")
	}
}

func TestConfiguredFinalColoUsesResolvedCountryColos(t *testing.T) {
	oldRaw := HttpingCFColo
	oldMode := HttpingCFColoMode
	oldMap := HttpingCFColomap
	t.Cleanup(func() {
		HttpingCFColo = oldRaw
		HttpingCFColoMode = oldMode
		HttpingCFColomap = oldMap
	})

	HttpingCFColo = "JP"
	HttpingCFColomap = MapColoSet([]string{"NRT", "KIX"})
	HttpingCFColoMode = ColoFilterModeAllow
	if _, ok := configuredColoAllowed("NRT"); !ok {
		t.Fatal("allow mode should accept COLO resolved from country token")
	}
	if _, ok := configuredColoAllowed("LAX"); ok {
		t.Fatal("allow mode should reject COLO outside resolved country set")
	}

	HttpingCFColoMode = ColoFilterModeDeny
	if _, ok := configuredColoAllowed("NRT"); ok {
		t.Fatal("deny mode should reject COLO resolved from country token")
	}
	if _, ok := configuredColoAllowed("LAX"); !ok {
		t.Fatal("deny mode should accept COLO outside resolved country set")
	}
}

func TestSourceColoFilterForResolvedCountryColos(t *testing.T) {
	oldFilters := SourceColoFilters
	t.Cleanup(func() { SourceColoFilters = oldFilters })

	ip := &net.IPAddr{IP: net.ParseIP("198.51.100.10")}
	SourceColoFilters = SourceColoFilterMap{
		ip.String(): NewSourceColoFilterForColos([]string{"NRT", "KIX"}, ColoFilterModeAllow, true),
	}
	if !sourceAllowsColo(ip, "NRT") {
		t.Fatal("source allow filter should accept COLO resolved from country token")
	}
	if sourceAllowsColo(ip, "LAX") {
		t.Fatal("source allow filter should reject other COLO")
	}

	SourceColoFilters = SourceColoFilterMap{
		ip.String(): NewSourceColoFilterForColos(nil, ColoFilterModeAllow, true),
	}
	if sourceAllowsColo(ip, "NRT") {
		t.Fatal("active allow filter with no resolved COLO should match none")
	}
}

func TestMergeSourceColoFiltersMatchNoneDoesNotOverrideValidSource(t *testing.T) {
	target := SourceColoFilterMap{}
	ip := "198.51.100.10"

	MergeSourceColoFiltersWithResolvedColos(target, []string{ip}, nil, ColoFilterModeAllow, true)
	if filter := target[ip]; !filter.MatchNone {
		t.Fatalf("initial filter = %#v, want match-none placeholder", filter)
	}

	MergeSourceColoFiltersWithResolvedColos(target, []string{ip}, []string{"NRT"}, ColoFilterModeAllow, true)
	filter := target[ip]
	if filter.MatchNone || filter.Unrestricted {
		t.Fatalf("merged filter = %#v, want valid allow filter", filter)
	}
	if _, ok := filter.Allowed["NRT"]; !ok {
		t.Fatalf("merged filter = %#v, missing NRT", filter)
	}

	oldFilters := SourceColoFilters
	t.Cleanup(func() { SourceColoFilters = oldFilters })
	addr := &net.IPAddr{IP: net.ParseIP(ip)}
	SourceColoFilters = target
	if !sourceAllowsColo(addr, "NRT") {
		t.Fatal("merged filter should allow COLO from valid source")
	}
	if sourceAllowsColo(addr, "LAX") {
		t.Fatal("merged filter should reject COLO outside valid source")
	}
}
