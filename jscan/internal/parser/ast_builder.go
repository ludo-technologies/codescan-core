package parser

import (
	sitter "github.com/smacker/go-tree-sitter"
)

// ASTBuilder builds our internal AST from tree-sitter CST
type ASTBuilder struct {
	filename string
	source   []byte
}

// NewASTBuilder creates a new AST builder
func NewASTBuilder(filename string, source []byte) *ASTBuilder {
	return &ASTBuilder{
		filename: filename,
		source:   source,
	}
}

// Build builds the AST from a tree-sitter node
func (b *ASTBuilder) Build(tsNode *sitter.Node) *Node {
	if tsNode == nil {
		return nil
	}

	node := b.buildNode(tsNode)
	return node
}

// buildNode converts a tree-sitter node to our internal AST node
func (b *ASTBuilder) buildNode(tsNode *sitter.Node) *Node {
	if tsNode == nil {
		return nil
	}

	nodeType := tsNode.Type()
	node := NewNode(NodeType(nodeType))

	// Set location information
	node.Location = Location{
		File:      b.filename,
		StartLine: int(tsNode.StartPoint().Row) + 1,
		StartCol:  int(tsNode.StartPoint().Column),
		EndLine:   int(tsNode.EndPoint().Row) + 1,
		EndCol:    int(tsNode.EndPoint().Column),
	}

	// Map tree-sitter node types to our AST node types
	switch nodeType {
	case "program":
		return b.buildProgram(tsNode)
	case "function_declaration", "function":
		return b.buildFunctionDeclaration(tsNode)
	case "arrow_function":
		return b.buildArrowFunction(tsNode)
	case "function_expression":
		return b.buildFunctionExpression(tsNode)
	case "generator_function_declaration":
		return b.buildGeneratorFunction(tsNode)
	case "method_definition":
		return b.buildMethodDefinition(tsNode)
	case "class_declaration":
		return b.buildClassDeclaration(tsNode)
	case "if_statement":
		return b.buildIfStatement(tsNode)
	case "switch_statement":
		return b.buildSwitchStatement(tsNode)
	case "switch_case":
		return b.buildSwitchCase(tsNode)
	case "switch_default":
		return b.buildSwitchDefault(tsNode)
	case "for_statement":
		return b.buildForStatement(tsNode)
	case "for_in_statement":
		return b.buildForInStatement(tsNode)
	case "for_of_statement": // Proper handling for for-of
		return b.buildForOfStatement(tsNode)
	case "while_statement":
		return b.buildWhileStatement(tsNode)
	case "do_statement":
		return b.buildDoWhileStatement(tsNode)
	case "try_statement":
		return b.buildTryStatement(tsNode)
	case "catch_clause":
		return b.buildCatchClause(tsNode)
	case "finally_clause":
		return b.buildFinallyClause(tsNode)
	case "return_statement":
		return b.buildReturnStatement(tsNode)
	case "break_statement":
		return b.buildBreakStatement(tsNode)
	case "continue_statement":
		return b.buildContinueStatement(tsNode)
	case "throw_statement":
		return b.buildThrowStatement(tsNode)
	case "variable_declaration":
		return b.buildVariableDeclaration(tsNode)
	case "lexical_declaration":
		return b.buildVariableDeclaration(tsNode)
	case "expression_statement":
		return b.buildExpressionStatement(tsNode)
	case "empty_statement":
		node := NewNode(NodeEmptyStatement)
		node.Location = b.getLocation(tsNode)
		return node
	case "call_expression":
		return b.buildCallExpression(tsNode)
	case "member_expression":
		return b.buildMemberExpression(tsNode)
	case "binary_expression":
		return b.buildBinaryExpression(tsNode)
	case "unary_expression":
		return b.buildUnaryExpression(tsNode)
	case "update_expression":
		return b.buildUpdateExpression(tsNode)
	case "assignment_expression":
		return b.buildAssignmentExpression(tsNode)
	case "conditional_expression", "ternary_expression":
		return b.buildConditionalExpression(tsNode)
	case "await_expression":
		return b.buildAwaitExpression(tsNode)
	case "yield_expression":
		return b.buildYieldExpression(tsNode)
	case "identifier", "property_identifier", "shorthand_property_identifier", "type_identifier":
		return b.buildIdentifier(tsNode)
	case "string", "number", "true", "false", "null":
		return b.buildLiteral(tsNode)
	case "import_statement":
		return b.buildImportStatement(tsNode)
	case "export_statement":
		return b.buildExportStatement(tsNode)
	case "statement_block":
		return b.buildBlockStatement(tsNode)
	default:
		// For unknown nodes, create a generic node and process children
		return b.buildGenericNode(tsNode)
	}
}

