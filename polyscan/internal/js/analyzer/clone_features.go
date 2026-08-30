package analyzer

// jsClonePatternNames are the JS/TS AST constructs surfaced as structural
// pattern features for clone feature extraction (core's extractor is
// language-neutral and emits pattern features only for configured names).
var jsClonePatternNames = []string{
	// Control flow
	"IfStatement", "SwitchStatement", "ForStatement", "ForInStatement",
	"ForOfStatement", "WhileStatement", "DoWhileStatement", "TryStatement",
	"WithStatement",
	// Functions
	"FunctionDeclaration", "FunctionExpression", "ArrowFunctionExpression",
	"MethodDefinition", "AsyncFunctionDeclaration", "GeneratorFunctionDeclaration",
	// Classes
	"ClassDeclaration", "ClassExpression",
	// Statements
	"ReturnStatement", "ThrowStatement", "BreakStatement", "ContinueStatement",
	"VariableDeclaration", "AssignmentExpression",
	// Expressions
	"CallExpression", "MemberExpression", "BinaryExpression", "LogicalExpression",
	"ConditionalExpression", "NewExpression", "AwaitExpression", "YieldExpression",
	// Modules
	"ImportDeclaration", "ExportNamedDeclaration", "ExportDefaultDeclaration",
	// TypeScript
	"InterfaceDeclaration", "TypeAliasDeclaration", "EnumDeclaration",
	// JSX
	"JSXElement", "JSXFragment",
}

// jsLiteralLikeNames are the JS/TS node labels that carry identifier or
// literal payloads; their label features are suppressed when literals are
// excluded so renames and literal changes do not perturb the feature set.
var jsLiteralLikeNames = []string{
	"Identifier", "Literal", "StringLiteral", "NumberLiteral",
	"BooleanLiteral", "NullLiteral", "RegExpLiteral",
}
