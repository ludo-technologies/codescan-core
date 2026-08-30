package domain

// ImportType represents the type of import statement
type ImportType string

const (
	// ImportTypeDefault represents default imports: import x from 'y'
	ImportTypeDefault ImportType = "default"

	// ImportTypeNamed represents named imports: import { x } from 'y'
	ImportTypeNamed ImportType = "named"

	// ImportTypeNamespace represents namespace imports: import * as x from 'y'
	ImportTypeNamespace ImportType = "namespace"

	// ImportTypeSideEffect represents side-effect imports: import 'y'
	ImportTypeSideEffect ImportType = "side_effect"

	// ImportTypeDynamic represents dynamic imports: import('y')
	ImportTypeDynamic ImportType = "dynamic"

	// ImportTypeRequire represents CommonJS require: require('y')
	ImportTypeRequire ImportType = "require"

	// ImportTypeTypeOnly represents TypeScript type imports: import type { x } from 'y'
	ImportTypeTypeOnly ImportType = "type_only"
)

// ModuleType represents the type of module source
type ModuleType string

const (
	// ModuleTypeRelative represents relative imports: ./foo, ../bar
	ModuleTypeRelative ModuleType = "relative"

	// ModuleTypeAbsolute represents absolute imports: /foo/bar
	ModuleTypeAbsolute ModuleType = "absolute"

	// ModuleTypePackage represents package imports: lodash, react
	ModuleTypePackage ModuleType = "package"

	// ModuleTypeBuiltin represents Node.js builtins: node:fs, fs (when builtin)
	ModuleTypeBuiltin ModuleType = "builtin"

	// ModuleTypeAlias represents aliased imports: @/components, ~/utils
	ModuleTypeAlias ModuleType = "alias"
)

// Import represents a single import statement in JavaScript/TypeScript
type Import struct {
	// Source is the module specifier (e.g., 'lodash', './utils')
	Source string `json:"source"`

	// SourceType is the type of module source (relative, package, builtin, etc.)
	SourceType ModuleType `json:"source_type"`

	// ImportType is the type of import (default, named, namespace, etc.)
	ImportType ImportType `json:"import_type"`

	// Specifiers are the individual imported items
	Specifiers []ImportSpecifier `json:"specifiers,omitempty"`

	// IsTypeOnly indicates TypeScript type-only imports
	IsTypeOnly bool `json:"is_type_only,omitempty"`

	// IsDynamic indicates dynamic import() expressions
	IsDynamic bool `json:"is_dynamic,omitempty"`

	// Location is the source code location
	Location SourceLocation `json:"location"`
}

// ImportSpecifier represents an individual imported item
type ImportSpecifier struct {
	// Imported is the original name from the module
	Imported string `json:"imported"`

	// Local is the local alias (or same as Imported if no alias)
	Local string `json:"local"`

	// IsType indicates TypeScript type-only specifier
	IsType bool `json:"is_type,omitempty"`
}

// Export represents a single export statement in JavaScript/TypeScript
type Export struct {
	// ExportType is the type of export: "named", "default", "all", "declaration"
	ExportType string `json:"export_type"`

	// Source is the re-export source (empty if not re-exporting)
	Source string `json:"source,omitempty"`

	// SourceType is the type of re-export source module
	SourceType ModuleType `json:"source_type,omitempty"`

	// Specifiers are the individual exported items
	Specifiers []ExportSpecifier `json:"specifiers,omitempty"`

	// Declaration is the declaration type (function, class, const, etc.)
	Declaration string `json:"declaration,omitempty"`

	// Name is the exported name
	Name string `json:"name,omitempty"`

	// IsTypeOnly indicates TypeScript type-only exports
	IsTypeOnly bool `json:"is_type_only,omitempty"`

	// Location is the source code location
	Location SourceLocation `json:"location"`
}

