// Package dnspush implements the provider-agnostic DNS record reconcile loop
// shared by DNS upload backends (Cloudflare, DNSHE, future platforms).
package dnspush

import (
	"context"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"time"
)

const (
	RecordTypeA     = "A"
	RecordTypeAAAA  = "AAAA"
	RecordTypeCNAME = "CNAME"

	OperationList   = "list"
	OperationCreate = "create"
	OperationUpdate = "update"
	OperationDelete = "delete"
)

// Record is the smallest common shape every supported DNS platform exposes.
// Attrs carries platform specific fields that must survive a reconcile (for
// example Cloudflare comment/proxied); backends compare them in EquivalentRecord.
type Record struct {
	Attrs   map[string]string `json:"attrs,omitempty"`
	Content string            `json:"content"`
	ID      string            `json:"id"`
	Name    string            `json:"name"`
	TTL     int               `json:"ttl"`
	Type    string            `json:"type"`
}

type Backend interface {
	ListRecords(ctx context.Context) ([]Record, error)
	CreateRecord(ctx context.Context, record Record) (Record, error)
	UpdateRecord(ctx context.Context, record Record) (Record, error)
	DeleteRecord(ctx context.Context, record Record) error
	EquivalentRecord(existing, desired Record) bool
}

type Summary struct {
	Created int `json:"created"`
	Deleted int `json:"deleted"`
	Ignored int `json:"ignored"`
	Updated int `json:"updated"`
}

type Result struct {
	Summary  Summary
	Warnings []string
}

// PushResult is the outcome shared by zone-scoped provider pushes: what was
// submitted, what the reconcile changed, and the records left on the name.
type PushResult struct {
	HasInputIPs    bool
	IgnoredEntries []string
	RecordsAfter   []Record
	Summary        Summary
	Warnings       []string
}

type IPGroups struct {
	A    []string
	AAAA []string
}

// DesiredRecords expands one managed record name into the A/AAAA records a
// zone-scoped backend should hold for ipsRaw.
func DesiredRecords(name string, ttl int, ipsRaw string) ([]Record, IPGroups, []string) {
	groups, ignored := NormalizeIPs(ipsRaw)
	desired := make([]Record, 0, len(groups.A)+len(groups.AAAA))
	for _, recordType := range []string{RecordTypeA, RecordTypeAAAA} {
		for _, content := range groups.ForType(recordType) {
			desired = append(desired, Record{Content: content, Name: name, TTL: ttl, Type: recordType})
		}
	}
	return desired, groups, ignored
}

// RelativeName turns a full record name into the label a zone-scoped provider
// expects: "@" for the zone apex, the leading label(s) otherwise.
func RelativeName(zone, name string) string {
	zone = normalizeName(zone)
	name = normalizeName(name)
	if zone == "" || name == "" || name == zone {
		return "@"
	}
	if strings.HasSuffix(name, "."+zone) {
		if prefix := strings.TrimSuffix(name, "."+zone); prefix != "" {
			return prefix
		}
	}
	return "@"
}

// AbsoluteName expands a provider-returned name ("@", a label, or a full
// domain) into the absolute record name inside zone. Multi-label relative names
// such as "a.b" must not be mistaken for absolute ones.
func AbsoluteName(zone, name string) string {
	zone = normalizeName(zone)
	normalized := normalizeName(name)
	switch {
	case normalized == "" || normalized == "@":
		return zone
	case zone != "" && strings.HasSuffix(normalized, "."+zone):
		return normalized
	case zone == "":
		return normalized
	default:
		return normalized + "." + zone
	}
}

// MatchZone picks the zone that owns fullName, preferring the longest suffix so
// nested zones win over their parent.
func MatchZone(zones []string, fullName string) (string, bool) {
	fullName = normalizeName(fullName)
	best := ""
	for _, zone := range zones {
		zone = normalizeName(zone)
		if zone == "" {
			continue
		}
		if fullName != zone && !strings.HasSuffix(fullName, "."+zone) {
			continue
		}
		if len(zone) > len(best) {
			best = zone
		}
	}
	return best, best != ""
}

func normalizeName(value string) string {
	return strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
}

type OperationError struct {
	Operation string
	Err       error
}

