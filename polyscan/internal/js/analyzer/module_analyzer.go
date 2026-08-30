package analyzer

import (
	"fmt"
	"strings"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

// Node.js built-in modules list (node: prefix or bare)
var nodeBuiltins = map[string]bool{
	"assert":         true,
	"buffer":         true,
	"child_process":  true,
	"cluster":        true,
	"console":        true,
	"constants":      true,
	"crypto":         true,
	"dgram":          true,
	"dns":            true,
	"domain":         true,
	"events":         true,
	"fs":             true,
	"http":           true,
	"http2":          true,
	"https":          true,
	"module":         true,
	"net":            true,
	"os":             true,
	"path":           true,
	"perf_hooks":     true,
	"process":        true,
	"punycode":       true,
	"querystring":    true,
	"readline":       true,
	"repl":           true,
	"stream":         true,
	"string_decoder": true,
	"sys":            true,
	"timers":         true,
	"tls":            true,
	"tty":            true,
	"url":            true,
	"util":           true,
	"v8":             true,
	"vm":             true,
	"wasi":           true,
	"worker_threads": true,
	"zlib":           true,
}

// ModuleAnalyzerConfig holds configuration for the module analyzer
type ModuleAnalyzerConfig struct {
	// IncludeBuiltins includes Node.js builtin modules in analysis
	IncludeBuiltins bool

	// ResolveRelative enables resolution of relative import paths
	ResolveRelative bool

	// IncludeTypeImports includes TypeScript type imports
	IncludeTypeImports bool

	// AliasPatterns are path alias patterns to recognize (@/, ~/, etc.)
	AliasPatterns []string
}

// DefaultModuleAnalyzerConfig returns the default configuration
func DefaultModuleAnalyzerConfig() *ModuleAnalyzerConfig {
	return &ModuleAnalyzerConfig{
		IncludeBuiltins:    true,
		ResolveRelative:    false,
		IncludeTypeImports: true,
		AliasPatterns:      []string{"@/", "~/"},
	}
}

// ModuleAnalyzer analyzes JavaScript/TypeScript module imports and exports
type ModuleAnalyzer struct {
	config *ModuleAnalyzerConfig
}

// NewModuleAnalyzer creates a new module analyzer with the given configuration
func NewModuleAnalyzer(config *ModuleAnalyzerConfig) *ModuleAnalyzer {
	if config == nil {
		config = DefaultModuleAnalyzerConfig()
	}
	return &ModuleAnalyzer{
		config: config,
	}
}

// AnalyzeFile analyzes a single file and returns its module info
func (ma *ModuleAnalyzer) AnalyzeFile(ast *parser.Node, filePath string) (*domain.ModuleInfo, error) {
	if ast == nil {
		return &domain.ModuleInfo{FilePath: filePath}, nil
	}

	info := &domain.ModuleInfo{
		FilePath:     filePath,
		Imports:      make([]*domain.Import, 0),
		Exports:      make([]*domain.Export, 0),
		Dependencies: make([]domain.ModuleDependency, 0),
	}

	// Extract imports
	ma.extractImports(ast, info)

	// Extract exports
	ma.extractExports(ast, info)

	// Build dependencies from imports
	ma.buildDependencies(info)

	return info, nil
}

// AnalyzeAll analyzes multiple files and returns aggregated results
func (ma *ModuleAnalyzer) AnalyzeAll(asts map[string]*parser.Node) (*domain.ModuleAnalysisResult, error) {
	result := &domain.ModuleAnalysisResult{
		Files:           make(map[string]*domain.ModuleInfo),
		DependencyGraph: make(map[string][]string),
		PackageDeps:     make([]string, 0),
		Summary:         domain.ModuleAnalysisSummary{},
	}

	packageSet := make(map[string]bool)

	for filePath, ast := range asts {
		info, err := ma.AnalyzeFile(ast, filePath)
		if err != nil {
			continue
		}

		result.Files[filePath] = info

		// Build dependency graph
		deps := make([]string, 0)
		for _, imp := range info.Imports {
			deps = append(deps, imp.Source)

			// Track package dependencies
			if imp.SourceType == domain.ModuleTypePackage {
				packageSet[imp.Source] = true
			}
		}
		result.DependencyGraph[filePath] = deps

		// Update summary
		result.Summary.TotalFiles++
		result.Summary.TotalImports += len(info.Imports)
		result.Summary.TotalExports += len(info.Exports)

		for _, imp := range info.Imports {
			switch imp.SourceType {
			case domain.ModuleTypeRelative:
				result.Summary.RelativeImports++
			case domain.ModuleTypeAbsolute:
				result.Summary.AbsoluteImports++
			}
			if imp.IsDynamic {
				result.Summary.DynamicImports++
			}
			if imp.IsTypeOnly {
				result.Summary.TypeOnlyImports++
			}
			if imp.ImportType == domain.ImportTypeRequire {
				result.Summary.CommonJSImports++
			}
		}
	}

	// Convert package set to slice
	for pkg := range packageSet {
		result.PackageDeps = append(result.PackageDeps, pkg)
	}
	result.Summary.UniquePackages = len(result.PackageDeps)

	return result, nil
}

// extractImports walks the AST and extracts all import statements
func (ma *ModuleAnalyzer) extractImports(ast *parser.Node, info *domain.ModuleInfo) {
	// Track visited nodes by their location to avoid duplicates
	// (nodes can appear in both Children and Body arrays)
	visited := make(map[string]bool)

	ast.Walk(func(node *parser.Node) bool {
		// Create a unique key for this node based on its location
		key := nodeLocationKey(node)

		switch node.Type {
		case parser.NodeImportDeclaration:
			if !visited[key] {
				visited[key] = true
				imp := ma.processImportDeclaration(node)
				if imp != nil {
					info.Imports = append(info.Imports, imp)
				}
			}
			return false // Don't walk children of import declaration

		case parser.NodeCallExpression:
			if !visited[key] {
				visited[key] = true
				// Check for dynamic imports: import('module')
				if dynamicImport := ma.processDynamicImport(node); dynamicImport != nil {
					info.Imports = append(info.Imports, dynamicImport)
				}
				// Check for CommonJS require: require('module')
				if requireImport := ma.processRequireCall(node); requireImport != nil {
					info.Imports = append(info.Imports, requireImport)
				}
			}
		}
		return true
	})
}

// nodeLocationKey creates a unique key for a node based on its location.
// The end position is part of the key because nested nodes of the same type
// can share a start position, as in `import('./m.js').then(...)` where the
// dynamic import and the `.then()` call are both call expressions starting at
// the same offset.
func nodeLocationKey(node *parser.Node) string {
	if node == nil {
		return ""
	}
	return fmt.Sprintf("%s:%s:%d:%d:%d:%d", node.Type, node.Location.File,
		node.Location.StartLine, node.Location.StartCol,
		node.Location.EndLine, node.Location.EndCol)
}

// extractExports walks the AST and extracts all export statements
func (ma *ModuleAnalyzer) extractExports(ast *parser.Node, info *domain.ModuleInfo) {
	// Track visited nodes by their location to avoid duplicates
	visited := make(map[string]bool)

	ast.Walk(func(node *parser.Node) bool {
		key := nodeLocationKey(node)

		switch node.Type {
		case parser.NodeExportNamedDeclaration:
			if !visited[key] {
				visited[key] = true
				exp := ma.processExportNamedDeclaration(node)
				if exp != nil {
					info.Exports = append(info.Exports, exp)
				}
			}
			return false // Don't walk children of export declaration

		case parser.NodeExportDefaultDeclaration:
			if !visited[key] {
				visited[key] = true
				exp := ma.processExportDefaultDeclaration(node)
				if exp != nil {
					info.Exports = append(info.Exports, exp)
				}
			}
			return false // Don't walk children of export declaration

		case parser.NodeExportAllDeclaration:
			if !visited[key] {
				visited[key] = true
				exp := ma.processExportAllDeclaration(node)
				if exp != nil {
					info.Exports = append(info.Exports, exp)
				}
			}
			return false // Don't walk children of export declaration

		case parser.NodeAssignmentExpression:
			if !visited[key] {
				visited[key] = true
				// Check for CommonJS exports: module.exports = ...
				if exp := ma.processModuleExports(node); exp != nil {
					info.Exports = append(info.Exports, exp)
				}
			}
		}
		return true
	})
}

// processImportDeclaration processes an ES6 import declaration
func (ma *ModuleAnalyzer) processImportDeclaration(node *parser.Node) *domain.Import {
	source := ma.extractSourceValue(node.Source)
	if source == "" {
		return nil
	}

	imp := &domain.Import{
		Source:     source,
		SourceType: ma.classifyModuleSource(source),
		Specifiers: make([]domain.ImportSpecifier, 0),
		Location:   ma.nodeToSourceLocation(node),
	}

	// Determine import type and extract specifiers
	hasDefault := false
	hasNamed := false
	hasNamespace := false

	for _, spec := range node.Specifiers {
		switch spec.Type {
		case parser.NodeImportDefaultSpecifier:
			hasDefault = true
			imp.Specifiers = append(imp.Specifiers, domain.ImportSpecifier{
				Imported: "default",
				Local:    spec.Name,
			})

		case parser.NodeImportNamespaceSpecifier:
			hasNamespace = true
			imp.Specifiers = append(imp.Specifiers, domain.ImportSpecifier{
				Imported: "*",
				Local:    spec.Name,
			})

		case parser.NodeImportSpecifier:
			hasNamed = true
			specifier := domain.ImportSpecifier{
				Local: spec.Name,
			}
			if spec.Imported != nil {
				specifier.Imported = spec.Imported.Name
			} else {
				specifier.Imported = spec.Name
			}
			imp.Specifiers = append(imp.Specifiers, specifier)
		}
	}

	// Determine import type
	if hasNamespace {
		imp.ImportType = domain.ImportTypeNamespace
	} else if hasDefault && !hasNamed {
		imp.ImportType = domain.ImportTypeDefault
	} else if hasNamed {
		imp.ImportType = domain.ImportTypeNamed
	} else if len(node.Specifiers) == 0 {
		imp.ImportType = domain.ImportTypeSideEffect
	}

	// Check for TypeScript type import
	// This would require additional AST parsing for 'import type'
	// For now, we check if the node might be a type import based on children

	return imp
}

// processDynamicImport checks if a call expression is a dynamic import
func (ma *ModuleAnalyzer) processDynamicImport(node *parser.Node) *domain.Import {
	if node.Callee == nil || node.Callee.Type != parser.NodeImport {
		return nil
	}

	if len(node.Arguments) == 0 {
		return nil
	}

	source := ma.extractSourceValue(node.Arguments[0])
	if source == "" {
		return nil
	}

	return &domain.Import{
		Source:     source,
		SourceType: ma.classifyModuleSource(source),
		ImportType: domain.ImportTypeDynamic,
		IsDynamic:  true,
		Location:   ma.nodeToSourceLocation(node),
	}
}

// processRequireCall checks if a call expression is a require() call
func (ma *ModuleAnalyzer) processRequireCall(node *parser.Node) *domain.Import {
	if node.Callee == nil {
		return nil
	}

	// Check if callee is 'require'
	if node.Callee.Type == parser.NodeIdentifier && node.Callee.Name == "require" {
		if len(node.Arguments) > 0 {
			source := ma.extractSourceValue(node.Arguments[0])
			if source != "" {
				return &domain.Import{
					Source:     source,
					SourceType: ma.classifyModuleSource(source),
					ImportType: domain.ImportTypeRequire,
					Location:   ma.nodeToSourceLocation(node),
				}
			}
		}
	}

	return nil
}

// processExportNamedDeclaration processes a named export declaration
func (ma *ModuleAnalyzer) processExportNamedDeclaration(node *parser.Node) *domain.Export {
	exp := &domain.Export{
		ExportType: "named",
		Specifiers: make([]domain.ExportSpecifier, 0),
		Location:   ma.nodeToSourceLocation(node),
	}

	// Check for re-export: export { ... } from 'source'
	if node.Source != nil {
		exp.Source = ma.extractSourceValue(node.Source)
		exp.SourceType = ma.classifyModuleSource(exp.Source)
	}

	// Process declaration if present
	if node.Declaration != nil {
		exp.Declaration = string(node.Declaration.Type)
		if node.Declaration.Name != "" {
			exp.Name = node.Declaration.Name
			exp.Specifiers = append(exp.Specifiers, domain.ExportSpecifier{
				Local:    node.Declaration.Name,
				Exported: node.Declaration.Name,
			})
		}
	}

	// Process specifiers
	for _, spec := range node.Specifiers {
		specifier := domain.ExportSpecifier{
			Local: spec.Name,
		}
		if spec.Local != nil {
			specifier.Local = spec.Local.Name
		}
		specifier.Exported = spec.Name
		exp.Specifiers = append(exp.Specifiers, specifier)
	}

	return exp
}

// processExportDefaultDeclaration processes a default export declaration
func (ma *ModuleAnalyzer) processExportDefaultDeclaration(node *parser.Node) *domain.Export {
	exp := &domain.Export{
		ExportType: "default",
		Location:   ma.nodeToSourceLocation(node),
	}

	if node.Declaration != nil {
		exp.Declaration = string(node.Declaration.Type)
		if node.Declaration.Name != "" {
			exp.Name = node.Declaration.Name
		}
	}

	return exp
}

// processExportAllDeclaration processes an export * declaration
func (ma *ModuleAnalyzer) processExportAllDeclaration(node *parser.Node) *domain.Export {
	exp := &domain.Export{
		ExportType: "all",
		Location:   ma.nodeToSourceLocation(node),
	}

	if node.Source != nil {
		exp.Source = ma.extractSourceValue(node.Source)
		exp.SourceType = ma.classifyModuleSource(exp.Source)
	}

	return exp
}

// processModuleExports checks for CommonJS module.exports assignments
func (ma *ModuleAnalyzer) processModuleExports(node *parser.Node) *domain.Export {
	if node.Left == nil {
		return nil
	}

	// Check for module.exports = ...
	if node.Left.Type == parser.NodeMemberExpression {
		if node.Left.Object != nil && node.Left.Property != nil {
			objName := ""
			propName := ""

			if node.Left.Object.Type == parser.NodeIdentifier {
				objName = node.Left.Object.Name
			}
			if node.Left.Property.Type == parser.NodeIdentifier {
				propName = node.Left.Property.Name
			}

			if objName == "module" && propName == "exports" {
				return &domain.Export{
					ExportType:  "default",
					Declaration: "module.exports",
					Location:    ma.nodeToSourceLocation(node),
				}
			}

			// Check for exports.foo = ...
			if objName == "exports" {
				return &domain.Export{
					ExportType: "named",
					Name:       propName,
					Specifiers: []domain.ExportSpecifier{
						{Local: propName, Exported: propName},
					},
					Location: ma.nodeToSourceLocation(node),
				}
			}
		}
	}

	return nil
}

// extractSourceValue extracts the string value from a source node
func (ma *ModuleAnalyzer) extractSourceValue(node *parser.Node) string {
	if node == nil {
		return ""
	}

	// The source is typically a string literal
	switch node.Type {
	case parser.NodeTemplateLiteral:
		// Only a template without substitutions names a fixed module; anything
		// interpolated is not statically resolvable.
		raw := node.Raw
		if strings.Contains(raw, "${") {
			return ""
		}
		if len(raw) >= 2 && raw[0] == '`' && raw[len(raw)-1] == '`' {
			return raw[1 : len(raw)-1]
		}
		return ""

	case parser.NodeStringLiteral, parser.NodeLiteral:
		// Remove quotes from the raw value
		raw := node.Raw
		if len(raw) >= 2 {
			if (raw[0] == '"' && raw[len(raw)-1] == '"') ||
				(raw[0] == '\'' && raw[len(raw)-1] == '\'') ||
				(raw[0] == '`' && raw[len(raw)-1] == '`') {
				return raw[1 : len(raw)-1]
			}
		}
		return raw
	}

	// Try to get value from Name or Raw
	if node.Name != "" {
		return node.Name
	}
	if node.Raw != "" {
		raw := node.Raw
		if len(raw) >= 2 {
			if (raw[0] == '"' && raw[len(raw)-1] == '"') ||
				(raw[0] == '\'' && raw[len(raw)-1] == '\'') {
				return raw[1 : len(raw)-1]
			}
		}
		return raw
	}

	return ""
}

// classifyModuleSource determines the type of module source
func (ma *ModuleAnalyzer) classifyModuleSource(source string) domain.ModuleType {
	if source == "" {
		return domain.ModuleTypePackage
	}

	// Check for node: prefix (explicit builtin)
	if strings.HasPrefix(source, "node:") {
		return domain.ModuleTypeBuiltin
	}

	// Check for relative paths
	if strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") {
		return domain.ModuleTypeRelative
	}

	// Check for absolute paths
	if strings.HasPrefix(source, "/") {
		return domain.ModuleTypeAbsolute
	}

	// Check for alias patterns
	for _, pattern := range ma.config.AliasPatterns {
		if strings.HasPrefix(source, pattern) {
			return domain.ModuleTypeAlias
		}
	}

	// Check if it's a Node.js builtin
	// Extract package name (before any /)
	pkgName := source
	if idx := strings.Index(source, "/"); idx > 0 {
		pkgName = source[:idx]
	}
	if nodeBuiltins[pkgName] {
		return domain.ModuleTypeBuiltin
	}

	// Default to package
	return domain.ModuleTypePackage
}

// nodeToSourceLocation converts a parser.Node location to domain.SourceLocation
func (ma *ModuleAnalyzer) nodeToSourceLocation(node *parser.Node) domain.SourceLocation {
	return domain.SourceLocation{
		FilePath:  node.Location.File,
		StartLine: node.Location.StartLine,
		EndLine:   node.Location.EndLine,
		StartCol:  node.Location.StartCol,
		EndCol:    node.Location.EndCol,
	}
}

// buildDependencies builds the dependency list from imports
func (ma *ModuleAnalyzer) buildDependencies(info *domain.ModuleInfo) {
	seen := make(map[string]bool)

	for _, imp := range info.Imports {
		if seen[imp.Source] {
			continue
		}
		seen[imp.Source] = true

		dep := domain.ModuleDependency{
			Source:     imp.Source,
			SourceType: imp.SourceType,
			Usages:     make([]string, 0),
		}

		// Add usages
		for _, spec := range imp.Specifiers {
			dep.Usages = append(dep.Usages, spec.Local)
		}

		info.Dependencies = append(info.Dependencies, dep)
	}
}
