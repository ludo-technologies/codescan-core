package analyzer

import (
	"sort"
	"strings"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

// CBOAnalyzerConfig holds configuration for the CBO analyzer
type CBOAnalyzerConfig struct {
	// IncludeBuiltins includes Node.js builtin modules in coupling count
	IncludeBuiltins bool

	// IncludeTypeImports includes TypeScript type imports in coupling count
	IncludeTypeImports bool

	// LowThreshold is the CBO threshold for low risk (CBO <= LowThreshold)
	LowThreshold int

	// MediumThreshold is the CBO threshold for medium risk (LowThreshold < CBO <= MediumThreshold)
	MediumThreshold int
}

// DefaultCBOAnalyzerConfig returns the default configuration
func DefaultCBOAnalyzerConfig() *CBOAnalyzerConfig {
	return &CBOAnalyzerConfig{
		IncludeBuiltins:    false,
		IncludeTypeImports: true,
		LowThreshold:       7,
		MediumThreshold:    14,
	}
}

// CBOAnalyzer analyzes Coupling Between Objects for JavaScript/TypeScript modules
type CBOAnalyzer struct {
	config         *CBOAnalyzerConfig
	moduleAnalyzer *ModuleAnalyzer
}

// NewCBOAnalyzer creates a new CBO analyzer with the given configuration
func NewCBOAnalyzer(config *CBOAnalyzerConfig) *CBOAnalyzer {
	if config == nil {
		config = DefaultCBOAnalyzerConfig()
	}
	return &CBOAnalyzer{
		config: config,
		moduleAnalyzer: NewModuleAnalyzer(&ModuleAnalyzerConfig{
			IncludeBuiltins:    config.IncludeBuiltins,
			IncludeTypeImports: config.IncludeTypeImports,
		}),
	}
}

// ClassDependencies holds all dependencies for a class/module
type ClassDependencies struct {
	// Unique modules/classes this class depends on
	DependentClasses map[string]bool

	// Breakdown by type
	ImportDependencies          map[string]bool
	InstantiationDependencies   map[string]bool
	TypeHintDependencies        map[string]bool
	AttributeAccessDependencies map[string]bool
}

// NewClassDependencies creates a new ClassDependencies instance
func NewClassDependencies() *ClassDependencies {
	return &ClassDependencies{
		DependentClasses:            make(map[string]bool),
		ImportDependencies:          make(map[string]bool),
		InstantiationDependencies:   make(map[string]bool),
		TypeHintDependencies:        make(map[string]bool),
		AttributeAccessDependencies: make(map[string]bool),
	}
}

// AnalyzeFile analyzes a single file and returns its CBO metrics
func (ca *CBOAnalyzer) AnalyzeFile(ast *parser.Node, filePath string) (*domain.ClassCoupling, error) {
	if ast == nil {
		return &domain.ClassCoupling{
			Name:      extractModuleName(filePath),
			FilePath:  filePath,
			StartLine: 1,
			EndLine:   1,
			Metrics:   domain.CBOMetrics{},
			RiskLevel: domain.RiskLevelLow,
		}, nil
	}

	deps := NewClassDependencies()

	// 1. Extract import dependencies using module analyzer
	ca.extractImportDependencies(ast, filePath, deps)

	// 2. Extract instantiation dependencies (new expressions)
	ca.extractInstantiationDependencies(ast, deps)

	// 3. Extract type hint dependencies (TypeScript)
	ca.extractTypeHintDependencies(ast, deps)

	// 4. Extract attribute access dependencies (method calls)
	ca.extractAttributeAccessDependencies(ast, deps)

	// Build metrics
	metrics := ca.buildMetrics(deps)

	// Calculate risk level
	riskLevel := ca.calculateRiskLevel(metrics.CouplingCount)

	// Get file extent
	startLine, endLine := ca.getFileExtent(ast)

	return &domain.ClassCoupling{
		Name:      extractModuleName(filePath),
		FilePath:  filePath,
		StartLine: startLine,
		EndLine:   endLine,
		Metrics:   metrics,
		RiskLevel: riskLevel,
	}, nil
}

// extractImportDependencies extracts dependencies from import statements
func (ca *CBOAnalyzer) extractImportDependencies(ast *parser.Node, filePath string, deps *ClassDependencies) {
	moduleInfo, err := ca.moduleAnalyzer.AnalyzeFile(ast, filePath)
	if err != nil {
		return
	}

	for _, imp := range moduleInfo.Imports {
		// Skip builtins if configured
		if !ca.config.IncludeBuiltins && imp.SourceType == domain.ModuleTypeBuiltin {
			continue
		}

		// Skip type-only imports if configured
		if !ca.config.IncludeTypeImports && imp.IsTypeOnly {
			continue
		}

		depName := normalizeModuleName(imp.Source)
		deps.ImportDependencies[depName] = true
		deps.DependentClasses[depName] = true
	}
}

// extractInstantiationDependencies extracts dependencies from new expressions
func (ca *CBOAnalyzer) extractInstantiationDependencies(ast *parser.Node, deps *ClassDependencies) {
	ast.Walk(func(node *parser.Node) bool {
		// Check for both tree-sitter type and our AST type
		if node.Type == parser.NodeNewExpression || node.Type == "new_expression" {
			className := ca.extractCalleeClassName(node)
			if className != "" && !isBuiltinClass(className) {
				deps.InstantiationDependencies[className] = true
				deps.DependentClasses[className] = true
			}
		}
		return true
	})
}

// extractTypeHintDependencies extracts dependencies from TypeScript type annotations
func (ca *CBOAnalyzer) extractTypeHintDependencies(ast *parser.Node, deps *ClassDependencies) {
	if !ca.config.IncludeTypeImports {
		return
	}

	ast.Walk(func(node *parser.Node) bool {
		switch node.Type {
		case parser.NodeTypeAnnotation:
			ca.extractTypesFromNode(node, deps)

		case parser.NodeAsExpression:
			if node.TypeAnnotation != nil {
				ca.extractTypesFromNode(node.TypeAnnotation, deps)
			}

		case parser.NodeInterfaceDeclaration, parser.NodeTypeAlias:
			// Extract types from interface/type declarations
			ca.extractTypesFromNode(node, deps)
		}
		return true
	})
}

// extractTypesFromNode extracts type names from a type annotation node
func (ca *CBOAnalyzer) extractTypesFromNode(node *parser.Node, deps *ClassDependencies) {
	if node == nil {
		return
	}

	// Walk through the node to find identifier types
	node.Walk(func(n *parser.Node) bool {
		if n.Type == parser.NodeIdentifier && n.Name != "" {
			typeName := n.Name
			// Skip primitive types and common utility types
			if !isPrimitiveType(typeName) && !isBuiltinType(typeName) {
				deps.TypeHintDependencies[typeName] = true
				deps.DependentClasses[typeName] = true
			}
		}
		return true
	})
}

// extractAttributeAccessDependencies extracts dependencies from method calls and property access
func (ca *CBOAnalyzer) extractAttributeAccessDependencies(ast *parser.Node, deps *ClassDependencies) {
	// Track imported identifiers for context
	importedIdentifiers := make(map[string]string) // local name -> module name

	// First pass: collect imported identifiers
	ast.Walk(func(node *parser.Node) bool {
		if node.Type == parser.NodeImportDeclaration && node.Source != nil {
			moduleName := ca.extractSourceValue(node.Source)
			for _, spec := range node.Specifiers {
				if spec.Name != "" {
					importedIdentifiers[spec.Name] = moduleName
				}
			}
		}
		return true
	})

	// Second pass: look for method calls on imported objects
	ast.Walk(func(node *parser.Node) bool {
		if node.Type == parser.NodeCallExpression {
			// Check for member expression calls: obj.method()
			if node.Callee != nil && node.Callee.Type == parser.NodeMemberExpression {
				objName := ca.extractObjectName(node.Callee.Object)
				if objName != "" {
					// If the object is an imported identifier, count the module as a dependency
					if moduleName, ok := importedIdentifiers[objName]; ok {
						depName := normalizeModuleName(moduleName)
						deps.AttributeAccessDependencies[depName] = true
						deps.DependentClasses[depName] = true
					} else {
						// Otherwise, count the object itself (could be a class instance)
						if !isBuiltinObject(objName) {
							deps.AttributeAccessDependencies[objName] = true
							deps.DependentClasses[objName] = true
						}
					}
				}
			}
		}
		return true
	})
}

// extractCalleeClassName extracts the class name from a new expression
func (ca *CBOAnalyzer) extractCalleeClassName(node *parser.Node) string {
	// First try the Callee field (for properly parsed nodes)
	if node.Callee != nil {
		switch node.Callee.Type {
		case parser.NodeIdentifier, "identifier":
			return node.Callee.Name
		case parser.NodeMemberExpression, "member_expression":
			// Handle cases like new module.ClassName()
			if node.Callee.Property != nil && (node.Callee.Property.Type == parser.NodeIdentifier || node.Callee.Property.Type == "identifier") {
				return node.Callee.Property.Name
			}
		}
	}

	// For generic nodes (tree-sitter), look in Children
	for _, child := range node.Children {
		if child.Type == parser.NodeIdentifier || child.Type == "identifier" {
			if child.Name != "" {
				return child.Name
			}
		}
		// Handle member expressions in children
		if child.Type == parser.NodeMemberExpression || child.Type == "member_expression" {
			if child.Property != nil && child.Property.Name != "" {
				return child.Property.Name
			}
			// Try to find identifier in member expression children
			for _, grandchild := range child.Children {
				if (grandchild.Type == parser.NodeIdentifier || grandchild.Type == "identifier" || grandchild.Type == "property_identifier") && grandchild.Name != "" {
					return grandchild.Name
				}
			}
		}
	}

	return ""
}

// extractObjectName extracts the object name from a node
func (ca *CBOAnalyzer) extractObjectName(node *parser.Node) string {
	if node == nil {
		return ""
	}
	switch node.Type {
	case parser.NodeIdentifier:
		return node.Name
	case parser.NodeMemberExpression:
		return ca.extractObjectName(node.Object)
	}
	return ""
}

// extractSourceValue extracts the string value from a source node
func (ca *CBOAnalyzer) extractSourceValue(node *parser.Node) string {
	if node == nil {
		return ""
	}

	switch node.Type {
	case parser.NodeStringLiteral, parser.NodeLiteral:
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

// buildMetrics builds CBOMetrics from ClassDependencies
func (ca *CBOAnalyzer) buildMetrics(deps *ClassDependencies) domain.CBOMetrics {
	dependentClasses := make([]string, 0, len(deps.DependentClasses))
	for class := range deps.DependentClasses {
		dependentClasses = append(dependentClasses, class)
	}
	sort.Strings(dependentClasses)

	return domain.CBOMetrics{
		CouplingCount:               len(deps.DependentClasses),
		ImportDependencies:          len(deps.ImportDependencies),
		InstantiationDependencies:   len(deps.InstantiationDependencies),
		TypeHintDependencies:        len(deps.TypeHintDependencies),
		AttributeAccessDependencies: len(deps.AttributeAccessDependencies),
		DependentClasses:            dependentClasses,
	}
}

// calculateRiskLevel determines the risk level based on CBO count
func (ca *CBOAnalyzer) calculateRiskLevel(cboCount int) domain.RiskLevel {
	if cboCount <= ca.config.LowThreshold {
		return domain.RiskLevelLow
	}
	if cboCount <= ca.config.MediumThreshold {
		return domain.RiskLevelMedium
	}
	return domain.RiskLevelHigh
}

// getFileExtent returns the start and end lines of the file
func (ca *CBOAnalyzer) getFileExtent(ast *parser.Node) (int, int) {
	startLine := 1
	endLine := 1

	if ast != nil {
		startLine = max(ast.Location.StartLine, 1)
		endLine = max(ast.Location.EndLine, startLine)
	}

	return startLine, endLine
}

// Helper functions

// extractModuleName extracts a module name from a file path
func extractModuleName(filePath string) string {
	// Remove directory path
	lastSlash := strings.LastIndex(filePath, "/")
	name := filePath
	if lastSlash >= 0 {
		name = filePath[lastSlash+1:]
	}

	// Remove extension
	if idx := strings.LastIndex(name, "."); idx > 0 {
		name = name[:idx]
	}

	return name
}

// normalizeModuleName normalizes a module path to a consistent name
func normalizeModuleName(source string) string {
	// Remove leading ./ or ../
	name := source
	for strings.HasPrefix(name, "./") {
		name = name[2:]
	}
	for strings.HasPrefix(name, "../") {
		name = name[3:]
	}

	// Extract package name for scoped packages (@scope/package)
	if strings.HasPrefix(name, "@") {
		parts := strings.SplitN(name, "/", 3)
		if len(parts) >= 2 {
			name = parts[0] + "/" + parts[1]
		}
	} else {
		// For regular packages, just get the package name
		if idx := strings.Index(name, "/"); idx > 0 {
			name = name[:idx]
		}
	}

	return name
}

// Package-level maps for builtin lookups (avoid allocation on each call)
var builtinClasses = map[string]bool{
	"Array": true, "Object": true, "String": true, "Number": true,
	"Boolean": true, "Function": true, "Symbol": true, "BigInt": true,
	"Date": true, "RegExp": true, "Error": true, "TypeError": true,
	"RangeError": true, "SyntaxError": true, "ReferenceError": true,
	"Map": true, "Set": true, "WeakMap": true, "WeakSet": true,
	"Promise": true, "Proxy": true, "Reflect": true,
	"ArrayBuffer": true, "SharedArrayBuffer": true, "DataView": true,
	"Int8Array": true, "Uint8Array": true, "Uint8ClampedArray": true,
	"Int16Array": true, "Uint16Array": true,
	"Int32Array": true, "Uint32Array": true,
	"Float32Array": true, "Float64Array": true,
	"BigInt64Array": true, "BigUint64Array": true,
	"JSON": true, "Math": true, "Intl": true,
	"URL": true, "URLSearchParams": true,
	"EventTarget": true, "Event": true, "CustomEvent": true,
	"AbortController": true, "AbortSignal": true,
	"TextEncoder": true, "TextDecoder": true,
	"Headers": true, "Request": true, "Response": true,
	"FormData": true, "Blob": true, "File": true, "FileReader": true,
}

var builtinObjects = map[string]bool{
	"console": true, "process": true, "global": true, "globalThis": true,
	"window": true, "document": true, "navigator": true, "location": true,
	"localStorage": true, "sessionStorage": true,
	"JSON": true, "Math": true, "Intl": true,
	"Object": true, "Array": true, "String": true, "Number": true,
	"Boolean": true, "Date": true, "RegExp": true,
	"Promise": true, "Proxy": true, "Reflect": true,
	"Buffer": true, "require": true, "module": true, "exports": true,
	"__dirname": true, "__filename": true,
	"setTimeout": true, "setInterval": true, "setImmediate": true,
	"clearTimeout": true, "clearInterval": true, "clearImmediate": true,
	"fetch": true, "XMLHttpRequest": true,
	"this": true, "super": true,
}

var primitiveTypes = map[string]bool{
	"string": true, "number": true, "boolean": true, "void": true,
	"null": true, "undefined": true, "never": true, "any": true,
	"unknown": true, "object": true, "symbol": true, "bigint": true,
}

var builtinTypes = map[string]bool{
	"Partial": true, "Required": true, "Readonly": true, "Record": true,
	"Pick": true, "Omit": true, "Exclude": true, "Extract": true,
	"NonNullable": true, "Parameters": true, "ConstructorParameters": true,
	"ReturnType": true, "InstanceType": true, "ThisParameterType": true,
	"OmitThisParameter": true, "ThisType": true,
	"Uppercase": true, "Lowercase": true, "Capitalize": true, "Uncapitalize": true,
	"Array": true, "Object": true, "String": true, "Number": true,
	"Boolean": true, "Function": true, "Symbol": true, "BigInt": true,
	"Date": true, "RegExp": true, "Error": true,
	"Map": true, "Set": true, "WeakMap": true, "WeakSet": true,
	"Promise": true, "PromiseLike": true,
	"ArrayLike": true, "Iterable": true, "IterableIterator": true,
}

// isBuiltinClass returns true if the class is a JavaScript built-in
func isBuiltinClass(name string) bool {
	return builtinClasses[name]
}

// isBuiltinObject returns true if the object is a JavaScript built-in
func isBuiltinObject(name string) bool {
	return builtinObjects[name]
}

// isPrimitiveType returns true if the type is a primitive TypeScript type
func isPrimitiveType(name string) bool {
	return primitiveTypes[name]
}

// isBuiltinType returns true if the type is a built-in TypeScript utility type
func isBuiltinType(name string) bool {
	return builtinTypes[name]
}
