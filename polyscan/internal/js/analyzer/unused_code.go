package analyzer

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

// suffixIndex provides O(1) alias import path resolution by pre-indexing
// all known files by their path suffixes up to maxSuffixDepth components.
type suffixIndex struct {
	bySuffix map[string][]string // normalized suffix → list of matching absolute file paths
}

const maxSuffixDepth = 5

// buildSuffixIndex constructs the suffix index from the set of known files.
// For a file "/repo/src/utils/math.ts" it inserts entries for:
// "math.ts", "utils/math.ts", "src/utils/math.ts", "repo/src/utils/math.ts"
// (up to maxSuffixDepth components).
func buildSuffixIndex(knownFiles map[string]bool) *suffixIndex {
	idx := &suffixIndex{bySuffix: make(map[string][]string, len(knownFiles))}
	for file := range knownFiles {
		normalized := filepath.ToSlash(file)
		parts := strings.Split(normalized, "/")
		start := len(parts) - maxSuffixDepth
		if start < 0 {
			start = 0
		}
		for i := start; i < len(parts); i++ {
			suffix := strings.Join(parts[i:], "/")
			if suffix == "" {
				continue
			}
			idx.bySuffix[suffix] = append(idx.bySuffix[suffix], file)
		}
	}
	return idx
}

// ImportGraph is the precomputed cross-file import relationship graph.
// Build it once with BuildImportGraph and pass it to the Detect* functions.
type ImportGraph struct {
	// importedNamesFromFile maps resolved file path → set of imported names from that file.
	// A "*" entry means the file is namespace-imported or side-effect imported.
	importedNamesFromFile map[string]map[string]bool
	// reverseEdges maps resolved file path → set of file paths that import it.
	reverseEdges map[string]map[string]bool
	// forwardEdges maps importing file path → list of resolved file paths it imports.
	forwardEdges map[string][]string
}

// BuildImportGraph constructs the ImportGraph in a single pass over allModuleInfos.
// It builds the suffix index once and reuses it for all alias resolutions.
// Returns a non-nil graph even for empty inputs.
func BuildImportGraph(allModuleInfos map[string]*domain.ModuleInfo, analyzedFiles map[string]bool) *ImportGraph {
	graph := &ImportGraph{
		importedNamesFromFile: make(map[string]map[string]bool),
		reverseEdges:          make(map[string]map[string]bool),
		forwardEdges:          make(map[string][]string),
	}
	if len(allModuleInfos) == 0 {
		return graph
	}
	idx := buildSuffixIndex(analyzedFiles)
	for importingFile, info := range allModuleInfos {
		for _, imp := range info.Imports {
			resolvedPaths := resolveImportPaths(importingFile, imp.Source, imp.SourceType, analyzedFiles, idx)
			for _, resolvedPath := range resolvedPaths {
				if graph.importedNamesFromFile[resolvedPath] == nil {
					graph.importedNamesFromFile[resolvedPath] = make(map[string]bool)
				}
				switch imp.ImportType {
				case domain.ImportTypeNamespace:
					graph.importedNamesFromFile[resolvedPath]["*"] = true
				case domain.ImportTypeDefault, domain.ImportTypeNamed:
					for _, spec := range imp.Specifiers {
						name := spec.Imported
						if name == "" {
							name = spec.Local
						}
						graph.importedNamesFromFile[resolvedPath][name] = true
					}
				case domain.ImportTypeSideEffect:
					graph.importedNamesFromFile[resolvedPath]["*"] = true
				}
				if graph.reverseEdges[resolvedPath] == nil {
					graph.reverseEdges[resolvedPath] = make(map[string]bool)
				}
				graph.reverseEdges[resolvedPath][importingFile] = true
			}
			if len(resolvedPaths) > 0 {
				graph.forwardEdges[importingFile] = append(graph.forwardEdges[importingFile], resolvedPaths...)
			}
		}
	}
	return graph
}

