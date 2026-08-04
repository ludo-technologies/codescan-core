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

				if h.isJSFile(filePath) && !h.isExcluded(relPath, excludePatterns) {
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
					if h.isJSFile(filePath) && !h.isExcluded(entry.Name(), excludePatterns) {
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

// isExcluded checks if a path matches any exclude pattern.
//
// Patterns without a slash are matched against the file name and against each
// directory segment of the path, so "dist" excludes "dist/bundle.js" but not
// "src/utils/distance.ts". Patterns with a slash are matched against the path
// as a whole, with "**" matching any number of segments.
//
// Note: filepath.Match errors are ignored throughout (invalid patterns simply
// don't match) so that the remaining valid patterns still apply.
func (h *FileHelper) isExcluded(path string, excludePatterns []string) bool {
	segments := strings.Split(filepath.ToSlash(path), "/")
	baseName := segments[len(segments)-1]
	dirSegments := segments[:len(segments)-1]

	for _, pattern := range excludePatterns {
		pattern = strings.Trim(filepath.ToSlash(pattern), "/")
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