// ExportSpecifier represents an individual exported item
type ExportSpecifier struct {
	// Local is the local name
	Local string `json:"local"`

	// Exported is the exported name (or same as Local if no alias)
	Exported string `json:"exported"`

	// IsType indicates TypeScript type-only specifier
	IsType bool `json:"is_type,omitempty"`
}

// ModuleInfo contains all module analysis results for a single file
type ModuleInfo struct {
	// FilePath is the path to the analyzed file
	FilePath string `json:"file_path"`

	// Imports are all import statements in the file
	Imports []*Import `json:"imports"`

	// Exports are all export statements in the file
	Exports []*Export `json:"exports"`

	// Dependencies are the module dependencies extracted from imports
	Dependencies []ModuleDependency `json:"dependencies"`
}

// ModuleDependency represents a dependency relationship between modules
type ModuleDependency struct {
	// Source is the source module path as written in the import
	Source string `json:"source"`

	// SourceType is the type of module (relative, package, builtin, etc.)
	SourceType ModuleType `json:"source_type"`

	// ResolvedPath is the resolved absolute path (if resolvable)
	ResolvedPath string `json:"resolved_path,omitempty"`

	// IsDevDep indicates whether this is a dev dependency
	IsDevDep bool `json:"is_dev_dep,omitempty"`

	// Usages lists what is imported from this module
	Usages []string `json:"usages,omitempty"`
}

// ModuleAnalysisResult is the complete result of module analysis across multiple files
type ModuleAnalysisResult struct {
	// Files maps file paths to their module info
	Files map[string]*ModuleInfo `json:"files"`

	// DependencyGraph maps modules to their dependencies
	DependencyGraph map[string][]string `json:"dependency_graph"`

	// PackageDeps lists all npm package dependencies found
	PackageDeps []string `json:"package_deps"`

	// Summary provides aggregate statistics
	Summary ModuleAnalysisSummary `json:"summary"`
}

// ModuleAnalysisSummary provides aggregate statistics for module analysis
type ModuleAnalysisSummary struct {
	// TotalFiles is the number of files analyzed
	TotalFiles int `json:"total_files"`

	// TotalImports is the total number of import statements
	TotalImports int `json:"total_imports"`

	// TotalExports is the total number of export statements
	TotalExports int `json:"total_exports"`

	// UniquePackages is the number of unique npm packages
	UniquePackages int `json:"unique_packages"`

	// RelativeImports is the count of relative imports
	RelativeImports int `json:"relative_imports"`

	// AbsoluteImports is the count of absolute imports
	AbsoluteImports int `json:"absolute_imports"`

	// DynamicImports is the count of dynamic imports
	DynamicImports int `json:"dynamic_imports"`

	// TypeOnlyImports is the count of TypeScript type-only imports
	TypeOnlyImports int `json:"type_only_imports"`

	// CommonJSImports is the count of CommonJS require() calls
	CommonJSImports int `json:"commonjs_imports"`
}

// ModuleAnalysisRequest represents a request for module analysis
type ModuleAnalysisRequest struct {
	// Paths are the input files or directories to analyze
	Paths []string `json:"paths"`

	// Recursive indicates whether to analyze directories recursively
	Recursive bool `json:"recursive"`

	// IncludeBuiltins indicates whether to include Node.js builtins
	IncludeBuiltins bool `json:"include_builtins"`

	// IncludeTypeImports indicates whether to include TypeScript type imports
	IncludeTypeImports bool `json:"include_type_imports"`

	// ResolveRelative indicates whether to resolve relative import paths
	ResolveRelative bool `json:"resolve_relative"`

	// AliasPatterns are path alias patterns to recognize (@/, ~/, etc.)
	AliasPatterns []string `json:"alias_patterns,omitempty"`
}

// ModuleAnalysisResponse represents the response from module analysis
type ModuleAnalysisResponse struct {
	// Result contains the analysis result
	Result *ModuleAnalysisResult `json:"result"`

	// Errors contains any errors encountered during analysis
	Errors []string `json:"errors,omitempty"`

	// Warnings contains any warnings from analysis
	Warnings []string `json:"warnings,omitempty"`
}
