package parser

import "fmt"

// NodeType represents the type of AST node
type NodeType string

// JavaScript/TypeScript AST node types
const (
	// Program and structure
	NodeProgram NodeType = "Program"
	NodeScript  NodeType = "Script"

	// Function declarations
	NodeFunction           NodeType = "FunctionDeclaration"
	NodeFunctionExpression NodeType = "FunctionExpression"
	NodeArrowFunction      NodeType = "ArrowFunctionExpression"
	NodeAsyncFunction      NodeType = "AsyncFunctionDeclaration"
	NodeGeneratorFunction  NodeType = "GeneratorFunctionDeclaration"
	NodeMethodDefinition   NodeType = "MethodDefinition"

	// Class declarations
	NodeClass           NodeType = "ClassDeclaration"
	NodeClassExpression NodeType = "ClassExpression"

	// Variable declarations
	NodeVariableDeclaration NodeType = "VariableDeclaration"
	NodeVariableDeclarator  NodeType = "VariableDeclarator"
	NodeIdentifier          NodeType = "Identifier"

	// Control flow statements
	NodeIfStatement       NodeType = "IfStatement"
	NodeSwitchStatement   NodeType = "SwitchStatement"
	NodeCaseClause        NodeType = "SwitchCase"
	NodeDefaultClause     NodeType = "SwitchDefault"
	NodeForStatement      NodeType = "ForStatement"
	NodeForInStatement    NodeType = "ForInStatement"
	NodeForOfStatement    NodeType = "ForOfStatement"
	NodeWhileStatement    NodeType = "WhileStatement"
	NodeDoWhileStatement  NodeType = "DoWhileStatement"
	NodeBreakStatement    NodeType = "BreakStatement"
	NodeContinueStatement NodeType = "ContinueStatement"
	NodeReturnStatement   NodeType = "ReturnStatement"
	NodeThrowStatement    NodeType = "ThrowStatement"

	// Exception handling
	NodeTryStatement  NodeType = "TryStatement"
	NodeCatchClause   NodeType = "CatchClause"
	NodeFinallyClause NodeType = "FinallyClause"

	// Expressions
	NodeCallExpression        NodeType = "CallExpression"
	NodeMemberExpression      NodeType = "MemberExpression"
	NodeBinaryExpression      NodeType = "BinaryExpression"
	NodeUnaryExpression       NodeType = "UnaryExpression"
	NodeLogicalExpression     NodeType = "LogicalExpression"
	NodeConditionalExpression NodeType = "ConditionalExpression"
	NodeAssignmentExpression  NodeType = "AssignmentExpression"
	NodeUpdateExpression      NodeType = "UpdateExpression"
	NodeNewExpression         NodeType = "NewExpression"
	NodeThisExpression        NodeType = "ThisExpression"
	NodeSequenceExpression    NodeType = "SequenceExpression"
	NodeAwaitExpression       NodeType = "AwaitExpression"
	NodeYieldExpression       NodeType = "YieldExpression"
	NodeSpreadElement         NodeType = "SpreadElement"
	NodeTemplateLiteral       NodeType = "TemplateLiteral"

	// Literals
	NodeLiteral          NodeType = "Literal"
	NodeStringLiteral    NodeType = "StringLiteral"
	NodeNumberLiteral    NodeType = "NumberLiteral"
	NodeBooleanLiteral   NodeType = "BooleanLiteral"
	NodeNullLiteral      NodeType = "NullLiteral"
	NodeRegExpLiteral    NodeType = "RegExpLiteral"
	NodeArrayExpression  NodeType = "ArrayExpression"
	NodeObjectExpression NodeType = "ObjectExpression"
	NodeProperty         NodeType = "Property"

	// Module system (ESM)
	// NodeImport is the `import` keyword used as an expression, either as the
	// callee of a dynamic import call or as the object of `import.meta`.
	NodeImport                   NodeType = "Import"
	NodeImportDeclaration        NodeType = "ImportDeclaration"
	NodeImportSpecifier          NodeType = "ImportSpecifier"
	NodeImportDefaultSpecifier   NodeType = "ImportDefaultSpecifier"
	NodeImportNamespaceSpecifier NodeType = "ImportNamespaceSpecifier"
	NodeExportNamedDeclaration   NodeType = "ExportNamedDeclaration"
	NodeExportDefaultDeclaration NodeType = "ExportDefaultDeclaration"
	NodeExportAllDeclaration     NodeType = "ExportAllDeclaration"
	NodeExportSpecifier          NodeType = "ExportSpecifier"

	// Module system (CommonJS)
	NodeRequireCall   NodeType = "RequireCall"
	NodeModuleExports NodeType = "ModuleExports"

	// Other statements
	NodeExpressionStatement NodeType = "ExpressionStatement"
	NodeBlockStatement      NodeType = "BlockStatement"
	NodeEmptyStatement      NodeType = "EmptyStatement"
	NodeLabeledStatement    NodeType = "LabeledStatement"
	NodeWithStatement       NodeType = "WithStatement"
	NodeDebuggerStatement   NodeType = "DebuggerStatement"

	// TypeScript-specific nodes
	NodeInterfaceDeclaration NodeType = "InterfaceDeclaration"
	NodeTypeAlias            NodeType = "TypeAliasDeclaration"
	NodeEnumDeclaration      NodeType = "EnumDeclaration"
	NodeTypeAnnotation       NodeType = "TypeAnnotation"
	NodeTypeParameter        NodeType = "TypeParameter"
	NodeImportType           NodeType = "ImportType"
	NodeAsExpression         NodeType = "AsExpression"
	NodeNonNullExpression    NodeType = "NonNullExpression"

	// JSX (if needed)
	NodeJSXElement   NodeType = "JSXElement"
	NodeJSXFragment  NodeType = "JSXFragment"
	NodeJSXAttribute NodeType = "JSXAttribute"

	// Tree-sitter specific structural nodes
	NodeStatementBlock NodeType = "StatementBlock"
	NodeElseClause     NodeType = "ElseClause"
)

