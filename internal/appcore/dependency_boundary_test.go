package appcore_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const repositoryModule = "github.com/axuitomo/CFST-GUI"

func TestSharedInternalPackagesDoNotImportPlatformAdapters(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve dependency boundary test path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	internalRoot := filepath.Join(repositoryRoot, "internal")
	excludedRoots := map[string]struct{}{
		filepath.Join(internalRoot, "app"):          {},
		filepath.Join(internalRoot, "contracttest"): {},
	}
	err := filepath.WalkDir(internalRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if _, excluded := excludedRoots[path]; excluded {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
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
		t.Fatalf("scan shared internal packages: %v", err)
	}
}

func TestSharedCorePackagesDoNotTransitivelyImportPlatform(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve dependency boundary test path")
	}
	repositoryRoot := filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain unavailable: %v", err)
	}

	protected, protectedSet := protectedCorePackages(t, repositoryRoot, goBin)

	args := append([]string{"list", "-deps", "-json"}, protected...)
	cmd := exec.Command(goBin, args...)
	cmd.Dir = repositoryRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("resolve transitive imports of shared core packages: %v\nstderr: %s", err, stderr.String())
	}

	var failures []string
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var pkg struct {
			ImportPath string   `json:"ImportPath"`
			Deps       []string `json:"Deps"`
		}
		if err := dec.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("decode go list output: %v", err)
		}
		if !protectedSet[pkg.ImportPath] {
			continue
		}
		for _, dep := range pkg.Deps {
			if isForbiddenPlatformImport(dep) {
				failures = append(failures, fmt.Sprintf("%s transitively imports %s", pkg.ImportPath, dep))
			}
		}
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		for _, failure := range failures {
			t.Error(failure)
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

// protectedCorePackages returns the shared core package set: every internal/...
// package except the platform shells (internal/app and its children) and the
// standalone contract-test support package. New shared core packages are
// protected automatically.
func protectedCorePackages(t *testing.T, repositoryRoot, goBin string) ([]string, map[string]bool) {
	t.Helper()
	cmd := exec.Command(goBin, "list", "./internal/...")
	cmd.Dir = repositoryRoot
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("enumerate internal packages: %v", err)
	}
	var protected []string
	set := make(map[string]bool)
	for _, pkg := range strings.Fields(string(out)) {
		if isPlatformShellPackage(pkg) {
			continue
		}
		protected = append(protected, pkg)
		set[pkg] = true
	}
	if len(protected) == 0 {
		t.Fatal("no protected shared core packages found")
	}
	return protected, set
}

func isPlatformShellPackage(importPath string) bool {
	return importPath == repositoryModule+"/internal/app" ||
		strings.HasPrefix(importPath, repositoryModule+"/internal/app/") ||
		importPath == repositoryModule+"/internal/contracttest"
}

func isForbiddenPlatformImport(importPath string) bool {
	return isPlatformShellPackage(importPath) ||
		importPath == repositoryModule+"/mobileapi" ||
		strings.HasPrefix(importPath, "github.com/wailsapp/wails") ||
		strings.HasPrefix(importPath, "golang.org/x/mobile")
}

func assertSharedCoreImportAllowed(t *testing.T, repositoryRoot, filename string, spec *ast.ImportSpec) {
	t.Helper()
	importPath, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		t.Fatalf("decode import in %s: %v", filename, err)
	}
	if isForbiddenPlatformImport(importPath) {
		relative, relErr := filepath.Rel(repositoryRoot, filename)
		if relErr != nil {
			relative = filename
		}
		t.Errorf("shared core file %s imports platform adapter %q", filepath.ToSlash(relative), importPath)
	}
}
