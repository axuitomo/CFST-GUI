package task

import (
	"net"
	"testing"
)

func TestSourceColoFilterAllowAndDenyModes(t *testing.T) {
	ip := &net.IPAddr{IP: net.ParseIP("203.0.113.10")}
	config := DefaultConfig()
	config.SourceColoFilters = SourceColoFilterMap{
		ip.String(): NewSourceColoFilterWithMode("HKG", ColoFilterModeAllow),
	}
	engine := NewEngine(config, Hooks{})
	if !engine.sourceAllowsColo(ip, "HKG") {
		t.Fatal("allow mode should accept listed COLO")
	}
	if engine.sourceAllowsColo(ip, "LAX") {
		t.Fatal("allow mode should reject unlisted COLO")
	}
	if engine.sourceAllowsColo(ip, "") {
		t.Fatal("allow mode should reject empty COLO when list is non-empty")
	}

	config.SourceColoFilters = SourceColoFilterMap{
		ip.String(): NewSourceColoFilterWithMode("HKG", ColoFilterModeDeny),
	}
	engine = NewEngine(config, Hooks{})
	if engine.sourceAllowsColo(ip, "HKG") {
		t.Fatal("deny mode should reject listed COLO")
	}
	if !engine.sourceAllowsColo(ip, "LAX") {
		t.Fatal("deny mode should accept unlisted COLO")
	}
	if !engine.sourceAllowsColo(ip, "") {
		t.Fatal("deny mode should accept empty COLO")
	}
}

func TestConfiguredFinalColoAllowAndDenyModes(t *testing.T) {
	config := DefaultConfig()
	config.HttpingCFColo = "HKG"
	config.HttpingCFColos = []string{"HKG"}
	config.HttpingCFColoMode = ColoFilterModeAllow
	engine := NewEngine(config, Hooks{})
	if _, ok := engine.configuredColoAllowed("HKG"); !ok {
		t.Fatal("allow mode should accept listed final COLO")
	}
	if _, ok := engine.configuredColoAllowed("LAX"); ok {
		t.Fatal("allow mode should reject unlisted final COLO")
	}
	if _, ok := engine.configuredColoAllowed(""); ok {
		t.Fatal("allow mode should reject empty final COLO")
	}

	config.HttpingCFColoMode = ColoFilterModeDeny
	engine = NewEngine(config, Hooks{})
	if _, ok := engine.configuredColoAllowed("HKG"); ok {
		t.Fatal("deny mode should reject listed final COLO")
	}
	if _, ok := engine.configuredColoAllowed("LAX"); !ok {
		t.Fatal("deny mode should accept unlisted final COLO")
	}
	if _, ok := engine.configuredColoAllowed(""); !ok {
		t.Fatal("deny mode should accept empty final COLO")
	}
}

func TestConfiguredFinalColoUsesResolvedCountryColos(t *testing.T) {
	config := DefaultConfig()
	config.HttpingCFColo = "JP"
	config.HttpingCFColos = []string{"NRT", "KIX"}
	config.HttpingCFColoMode = ColoFilterModeAllow
	engine := NewEngine(config, Hooks{})
	if _, ok := engine.configuredColoAllowed("NRT"); !ok {
		t.Fatal("allow mode should accept COLO resolved from country token")
	}
	if _, ok := engine.configuredColoAllowed("LAX"); ok {
		t.Fatal("allow mode should reject COLO outside resolved country set")
	}

	config.HttpingCFColoMode = ColoFilterModeDeny
	engine = NewEngine(config, Hooks{})
	if _, ok := engine.configuredColoAllowed("NRT"); ok {
		t.Fatal("deny mode should reject COLO resolved from country token")
	}
	if _, ok := engine.configuredColoAllowed("LAX"); !ok {
		t.Fatal("deny mode should accept COLO outside resolved country set")
	}
}

func TestSourceColoFilterForResolvedCountryColos(t *testing.T) {
	ip := &net.IPAddr{IP: net.ParseIP("198.51.100.10")}
	config := DefaultConfig()
	config.SourceColoFilters = SourceColoFilterMap{
		ip.String(): NewSourceColoFilterForColos([]string{"NRT", "KIX"}, ColoFilterModeAllow, true),
	}
	engine := NewEngine(config, Hooks{})
	if !engine.sourceAllowsColo(ip, "NRT") {
		t.Fatal("source allow filter should accept COLO resolved from country token")
	}
	if engine.sourceAllowsColo(ip, "LAX") {
		t.Fatal("source allow filter should reject other COLO")
	}

	config.SourceColoFilters = SourceColoFilterMap{
		ip.String(): NewSourceColoFilterForColos(nil, ColoFilterModeAllow, true),
	}
	engine = NewEngine(config, Hooks{})
	if engine.sourceAllowsColo(ip, "NRT") {
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

	addr := &net.IPAddr{IP: net.ParseIP(ip)}
	config := DefaultConfig()
	config.SourceColoFilters = target
	engine := NewEngine(config, Hooks{})
	if !engine.sourceAllowsColo(addr, "NRT") {
		t.Fatal("merged filter should allow COLO from valid source")
	}
	if engine.sourceAllowsColo(addr, "LAX") {
		t.Fatal("merged filter should reject COLO outside valid source")
	}
}