// buildProgram builds a program node
func (b *ASTBuilder) buildProgram(tsNode *sitter.Node) *Node {
	node := NewNode(NodeProgram)
	node.Location = b.getLocation(tsNode)

	for i := 0; i < int(tsNode.ChildCount()); i++ {
		child := tsNode.Child(i)
		if child != nil && !b.isTrivia(child) {
			childNode := b.buildNode(child)
			if childNode != nil {
				node.AddChild(childNode)
				node.Body = append(node.Body, childNode)
			}
		}
	}

	return node
}

// buildFunctionDeclaration builds a function declaration node
func (b *ASTBuilder) buildFunctionDeclaration(tsNode *sitter.Node) *Node {
	node := NewNode(NodeFunction)
	node.Location = b.getLocation(tsNode)

	// Extract function name
	if nameNode := b.getChildByFieldName(tsNode, "name"); nameNode != nil {
		node.Name = nameNode.Content(b.source)
	}

	// Extract parameters
	if paramsNode := b.getChildByFieldName(tsNode, "parameters"); paramsNode != nil {
		node.Params = b.buildParameters(paramsNode)
	}

	// Extract body
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		bodyAST := b.buildNode(bodyNode)
		if bodyAST != nil {
			node.Body = bodyAST.Body
		}
	}

	return node
}

// buildArrowFunction builds an arrow function node
func (b *ASTBuilder) buildArrowFunction(tsNode *sitter.Node) *Node {
	node := NewNode(NodeArrowFunction)
	node.Location = b.getLocation(tsNode)

	// Extract parameters
	if paramsNode := b.getChildByFieldName(tsNode, "parameter"); paramsNode != nil {
		// Single parameter without parentheses
		param := b.buildNode(paramsNode)
		if param != nil {
			node.Params = []*Node{param}
		}
	} else if paramsNode := b.getChildByFieldName(tsNode, "parameters"); paramsNode != nil {
		// Multiple parameters with parentheses
		node.Params = b.buildParameters(paramsNode)
	}

	// Extract body
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		bodyAST := b.buildNode(bodyNode)
		if bodyAST != nil {
			if bodyAST.Type == NodeBlockStatement {
				node.Body = bodyAST.Body
			} else {
				// Expression body
				node.Body = []*Node{bodyAST}
			}
		}
	}

	return node
}

// buildFunctionExpression builds a function expression node
func (b *ASTBuilder) buildFunctionExpression(tsNode *sitter.Node) *Node {
	node := NewNode(NodeFunctionExpression)
	node.Location = b.getLocation(tsNode)

	// Extract function name (optional)
	if nameNode := b.getChildByFieldName(tsNode, "name"); nameNode != nil {
		node.Name = nameNode.Content(b.source)
	}

	// Extract parameters
	if paramsNode := b.getChildByFieldName(tsNode, "parameters"); paramsNode != nil {
		node.Params = b.buildParameters(paramsNode)
	}

	// Extract body
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		bodyAST := b.buildNode(bodyNode)
		if bodyAST != nil {
			node.Body = bodyAST.Body
		}
	}

	return node
}

// buildGeneratorFunction builds a generator function node
func (b *ASTBuilder) buildGeneratorFunction(tsNode *sitter.Node) *Node {
	node := NewNode(NodeGeneratorFunction)
	node.Location = b.getLocation(tsNode)
	node.Generator = true

	// Extract function name
	if nameNode := b.getChildByFieldName(tsNode, "name"); nameNode != nil {
		node.Name = nameNode.Content(b.source)
	}

	// Extract parameters
	if paramsNode := b.getChildByFieldName(tsNode, "parameters"); paramsNode != nil {
		node.Params = b.buildParameters(paramsNode)
	}

	// Extract body
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		bodyAST := b.buildNode(bodyNode)
		if bodyAST != nil {
			node.Body = bodyAST.Body
		}
	}

	return node
}

// buildMethodDefinition builds a method definition node
func (b *ASTBuilder) buildMethodDefinition(tsNode *sitter.Node) *Node {
	node := NewNode(NodeMethodDefinition)
	node.Location = b.getLocation(tsNode)

	// Extract method name
	if nameNode := b.getChildByFieldName(tsNode, "name"); nameNode != nil {
		node.Name = nameNode.Content(b.source)
	}

	// Extract parameters
	if paramsNode := b.getChildByFieldName(tsNode, "parameters"); paramsNode != nil {
		node.Params = b.buildParameters(paramsNode)
	}

	// Extract body
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		bodyAST := b.buildNode(bodyNode)
		if bodyAST != nil {
			node.Body = bodyAST.Body
		}
	}

	return node
}

