package dnspush

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
)

type rollbackBackend struct {
	records          []Record
	calls            []string
	failCreate       bool
	failDelete       string
	applyBeforeError bool
	cancel           context.CancelFunc
}

func (b *rollbackBackend) ListRecords(context.Context) ([]Record, error) {
	return slices.Clone(b.records), nil
}
func (b *rollbackBackend) EquivalentRecord(a, c Record) bool { return a.Content == c.Content }
func (b *rollbackBackend) UpdateRecord(_ context.Context, r Record) (Record, error) {
	return r, nil
}
func (b *rollbackBackend) CreateRecord(ctx context.Context, r Record) (Record, error) {
	if err := ctx.Err(); err != nil {
		return Record{}, err
	}
	b.calls = append(b.calls, "create:"+r.Content)
	if b.failCreate && r.Content == "2.2.2.2" {
		if b.cancel != nil {
			b.cancel()
		}
		if !b.applyBeforeError {
			return Record{}, errors.New("fixture create failure")
		}
	}
	for _, existing := range b.records {
		if (existing.Type == RecordTypeCNAME) != (r.Type == RecordTypeCNAME) {
			return Record{}, errors.New("CNAME conflict")
		}
	}
	r.ID = fmt.Sprintf("new-%d", len(b.calls))
	b.records = append(b.records, r)
	if b.failCreate && r.Content == "2.2.2.2" {
		return Record{}, errors.New("fixture response lost after create")
	}
	return r, nil
}
func (b *rollbackBackend) DeleteRecord(ctx context.Context, r Record) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.calls = append(b.calls, "delete:"+r.Content)
	if r.ID == b.failDelete && !b.applyBeforeError {
		return errors.New("fixture delete failure")
	}
	index := slices.IndexFunc(b.records, func(existing Record) bool { return existing.ID == r.ID })
	if index < 0 {
		return errors.New("missing record identity")
	}
	b.records = slices.Delete(b.records, index, index+1)
	if r.ID == b.failDelete {
		return errors.New("fixture response lost after delete")
	}
	return nil
}

func TestReconcileRollsBackCNAMEReplacement(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		t.Run(fmt.Sprint(cancelled), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			backend := &rollbackBackend{records: []Record{{ID: "old", Type: RecordTypeCNAME, Content: "old.example.com"}}, failCreate: true}
			if cancelled {
				backend.cancel = cancel
			}
			result, err := Reconcile(ctx, backend, []Record{{Type: RecordTypeA, Content: "1.1.1.1"}, {Type: RecordTypeA, Content: "2.2.2.2"}})
			if err == nil || OperationFromError(err) != OperationCreate || len(result.Warnings) != 0 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if len(backend.records) != 1 || backend.records[0].Content != "old.example.com" || result.Summary.Created != 0 || result.Summary.Deleted != 0 {
				t.Fatalf("rollback left records=%+v summary=%+v", backend.records, result.Summary)
			}
			want := []string{"delete:old.example.com", "create:1.1.1.1", "create:2.2.2.2", "delete:1.1.1.1", "create:old.example.com"}
			if !slices.Equal(backend.calls, want) {
				t.Fatalf("calls=%v", backend.calls)
			}
		})
	}
}

func TestReconcileRestoresOnlyDeletedCNAMEs(t *testing.T) {
	backend := &rollbackBackend{records: []Record{{ID: "first", Type: RecordTypeCNAME, Content: "one.example.com"}, {ID: "second", Type: RecordTypeCNAME, Content: "two.example.com"}}, failDelete: "second"}
	result, err := Reconcile(context.Background(), backend, []Record{{Type: RecordTypeA, Content: "1.1.1.1"}})
	if err == nil || OperationFromError(err) != OperationDelete || len(result.Warnings) != 0 || len(backend.records) != 2 {
		t.Fatalf("result=%+v records=%+v err=%v", result, backend.records, err)
	}
	if !slices.Equal(backend.calls, []string{"delete:one.example.com", "delete:two.example.com", "create:one.example.com"}) {
		t.Fatalf("calls=%v", backend.calls)
	}
}

func TestReconcileRollsBackLostWriteResponses(t *testing.T) {
	for _, operation := range []string{OperationCreate, OperationDelete} {
		t.Run(operation, func(t *testing.T) {
			backend := &rollbackBackend{
				records:          []Record{{ID: "old", Type: RecordTypeCNAME, Content: "old.example.com"}},
				applyBeforeError: true,
			}
			if operation == OperationCreate {
				backend.failCreate = true
			} else {
				backend.failDelete = "old"
			}
			result, err := Reconcile(context.Background(), backend, []Record{
				{Type: RecordTypeA, Content: "1.1.1.1"}, {Type: RecordTypeA, Content: "2.2.2.2"},
			})
			if err == nil || OperationFromError(err) != operation || len(result.Warnings) != 0 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			if len(backend.records) != 1 || backend.records[0].Content != "old.example.com" {
				t.Fatalf("rollback left records=%+v", backend.records)
			}
		})
	}
}

func TestReconcileReportsRollbackFailure(t *testing.T) {
	backend := &rollbackBackend{
		records:    []Record{{ID: "old", Type: RecordTypeCNAME, Content: "old.example.com"}},
		failCreate: true, failDelete: "new-2",
	}
	result, err := Reconcile(context.Background(), backend, []Record{
		{Type: RecordTypeA, Content: "1.1.1.1"}, {Type: RecordTypeA, Content: "2.2.2.2"},
	})
	if err == nil || len(result.Warnings) != 2 || result.Summary.Created != 1 || result.Summary.Deleted != 1 {
		t.Fatalf("unreported partial rollback: result=%+v err=%v", result, err)
	}
}
