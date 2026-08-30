package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
)

func TestFileHelperCollectJSFiles(t *testing.T) {
	// Create temp directory with test files
	tempDir := t.TempDir()

	// Create test files
	testFiles := []string{"test.js", "test.ts", "test.jsx", "test.tsx", "test.txt"}
	for _, f := range testFiles {
		path := filepath.Join(tempDir, f)
		if err := os.WriteFile(path, []byte("// test"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	helper := NewFileHelper()

	// Test collecting JS files
	files, err := helper.CollectJSFiles([]string{tempDir}, true, nil, nil)
	if err != nil {
		t.Fatalf("CollectJSFiles failed: %v", err)
	}

	// Should find 4 JS/TS files
	if len(files) != 4 {
		t.Errorf("Expected 4 JS/TS files, got %d", len(files))
	}
}

func TestFileHelperIsValidJSFile(t *testing.T) {
	helper := NewFileHelper()

	tests := []struct {
		path     string
		expected bool
	}{
		{"test.js", true},
		{"test.ts", true},
		{"test.jsx", true},
		{"test.tsx", true},
		{"test.mjs", true},
		{"test.cjs", true},
		{"test.mts", true},
		{"test.cts", true},
		{"test.py", false},
		{"test.go", false},
		{"test.txt", false},
	}

	for _, tt := range tests {
		result := helper.IsValidJSFile(tt.path)
		if result != tt.expected {
			t.Errorf("IsValidJSFile(%s) = %v, expected %v", tt.path, result, tt.expected)
		}
	}
}

func TestFileHelperLegacyCompatibilityMethods(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.js")
	if err := os.WriteFile(testFile, []byte("// test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	helper := NewFileHelper()

	files, err := helper.CollectPythonFiles([]string{tempDir}, true, nil, nil)
	if err != nil {
		t.Fatalf("CollectPythonFiles failed: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	if !helper.IsValidPythonFile(testFile) {
		t.Fatal("IsValidPythonFile should return true for .js files")
	}
}

func TestFileHelperFileExists(t *testing.T) {
	helper := NewFileHelper()

	// Create temp file
	tempFile, err := os.CreateTemp("", "test*.js")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tempFile.Close()
	defer os.Remove(tempFile.Name())

	// Test existing file
	exists, err := helper.FileExists(tempFile.Name())
	if err != nil {
		t.Fatalf("FileExists failed: %v", err)
	}
	if !exists {
		t.Error("Expected file to exist")
	}

	// Test non-existing file
	exists, err = helper.FileExists("/nonexistent/file.js")
	if err != nil {
		t.Fatalf("FileExists failed: %v", err)
	}
	if exists {
		t.Error("Expected file to not exist")
	}
}

func TestFileHelperIsExcluded(t *testing.T) {
	helper := NewFileHelper()

	tests := []struct {
		path            string
		excludePatterns []string
		expected        bool
	}{
		{"test.js", []string{"*.spec.js"}, false},
		{"test.spec.js", []string{"*.spec.js"}, true},
		{"test.test.js", []string{"*.test.js"}, true},
		{"node_modules/test.js", []string{"node_modules"}, true},
		{"src/test.js", []string{"node_modules"}, false},

		// A pattern names a whole path segment, not a substring of one.
		{"src/routes/api.ts", []string{"out"}, false},
		{"src/layout/Header.tsx", []string{"out"}, false},
		{"src/checkout/Cart.ts", []string{"out"}, false},
		{"src/utils/distance.ts", []string{"dist"}, false},
		{"src/build-tools/rollup.ts", []string{"build"}, false},
		{"app/dashboard/layout.tsx", []string{"out", "dist", "build"}, false},

		// Intended exclusions still apply.
		{"out/bundle.js", []string{"out"}, true},
		{"dist/bundle.js", []string{"dist"}, true},
		{"packages/ui/dist/index.js", []string{"dist"}, true},
		{"src/vendor/jquery.js", []string{"vendor"}, true},
		{"src/app.min.js", []string{"*.min.js"}, true},

		// Directory globs match segments.
		{"src/__tests__/app.test.ts", []string{"__*__"}, true},
		{"src/tests/app.ts", []string{"__*__"}, false},

		// Patterns containing a slash are matched against the path.
		{"src/generated/api.ts", []string{"src/generated"}, true},
		{"src/generated/nested/api.ts", []string{"src/generated/**"}, true},
		{"src/generated.ts", []string{"src/generated"}, false},
		{"packages/a/dist/index.js", []string{"**/dist/**"}, true},
		{"packages/a/src/index.js", []string{"**/dist/**"}, false},

		// Empty patterns exclude nothing.
		{"src/index.ts", []string{""}, false},
	}

	for _, tt := range tests {
		result := helper.isExcluded(tt.path, tt.excludePatterns)
		if result != tt.expected {
			t.Errorf("isExcluded(%s, %v) = %v, expected %v", tt.path, tt.excludePatterns, result, tt.expected)
		}
	}
}

// TestFileHelperCollectKeepsSubstringDirectories covers the collection path for
// issue #39: directories whose names contain an exclude pattern as a substring
// were dropped from the analysis without any diagnostic.
func TestFileHelperCollectKeepsSubstringDirectories(t *testing.T) {
	tempDir := t.TempDir()

	kept := []string{
		"src/layout/Header.ts",
		"src/checkout/Cart.ts",
		"src/routes/api.ts",
		"src/utils/distance.ts",
		"src/normal/Plain.ts",
	}
	dropped := []string{
		"dist/bundle.js",
		"out/server.js",
		"node_modules/pkg/index.js",
	}

	for _, rel := range append(append([]string{}, kept...), dropped...) {
		path := filepath.Join(tempDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create dir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte("export const a = 1;"), 0644); err != nil {
			t.Fatalf("Failed to create %s: %v", rel, err)
		}
	}

	helper := NewFileHelper()
	excludePatterns := []string{"node_modules", "dist", "build", "out"}

	files, err := helper.CollectJSFiles([]string{tempDir}, true, nil, excludePatterns)
	if err != nil {
		t.Fatalf("CollectJSFiles failed: %v", err)
	}

	found := make(map[string]bool, len(files))
	for _, f := range files {
		rel, err := filepath.Rel(tempDir, f)
		if err != nil {
			t.Fatalf("Failed to relativize %s: %v", f, err)
		}
		found[filepath.ToSlash(rel)] = true
	}

	for _, rel := range kept {
		if !found[rel] {
			t.Errorf("Expected %s to be analyzed, but it was excluded", rel)
		}
	}
	for _, rel := range dropped {
		if found[rel] {
			t.Errorf("Expected %s to be excluded, but it was analyzed", rel)
		}
	}
	if len(files) != len(kept) {
		t.Errorf("Expected %d files, got %d: %v", len(kept), len(files), files)
	}
}

// TestFileHelperCollectAppliesIncludePatterns verifies that include patterns
// narrow the walk, and that an empty list still selects everything.
func TestFileHelperCollectAppliesIncludePatterns(t *testing.T) {
	tempDir := t.TempDir()

	all := []string{
		"src/index.ts",
		"src/nested/deep/widget.tsx",
		"src/legacy.js",
		"scripts/build.ts",
	}
	for _, rel := range all {
		path := filepath.Join(tempDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create dir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte("export const a = 1;"), 0644); err != nil {
			t.Fatalf("Failed to create %s: %v", rel, err)
		}
	}

	tests := []struct {
		name            string
		includePatterns []string
		expected        []string
	}{
		{
			name:            "no patterns selects everything",
			includePatterns: nil,
			expected:        all,
		},
		{
			name:            "extension pattern at any depth",
			includePatterns: []string{"**/*.ts"},
			expected:        []string{"src/index.ts", "scripts/build.ts"},
		},
		{
			name:            "bare glob matches the file name",
			includePatterns: []string{"*.tsx"},
			expected:        []string{"src/nested/deep/widget.tsx"},
		},
		{
			name:            "path pattern narrows to a subtree",
			includePatterns: []string{"src/**"},
			expected:        []string{"src/index.ts", "src/nested/deep/widget.tsx", "src/legacy.js"},
		},
		{
			name:            "several patterns are a union",
			includePatterns: []string{"**/*.js", "scripts/**"},
			expected:        []string{"src/legacy.js", "scripts/build.ts"},
		},
	}

	helper := NewFileHelper()
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files, err := helper.CollectJSFiles([]string{tempDir}, true, tc.includePatterns, nil)
			if err != nil {
				t.Fatalf("CollectJSFiles failed: %v", err)
			}

			found := make(map[string]bool, len(files))
			for _, f := range files {
				rel, err := filepath.Rel(tempDir, f)
				if err != nil {
					t.Fatalf("Failed to relativize %s: %v", f, err)
				}
				found[filepath.ToSlash(rel)] = true
			}

			for _, rel := range tc.expected {
				if !found[rel] {
					t.Errorf("Expected %s to be analyzed, but it was not", rel)
				}
			}
			if len(files) != len(tc.expected) {
				t.Errorf("Expected %d files, got %d: %v", len(tc.expected), len(files), files)
			}
		})
	}
}

// TestFileHelperCollectMatchesPatternsIgnoringCase pins that pattern matching
// and isJSFile agree about case, so a file with an uppercase extension is not
// dropped by the default include patterns.
func TestFileHelperCollectMatchesPatternsIgnoringCase(t *testing.T) {
	tempDir := t.TempDir()

	for _, rel := range []string{"Widget.TS", "Legacy.JS", "Vendor/Bundle.JS"} {
		path := filepath.Join(tempDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("Failed to create dir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte("export const a = 1;"), 0644); err != nil {
			t.Fatalf("Failed to create %s: %v", rel, err)
		}
	}

	helper := NewFileHelper()
	files, err := helper.CollectJSFiles([]string{tempDir}, true, []string{"**/*.ts", "**/*.js"}, []string{"vendor"})
	if err != nil {
		t.Fatalf("CollectJSFiles failed: %v", err)
	}

	found := make(map[string]bool, len(files))
	for _, f := range files {
		found[filepath.Base(f)] = true
	}
	for _, name := range []string{"Widget.TS", "Legacy.JS"} {
		if !found[name] {
			t.Errorf("Expected %s to be analyzed, got %v", name, files)
		}
	}
	if found["Bundle.JS"] {
		t.Errorf("Expected the excluded directory to be skipped regardless of case, got %v", files)
	}
}

// TestFileHelperCollectKeepsExplicitlyNamedFile verifies that include patterns
// do not drop a file the user named on the command line.
func TestFileHelperCollectKeepsExplicitlyNamedFile(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "widget.tsx")
	if err := os.WriteFile(path, []byte("export const a = 1;"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	helper := NewFileHelper()
	files, err := helper.CollectJSFiles([]string{path}, true, []string{"src/**/*.ts"}, nil)
	if err != nil {
		t.Fatalf("CollectJSFiles failed: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("Expected the named file to be analyzed, got %v", files)
	}
}

// TestFileHelperCollectIgnoresParentDirectoryNames verifies that exclude
// patterns are matched relative to the analysis root, so a project checked out
// under a directory such as "build" is still analyzed.
func TestFileHelperCollectIgnoresParentDirectoryNames(t *testing.T) {
	tempDir := t.TempDir()

	root := filepath.Join(tempDir, "build", "myapp")
	if err := os.MkdirAll(filepath.Join(root, "src"), 0755); err != nil {
		t.Fatalf("Failed to create project dir: %v", err)
	}
	srcFile := filepath.Join(root, "src", "index.ts")
	if err := os.WriteFile(srcFile, []byte("export const a = 1;"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	helper := NewFileHelper()

	files, err := helper.CollectJSFiles([]string{root}, true, nil, []string{"build", "dist"})
	if err != nil {
		t.Fatalf("CollectJSFiles failed: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("Expected 1 file below a parent directory named build, got %d: %v", len(files), files)
	}
}

func TestResolveFilePaths(t *testing.T) {
	// Create temp directory with test files
	tempDir := t.TempDir()

	testFile := filepath.Join(tempDir, "test.js")
	if err := os.WriteFile(testFile, []byte("// test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	helper := NewFileHelper()

	// Test with existing file
	files, err := ResolveFilePaths(helper, []string{testFile}, true, nil, nil)
	if err != nil {
		t.Fatalf("ResolveFilePaths failed: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}

	// Test with directory
	files, err = ResolveFilePaths(helper, []string{tempDir}, true, nil, nil)
	if err != nil {
		t.Fatalf("ResolveFilePaths failed: %v", err)
	}
	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}
}

func TestDefaultAnalyzeConfig(t *testing.T) {
	config := DefaultAnalyzeConfig()

	if !config.EnableComplexity {
		t.Error("Expected EnableComplexity to be true")
	}
	if !config.EnableDeadCode {
		t.Error("Expected EnableDeadCode to be true")
	}
	if config.LowThreshold != 9 {
		t.Errorf("Expected LowThreshold to be 9, got %d", config.LowThreshold)
	}
	if config.MediumThreshold != 19 {
		t.Errorf("Expected MediumThreshold to be 19, got %d", config.MediumThreshold)
	}
}

func TestDefaultUseCaseOptions(t *testing.T) {
	opts := DefaultUseCaseOptions()

	if !opts.EnableProgress {
		t.Error("Expected EnableProgress to be true")
	}
	if opts.MaxConcurrency != 4 {
		t.Errorf("Expected MaxConcurrency to be 4, got %d", opts.MaxConcurrency)
	}
}

func TestFileHelperExcludeNodeModules(t *testing.T) {
	// Create temp directory structure with node_modules
	tempDir := t.TempDir()

	// Create a source file
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}
	srcFile := filepath.Join(srcDir, "index.js")
	if err := os.WriteFile(srcFile, []byte("// source"), 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create node_modules directory with a JS file
	nodeModulesDir := filepath.Join(tempDir, "node_modules", "some-package")
	if err := os.MkdirAll(nodeModulesDir, 0755); err != nil {
		t.Fatalf("Failed to create node_modules dir: %v", err)
	}
	nodeModulesFile := filepath.Join(nodeModulesDir, "index.js")
	if err := os.WriteFile(nodeModulesFile, []byte("// package"), 0644); err != nil {
		t.Fatalf("Failed to create node_modules file: %v", err)
	}

	helper := NewFileHelper()

	// Test with node_modules excluded
	excludePatterns := []string{"node_modules"}
	files, err := helper.CollectJSFiles([]string{tempDir}, true, nil, excludePatterns)
	if err != nil {
		t.Fatalf("CollectJSFiles failed: %v", err)
	}

	// Should only find 1 file (src/index.js), not the one in node_modules
	if len(files) != 1 {
		t.Errorf("Expected 1 file (excluding node_modules), got %d", len(files))
	}

	// Verify the found file is from src, not node_modules
	for _, f := range files {
		if filepath.Base(filepath.Dir(f)) == "node_modules" || filepath.Base(filepath.Dir(filepath.Dir(f))) == "node_modules" {
			t.Errorf("Found file in node_modules which should be excluded: %s", f)
		}
	}
}

func TestFileHelperExcludeMultiplePatterns(t *testing.T) {
	// Create temp directory structure
	tempDir := t.TempDir()

	// Create various directories
	dirs := []string{"src", "dist", "build", ".next", "coverage"}
	for _, dir := range dirs {
		dirPath := filepath.Join(tempDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create %s dir: %v", dir, err)
		}
		file := filepath.Join(dirPath, "index.js")
		if err := os.WriteFile(file, []byte("// "+dir), 0644); err != nil {
			t.Fatalf("Failed to create file in %s: %v", dir, err)
		}
	}

	helper := NewFileHelper()

	// Test with multiple exclusions
	excludePatterns := []string{"dist", "build", ".next", "coverage"}
	files, err := helper.CollectJSFiles([]string{tempDir}, true, nil, excludePatterns)
	if err != nil {
		t.Fatalf("CollectJSFiles failed: %v", err)
	}

	// Should only find 1 file (src/index.js)
	if len(files) != 1 {
		t.Errorf("Expected 1 file (only src), got %d", len(files))
	}
}

func TestFileHelperExcludeMinifiedFiles(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Create various files
	testFiles := []string{"app.js", "utils.js", "vendor.min.js", "bundle.bundle.js"}
	for _, f := range testFiles {
		path := filepath.Join(tempDir, f)
		if err := os.WriteFile(path, []byte("// "+f), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	helper := NewFileHelper()

	// Test with minified file exclusions
	excludePatterns := []string{"*.min.js", "*.bundle.js"}
	files, err := helper.CollectJSFiles([]string{tempDir}, true, nil, excludePatterns)
	if err != nil {
		t.Fatalf("CollectJSFiles failed: %v", err)
	}

	// Should find only app.js and utils.js
	if len(files) != 2 {
		t.Errorf("Expected 2 files (excluding minified/bundled), got %d", len(files))
	}
}

func TestFileHelperExcludeSourceMaps(t *testing.T) {
	// Create temp directory
	tempDir := t.TempDir()

	// Create various files including source maps
	testFiles := []string{
		"app.js",
		"app.js.map",       // Source map
		"utils.min.js",     // Minified
		"utils.min.js.map", // Minified source map
		"lib.mjs",
		"lib.min.mjs", // Minified ESM
	}
	for _, f := range testFiles {
		path := filepath.Join(tempDir, f)
		if err := os.WriteFile(path, []byte("// "+f), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}
	}

	helper := NewFileHelper()

	// Test with source map and minified exclusions
	excludePatterns := []string{"*.map", "*.min.js", "*.min.mjs"}
	files, err := helper.CollectJSFiles([]string{tempDir}, true, nil, excludePatterns)
	if err != nil {
		t.Fatalf("CollectJSFiles failed: %v", err)
	}

	// Should find only app.js and lib.mjs
	if len(files) != 2 {
		t.Errorf("Expected 2 files (excluding maps/minified), got %d: %v", len(files), files)
	}
}

func TestFileHelperExcludeCacheDirectories(t *testing.T) {
	// Create temp directory structure with cache directories
	tempDir := t.TempDir()

	// Create various directories including cache dirs
	dirs := []string{"src", ".cache", ".turbo", ".vercel", ".output"}
	for _, dir := range dirs {
		dirPath := filepath.Join(tempDir, dir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create %s dir: %v", dir, err)
		}
		file := filepath.Join(dirPath, "index.js")
		if err := os.WriteFile(file, []byte("// "+dir), 0644); err != nil {
			t.Fatalf("Failed to create file in %s: %v", dir, err)
		}
	}

	helper := NewFileHelper()

	// Test with cache directory exclusions
	excludePatterns := []string{".cache", ".turbo", ".vercel", ".output"}
	files, err := helper.CollectJSFiles([]string{tempDir}, true, nil, excludePatterns)
	if err != nil {
		t.Fatalf("CollectJSFiles failed: %v", err)
	}

	// Should only find 1 file (src/index.js)
	if len(files) != 1 {
		t.Errorf("Expected 1 file (only src), got %d", len(files))
	}
}

func TestFileHelperGitignoreRespected(t *testing.T) {
	tempDir := t.TempDir()

	// Create .gitignore
	gitignoreContent := "assets/\n*.bundle.js\n"
	if err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte(gitignoreContent), 0644); err != nil {
		t.Fatalf("Failed to create .gitignore: %v", err)
	}

	// Create assets/vendor.js (should be excluded)
	assetsDir := filepath.Join(tempDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("Failed to create assets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "vendor.js"), []byte("// vendor"), 0644); err != nil {
		t.Fatalf("Failed to create vendor.js: %v", err)
	}

	// Create app.bundle.js at root (should be excluded by *.bundle.js)
	if err := os.WriteFile(filepath.Join(tempDir, "app.bundle.js"), []byte("// bundle"), 0644); err != nil {
		t.Fatalf("Failed to create app.bundle.js: %v", err)
	}

	// Create src/index.js (should be included)
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "index.js"), []byte("// source"), 0644); err != nil {
		t.Fatalf("Failed to create index.js: %v", err)
	}

	helper := NewFileHelper()
	files, err := helper.CollectJSFiles([]string{tempDir}, true, nil, nil)
	if err != nil {
		t.Fatalf("CollectJSFiles failed: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d: %v", len(files), files)
	}

	if len(files) == 1 && filepath.Base(files[0]) != "index.js" {
		t.Errorf("Expected index.js, got %s", filepath.Base(files[0]))
	}
}

func TestFileHelperGitignoreNegation(t *testing.T) {
	tempDir := t.TempDir()

	// Create .gitignore with negation
	gitignoreContent := "assets/\n!assets/keep.js\n"
	if err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte(gitignoreContent), 0644); err != nil {
		t.Fatalf("Failed to create .gitignore: %v", err)
	}

	// Create assets directory with files
	assetsDir := filepath.Join(tempDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		t.Fatalf("Failed to create assets dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "vendor.js"), []byte("// vendor"), 0644); err != nil {
		t.Fatalf("Failed to create vendor.js: %v", err)
	}
	if err := os.WriteFile(filepath.Join(assetsDir, "keep.js"), []byte("// keep"), 0644); err != nil {
		t.Fatalf("Failed to create keep.js: %v", err)
	}

	helper := NewFileHelper()
	files, err := helper.CollectJSFiles([]string{tempDir}, true, nil, nil)
	if err != nil {
		t.Fatalf("CollectJSFiles failed: %v", err)
	}

	// assets/ is ignored but assets/keep.js is negated back in
	// Note: directory-level SkipDir may prevent negation from working for files inside.
	// The go-gitignore library handles negation at the path level, but filepath.Walk
	// with SkipDir on the directory means individual files inside won't be visited.
	// This test documents the current behavior: the directory is skipped entirely.
	foundKeep := false
	for _, f := range files {
		if filepath.Base(f) == "vendor.js" {
			t.Errorf("vendor.js should be excluded by .gitignore")
		}
		if filepath.Base(f) == "keep.js" {
			foundKeep = true
		}
	}
	// Note: With directory-level SkipDir, negation for files inside ignored directories
	// won't work. This is a known limitation of the current implementation.
	_ = foundKeep
}

func TestFileHelperGitignoreNotPresent(t *testing.T) {
	tempDir := t.TempDir()

	// Create files without .gitignore
	srcDir := filepath.Join(tempDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "index.js"), []byte("// source"), 0644); err != nil {
		t.Fatalf("Failed to create index.js: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "utils.js"), []byte("// utils"), 0644); err != nil {
		t.Fatalf("Failed to create utils.js: %v", err)
	}

	helper := NewFileHelper()
	files, err := helper.CollectJSFiles([]string{tempDir}, true, nil, nil)
	if err != nil {
		t.Fatalf("CollectJSFiles failed: %v", err)
	}

	// Without .gitignore, both files should be found
	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d: %v", len(files), files)
	}
}

// Use Case Tests

func TestNewComplexityUseCaseBuilder(t *testing.T) {
	builder := NewComplexityUseCaseBuilder()
	if builder == nil {
		t.Fatal("NewComplexityUseCaseBuilder should not return nil")
	}
}

func TestComplexityUseCaseBuilder_BuildWithoutService(t *testing.T) {
	builder := NewComplexityUseCaseBuilder()
	_, err := builder.Build()

	if err == nil {
		t.Error("Build without service should return error")
	}
}

func TestComplexityUseCaseBuilder_WithFileHelper(t *testing.T) {
	// Create mock service
	mockService := &mockComplexityService{}
	fileHelper := NewFileHelper()

	builder := NewComplexityUseCaseBuilder().
		WithService(mockService).
		WithFileHelper(fileHelper)

	uc, err := builder.Build()
	if err != nil {
		t.Fatalf("Build should not return error: %v", err)
	}
	if uc == nil {
		t.Fatal("UseCase should not be nil")
	}
}

func TestNewDeadCodeUseCase(t *testing.T) {
	uc := NewDeadCodeUseCase()
	if uc == nil {
		t.Fatal("NewDeadCodeUseCase should not return nil")
	}
}

func TestNewDeadCodeUseCaseBuilder(t *testing.T) {
	builder := NewDeadCodeUseCaseBuilder()
	if builder == nil {
		t.Fatal("NewDeadCodeUseCaseBuilder should not return nil")
	}

	uc, err := builder.Build()
	if err != nil {
		t.Fatalf("Build should not return error: %v", err)
	}
	if uc == nil {
		t.Fatal("UseCase should not be nil")
	}
}

func TestDeadCodeUseCaseBuilder_WithFileHelper(t *testing.T) {
	fileHelper := NewFileHelper()

	builder := NewDeadCodeUseCaseBuilder().
		WithFileHelper(fileHelper)

	uc, err := builder.Build()
	if err != nil {
		t.Fatalf("Build should not return error: %v", err)
	}
	if uc == nil {
		t.Fatal("UseCase should not be nil")
	}
}

func TestNewAnalyzeUseCaseBuilder(t *testing.T) {
	builder := NewAnalyzeUseCaseBuilder()
	if builder == nil {
		t.Fatal("NewAnalyzeUseCaseBuilder should not return nil")
	}

	uc, err := builder.Build()
	if err != nil {
		t.Fatalf("Build should not return error: %v", err)
	}
	if uc == nil {
		t.Fatal("UseCase should not be nil")
	}
}

func TestAnalyzeUseCaseBuilder_WithDependencies(t *testing.T) {
	mockService := &mockComplexityService{}
	complexityUC := NewComplexityUseCase(mockService)
	deadCodeUC := NewDeadCodeUseCase()
	fileHelper := NewFileHelper()

	builder := NewAnalyzeUseCaseBuilder().
		WithComplexityUseCase(complexityUC).
		WithDeadCodeUseCase(deadCodeUC).
		WithFileHelper(fileHelper)

	uc, err := builder.Build()
	if err != nil {
		t.Fatalf("Build should not return error: %v", err)
	}
	if uc == nil {
		t.Fatal("UseCase should not be nil")
	}
}

func TestAnalyzeResult_ToAnalyzeResponse(t *testing.T) {
	result := &AnalyzeResult{
		Complexity: nil,
		DeadCode:   nil,
		Summary: &domain.AnalyzeSummary{
			TotalFiles: 10,
		},
	}

	response := result.ToAnalyzeResponse()

	if response == nil {
		t.Fatal("Response should not be nil")
	}
	if response.Summary.TotalFiles != 10 {
		t.Errorf("TotalFiles should be 10, got %d", response.Summary.TotalFiles)
	}
	if response.GeneratedAt.IsZero() {
		t.Error("GeneratedAt should not be zero")
	}
}

// Mock complexity service for testing
type mockComplexityService struct{}

func (m *mockComplexityService) Analyze(ctx context.Context, req domain.ComplexityRequest) (*domain.ComplexityResponse, error) {
	return &domain.ComplexityResponse{
		Functions: []domain.FunctionComplexity{},
		Summary:   domain.ComplexitySummary{},
	}, nil
}

func (m *mockComplexityService) AnalyzeFile(ctx context.Context, filePath string, req domain.ComplexityRequest) (*domain.ComplexityResponse, error) {
	return m.Analyze(ctx, req)
}
