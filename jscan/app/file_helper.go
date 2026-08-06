package app

import (
	"os"
	"path/filepath"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

// FileHelper provides file operation utilities
type FileHelper struct{}

// NewFileHelper creates a new FileHelper
func NewFileHelper() *FileHelper {
	return &FileHelper{}
}

// CollectJSFiles collects JavaScript/TypeScript files from paths
func (h *FileHelper) CollectJSFiles(paths []string, recursive bool, includePatterns, excludePatterns []string) ([]string, error) {
	var files []string

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}

		if !info.IsDir() {
			// A file named directly is only matched on its own name: the
			// directories it happens to live under are not part of the request.
			// Include patterns are skipped entirely here, since dropping a file
			// the user named explicitly would be the wrong answer.
			if h.isJSFile(path) && !h.isExcluded(filepath.Base(path), excludePatterns) {
				files = append(files, path)
			}
			continue
		}

		// Directory handling
		if recursive {
			// Load .gitignore from root directory
			gi := loadGitIgnore(path)

			err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}

				// Exclusions apply to the path relative to the analysis root, so
				// directories above the root never exclude the whole tree.
				relPath, relErr := filepath.Rel(path, filePath)
				if relErr != nil {
					relPath = filePath
				}

				if gi != nil && relErr == nil && gi.MatchesPath(relPath) {
					if info.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}

				// Skip excluded directories early
				if info.IsDir() {
					// The walk root itself is never excluded: the user asked for it.
					if relPath == "." {
						return nil
					}
					if h.isExcluded(relPath, excludePatterns) {
						return filepath.SkipDir
					}
					return nil
				}

				if h.isJSFile(filePath) && h.isIncluded(relPath, includePatterns) && !h.isExcluded(relPath, excludePatterns) {
					files = append(files, filePath)
				}

				return nil
			})
		} else {
			entries, err := os.ReadDir(path)
			if err != nil {
				return nil, err
			}

			for _, entry := range entries {
				if !entry.IsDir() {
					filePath := filepath.Join(path, entry.Name())
					if h.isJSFile(filePath) && h.isIncluded(entry.Name(), includePatterns) && !h.isExcluded(entry.Name(), excludePatterns) {
						files = append(files, filePath)
					}
				}
			}
		}

		if err != nil {
			return nil, err
		}
	}

	return files, nil
}

// CollectPythonFiles is a compatibility wrapper for legacy domain.FileReader.
func (h *FileHelper) CollectPythonFiles(paths []string, recursive bool, includePatterns, excludePatterns []string) ([]string, error) {
	return h.CollectJSFiles(paths, recursive, includePatterns, excludePatterns)
}

// IsValidJSFile checks if a file is a valid JavaScript/TypeScript file
func (h *FileHelper) IsValidJSFile(path string) bool {
	return h.isJSFile(path)
}

// IsValidPythonFile is a compatibility wrapper for legacy domain.FileReader.
func (h *FileHelper) IsValidPythonFile(path string) bool {
	return h.IsValidJSFile(path)
}

// FileExists checks if a file exists
func (h *FileHelper) FileExists(path string) (bool, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return !info.IsDir(), nil
}

// ReadFile reads file content
func (h *FileHelper) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// isJSFile checks if a file is JavaScript/TypeScript based on extension
func (h *FileHelper) isJSFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".js" || ext == ".ts" || ext == ".jsx" || ext == ".tsx" ||
		ext == ".mjs" || ext == ".cjs" || ext == ".mts" || ext == ".cts"
}

// isIncluded reports whether a path is selected by the include patterns.
//
// An empty pattern list selects everything, so a caller that does not care
// about include patterns can pass nil. Otherwise a path has to match at least
// one pattern, under the same rules exclude patterns use.
//
// Include patterns narrow the extensions jscan already understands; they cannot
// widen them, because a file it cannot parse is of no use to an analysis.
func (h *FileHelper) isIncluded(path string, includePatterns []string) bool {
	if len(includePatterns) == 0 {
		return true
	}
	return matchesAnyPattern(path, includePatterns)
}