// buildClassDeclaration builds a class declaration node
func (b *ASTBuilder) buildClassDeclaration(tsNode *sitter.Node) *Node {
	node := NewNode(NodeClass)
	node.Location = b.getLocation(tsNode)

	// Extract class name
	if nameNode := b.getChildByFieldName(tsNode, "name"); nameNode != nil {
		node.Name = nameNode.Content(b.source)
	}

	// Extract class body
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		for i := 0; i < int(bodyNode.ChildCount()); i++ {
			child := bodyNode.Child(i)
			if child != nil && !b.isTrivia(child) {
				childAST := b.buildNode(child)
				if childAST != nil {
					node.Body = append(node.Body, childAST)
				}
			}
		}
	}

	return node
}

// buildIfStatement builds an if statement node
func (b *ASTBuilder) buildIfStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeIfStatement)
	node.Location = b.getLocation(tsNode)

	// Extract condition
	if condNode := b.getChildByFieldName(tsNode, "condition"); condNode != nil {
		node.Test = b.buildNode(condNode)
	}

	// Extract consequence (then branch)
	if consNode := b.getChildByFieldName(tsNode, "consequence"); consNode != nil {
		node.Consequent = b.buildNode(consNode)
	}

	// Extract alternative (else branch)
	if altNode := b.getChildByFieldName(tsNode, "alternative"); altNode != nil {
		node.Alternate = b.buildNode(altNode)
	}

	return node
}

// buildSwitchStatement builds a switch statement node
func (b *ASTBuilder) buildSwitchStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeSwitchStatement)
	node.Location = b.getLocation(tsNode)

	// Extract discriminant (value being switched on)
	if valueNode := b.getChildByFieldName(tsNode, "value"); valueNode != nil {
		node.Test = b.buildNode(valueNode)
	}

	// Extract cases. The switch body also contains its braces, so keep only
	// the nodes that are actual case or default clauses.
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		for i := 0; i < int(bodyNode.ChildCount()); i++ {
			child := bodyNode.Child(i)
			if child == nil || b.isTrivia(child) {
				continue
			}
			caseNode := b.buildNode(child)
			if caseNode == nil {
				continue
			}
			if caseNode.Type != NodeCaseClause && caseNode.Type != NodeDefaultClause {
				continue
			}
			node.Cases = append(node.Cases, caseNode)
		}
	}

	return node
}

// buildSwitchCase builds a switch case node
func (b *ASTBuilder) buildSwitchCase(tsNode *sitter.Node) *Node {
	node := NewNode(NodeCaseClause)
	node.Location = b.getLocation(tsNode)

	// Extract case value
	if valueNode := b.getChildByFieldName(tsNode, "value"); valueNode != nil {
		node.Test = b.buildNode(valueNode)
	}

	// Extract case body
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		node.Body = []*Node{b.buildNode(bodyNode)}
	} else {
		// Extract all children as body statements
		for i := 0; i < int(tsNode.ChildCount()); i++ {
			child := tsNode.Child(i)
			if child != nil && !b.isTrivia(child) && child.Type() != "case" && child.Type() != ":" {
				childNode := b.buildNode(child)
				if childNode != nil {
					node.Body = append(node.Body, childNode)
				}
			}
		}
	}

	return node
}

// buildSwitchDefault builds a switch default node
func (b *ASTBuilder) buildSwitchDefault(tsNode *sitter.Node) *Node {
	node := NewNode(NodeDefaultClause)
	node.Location = b.getLocation(tsNode)

	// Extract default body
	for i := 0; i < int(tsNode.ChildCount()); i++ {
		child := tsNode.Child(i)
		if child != nil && !b.isTrivia(child) && child.Type() != "default" && child.Type() != ":" {
			childNode := b.buildNode(child)
			if childNode != nil {
				node.Body = append(node.Body, childNode)
			}
		}
	}

	return node
}

// buildForStatement builds a for statement node
func (b *ASTBuilder) buildForStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeForStatement)
	node.Location = b.getLocation(tsNode)

	// Extract initializer
	if initNode := b.getChildByFieldName(tsNode, "initializer"); initNode != nil {
		node.Init = b.buildNode(initNode)
	}

	// Extract condition
	if condNode := b.getChildByFieldName(tsNode, "condition"); condNode != nil {
		node.Test = b.buildNode(condNode)
	}

	// Extract increment
	if incrNode := b.getChildByFieldName(tsNode, "increment"); incrNode != nil {
		node.Update = b.buildNode(incrNode)
	}

	// Extract body
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		node.Body = []*Node{b.buildNode(bodyNode)}
	}

	return node
}

// buildForInStatement builds a for-in statement node
func (b *ASTBuilder) buildForInStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeForInStatement)
	node.Location = b.getLocation(tsNode)

	// Extract left (variable)
	if leftNode := b.getChildByFieldName(tsNode, "left"); leftNode != nil {
		node.Init = b.buildNode(leftNode)
	}

	// Extract right (object)
	if rightNode := b.getChildByFieldName(tsNode, "right"); rightNode != nil {
		node.Test = b.buildNode(rightNode)
	}

	// Extract body
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		node.Body = []*Node{b.buildNode(bodyNode)}
	}

	return node
}

