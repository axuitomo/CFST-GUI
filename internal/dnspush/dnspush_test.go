package dnspush

import (
	"slices"
	"testing"
)

func TestMatchZonePrefersLongestSuffix(t *testing.T) {
	cases := []struct {
		name     string
		zones    []string
		fullName string
		want     string
		ok       bool
	}{
		{name: "nested zone wins", zones: []string{"example.com", "edge.example.com"}, fullName: "www.edge.example.com", want: "edge.example.com", ok: true},
		{name: "apex belongs to its zone", zones: []string{"example.com", "edge.example.com"}, fullName: "edge.example.com", want: "edge.example.com", ok: true},
		{name: "case and trailing dot", zones: []string{"Example.COM."}, fullName: "API.example.com", want: "example.com", ok: true},
		{name: "no partial label match", zones: []string{"ample.com"}, fullName: "example.com", ok: false},
		{name: "empty list", zones: nil, fullName: "example.com", ok: false},
	}
	for _, testCase := range cases {
		got, ok := MatchZone(testCase.zones, testCase.fullName)
		if got != testCase.want || ok != testCase.ok {
			t.Errorf("%s: MatchZone = (%q, %v), want (%q, %v)", testCase.name, got, ok, testCase.want, testCase.ok)
		}
	}
}

func TestRelativeAndAbsoluteNameRoundTrip(t *testing.T) {
	cases := []struct {
		full     string
		relative string
	}{
		{full: "example.com", relative: "@"},
		{full: "api.example.com", relative: "api"},
		{full: "a.b.example.com", relative: "a.b"},
	}
	for _, testCase := range cases {
		if got := RelativeName("example.com", testCase.full); got != testCase.relative {
			t.Errorf("RelativeName(%q) = %q, want %q", testCase.full, got, testCase.relative)
		}
		if got := AbsoluteName("example.com", testCase.relative); got != testCase.full {
			t.Errorf("AbsoluteName(%q) = %q, want %q", testCase.relative, got, testCase.full)
		}
	}
	if got := AbsoluteName("edge.example.com", "www.edge.example.com"); got != "www.edge.example.com" {
		t.Errorf("already absolute name = %q", got)
	}
}

func TestDesiredRecordsSkipsDuplicatesAndInvalidInput(t *testing.T) {
	desired, groups, ignored := DesiredRecords("api.example.com", 600, "2.2.2.2\n2.2.2.2\n2001:db8::1\nbad")
	if !groups.HasIPs() {
		t.Fatal("groups report no IPs")
	}
	contents := make([]string, 0, len(desired))
	for _, record := range desired {
		contents = append(contents, record.Type+"/"+record.Content)
	}
	slices.Sort(contents)
	want := []string{"A/2.2.2.2", "AAAA/2001:db8::1"}
	if !slices.Equal(contents, want) {
		t.Fatalf("desired records = %v, want %v", contents, want)
	}
	if !slices.Equal(ignored, []string{"2.2.2.2", "bad"}) {
		t.Fatalf("ignored = %v", ignored)
	}
	for _, record := range desired {
		if record.Name != "api.example.com" || record.TTL != 600 {
			t.Fatalf("record = %#v", record)
		}
	}
}