// DetectUnusedImports detects imported names that are never referenced in the file.
// It walks the AST to collect all identifier references (excluding import/export declarations)
// and compares them against the locally-bound import names. content is the source
// the AST was parsed from; both must describe the same version of the file.
func DetectUnusedImports(ast *parser.Node, moduleInfo *domain.ModuleInfo, filePath string, content []byte) []*DeadCodeFinding {
	if ast == nil || moduleInfo == nil {
		return nil
	}

	// Collect local names from imports (skip side-effect, type-only, dynamic)
	typeOnlyImportLines := detectTypeOnlyImportLines(content)
	type importEntry struct {
		localName string
		line      int
		source    string
	}
	var importedNames []importEntry

	for _, imp := range moduleInfo.Imports {
		// Skip side-effect imports (import 'polyfill')
		if imp.ImportType == domain.ImportTypeSideEffect {
			continue
		}
		// Skip type-only imports (import type { Foo } from 'bar')
		if imp.IsTypeOnly || imp.ImportType == domain.ImportTypeTypeOnly || typeOnlyImportLines[imp.Location.StartLine] {
			continue
		}
		// Skip dynamic imports (import('foo'))
		if imp.IsDynamic || imp.ImportType == domain.ImportTypeDynamic {
			continue
		}

		for _, spec := range imp.Specifiers {
			// Skip type-only specifiers
			if spec.IsType {
				continue
			}
			if spec.Local != "" {
				importedNames = append(importedNames, importEntry{
					localName: spec.Local,
					line:      imp.Location.StartLine,
					source:    imp.Source,
				})
			}
		}
	}

	if len(importedNames) == 0 {
		return nil
	}

	// Walk the AST and collect all Identifier references,
	// skipping import declaration subtrees (which define the local bindings).
	// Export declarations are NOT skipped because they reference imported names
	// (e.g. `export { foo }` or `export default foo` means foo is used).
	referenced := make(map[string]bool)
	ast.Walk(func(n *parser.Node) bool {
		// Skip import declaration subtrees only
		if n.Type == parser.NodeImportDeclaration {
			return false
		}

		if n.Type == parser.NodeIdentifier && n.Name != "" {
			referenced[n.Name] = true
		}

		// ExportSpecifier nodes reference local names (e.g. `export { foo }`)
		// The Name field holds the local identifier being exported
		if n.Type == parser.NodeExportSpecifier && n.Name != "" {
			referenced[n.Name] = true
		}

		// Also check JSX element tags which reference identifiers
		if n.Type == parser.NodeJSXElement && n.Name != "" {
			referenced[n.Name] = true
		}

		return true
	})

	// Generate findings for unreferenced imports
	var findings []*DeadCodeFinding
	for _, entry := range importedNames {
		if !referenced[entry.localName] {
			findings = append(findings, &DeadCodeFinding{
				FilePath:  filePath,
				StartLine: entry.line,
				EndLine:   entry.line,
				Reason:    ReasonUnusedImport,
				Severity:  SeverityLevelWarning,
				Description: "Imported name '" + entry.localName + "' from '" +
					entry.source + "' is never used",
			})
		}
	}

	return findings
}

// detectTypeOnlyImportLines returns source line numbers where import declarations are
// explicitly type-only (e.g. `import type { Foo } from 'bar'`).
func detectTypeOnlyImportLines(content []byte) map[int]bool {
	lines := make(map[int]bool)
	if len(content) == 0 {
		return lines
	}

	for i, line := range strings.Split(string(content), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "import type ") {
			lines[i+1] = true
		}
	}
	return lines
}

// DetectUnusedExports detects exported names that are not imported by any other analyzed file.
// It uses the precomputed ImportGraph to check each export against the reverse import index.
func DetectUnusedExports(allModuleInfos map[string]*domain.ModuleInfo, graph *ImportGraph) []*DeadCodeFinding {
	if len(allModuleInfos) == 0 {
		return nil
	}

	importedNamesFromFile := graph.importedNamesFromFile

	var findings []*DeadCodeFinding

	for _, filePath := range sortedModulePaths(allModuleInfos) {
		info := allModuleInfos[filePath]
		// Skip entry-point files whose exports are meant to be public
		if isEntryPointFile(filePath) {
			continue
		}
		// Skip test files
		if isTestFile(filePath) {
			continue
		}

		importedNames := importedNamesFromFile[filePath]

		// If namespace import (*) exists, all exports are considered used
		if importedNames != nil && importedNames["*"] {
			continue
		}

		for _, exp := range info.Exports {
			// Skip re-exports (export { x } from './other')
			if exp.Source != "" {
				continue
			}
			// Skip type-only exports
			if exp.IsTypeOnly {
				continue
			}
			// Skip export * (re-export all)
			if exp.ExportType == "all" {
				continue
			}

			// Determine the exported name(s)
			exportedNames := getExportedNames(exp)

			for _, name := range exportedNames {
				if isFrameworkReservedExport(filePath, exp, name) {
					continue
				}
				if importedNames == nil || !importedNames[name] {
					findings = append(findings, &DeadCodeFinding{
						FilePath:    filePath,
						StartLine:   exp.Location.StartLine,
						EndLine:     exp.Location.EndLine,
						Reason:      ReasonUnusedExport,
						Severity:    SeverityLevelInfo,
						Description: "Export '" + name + "' is not imported by any other analyzed file",
					})
				}
			}
		}
	}

	return findings
}