// buildForOfStatement builds a for-of statement node
func (b *ASTBuilder) buildForOfStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeForOfStatement)
	node.Location = b.getLocation(tsNode)

	// Extract left (variable)
	if leftNode := b.getChildByFieldName(tsNode, "left"); leftNode != nil {
		node.Init = b.buildNode(leftNode)
	}

	// Extract right (iterable)
	if rightNode := b.getChildByFieldName(tsNode, "right"); rightNode != nil {
		node.Test = b.buildNode(rightNode)
	}

	// Extract body
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		node.Body = []*Node{b.buildNode(bodyNode)}
	}

	return node
}

// buildWhileStatement builds a while statement node
func (b *ASTBuilder) buildWhileStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeWhileStatement)
	node.Location = b.getLocation(tsNode)

	// Extract condition
	if condNode := b.getChildByFieldName(tsNode, "condition"); condNode != nil {
		node.Test = b.buildNode(condNode)
	}

	// Extract body
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		node.Body = []*Node{b.buildNode(bodyNode)}
	}

	return node
}

// buildDoWhileStatement builds a do-while statement node
func (b *ASTBuilder) buildDoWhileStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeDoWhileStatement)
	node.Location = b.getLocation(tsNode)

	// Extract body
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		node.Body = []*Node{b.buildNode(bodyNode)}
	}

	// Extract condition
	if condNode := b.getChildByFieldName(tsNode, "condition"); condNode != nil {
		node.Test = b.buildNode(condNode)
	}

	return node
}

// buildTryStatement builds a try statement node
func (b *ASTBuilder) buildTryStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeTryStatement)
	node.Location = b.getLocation(tsNode)

	// Extract try body
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		bodyAST := b.buildNode(bodyNode)
		if bodyAST != nil {
			node.Body = bodyAST.Body
		}
	}

	// Extract catch clause
	if handlerNode := b.getChildByFieldName(tsNode, "handler"); handlerNode != nil {
		node.Handler = b.buildNode(handlerNode)
	}

	// Extract finally clause
	if finalizerNode := b.getChildByFieldName(tsNode, "finalizer"); finalizerNode != nil {
		node.Finalizer = b.buildNode(finalizerNode)
	}

	return node
}

// buildCatchClause builds a catch clause node
func (b *ASTBuilder) buildCatchClause(tsNode *sitter.Node) *Node {
	node := NewNode(NodeCatchClause)
	node.Location = b.getLocation(tsNode)

	// Extract parameter (error variable)
	if paramNode := b.getChildByFieldName(tsNode, "parameter"); paramNode != nil {
		node.Params = []*Node{b.buildNode(paramNode)}
	}

	// Extract body
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		bodyAST := b.buildNode(bodyNode)
		if bodyAST != nil {
			node.Body = bodyAST.Body
		}
	}

	return node
}

// buildFinallyClause builds a finally clause node
func (b *ASTBuilder) buildFinallyClause(tsNode *sitter.Node) *Node {
	node := NewNode(NodeFinallyClause)
	node.Location = b.getLocation(tsNode)

	// Extract body
	if bodyNode := b.getChildByFieldName(tsNode, "body"); bodyNode != nil {
		bodyAST := b.buildNode(bodyNode)
		if bodyAST != nil {
			node.Body = bodyAST.Body
		}
	}

	return node
}

// buildReturnStatement builds a return statement node
func (b *ASTBuilder) buildReturnStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeReturnStatement)
	node.Location = b.getLocation(tsNode)

	// Extract return value
	for i := 0; i < int(tsNode.ChildCount()); i++ {
		child := tsNode.Child(i)
		if child != nil && !b.isTrivia(child) && child.Type() != "return" {
			node.Argument = b.buildNode(child)
			break
		}
	}

	return node
}

// buildBreakStatement builds a break statement node
func (b *ASTBuilder) buildBreakStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeBreakStatement)
	node.Location = b.getLocation(tsNode)
	return node
}

// buildContinueStatement builds a continue statement node
func (b *ASTBuilder) buildContinueStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeContinueStatement)
	node.Location = b.getLocation(tsNode)
	return node
}

// buildThrowStatement builds a throw statement node
func (b *ASTBuilder) buildThrowStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeThrowStatement)
	node.Location = b.getLocation(tsNode)

	// Extract thrown value
	for i := 0; i < int(tsNode.ChildCount()); i++ {
		child := tsNode.Child(i)
		if child != nil && !b.isTrivia(child) && child.Type() != "throw" {
			node.Argument = b.buildNode(child)
			break
		}
	}

	return node
}

