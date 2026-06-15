package appcore

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

func TestWriteFileAtomicCreatesAndReplacesFile(t *testing.T) {
	targetPath := filepath.Join(t.TempDir(), "nested", "config.json")
	if err := WriteFileAtomic(targetPath, []byte(`{"value":"first"}`), 0o640); err != nil {
		t.Fatalf("WriteFileAtomic create: %v", err)
	}
	if err := WriteFileAtomic(targetPath, []byte(`{"value":"second"}`), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic replace: %v", err)
	}
	raw, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(raw) != `{"value":"second"}` {
		t.Fatalf("raw = %q, want second payload", string(raw))
	}
	info, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o, want 600", info.Mode().Perm())
	}
}

func TestCaptureAndRestoreFileStates(t *testing.T) {
	dir := t.TempDir()
	existingPath := filepath.Join(dir, "config.json")
	missingPath := filepath.Join(dir, "missing.json")
	if err := os.WriteFile(existingPath, []byte(`{"value":"old"}`), 0o600); err != nil {
		t.Fatalf("write existing file: %v", err)
	}
	states, err := CaptureFileStates(existingPath, missingPath)
	if err != nil {
		t.Fatalf("CaptureFileStates: %v", err)
	}
	if err := WriteFileAtomic(existingPath, []byte(`{"value":"new"}`), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic update existing: %v", err)
	}
	if err := WriteFileAtomic(missingPath, []byte(`{"value":"created"}`), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic create missing: %v", err)
	}
	if err := RestoreFileStates(states); err != nil {
		t.Fatalf("RestoreFileStates: %v", err)
	}
	raw, err := os.ReadFile(existingPath)
	if err != nil {
		t.Fatalf("read restored existing file: %v", err)
	}
	if string(raw) != `{"value":"old"}` {
		t.Fatalf("restored existing raw = %q, want old payload", string(raw))
	}
	if _, err := os.Stat(missingPath); !os.IsNotExist(err) {
		t.Fatalf("missingPath stat err = %v, want not exist", err)
	}
}

func TestCaptureAndRestoreBrokenSymlinkState(t *testing.T) {
	dir := t.TempDir()
	brokenTargetPath := filepath.Join(dir, "real", "missing-config.json")
	if err := os.MkdirAll(filepath.Dir(brokenTargetPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	linkPath := filepath.Join(dir, "broken-link.json")
	if err := os.Symlink(brokenTargetPath, linkPath); err != nil {
		t.Skipf("Symlink unsupported in this environment: %v", err)
	}
	states, err := CaptureFileStates(linkPath)
	if err != nil {
		t.Fatalf("CaptureFileStates broken symlink: %v", err)
	}
	if len(states) != 1 || states[0].SymlinkTarget == "" || states[0].Exists {
		t.Fatalf("broken symlink state = %#v, want symlink without target payload", states)
	}
	if err := WriteFileAtomic(linkPath, []byte(`{"value":"created"}`), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic broken symlink: %v", err)
	}
	if err := RestoreFileStates(states); err != nil {
		t.Fatalf("RestoreFileStates broken symlink: %v", err)
	}
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat broken symlink after restore: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("linkPath is no longer a symlink after restore")
	}
	if _, err := os.Stat(brokenTargetPath); !os.IsNotExist(err) {
		t.Fatalf("broken target stat err = %v, want not exist", err)
	}
}

func TestRestoreFileStatesRecreatesSymlinkAndRestoresTarget(t *testing.T) {
	dir := t.TempDir()
	realPath := filepath.Join(dir, "real", "config.json")
	if err := os.MkdirAll(filepath.Dir(realPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(realPath, []byte(`{"value":"old"}`), 0o600); err != nil {
		t.Fatalf("write real file: %v", err)
	}
	linkPath := filepath.Join(dir, "config-link.json")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Skipf("Symlink unsupported in this environment: %v", err)
	}
	states, err := CaptureFileStates(linkPath)
	if err != nil {
		t.Fatalf("CaptureFileStates symlink: %v", err)
	}
	if err := os.Remove(linkPath); err != nil {
		t.Fatalf("remove original symlink: %v", err)
	}
	if err := os.WriteFile(linkPath, []byte(`{"value":"plain"}`), 0o600); err != nil {
		t.Fatalf("replace symlink path with plain file: %v", err)
	}
	if err := os.WriteFile(realPath, []byte(`{"value":"new"}`), 0o600); err != nil {
		t.Fatalf("mutate real file: %v", err)
	}
	if err := RestoreFileStates(states); err != nil {
		t.Fatalf("RestoreFileStates symlink recreation: %v", err)
	}
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat restored symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("linkPath is not a symlink after restore")
	}
	raw, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("ReadFile real target after restore: %v", err)
	}
	if string(raw) != `{"value":"old"}` {
		t.Fatalf("real target raw after restore = %q, want old payload", string(raw))
	}
}

func TestWriteFileAtomicPreservesSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	realPath := filepath.Join(dir, "real", "config.json")
	if err := os.MkdirAll(filepath.Dir(realPath), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(realPath, []byte(`{"value":"old"}`), 0o600); err != nil {
		t.Fatalf("write real file: %v", err)
	}
	linkPath := filepath.Join(dir, "config-link.json")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Skipf("Symlink unsupported in this environment: %v", err)
	}
	if err := WriteFileAtomic(linkPath, []byte(`{"value":"new"}`), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic symlink target: %v", err)
	}
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat symlink: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("linkPath is no longer a symlink")
	}
	raw, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("ReadFile real target: %v", err)
	}
	if string(raw) != `{"value":"new"}` {
		t.Fatalf("real target raw = %q, want updated payload", string(raw))
	}
}

func TestWriteFileAtomicCreatesBrokenSymlinkTarget(t *testing.T) {
	dir := t.TempDir()
	realPath := filepath.Join(dir, "real", "missing-config.json")
	linkPath := filepath.Join(dir, "broken-link.json")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Skipf("Symlink unsupported in this environment: %v", err)
	}
	if err := WriteFileAtomic(linkPath, []byte(`{"value":"new"}`), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic broken symlink target: %v", err)
	}
	info, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatalf("Lstat broken symlink path: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("linkPath is no longer a symlink")
	}
	raw, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatalf("ReadFile created target: %v", err)
	}
	if string(raw) != `{"value":"new"}` {
		t.Fatalf("created target raw = %q, want updated payload", string(raw))
	}
}

func TestWriteFileAtomicSyncsParentDirectoryOnNonWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses a dedicated commit path")
	}
	targetPath := filepath.Join(t.TempDir(), "nested", "config.json")
	originalHook := syncParentDirAfterCommit
	syncedPath := ""
	syncParentDirAfterCommit = func(path string) error {
		syncedPath = path
		return nil
	}
	t.Cleanup(func() {
		syncParentDirAfterCommit = originalHook
	})
	if err := WriteFileAtomic(targetPath, []byte(`{"value":"synced"}`), 0o600); err != nil {
		t.Fatalf("WriteFileAtomic: %v", err)
	}
	if syncedPath != targetPath {
		t.Fatalf("syncedPath = %q, want %q", syncedPath, targetPath)
	}
}

func TestShouldIgnoreParentDirSyncError(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{err: nil, want: false},
		{err: &os.PathError{Op: "sync", Path: "/tmp", Err: syscall.ENOTSUP}, want: true},
		{err: &os.PathError{Op: "sync", Path: "/tmp", Err: syscall.EOPNOTSUPP}, want: true},
		{err: &os.PathError{Op: "sync", Path: "/tmp", Err: syscall.ENOSYS}, want: true},
		{err: &os.PathError{Op: "sync", Path: "/tmp", Err: syscall.EINVAL}, want: true},
		{err: &os.PathError{Op: "sync", Path: "/tmp", Err: syscall.EIO}, want: false},
		{err: errors.New("plain error"), want: false},
	}
	for _, tc := range cases {
		if got := shouldIgnoreParentDirSyncError(tc.err); got != tc.want {
			t.Fatalf("shouldIgnoreParentDirSyncError(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}
