package host

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

	got := DroppedSourceFiles([]string{source, source})
	if len(got) != 1 || got[0] != source {
		t.Fatalf("DroppedSourceFiles duplicate = %#v, want [%s]", got, source)
	}

	rejected := [][]string{
		nil,
		{"--cli", source},
		{dir},
		{empty},
		{filepath.Join(dir, "missing.txt")},
	}
	for _, args := range rejected {
		if got := DroppedSourceFiles(args); got != nil {
			t.Fatalf("DroppedSourceFiles(%#v) = %#v, want nil", args, got)
		}
	}
}

func TestSecondInstanceDroppedSources(t *testing.T) {
	dir := t.TempDir()
	source := writeTempSourceFile(t, dir, "ip.txt", "1.1.1.1\n")

	if got := SecondInstanceDroppedSources([]string{"cfst-gui.exe"}); got != nil {
		t.Fatalf("no file args = %#v, want nil", got)
	}
	got := SecondInstanceDroppedSources([]string{"cfst-gui.exe", source})
	if len(got) != 1 || got[0] != source {
		t.Fatalf("SecondInstanceDroppedSources = %#v, want [%s]", got, source)
	}
}

func TestDroppedSourcesStateConsume(t *testing.T) {
	var state DroppedSourcesState
	state.Add(nil)
	state.Add([]string{"a.txt", "b.txt"})
	state.Add([]string{"b.txt", "c.txt"})

	got := state.Consume()
	want := []string{"a.txt", "b.txt", "c.txt"}
	if len(got) != len(want) {
		t.Fatalf("Consume = %#v, want %#v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("Consume = %#v, want %#v", got, want)
		}
	}
	if got := state.Consume(); got == nil || len(got) != 0 {
		t.Fatalf("second Consume = %#v, want empty non-nil", got)
	}
}