// buildVariableDeclaration builds a variable declaration node
func (b *ASTBuilder) buildVariableDeclaration(tsNode *sitter.Node) *Node {
	node := NewNode(NodeVariableDeclaration)
	node.Location = b.getLocation(tsNode)

	// Extract kind (var, let, const)
	if tsNode.Type() == "lexical_declaration" {
		// For lexical declarations, check first child for kind
		if tsNode.ChildCount() > 0 {
			firstChild := tsNode.Child(0)
			if firstChild != nil {
				kind := firstChild.Content(b.source)
				if kind == "let" || kind == "const" {
					node.Kind = kind
				}
			}
		}
	} else {
		node.Kind = "var"
	}

	// Extract declarators
	for i := 0; i < int(tsNode.ChildCount()); i++ {
		child := tsNode.Child(i)
		if child != nil && child.Type() == "variable_declarator" {
			declNode := b.buildNode(child)
			if declNode != nil {
				node.Declarations = append(node.Declarations, declNode)
			}
		}
	}

	return node
}

// buildExpressionStatement builds an expression statement node
func (b *ASTBuilder) buildExpressionStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeExpressionStatement)
	node.Location = b.getLocation(tsNode)

	// Extract the expression
	for i := 0; i < int(tsNode.ChildCount()); i++ {
		child := tsNode.Child(i)
		if child != nil && !b.isTrivia(child) && child.Type() != ";" {
			return b.buildNode(child)
		}
	}

	return node
}

// buildCallExpression builds a call expression node
func (b *ASTBuilder) buildCallExpression(tsNode *sitter.Node) *Node {
	node := NewNode(NodeCallExpression)
	node.Location = b.getLocation(tsNode)

	// Extract function being called
	if funcNode := b.getChildByFieldName(tsNode, "function"); funcNode != nil {
		node.Callee = b.buildNode(funcNode)
	}

	// Extract arguments
	if argsNode := b.getChildByFieldName(tsNode, "arguments"); argsNode != nil {
		for i := 0; i < int(argsNode.ChildCount()); i++ {
			child := argsNode.Child(i)
			if child != nil && !b.isTrivia(child) && child.Type() != "(" && child.Type() != ")" && child.Type() != "," {
				argNode := b.buildNode(child)
				if argNode != nil {
					node.Arguments = append(node.Arguments, argNode)
				}
			}
		}
	}

	return node
}

// buildMemberExpression builds a member expression node
func (b *ASTBuilder) buildMemberExpression(tsNode *sitter.Node) *Node {
	node := NewNode(NodeMemberExpression)
	node.Location = b.getLocation(tsNode)

	// Extract object
	if objNode := b.getChildByFieldName(tsNode, "object"); objNode != nil {
		node.Object = b.buildNode(objNode)
	}

	// Extract property
	if propNode := b.getChildByFieldName(tsNode, "property"); propNode != nil {
		node.Property = b.buildNode(propNode)
	}

	return node
}

// buildBinaryExpression builds a binary expression node
func (b *ASTBuilder) buildBinaryExpression(tsNode *sitter.Node) *Node {
	node := NewNode(NodeBinaryExpression)
	node.Location = b.getLocation(tsNode)

	// Extract left operand
	if leftNode := b.getChildByFieldName(tsNode, "left"); leftNode != nil {
		node.Left = b.buildNode(leftNode)
	}

	// Extract operator
	if opNode := b.getChildByFieldName(tsNode, "operator"); opNode != nil {
		node.Operator = opNode.Content(b.source)
	} else {
		// Try to find operator as a child
		for i := 0; i < int(tsNode.ChildCount()); i++ {
			child := tsNode.Child(i)
			if child != nil && b.isOperator(child.Type()) {
				node.Operator = child.Content(b.source)
				break
			}
		}
	}

	// Extract right operand
	if rightNode := b.getChildByFieldName(tsNode, "right"); rightNode != nil {
		node.Right = b.buildNode(rightNode)
	}

	// Check if it's a logical operator (&&, ||, ??)
	if node.Operator == "&&" || node.Operator == "||" || node.Operator == "??" {
		node.Type = NodeLogicalExpression
	}

	return node
}

// buildUnaryExpression builds a unary expression node
func (b *ASTBuilder) buildUnaryExpression(tsNode *sitter.Node) *Node {
	node := NewNode(NodeUnaryExpression)
	node.Location = b.getLocation(tsNode)

	// Extract operator
	if opNode := b.getChildByFieldName(tsNode, "operator"); opNode != nil {
		node.Operator = opNode.Content(b.source)
	}

	// Extract argument
	if argNode := b.getChildByFieldName(tsNode, "argument"); argNode != nil {
		node.Argument = b.buildNode(argNode)
	}

	return node
}

// buildUpdateExpression builds an update expression node (++, --)
func (b *ASTBuilder) buildUpdateExpression(tsNode *sitter.Node) *Node {
	node := NewNode(NodeUpdateExpression)
	node.Location = b.getLocation(tsNode)

	// Extract operator
	if opNode := b.getChildByFieldName(tsNode, "operator"); opNode != nil {
		node.Operator = opNode.Content(b.source)
	}

	// Extract argument
	if argNode := b.getChildByFieldName(tsNode, "argument"); argNode != nil {
		node.Argument = b.buildNode(argNode)
	}

	return node
}

