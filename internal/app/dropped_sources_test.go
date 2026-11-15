package app

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempSourceFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestDroppedSourceFiles(t *testing.T) {
	dir := t.TempDir()
	source := writeTempSourceFile(t, dir, "ip.txt", "1.1.1.1\n")
	empty := filepath.Join(dir, "empty.txt")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatalf("write empty: %v", err)
	}

	got := droppedSourceFiles([]string{source, source})
	if len(got) != 1 || got[0] != source {
		t.Fatalf("droppedSourceFiles duplicate = %#v, want [%s]", got, source)
	}

	rejected := [][]string{
		nil,
		{"--cli", source},
		{dir},
		{empty},
		{filepath.Join(dir, "missing.txt")},
	}
	for _, args := range rejected {
		if got := droppedSourceFiles(args); got != nil {
			t.Fatalf("droppedSourceFiles(%#v) = %#v, want nil", args, got)
		}
	}
}

func TestSecondInstanceDroppedSources(t *testing.T) {
	dir := t.TempDir()
	source := writeTempSourceFile(t, dir, "ip.txt", "1.1.1.1\n")

	if got := secondInstanceDroppedSources([]string{"cfst-gui.exe"}); got != nil {
		t.Fatalf("no file args = %#v, want nil", got)
	}
	got := secondInstanceDroppedSources([]string{"cfst-gui.exe", source})
	if len(got) != 1 || got[0] != source {
		t.Fatalf("secondInstanceDroppedSources = %#v, want [%s]", got, source)
	}
}

func TestDroppedSourcesStateConsume(t *testing.T) {
	var state droppedSourcesState
	state.add(nil)
	state.add([]string{"a.txt", "b.txt"})
	state.add([]string{"b.txt", "c.txt"})

	got := state.consume()
	want := []string{"a.txt", "b.txt", "c.txt"}
	if len(got) != len(want) {
		t.Fatalf("consume = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("consume = %#v, want %#v", got, want)
		}
	}
	if got := state.consume(); got == nil || len(got) != 0 {
		t.Fatalf("second consume = %#v, want empty non-nil", got)
	}
}
