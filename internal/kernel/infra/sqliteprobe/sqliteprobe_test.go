package sqliteprobe

import (
	"path/filepath"
	"testing"
)

func TestCRUD(t *testing.T) {
	value, err := CRUD(filepath.Join(t.TempDir(), "hot.db"))
	if err != nil {
		t.Fatal(err)
	}
	if value != "ok" {
		t.Fatalf("value = %q", value)
	}
}

func TestConcurrentWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hot.db")
	if err := ConcurrentWrites(path, 200); err != nil {
		t.Fatal(err)
	}
}