// buildAssignmentExpression builds an assignment expression node
func (b *ASTBuilder) buildAssignmentExpression(tsNode *sitter.Node) *Node {
	node := NewNode(NodeAssignmentExpression)
	node.Location = b.getLocation(tsNode)

	// Extract left side
	if leftNode := b.getChildByFieldName(tsNode, "left"); leftNode != nil {
		node.Left = b.buildNode(leftNode)
	}

	// Extract operator
	if opNode := b.getChildByFieldName(tsNode, "operator"); opNode != nil {
		node.Operator = opNode.Content(b.source)
	}

	// Extract right side
	if rightNode := b.getChildByFieldName(tsNode, "right"); rightNode != nil {
		node.Right = b.buildNode(rightNode)
	}

	return node
}

// buildConditionalExpression builds a conditional (ternary) expression node
func (b *ASTBuilder) buildConditionalExpression(tsNode *sitter.Node) *Node {
	node := NewNode(NodeConditionalExpression)
	node.Location = b.getLocation(tsNode)

	// Extract condition
	if condNode := b.getChildByFieldName(tsNode, "condition"); condNode != nil {
		node.Test = b.buildNode(condNode)
	}

	// Extract consequence
	if consNode := b.getChildByFieldName(tsNode, "consequence"); consNode != nil {
		node.Consequent = b.buildNode(consNode)
	}

	// Extract alternative
	if altNode := b.getChildByFieldName(tsNode, "alternative"); altNode != nil {
		node.Alternate = b.buildNode(altNode)
	}

	return node
}

// buildAwaitExpression builds an await expression node
func (b *ASTBuilder) buildAwaitExpression(tsNode *sitter.Node) *Node {
	node := NewNode(NodeAwaitExpression)
	node.Location = b.getLocation(tsNode)

	// Extract argument
	for i := 0; i < int(tsNode.ChildCount()); i++ {
		child := tsNode.Child(i)
		if child != nil && !b.isTrivia(child) && child.Type() != "await" {
			node.Argument = b.buildNode(child)
			break
		}
	}

	return node
}

// buildYieldExpression builds a yield expression node
func (b *ASTBuilder) buildYieldExpression(tsNode *sitter.Node) *Node {
	node := NewNode(NodeYieldExpression)
	node.Location = b.getLocation(tsNode)

	// Extract argument
	for i := 0; i < int(tsNode.ChildCount()); i++ {
		child := tsNode.Child(i)
		if child != nil && !b.isTrivia(child) && child.Type() != "yield" && child.Type() != "*" {
			node.Argument = b.buildNode(child)
			break
		}
	}

	return node
}

// buildIdentifier builds an identifier node
func (b *ASTBuilder) buildIdentifier(tsNode *sitter.Node) *Node {
	node := NewNode(NodeIdentifier)
	node.Location = b.getLocation(tsNode)
	node.Name = tsNode.Content(b.source)
	return node
}

// buildLiteral builds a literal node
func (b *ASTBuilder) buildLiteral(tsNode *sitter.Node) *Node {
	node := NewNode(NodeLiteral)
	node.Location = b.getLocation(tsNode)
	node.Raw = tsNode.Content(b.source)

	// Set the value based on type
	switch tsNode.Type() {
	case "string":
		node.Type = NodeStringLiteral
	case "number":
		node.Type = NodeNumberLiteral
	case "true", "false":
		node.Type = NodeBooleanLiteral
	case "null":
		node.Type = NodeNullLiteral
	}

	return node
}

// buildImportStatement builds an import statement node
func (b *ASTBuilder) buildImportStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeImportDeclaration)
	node.Location = b.getLocation(tsNode)

	// Extract source
	if sourceNode := b.getChildByFieldName(tsNode, "source"); sourceNode != nil {
		node.Source = b.buildNode(sourceNode)
	}

	// Extract specifiers from different tree-sitter node types
	for i := 0; i < int(tsNode.ChildCount()); i++ {
		child := tsNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "import_clause":
			// Handle import clause (contains default import and/or named imports)
			b.extractImportClause(child, node)

		case "namespace_import":
			// Handle: import * as name from 'module'
			specNode := NewNode(NodeImportNamespaceSpecifier)
			specNode.Location = b.getLocation(child)
			// Find the identifier (the "as name" part)
			for j := 0; j < int(child.ChildCount()); j++ {
				grandchild := child.Child(j)
				if grandchild != nil && grandchild.Type() == "identifier" {
					specNode.Name = grandchild.Content(b.source)
				}
			}
			node.Specifiers = append(node.Specifiers, specNode)

		case "named_imports":
			// Handle: import { a, b } from 'module'
			for j := 0; j < int(child.ChildCount()); j++ {
				importSpec := child.Child(j)
				if importSpec != nil && importSpec.Type() == "import_specifier" {
					specNode := b.buildImportSpecifier(importSpec)
					if specNode != nil {
						node.Specifiers = append(node.Specifiers, specNode)
					}
				}
			}

		case "import_specifier":
			// Direct import specifier
			specNode := b.buildImportSpecifier(child)
			if specNode != nil {
				node.Specifiers = append(node.Specifiers, specNode)
			}
		}
	}

	return node
}

