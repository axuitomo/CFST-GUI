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