// getExportedNames extracts the exported name(s) from an export declaration.
func getExportedNames(exp *domain.Export) []string {
	var names []string

	// Named exports with specifiers: export { foo, bar }
	if len(exp.Specifiers) > 0 {
		for _, spec := range exp.Specifiers {
			if spec.IsType {
				continue
			}
			name := spec.Exported
			if name == "" {
				name = spec.Local
			}
			if name != "" {
				names = append(names, name)
			}
		}
		return names
	}

	// Default export
	if exp.ExportType == "default" {
		return []string{"default"}
	}

	// Declaration export: export function foo() / export const bar
	if exp.Name != "" {
		return []string{exp.Name}
	}

	return nil
}

// resolveImportPath resolves a relative import source to an actual file path.
// It tries the raw path, then common extensions, then index files.
func resolveImportPath(importingFile, source string, knownFiles map[string]bool) string {
	// Only handle relative imports
	if !strings.HasPrefix(source, "./") && !strings.HasPrefix(source, "../") {
		return ""
	}

	dir := filepath.Dir(importingFile)
	resolved := filepath.Join(dir, source)
	resolved = filepath.Clean(resolved)

	// Try exact path first
	if knownFiles[resolved] {
		return resolved
	}

	// Try adding extensions
	extensions := []string{".ts", ".tsx", ".js", ".jsx", ".mts", ".cts", ".mjs", ".cjs"}
	for _, ext := range extensions {
		candidate := resolved + ext
		if knownFiles[candidate] {
			return candidate
		}
	}

	// Try as directory with index files
	indexFiles := []string{
		"index.ts", "index.tsx", "index.js", "index.jsx",
		"index.mts", "index.cts", "index.mjs", "index.cjs",
	}
	for _, idx := range indexFiles {
		candidate := filepath.Join(resolved, idx)
		if knownFiles[candidate] {
			return candidate
		}
	}

	return ""
}

// isEntryPointFile checks if a file is an entry point (barrel file / index file).
func isEntryPointFile(filePath string) bool {
	base := filepath.Base(filePath)
	nameWithoutExt := strings.TrimSuffix(base, filepath.Ext(base))
	switch nameWithoutExt {
	case "index", "main", "app", "server":
		return true
	}
	return false
}

// isTestFile checks if a file is a test file.
func isTestFile(filePath string) bool {
	base := filepath.Base(filePath)

	// Check for *.test.* and *.spec.* patterns
	parts := strings.Split(base, ".")
	for _, part := range parts {
		if part == "test" || part == "spec" {
			return true
		}
	}

	// Check for __tests__ directory
	if strings.Contains(filePath, "__tests__") {
		return true
	}

	return false
}

// isFunctionOrClassDeclaration checks if a declaration string represents
// a function or class. The parser stores AST node types like
// "FunctionDeclaration", "AsyncFunctionDeclaration", "ClassDeclaration".
func isFunctionOrClassDeclaration(decl string) bool {
	return strings.Contains(decl, "Function") || strings.Contains(decl, "Class")
}

// isConfigFile checks if a file is a configuration or setup file.
func isConfigFile(filePath string) bool {
	base := filepath.Base(filePath)
	parts := strings.Split(base, ".")
	for _, part := range parts {
		if part == "config" || part == "setup" {
			return true
		}
	}
	return false
}