// Location represents the position of a node in the source code
type Location struct {
	File      string
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
}

// String returns a string representation of the location
func (l Location) String() string {
	return fmt.Sprintf("%s:%d:%d", l.File, l.StartLine, l.StartCol)
}

// Node represents an AST node
type Node struct {
	Type     NodeType
	Value    interface{} // Can hold various values depending on node type
	Children []*Node
	Location Location
	Parent   *Node

	// Common fields for various node types
	Name string // For function/class/variable names

	// Function-related fields
	Params    []*Node // Function parameters
	Body      []*Node // Function/block body
	Async     bool    // Async function
	Generator bool    // Generator function

	// Control flow fields
	Test       *Node   // Condition for if/while/for
	Consequent *Node   // Then branch for if
	Alternate  *Node   // Else branch for if
	Init       *Node   // For loop initializer
	Update     *Node   // For loop update
	Cases      []*Node // Switch cases

	// Try-catch fields
	Handler   *Node   // Catch clause
	Finalizer *Node   // Finally block
	Handlers  []*Node // Multiple catch handlers

	// Expression fields
	Left      *Node   // Left operand
	Right     *Node   // Right operand
	Operator  string  // Operator (+, -, *, etc.)
	Argument  *Node   // Unary expression argument
	Arguments []*Node // Function call arguments
	Callee    *Node   // Function being called
	Object    *Node   // Object in member expression
	Property  *Node   // Property in member expression

	// Variable declaration fields
	Kind         string  // var, let, const
	Declarations []*Node // Variable declarators

	// Import/Export fields
	Source      *Node   // Import source
	Specifiers  []*Node // Import/export specifiers
	Declaration *Node   // Export declaration
	Imported    *Node   // Imported name
	Local       *Node   // Local binding

	// TypeScript fields
	TypeAnnotation *Node   // Type annotation
	TypeParameters []*Node // Generic type parameters

	// Utility fields
	Computed bool   // Computed property
	Optional bool   // Optional chaining
	Raw      string // Raw literal value
}

// NewNode creates a new AST node
func NewNode(nodeType NodeType) *Node {
	return &Node{
		Type:           nodeType,
		Children:       []*Node{},
		Params:         []*Node{},
		Body:           []*Node{},
		Cases:          []*Node{},
		Handlers:       []*Node{},
		Arguments:      []*Node{},
		Declarations:   []*Node{},
		Specifiers:     []*Node{},
		TypeParameters: []*Node{},
	}
}

// AddChild adds a child node
func (n *Node) AddChild(child *Node) {
	if child == nil {
		return
	}
	child.Parent = n
	n.Children = append(n.Children, child)
}