// extractImportClause extracts specifiers from an import_clause node
func (b *ASTBuilder) extractImportClause(clauseNode *sitter.Node, node *Node) {
	for i := 0; i < int(clauseNode.ChildCount()); i++ {
		child := clauseNode.Child(i)
		if child == nil {
			continue
		}

		switch child.Type() {
		case "identifier":
			// Default import: import React from 'react'
			specNode := NewNode(NodeImportDefaultSpecifier)
			specNode.Location = b.getLocation(child)
			specNode.Name = child.Content(b.source)
			node.Specifiers = append(node.Specifiers, specNode)

		case "namespace_import":
			// Namespace import: import * as React from 'react'
			specNode := NewNode(NodeImportNamespaceSpecifier)
			specNode.Location = b.getLocation(child)
			for j := 0; j < int(child.ChildCount()); j++ {
				grandchild := child.Child(j)
				if grandchild != nil && grandchild.Type() == "identifier" {
					specNode.Name = grandchild.Content(b.source)
				}
			}
			node.Specifiers = append(node.Specifiers, specNode)

		case "named_imports":
			// Named imports: import { useState, useEffect } from 'react'
			for j := 0; j < int(child.ChildCount()); j++ {
				importSpec := child.Child(j)
				if importSpec != nil && importSpec.Type() == "import_specifier" {
					specNode := b.buildImportSpecifier(importSpec)
					if specNode != nil {
						node.Specifiers = append(node.Specifiers, specNode)
					}
				}
			}
		}
	}
}

// buildImportSpecifier builds an import specifier node
func (b *ASTBuilder) buildImportSpecifier(tsNode *sitter.Node) *Node {
	specNode := NewNode(NodeImportSpecifier)
	specNode.Location = b.getLocation(tsNode)

	// An import specifier can have: name or name as alias
	identifiers := []*sitter.Node{}
	for i := 0; i < int(tsNode.ChildCount()); i++ {
		child := tsNode.Child(i)
		if child != nil && child.Type() == "identifier" {
			identifiers = append(identifiers, child)
		}
	}

	if len(identifiers) == 1 {
		// import { foo } - same name for imported and local
		specNode.Name = identifiers[0].Content(b.source)
		specNode.Imported = NewNode(NodeIdentifier)
		specNode.Imported.Name = specNode.Name
	} else if len(identifiers) == 2 {
		// import { foo as bar } - first is imported, second is local
		specNode.Imported = NewNode(NodeIdentifier)
		specNode.Imported.Name = identifiers[0].Content(b.source)
		specNode.Name = identifiers[1].Content(b.source)
	}

	return specNode
}

// buildExportStatement builds an export statement node
func (b *ASTBuilder) buildExportStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeExportNamedDeclaration)
	node.Location = b.getLocation(tsNode)

	// Check for default, export *, etc.
	hasDefault := false
	hasWildcard := false

	for i := 0; i < int(tsNode.ChildCount()); i++ {
		child := tsNode.Child(i)
		if child == nil {
			continue
		}
		switch child.Type() {
		case "default":
			hasDefault = true
		case "*":
			hasWildcard = true
		case "export_clause":
			// Handle: export { foo, bar } or export { foo as bar }
			b.extractExportClause(child, node)
		}
	}

	// Set node type based on export kind
	if hasDefault {
		node.Type = NodeExportDefaultDeclaration
	} else if hasWildcard {
		node.Type = NodeExportAllDeclaration
	}

	// Extract declaration (for named and default exports)
	if declNode := b.getChildByFieldName(tsNode, "declaration"); declNode != nil {
		node.Declaration = b.buildNode(declNode)
	}

	// Extract value (for default exports like: export default function() {})
	if valueNode := b.getChildByFieldName(tsNode, "value"); valueNode != nil {
		node.Declaration = b.buildNode(valueNode)
	}

	// Extract source if re-exporting
	if sourceNode := b.getChildByFieldName(tsNode, "source"); sourceNode != nil {
		node.Source = b.buildNode(sourceNode)
	}

	return node
}

