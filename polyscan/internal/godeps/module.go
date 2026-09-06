// Package godeps builds the package import graph of a Go tree. A package's
// identity is its import path, which the nearest go.mod's module path and the
// package directory determine exactly, so an import statement resolves without
// the alias heuristics other languages need.
package godeps

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// module is the go.mod that contains a directory.
type module struct {
	// dir is the directory holding go.mod.
	dir string
	// path is the module path declared by its module directive.
	path string
}

// moduleFinder resolves directories to their enclosing module and remembers
// every answer, since a package tree asks about each of its directories and
// most of them share one go.mod.
type moduleFinder struct {
	byDir map[string]*module
}

func newModuleFinder() *moduleFinder {
	return &moduleFinder{byDir: map[string]*module{}}
}

// find returns the module of the nearest go.mod at or above dir, or nil when
// no directory up to the root has one.
func (f *moduleFinder) find(dir string) (*module, error) {
	if m, ok := f.byDir[dir]; ok {
		return m, nil
	}
	var m *module
	path := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(path); err == nil {
		modulePath, err := readModulePath(path)
		if err != nil {
			return nil, err
		}
		m = &module{dir: dir, path: modulePath}
	} else if parent := filepath.Dir(dir); parent != dir {
		m, err = f.find(parent)
		if err != nil {
			return nil, err
		}
	}
	f.byDir[dir] = m
	return m, nil
}

// readModulePath returns the module directive of a go.mod file. Only that
// directive matters here, so the file is not parsed any further.
func readModulePath(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if comment := strings.Index(line, "//"); comment >= 0 {
			line = strings.TrimSpace(line[:comment])
		}
		rest, ok := strings.CutPrefix(line, "module")
		if !ok || (rest != "" && rest[0] != ' ' && rest[0] != '\t') {
			continue
		}
		modulePath := strings.Trim(strings.TrimSpace(rest), "\"`")
		if modulePath == "" {
			return "", fmt.Errorf("%s: module directive has no path", path)
		}
		return modulePath, nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%s: no module directive", path)
}

// importPath is the import path of the package in dir, which must lie inside
// the module. ignored reports a directory the go tool does not build as part
// of the module: vendor, testdata, and any directory whose name starts with a
// dot or an underscore.
func (m *module) importPath(dir string) (path string, ignored bool, err error) {
	rel, err := filepath.Rel(m.dir, dir)
	if err != nil {
		return "", false, err
	}
	if rel == "." {
		return m.path, false, nil
	}
	rel = filepath.ToSlash(rel)
	for _, component := range strings.Split(rel, "/") {
		if component == "vendor" || component == "testdata" ||
			strings.HasPrefix(component, ".") || strings.HasPrefix(component, "_") {
			return "", true, nil
		}
	}
	return m.path + "/" + rel, false, nil
}