// isExcluded checks if a path matches any exclude pattern.
func (h *FileHelper) isExcluded(path string, excludePatterns []string) bool {
	return matchesAnyPattern(path, excludePatterns)
}

// matchesAnyPattern reports whether a path matches at least one of the patterns.
//
// Patterns without a slash are matched against the file name and against each
// directory segment of the path, so "dist" matches "dist/bundle.js" but not
// "src/utils/distance.ts". Patterns with a slash are matched against the path
// as a whole, with "**" matching any number of segments.
//
// Matching ignores case, on both sides, so that the default include pattern
// "**/*.ts" selects Widget.TS exactly as isJSFile accepts it. Treating the two
// differently would drop such a file with nothing said about it.
//
// Note: filepath.Match errors are ignored throughout (invalid patterns simply
// don't match) so that the remaining valid patterns still apply.
func matchesAnyPattern(path string, patterns []string) bool {
	segments := strings.Split(strings.ToLower(filepath.ToSlash(path)), "/")
	baseName := segments[len(segments)-1]
	dirSegments := segments[:len(segments)-1]

	for _, pattern := range patterns {
		pattern = strings.ToLower(strings.Trim(filepath.ToSlash(pattern), "/"))
		if pattern == "" {
			continue
		}

		if strings.Contains(pattern, "/") {
			if matchesPathPattern(pattern, segments) {
				return true
			}
			continue
		}

		if matched, err := filepath.Match(pattern, baseName); err == nil && matched {
			return true
		}
		for _, segment := range dirSegments {
			if segment == pattern {
				return true
			}
			if matched, err := filepath.Match(pattern, segment); err == nil && matched {
				return true
			}
		}
	}
	return false
}

// matchesPathPattern reports whether a multi-segment pattern such as
// "src/generated/**" matches the given path segments. The pattern may start at
// any segment boundary, so it behaves like a relative path fragment.
func matchesPathPattern(pattern string, segments []string) bool {
	patternSegments := strings.Split(pattern, "/")
	for i := range segments {
		if matchSegments(patternSegments, segments[i:]) {
			return true
		}
	}
	return false
}

// matchSegments matches pattern segments against a leading run of path
// segments, where "**" matches zero or more segments and every other segment is
// a glob. Matching a prefix is enough, so a pattern naming a directory such as
// "src/generated" also matches everything below it.
func matchSegments(pattern, segments []string) bool {
	if len(pattern) == 0 {
		return true
	}
	if pattern[0] == "**" {
		for i := 0; i <= len(segments); i++ {
			if matchSegments(pattern[1:], segments[i:]) {
				return true
			}
		}
		return false
	}
	if len(segments) == 0 {
		return false
	}
	if matched, err := filepath.Match(pattern[0], segments[0]); err != nil || !matched {
		return false
	}
	return matchSegments(pattern[1:], segments[1:])
}

// loadGitIgnore loads a .gitignore file from the root directory.
// Returns nil if the file does not exist or cannot be read.
func loadGitIgnore(root string) *ignore.GitIgnore {
	gitignorePath := filepath.Join(root, ".gitignore")
	gi, err := ignore.CompileIgnoreFile(gitignorePath)
	if err != nil {
		return nil
	}
	return gi
}

// ResolveFilePaths resolves file paths, returning existing files directly
// or collecting files from directories
func ResolveFilePaths(
	fileHelper *FileHelper,
	paths []string,
	recursive bool,
	includePatterns []string,
	excludePatterns []string,
) ([]string, error) {
	// Check if all paths are already files
	allFiles := true
	for _, path := range paths {
		exists, err := fileHelper.FileExists(path)
		if err != nil || !exists {
			allFiles = false
			break
		}
	}

	// If all paths are already files, no need to collect again
	if allFiles {
		return paths, nil
	}

	// Collect files from directories
	return fileHelper.CollectJSFiles(paths, recursive, includePatterns, excludePatterns)
}