// DetectOrphanFiles detects files that are not reachable from any entry point via import chains.
// Entry points are: index/main/app/server files, and files not imported by any other file.
// Test files and config files are skipped.
func DetectOrphanFiles(allModuleInfos map[string]*domain.ModuleInfo, graph *ImportGraph) []*DeadCodeFinding {
	if len(allModuleInfos) == 0 {
		return nil
	}

	reverseEdges := graph.reverseEdges

	// Determine entry points:
	// 1. Files matching isEntryPointFile (index, main, app, server)
	// 2. Files not imported by any other file (root files with no reverse edges)
	entryPoints := make(map[string]bool)
	for filePath := range allModuleInfos {
		if isTestFile(filePath) || isConfigFile(filePath) {
			continue
		}
		if isEntryPointFile(filePath) {
			entryPoints[filePath] = true
			continue
		}
		if len(reverseEdges[filePath]) == 0 {
			entryPoints[filePath] = true
		}
	}

	// BFS from entry points using precomputed forward edges
	reachable := make(map[string]bool)
	queue := make([]string, 0, len(entryPoints))
	for ep := range entryPoints {
		reachable[ep] = true
		queue = append(queue, ep)
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		for _, resolvedPath := range graph.forwardEdges[current] {
			if !reachable[resolvedPath] {
				reachable[resolvedPath] = true
				queue = append(queue, resolvedPath)
			}
		}
	}

	// Files not reachable and not test/config files are orphans
	var findings []*DeadCodeFinding
	for _, filePath := range sortedModulePaths(allModuleInfos) {
		if reachable[filePath] {
			continue
		}
		if isTestFile(filePath) || isConfigFile(filePath) {
			continue
		}
		findings = append(findings, &DeadCodeFinding{
			FilePath:    filePath,
			Reason:      ReasonOrphanFile,
			Severity:    SeverityLevelWarning,
			Description: "File '" + filepath.Base(filePath) + "' is not reachable from any entry point",
		})
	}

	return findings
}

// DetectUnusedExportedFunctions detects exported functions and classes that are not imported
// by any other file in the project. Unlike DetectUnusedExports which covers all exports at
// info severity, this targets only function/class declarations at warning severity.
func DetectUnusedExportedFunctions(allModuleInfos map[string]*domain.ModuleInfo, graph *ImportGraph) []*DeadCodeFinding {
	if len(allModuleInfos) == 0 {
		return nil
	}

	importedNamesFromFile := graph.importedNamesFromFile

	var findings []*DeadCodeFinding

	for _, filePath := range sortedModulePaths(allModuleInfos) {
		info := allModuleInfos[filePath]
		if isEntryPointFile(filePath) {
			continue
		}
		if isTestFile(filePath) {
			continue
		}

		importedNames := importedNamesFromFile[filePath]

		// If namespace import (*) exists, all exports are considered used
		if importedNames != nil && importedNames["*"] {
			continue
		}

		for _, exp := range info.Exports {
			// Skip re-exports
			if exp.Source != "" {
				continue
			}
			// Skip type-only exports
			if exp.IsTypeOnly {
				continue
			}
			// Skip export * (re-export all)
			if exp.ExportType == "all" {
				continue
			}

			// Only target function and class declarations.
			// The parser sets Declaration to AST node types like
			// "FunctionDeclaration", "AsyncFunctionDeclaration", "ClassDeclaration", etc.
			if !isFunctionOrClassDeclaration(exp.Declaration) {
				continue
			}

			exportedNames := getExportedNames(exp)

			for _, name := range exportedNames {
				if isFrameworkReservedExport(filePath, exp, name) {
					continue
				}
				if importedNames == nil || !importedNames[name] {
					findings = append(findings, &DeadCodeFinding{
						FilePath:    filePath,
						StartLine:   exp.Location.StartLine,
						EndLine:     exp.Location.EndLine,
						Reason:      ReasonUnusedExportedFunction,
						Severity:    SeverityLevelWarning,
						Description: "Exported function '" + name + "' is not imported by any other analyzed file",
					})
				}
			}
		}
	}

	return findings
}