// extractExportClause extracts specifiers from an export_clause node
func (b *ASTBuilder) extractExportClause(clauseNode *sitter.Node, node *Node) {
	for i := 0; i < int(clauseNode.ChildCount()); i++ {
		child := clauseNode.Child(i)
		if child == nil {
			continue
		}

		if child.Type() == "export_specifier" {
			specNode := NewNode(NodeExportSpecifier)
			specNode.Location = b.getLocation(child)

			// Extract the identifiers (local and exported names)
			identifiers := []*sitter.Node{}
			for j := 0; j < int(child.ChildCount()); j++ {
				grandchild := child.Child(j)
				if grandchild != nil && grandchild.Type() == "identifier" {
					identifiers = append(identifiers, grandchild)
				}
			}

			if len(identifiers) == 1 {
				// export { foo } - same name
				specNode.Name = identifiers[0].Content(b.source)
				specNode.Local = NewNode(NodeIdentifier)
				specNode.Local.Name = specNode.Name
			} else if len(identifiers) == 2 {
				// export { foo as bar } - first is local, second is exported
				specNode.Local = NewNode(NodeIdentifier)
				specNode.Local.Name = identifiers[0].Content(b.source)
				specNode.Name = identifiers[1].Content(b.source)
			}

			node.Specifiers = append(node.Specifiers, specNode)
		}
	}
}

// buildBlockStatement builds a block statement node
func (b *ASTBuilder) buildBlockStatement(tsNode *sitter.Node) *Node {
	node := NewNode(NodeBlockStatement)
	node.Location = b.getLocation(tsNode)

	for i := 0; i < int(tsNode.ChildCount()); i++ {
		child := tsNode.Child(i)
		if child != nil && !b.isTrivia(child) && child.Type() != "{" && child.Type() != "}" {
			childNode := b.buildNode(child)
			if childNode != nil {
				node.Body = append(node.Body, childNode)
			}
		}
	}

	return node
}

// buildGenericNode builds a generic node for unknown types
func (b *ASTBuilder) buildGenericNode(tsNode *sitter.Node) *Node {
	node := NewNode(NodeType(tsNode.Type()))
	node.Location = b.getLocation(tsNode)

	for i := 0; i < int(tsNode.ChildCount()); i++ {
		child := tsNode.Child(i)
		if child != nil && !b.isTrivia(child) {
			childNode := b.buildNode(child)
			if childNode != nil {
				node.AddChild(childNode)
			}
		}
	}

	return node
}

// buildParameters builds parameter list from formal_parameters node
func (b *ASTBuilder) buildParameters(tsNode *sitter.Node) []*Node {
	var params []*Node

	for i := 0; i < int(tsNode.ChildCount()); i++ {
		child := tsNode.Child(i)
		if child != nil && !b.isTrivia(child) && child.Type() != "(" && child.Type() != ")" && child.Type() != "," {
			paramNode := b.buildNode(child)
			if paramNode != nil {
				params = append(params, paramNode)
			}
		}
	}

	return params
}

// Helper methods

// getLocation extracts location information from a tree-sitter node
func (b *ASTBuilder) getLocation(tsNode *sitter.Node) Location {
	return Location{
		File:      b.filename,
		StartLine: int(tsNode.StartPoint().Row) + 1,
		StartCol:  int(tsNode.StartPoint().Column),
		EndLine:   int(tsNode.EndPoint().Row) + 1,
		EndCol:    int(tsNode.EndPoint().Column),
	}
}

// getChildByFieldName gets a child node by field name
func (b *ASTBuilder) getChildByFieldName(tsNode *sitter.Node, fieldName string) *sitter.Node {
	for i := 0; i < int(tsNode.ChildCount()); i++ {
		child := tsNode.Child(i)
		if child != nil && tsNode.FieldNameForChild(i) == fieldName {
			return child
		}
	}
	return nil
}

// isTrivia checks if a node is trivia (whitespace, comments, etc.)
func (b *ASTBuilder) isTrivia(tsNode *sitter.Node) bool {
	nodeType := tsNode.Type()
	return nodeType == "comment" ||
		nodeType == "line_comment" ||
		nodeType == "block_comment" ||
		nodeType == ""
}

// isOperator checks if a node type is an operator
func (b *ASTBuilder) isOperator(nodeType string) bool {
	operators := map[string]bool{
		"+": true, "-": true, "*": true, "/": true, "%": true,
		"==": true, "!=": true, "===": true, "!==": true,
		"<": true, ">": true, "<=": true, ">=": true,
		"&&": true, "||": true, "??": true,
		"&": true, "|": true, "^": true, "~": true,
		"<<": true, ">>": true, ">>>": true,
		"!": true, "typeof": true, "void": true, "delete": true,
		"++": true, "--": true,
		"=": true, "+=": true, "-=": true, "*=": true, "/=": true,
		"%=": true, "&=": true, "|=": true, "^=": true,
		"<<=": true, ">>=": true, ">>>=": true,
		"in": true, "instanceof": true, "of": true,
	}
	return operators[nodeType]
}