// Walk traverses the AST depth-first and calls the visitor function for each node
// If the visitor returns false, traversal of that branch is stopped
func (n *Node) Walk(visitor func(*Node) bool) {
	if n == nil {
		return
	}

	if !visitor(n) {
		return
	}

	for _, child := range n.Children {
		child.Walk(visitor)
	}
	for _, param := range n.Params {
		param.Walk(visitor)
	}
	for _, stmt := range n.Body {
		stmt.Walk(visitor)
	}
	for _, caseNode := range n.Cases {
		caseNode.Walk(visitor)
	}
	for _, handler := range n.Handlers {
		handler.Walk(visitor)
	}
	for _, arg := range n.Arguments {
		arg.Walk(visitor)
	}
	for _, decl := range n.Declarations {
		decl.Walk(visitor)
	}
	for _, spec := range n.Specifiers {
		spec.Walk(visitor)
	}

	// Walk individual nodes
	if n.Test != nil {
		n.Test.Walk(visitor)
	}
	if n.Consequent != nil {
		n.Consequent.Walk(visitor)
	}
	if n.Alternate != nil {
		n.Alternate.Walk(visitor)
	}
	if n.Init != nil {
		n.Init.Walk(visitor)
	}
	if n.Update != nil {
		n.Update.Walk(visitor)
	}
	if n.Handler != nil {
		n.Handler.Walk(visitor)
	}
	if n.Finalizer != nil {
		n.Finalizer.Walk(visitor)
	}
	if n.Left != nil {
		n.Left.Walk(visitor)
	}
	if n.Right != nil {
		n.Right.Walk(visitor)
	}
	if n.Argument != nil {
		n.Argument.Walk(visitor)
	}
	if n.Callee != nil {
		n.Callee.Walk(visitor)
	}
	if n.Object != nil {
		n.Object.Walk(visitor)
	}
	if n.Property != nil {
		n.Property.Walk(visitor)
	}
	if n.Source != nil {
		n.Source.Walk(visitor)
	}
	if n.Declaration != nil {
		n.Declaration.Walk(visitor)
	}
	if n.TypeAnnotation != nil {
		n.TypeAnnotation.Walk(visitor)
	}
}

// GetChildren returns all child nodes in the parser's canonical order.
func (n *Node) GetChildren() []*Node {
	return OrderedChildren(n)
}

// OrderedChildren returns every structural child exactly once in a stable
// order. Expression operands come before statement bodies and branches so the
// resulting order roughly follows source evaluation order.
func OrderedChildren(node *Node) []*Node {
	if node == nil {
		return nil
	}

	children := make([]*Node, 0, len(node.Children)+len(node.Body)+len(node.Params))
	seen := make(map[*Node]struct{})

	appendNode := func(child *Node) {
		if child == nil {
			return
		}
		if _, ok := seen[child]; ok {
			return
		}
		seen[child] = struct{}{}
		children = append(children, child)
	}
	appendNodes := func(nodes []*Node) {
		for _, child := range nodes {
			appendNode(child)
		}
	}

	appendNodes(node.Children)
	appendNodes(node.Params)
	appendNodes(node.TypeParameters)
	appendNode(node.Init)
	appendNode(node.Test)
	appendNode(node.Update)
	appendNode(node.Left)
	appendNode(node.Right)
	appendNode(node.Argument)
	appendNode(node.Callee)
	appendNode(node.Object)
	appendNode(node.Property)
	appendNodes(node.Arguments)
	appendNodes(node.Declarations)
	appendNode(node.Source)
	appendNodes(node.Specifiers)
	appendNode(node.Imported)
	appendNode(node.Local)
	appendNode(node.Declaration)
	appendNode(node.TypeAnnotation)
	appendNodes(node.Body)
	appendNode(node.Consequent)
	appendNode(node.Alternate)
	appendNodes(node.Cases)
	appendNode(node.Handler)
	appendNodes(node.Handlers)
	appendNode(node.Finalizer)

	return children
}

// String returns a string representation of the node
func (n *Node) String() string {
	if n.Name != "" {
		return fmt.Sprintf("%s(%s) at %s", n.Type, n.Name, n.Location)
	}
	return fmt.Sprintf("%s at %s", n.Type, n.Location)
}

// IsStatement returns true if the node is a statement
func (n *Node) IsStatement() bool {
	switch n.Type {
	case NodeIfStatement, NodeSwitchStatement,
		NodeForStatement, NodeForInStatement, NodeForOfStatement,
		NodeWhileStatement, NodeDoWhileStatement,
		NodeTryStatement, NodeReturnStatement, NodeThrowStatement,
		NodeBreakStatement, NodeContinueStatement,
		NodeVariableDeclaration, NodeFunctionExpression,
		NodeExpressionStatement, NodeBlockStatement:
		return true
	}
	return false
}

// IsExpression returns true if the node is an expression
func (n *Node) IsExpression() bool {
	switch n.Type {
	case NodeCallExpression, NodeMemberExpression,
		NodeBinaryExpression, NodeUnaryExpression,
		NodeLogicalExpression, NodeConditionalExpression,
		NodeAssignmentExpression, NodeUpdateExpression,
		NodeNewExpression, NodeAwaitExpression, NodeYieldExpression,
		NodeIdentifier, NodeLiteral, NodeArrayExpression, NodeObjectExpression:
		return true
	}
	return false
}

// IsFunction returns true if the node is a function
func (n *Node) IsFunction() bool {
	switch n.Type {
	case NodeFunction, NodeArrowFunction, NodeAsyncFunction, NodeGeneratorFunction,
		NodeFunctionExpression, NodeMethodDefinition:
		return true
	}
	return false
}