func (e *OperationError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *OperationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func OperationFromError(err error) string {
	var opErr *OperationError
	if errors.As(err, &opErr) {
		return opErr.Operation
	}
	return ""
}

// Reconcile makes the platform records of the managed name match desired:
// address records are created, updated in place or deleted, and any blocking
// record type (a CNAME on the same name) is replaced.
func Reconcile(ctx context.Context, backend Backend, desired []Record) (Result, error) {
	result := Result{Warnings: []string{}}
	existing, err := backend.ListRecords(ctx)
	if err != nil {
		return result, &OperationError{Operation: OperationList, Err: err}
	}

	if blocking := blockingRecords(existing); len(blocking) > 0 {
		var deleted, created []Record
		rollback := func() {
			rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 20*time.Second)
			defer cancel()
			// A provider can apply a write before its response is lost; re-read actual state.
			if current, err := backend.ListRecords(rollbackCtx); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("failed to confirm DNS state for rollback: %v", err))
			} else {
				created, deleted = nil, nil
				for _, record := range current {
					wasPresent := slices.ContainsFunc(existing, func(old Record) bool { return old.ID == record.ID })
					isDesired := slices.ContainsFunc(desired, func(want Record) bool {
						return want.Type == record.Type && want.Content == record.Content && normalizeName(want.Name) == normalizeName(record.Name)
					})
					if !wasPresent && isDesired {
						created = append(created, record)
					}
				}
				for _, record := range blocking {
					if !slices.ContainsFunc(current, func(now Record) bool {
						return now.Type == record.Type && now.Content == record.Content && normalizeName(now.Name) == normalizeName(record.Name)
					}) {
						deleted = append(deleted, record)
					}
				}
				result.Summary.Created, result.Summary.Deleted = len(created), len(deleted)
			}
			// Remove new address records before restoring CNAMEs, which cannot coexist.
			for _, record := range created {
				if err := backend.DeleteRecord(rollbackCtx, record); err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("failed to remove new %s %q: %v", record.Type, record.Content, err))
				} else {
					result.Summary.Created--
				}
			}
			for _, record := range deleted {
				record.ID = ""
				if _, err := backend.CreateRecord(rollbackCtx, record); err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("failed to restore previous %s %q: %v", record.Type, record.Content, err))
				} else {
					result.Summary.Deleted--
				}
			}
		}
		for _, record := range blocking {
			if err := backend.DeleteRecord(ctx, record); err != nil {
				rollback()
				return result, &OperationError{Operation: OperationDelete, Err: err}
			}
			deleted = append(deleted, record)
			result.Summary.Deleted++
		}
		for _, record := range desired {
			added, err := backend.CreateRecord(ctx, record)
			if err != nil {
				rollback()
				return result, &OperationError{Operation: OperationCreate, Err: err}
			}
			created = append(created, added)
			result.Summary.Created++
		}
		return result, nil
	}

	obsolete := make([]Record, 0)
	for _, recordType := range []string{RecordTypeA, RecordTypeAAAA} {
		existingOfType := recordsByType(existing, recordType)
		byContent := make(map[string][]Record, len(existingOfType))
		for _, record := range existingOfType {
			byContent[record.Content] = append(byContent[record.Content], record)
		}
		for _, record := range recordsByType(desired, recordType) {
			matches := byContent[record.Content]
			if len(matches) == 0 {
				if _, err := backend.CreateRecord(ctx, record); err != nil {
					return result, &OperationError{Operation: OperationCreate, Err: err}
				}
				result.Summary.Created++
				continue
			}
			keep := matches[0]
			if !backend.EquivalentRecord(keep, record) {
				if _, err := backend.UpdateRecord(ctx, mergeRecord(keep, record)); err != nil {
					return result, &OperationError{Operation: OperationUpdate, Err: err}
				}
				result.Summary.Updated++
			}
			obsolete = append(obsolete, matches[1:]...)
			delete(byContent, record.Content)
		}
		for _, matches := range byContent {
			obsolete = append(obsolete, matches...)
		}
	}
	for _, record := range obsolete {
		if err := backend.DeleteRecord(ctx, record); err != nil {
			return result, &OperationError{Operation: OperationDelete, Err: err}
		}
		result.Summary.Deleted++
	}
	return result, nil
}

// mergeRecord keeps the platform identity of the existing record while taking
// the desired payload, so backends can address the update request.
func mergeRecord(existing, desired Record) Record {
	merged := desired
	merged.ID = existing.ID
	return merged
}

// blockingRecords returns the records that must be removed before address
// records can exist on the name. Only CNAME conflicts: TXT/MX records coexist
// with A/AAAA and must be left untouched.
func blockingRecords(records []Record) []Record {
	blocking := make([]Record, 0)
	for _, record := range records {
		if strings.EqualFold(strings.TrimSpace(record.Type), RecordTypeCNAME) {
			blocking = append(blocking, record)
		}
	}
	return blocking
}

func recordsByType(records []Record, recordType string) []Record {
	result := make([]Record, 0)
	for _, record := range records {
		if strings.EqualFold(strings.TrimSpace(record.Type), recordType) {
			result = append(result, record)
		}
	}
	return result
}

func NormalizeIPs(raw string) (IPGroups, []string) {
	tokens := strings.FieldsFunc(strings.ReplaceAll(raw, "\r\n", "\n"), func(r rune) bool {
		return r == ',' || r == ';' || r == '\t' || r == ' ' || r == '\n'
	})
	seen := map[string]struct{}{}
	groups := IPGroups{A: make([]string, 0), AAAA: make([]string, 0)}
	ignored := make([]string, 0)
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		ip := net.ParseIP(token)
		if ip == nil {
			ignored = append(ignored, token)
			continue
		}
		normalized := ip.String()
		if _, exists := seen[normalized]; exists {
			ignored = append(ignored, token)
			continue
		}
		seen[normalized] = struct{}{}
		if ip.To4() != nil {
			groups.A = append(groups.A, normalized)
		} else {
			groups.AAAA = append(groups.AAAA, normalized)
		}
	}
	return groups, ignored
}

func (groups IPGroups) HasIPs() bool {
	return len(groups.A) > 0 || len(groups.AAAA) > 0
}

func (groups IPGroups) ForType(recordType string) []string {
	if strings.EqualFold(strings.TrimSpace(recordType), RecordTypeAAAA) {
		return groups.AAAA
	}
	return groups.A
}

func SummaryMap(summary Summary) map[string]any {
	return map[string]any{
		"created": summary.Created,
		"deleted": summary.Deleted,
		"ignored": summary.Ignored,
		"updated": summary.Updated,
	}
}
