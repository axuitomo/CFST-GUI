package dnspush

import (
	"context"
	"errors"
	"slices"
	"strconv"
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
	// DNSHE lists apex records by their full domain, which equals the zone.
	if got := AbsoluteName("axuitomo.l.cd", "axuitomo.l.cd"); got != "axuitomo.l.cd" {
		t.Errorf("apex full name = %q, want axuitomo.l.cd", got)
	}
	if got := RelativeName("axuitomo.l.cd", "axuitomo.l.cd"); got != "@" {
		t.Errorf("apex relative name = %q, want @", got)
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

type recordingBackend struct {
	creates int
	deletes int
	updates int
	records []Record
}

func (b *recordingBackend) ListRecords(context.Context) ([]Record, error) {
	return append([]Record(nil), b.records...), nil
}

func (b *recordingBackend) CreateRecord(_ context.Context, record Record) (Record, error) {
	b.creates++
	record.ID = "new-" + strconv.Itoa(b.creates)
	b.records = append(b.records, record)
	return record, nil
}

func (b *recordingBackend) UpdateRecord(_ context.Context, record Record) (Record, error) {
	b.updates++
	for index, existing := range b.records {
		if existing.ID == record.ID {
			b.records[index] = record
			return record, nil
		}
	}
	return record, nil
}

func (b *recordingBackend) DeleteRecord(_ context.Context, record Record) error {
	b.deletes++
	for index, existing := range b.records {
		if existing.ID == record.ID {
			b.records = append(b.records[:index], b.records[index+1:]...)
			return nil
		}
	}
	return nil
}

func (b *recordingBackend) EquivalentRecord(existing, desired Record) bool {
	return existing.Content == desired.Content && existing.Type == desired.Type && existing.Name == desired.Name
}

func TestReconcileWithOptionsRefusesCNAMEByDefault(t *testing.T) {
	backend := &recordingBackend{records: []Record{
		{ID: "cname", Type: RecordTypeCNAME, Name: "edge.example.com", Content: "origin.example.net"},
		{ID: "txt", Type: "TXT", Name: "edge.example.com", Content: "keep-me"},
	}}
	desired := []Record{{Type: RecordTypeA, Name: "edge.example.com", Content: "2.2.2.2", TTL: 300}}

	result, err := ReconcileWithOptions(context.Background(), backend, desired, ReconcileOptions{})
	if !errors.Is(err, ErrCNAMEProtected) {
		t.Fatalf("error = %v, want ErrCNAMEProtected", err)
	}
	if result.Summary.Created != 0 || result.Summary.Deleted != 0 || result.Summary.Updated != 0 {
		t.Fatalf("summary = %#v, want no writes", result.Summary)
	}
	if backend.creates != 0 || backend.deletes != 0 || backend.updates != 0 {
		t.Fatalf("backend writes = create %d delete %d update %d, want 0/0/0", backend.creates, backend.deletes, backend.updates)
	}
	if len(backend.records) != 2 || backend.records[0].Type != RecordTypeCNAME {
		t.Fatalf("CNAME was modified: %#v", backend.records)
	}
}

func TestReconcileWithOptionsReplacesCNAMEWhenAllowed(t *testing.T) {
	backend := &recordingBackend{records: []Record{
		{ID: "cname", Type: RecordTypeCNAME, Name: "edge.example.com", Content: "origin.example.net"},
	}}
	desired := []Record{{Type: RecordTypeA, Name: "edge.example.com", Content: "2.2.2.2", TTL: 300}}

	result, err := ReconcileWithOptions(context.Background(), backend, desired, ReconcileOptions{AllowCNAME: true})
	if err != nil {
		t.Fatalf("ReconcileWithOptions returned error: %v", err)
	}
	if result.Summary.Created != 1 || result.Summary.Deleted != 1 {
		t.Fatalf("summary = %#v, want created 1 deleted 1", result.Summary)
	}
	if len(backend.records) != 1 || backend.records[0].Type != RecordTypeA {
		t.Fatalf("records = %#v, want only the A record", backend.records)
	}
}
