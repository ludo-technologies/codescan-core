package analysis

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ludo-technologies/polyscan/polyscan/internal/lang"
)

// skippedDirs are the directories a walk never descends into: version
// control, dependencies and build output are not the code under review.
// A path given on the command line is walked whatever its name.
var skippedDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	"target":       true,
	"build":        true,
	"dist":         true,
	"third_party":  true,
}

// skipDir reports whether a directory met while walking is left out: the
// named build and dependency directories, and every hidden one such as
// .git or .cache.
func skipDir(name string) bool {
	return skippedDirs[name] || strings.HasPrefix(name, ".")
}

// collectFiles returns, sorted and without duplicates, the absolute paths
// of the files of a supported language under paths. A path that is a file
// is taken as given when its language is supported.
func collectFiles(paths []string) ([]string, error) {
	seen := map[string]bool{}
	var files []string
	add := func(path string) {
		if _, ok := lang.ByPath(path); ok && !seen[path] {
			seen[path] = true
			files = append(files, path)
		}
	}
	for _, p := range paths {
		root, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(root)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			add(root)
			continue
		}
		err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if path != root && skipDir(d.Name()) {
					return fs.SkipDir
				}
				return nil
			}
			add(path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}