func isFrameworkReservedExport(filePath string, exp *domain.Export, exportedName string) bool {
	if !isNextAppRouterConventionFile(filePath) {
		return false
	}

	// Next.js App Router convention files expose exports consumed by the framework.
	if exportedName == "default" {
		return true
	}

	nextReserved := map[string]bool{
		"generateMetadata":     true,
		"metadata":             true,
		"generateViewport":     true,
		"viewport":             true,
		"generateStaticParams": true,
		"dynamic":              true,
		"dynamicParams":        true,
		"revalidate":           true,
		"fetchCache":           true,
		"runtime":              true,
		"preferredRegion":      true,
		"maxDuration":          true,
	}
	if nextReserved[exportedName] {
		return true
	}

	// Route handlers in app/**/route.ts are framework entry points.
	if isNextRouteHandlerFile(filePath) {
		switch exportedName {
		case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
			return true
		}
	}

	_ = exp
	return false
}

func isNextAppRouterConventionFile(filePath string) bool {
	path := filepath.ToSlash(filePath)
	if !strings.Contains(path, "/app/") {
		return false
	}

	base := filepath.Base(path)
	nameWithoutExt := strings.TrimSuffix(base, filepath.Ext(base))
	switch nameWithoutExt {
	case "page", "layout", "template", "loading", "error", "not-found", "default", "route":
		return true
	default:
		return false
	}
}

func isNextRouteHandlerFile(filePath string) bool {
	path := filepath.ToSlash(filePath)
	base := filepath.Base(path)
	nameWithoutExt := strings.TrimSuffix(base, filepath.Ext(base))
	return strings.Contains(path, "/app/") && nameWithoutExt == "route"
}

// resolveImportPaths resolves an import source to zero or more known file paths.
// Relative imports resolve to a single concrete path. Alias imports may resolve to
// multiple candidates when the alias root is ambiguous.
func resolveImportPaths(importingFile, source string, sourceType domain.ModuleType, knownFiles map[string]bool, idx *suffixIndex) []string {
	switch sourceType {
	case domain.ModuleTypeRelative:
		resolved := resolveImportPath(importingFile, source, knownFiles)
		if resolved == "" {
			return nil
		}
		return []string{resolved}
	case domain.ModuleTypeAlias:
		return resolveAliasImportPaths(source, idx)
	default:
		return nil
	}
}

// resolveAliasImportPaths resolves aliased import paths (e.g. "@/utils") by looking up
// suffix candidates in the precomputed suffix index for O(1) lookup per candidate.
func resolveAliasImportPaths(source string, idx *suffixIndex) []string {
	candidateBases := make(map[string]bool)
	candidateBases[source] = true

	if slashIdx := strings.Index(source, "/"); slashIdx >= 0 && slashIdx+1 < len(source) {
		candidateBases[source[slashIdx+1:]] = true
	}
	if strings.HasPrefix(source, "@/") || strings.HasPrefix(source, "~/") {
		candidateBases[source[2:]] = true
	}

	extensions := []string{".ts", ".tsx", ".js", ".jsx", ".mts", ".cts", ".mjs", ".cjs"}
	matches := make(map[string]bool)

	for base := range candidateBases {
		base = strings.TrimSpace(base)
		base = strings.TrimPrefix(base, "/")
		if base == "" {
			continue
		}

		candidates := []string{base}
		for _, ext := range extensions {
			candidates = append(candidates, base+ext)
			candidates = append(candidates, filepath.ToSlash(filepath.Join(base, "index"+ext)))
		}

		for _, c := range candidates {
			c = filepath.ToSlash(c)
			if files, ok := idx.bySuffix[c]; ok {
				for _, f := range files {
					matches[f] = true
				}
			}
		}
	}

	if len(matches) == 0 {
		return nil
	}

	result := make([]string, 0, len(matches))
	for file := range matches {
		result = append(result, file)
	}
	sort.Strings(result)
	return result
}

// sortedModulePaths returns the analyzed module paths in a fixed order, so
// findings collected across modules do not depend on Go's map iteration order.
func sortedModulePaths(allModuleInfos map[string]*domain.ModuleInfo) []string {
	paths := make([]string, 0, len(allModuleInfos))
	for filePath := range allModuleInfos {
		paths = append(paths, filePath)
	}
	sort.Strings(paths)
	return paths
}
