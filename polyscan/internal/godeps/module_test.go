package godeps

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadModulePath(t *testing.T) {
	dir := t.TempDir()
	cases := map[string]string{
		"plain":   "module example.com/plain\n\ngo 1.24\n",
		"quoted":  "// leading comment\nmodule \"example.com/quoted\" // trailing\n",
		"indent":  "\n  module\texample.com/indent\n",
		"prefix":  "modules := 1\nmodule example.com/prefix\n",
		"missing": "go 1.24\n",
		"empty":   "module\n",
	}
	want := map[string]string{
		"plain":  "example.com/plain",
		"quoted": "example.com/quoted",
		"indent": "example.com/indent",
		"prefix": "example.com/prefix",
	}
	for name, content := range cases {
		path := filepath.Join(dir, name, "go.mod")
		writeFile(t, path, content)
		got, err := readModulePath(path)
		if expected, ok := want[name]; ok {
			if err != nil || got != expected {
				t.Errorf("%s: got %q, %v; want %q", name, got, err, expected)
			}
		} else if err == nil {
			t.Errorf("%s: got %q, want an error", name, got)
		}
	}
}

func TestModuleFinder(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "outer", "go.mod"), "module example.com/outer\n")
	writeFile(t, filepath.Join(root, "outer", "nested", "go.mod"), "module example.com/nested\n")
	finder := newModuleFinder()

	cases := []struct {
		dir  string
		path string
	}{
		{filepath.Join(root, "outer"), "example.com/outer"},
		{filepath.Join(root, "outer", "a", "b"), "example.com/outer"},
		{filepath.Join(root, "outer", "nested", "c"), "example.com/nested"},
		{filepath.Join(root, "elsewhere"), ""},
	}
	for _, tc := range cases {
		m, err := finder.find(tc.dir)
		if err != nil {
			t.Fatal(err)
		}
		if tc.path == "" {
			if m != nil {
				t.Errorf("%s: found module %q, want none", tc.dir, m.path)
			}
			continue
		}
		if m == nil || m.path != tc.path {
			t.Errorf("%s: got %v, want module %q", tc.dir, m, tc.path)
		}
	}
}

func TestImportPath(t *testing.T) {
	m := &module{dir: filepath.Join("root", "mod"), path: "example.com/mod"}
	cases := []struct {
		dir     string
		path    string
		ignored bool
	}{
		{filepath.Join("root", "mod"), "example.com/mod", false},
		{filepath.Join("root", "mod", "internal", "x"), "example.com/mod/internal/x", false},
		{filepath.Join("root", "mod", "vendor", "example.com", "dep"), "", true},
		{filepath.Join("root", "mod", "x", "testdata"), "", true},
		{filepath.Join("root", "mod", "_tools"), "", true},
		{filepath.Join("root", "mod", ".git", "x"), "", true},
	}
	for _, tc := range cases {
		path, ignored, err := m.importPath(tc.dir)
		if err != nil {
			t.Fatal(err)
		}
		if path != tc.path || ignored != tc.ignored {
			t.Errorf("%s: got %q ignored=%v, want %q ignored=%v", tc.dir, path, ignored, tc.path, tc.ignored)
		}
	}
}
