package appcore_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func TestSharedCoreDoesNotImportPlatformAdapters(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve dependency boundary test path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	for _, relativeRoot := range []string{filepath.Join("internal", "appcore"), filepath.Join("internal", "task")} {
		root := filepath.Join(repositoryRoot, relativeRoot)
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, spec := range file.Imports {
				assertSharedCoreImportAllowed(t, repositoryRoot, path, spec)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("scan %s: %v", relativeRoot, err)
		}
	}
}

func TestPlatformInvokeDoesNotReimplementSharedCommands(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve dependency boundary test path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	adapterFiles := []string{
		filepath.Join(repositoryRoot, "internal", "app", "invoke.go"),
		filepath.Join(repositoryRoot, "mobileapi", "invoke.go"),
	}
	sharedCommands := []string{
		"probe.pause", "probe.cancel", "probe.resume", "task.get", "task.list",
		"draft.load", "draft.save", "draft.discard",
		"source_profiles.load", "source_profiles.save", "source_profiles.update_current",
		"source_profiles.save_store", "source_profiles.switch", "source_profiles.delete",
	}
	for _, adapterFile := range adapterFiles {
		raw, err := os.ReadFile(adapterFile)
		if err != nil {
			t.Fatal(err)
		}
		for _, command := range sharedCommands {
			if strings.Contains(string(raw), `"`+command+`"`) {
				relative, _ := filepath.Rel(repositoryRoot, adapterFile)
				t.Errorf("platform adapter %s reimplements shared command %q", filepath.ToSlash(relative), command)
			}
		}
	}
}

func assertSharedCoreImportAllowed(t *testing.T, repositoryRoot, filename string, spec *ast.ImportSpec) {
	t.Helper()
	importPath, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		t.Fatalf("decode import in %s: %v", filename, err)
	}
	forbidden := importPath == "github.com/axuitomo/CFST-GUI/internal/app" ||
		strings.HasPrefix(importPath, "github.com/axuitomo/CFST-GUI/internal/app/") ||
		importPath == "github.com/axuitomo/CFST-GUI/mobileapi" ||
		strings.HasPrefix(importPath, "github.com/wailsapp/wails") ||
		strings.HasPrefix(importPath, "golang.org/x/mobile")
	if forbidden {
		relative, relErr := filepath.Rel(repositoryRoot, filename)
		if relErr != nil {
			relative = filename
		}
		t.Errorf("shared core file %s imports platform adapter %q", filepath.ToSlash(relative), importPath)
	}
}
