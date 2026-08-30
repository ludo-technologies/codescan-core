package analyzer

import (
	"fmt"
	"log"
	"strconv"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/domain"
	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

// Block label constants
const (
	LabelFunctionBody             = "func_body"
	LabelClassBody                = "class_body"
	LabelUnreachable              = "unreachable"
	LabelUnreachableAfterReturn   = "unreachable_after_return"
	LabelUnreachableAfterBreak    = "unreachable_after_break"
	LabelUnreachableAfterContinue = "unreachable_after_continue"
	LabelUnreachableAfterThrow    = "unreachable_after_throw"
	LabelEntry                    = "ENTRY"
	LabelExit                     = "EXIT"

	// Loop-related labels
	LabelLoopHeader = "loop_header"
	LabelLoopBody   = "loop_body"
	LabelLoopExit   = "loop_exit"

	// Exception-related labels
	LabelTryBlock     = "try_block"
	LabelCatchBlock   = "catch_block"
	LabelFinallyBlock = "finally_block"

	// Switch-related labels
	LabelSwitchCase  = "switch_case"
	LabelSwitchMerge = "switch_merge"
)

// loopContext tracks the context of a loop for break/continue handling
type loopContext struct {
	headerBlock *BasicBlock // Loop condition/iterator block
	exitBlock   *BasicBlock // Loop exit point
	loopType    string      // "for", "while", "for-in", "for-of"
}

// exceptionContext tracks the context of a try block for exception handling
type exceptionContext struct {
	catchBlock   *BasicBlock // Catch block (optional)
	finallyBlock *BasicBlock // Finally block (optional)
}

// CFGBuilder builds control flow graphs from AST nodes
type CFGBuilder struct {
	cfg            *CFG
	currentBlock   *BasicBlock
	scopeStack     []string
	functionCFGs   map[string]*CFG
	blockCounter   uint
	logger         *log.Logger
	loopStack      []*loopContext
	exceptionStack []*exceptionContext
}

// NewCFGBuilder creates a new CFG builder
func NewCFGBuilder() *CFGBuilder {
	return &CFGBuilder{
		scopeStack:     []string{},
		functionCFGs:   make(map[string]*CFG),
		blockCounter:   0,
		logger:         nil,
		loopStack:      []*loopContext{},
		exceptionStack: []*exceptionContext{},
	}
}

// SetLogger sets an optional logger for error reporting
func (b *CFGBuilder) SetLogger(logger *log.Logger) {
	b.logger = logger
}

// Build constructs a CFG from an AST node
func (b *CFGBuilder) Build(node *parser.Node) (*CFG, error) {
	if node == nil {
		return nil, fmt.Errorf("cannot build CFG from nil node")
	}

	// Initialize CFG based on node type
	cfgName := domain.ModuleFunctionName
	if node.IsFunction() && node.Name != "" {
		cfgName = node.Name
	} else if node.Type == parser.NodeClass && node.Name != "" {
		cfgName = node.Name
	}

	b.cfg = NewCFG(cfgName)
	b.cfg.FunctionNode = node
	b.currentBlock = b.cfg.Entry

	// Build CFG based on node type
	switch node.Type {
	case parser.NodeProgram:
		b.buildProgram(node)
	case parser.NodeFunction, parser.NodeArrowFunction, parser.NodeAsyncFunction,
		parser.NodeGeneratorFunction, parser.NodeFunctionExpression, parser.NodeMethodDefinition:
		b.buildFunction(node)
	case parser.NodeClass:
		b.buildClass(node)
	default:
		// For single statements, process directly
		b.processStatement(node)
	}

	// Connect current block to exit if not already connected
	if b.currentBlock != nil && b.currentBlock != b.cfg.Exit && !b.hasSuccessor(b.currentBlock, b.cfg.Exit) {
		b.cfg.ConnectBlocks(b.currentBlock, b.cfg.Exit, EdgeNormal)
	}

	return b.cfg, nil
}

// resolveFunctionName returns the name of a function node, or a generated name
// based on its source location if it is anonymous.
func resolveFunctionName(node *parser.Node) string {
	if node.Name != "" {
		return node.Name
	}
	return fmt.Sprintf("anonymous_%d", node.Location.StartLine)
}

// BuildAll builds CFGs for all functions in the AST
func (b *CFGBuilder) BuildAll(node *parser.Node) (map[string]*CFG, error) {
	if node == nil {
		return nil, fmt.Errorf("cannot build CFGs from nil node")
	}

	allCFGs := make(map[string]*CFG)

	// Build main CFG
	mainCFG, err := b.Build(node)
	if err != nil {
		return nil, err
	}
	allCFGs[domain.ModuleFunctionName] = mainCFG

	// Add all function CFGs discovered during Build (via processStatement)
	for name, cfg := range b.functionCFGs {
		allCFGs[name] = cfg
	}

	// Track already-discovered function locations to avoid duplicates.
	// Scan all blocks (not just Entry) because processStatement may place
	// function nodes inside control-flow blocks (if_then, loop_body, etc.).
	discoveredLocations := make(map[string]bool)
	for _, cfg := range allCFGs {
		for _, block := range cfg.Blocks {
			for _, value := range block.Statements {
				stmt, ok := jsNode(value)
				if !ok {
					continue
				}
				if stmt.IsFunction() {
					key := fmt.Sprintf("%d:%d", stmt.Location.StartLine, stmt.Location.StartCol)
					discoveredLocations[key] = true
				}
			}
		}
	}

	// Discover functions nested inside expressions (variable declarations,
	// assignments, callbacks, object methods, etc.) that processStatement
	// doesn't reach.
	node.Walk(func(n *parser.Node) bool {
		if n == node {
			return true
		}
		if !n.IsFunction() {
			return true
		}

		funcName := resolveFunctionName(n)

		// Skip if already discovered
		locationKey := fmt.Sprintf("%d:%d", n.Location.StartLine, n.Location.StartCol)
		if discoveredLocations[locationKey] {
			return true
		}
		discoveredLocations[locationKey] = true

		// Already have this name? Find a unique suffix
		if _, exists := allCFGs[funcName]; exists {
			base := funcName
			funcName = fmt.Sprintf("%s_%d", base, n.Location.StartLine)
			for seq := 2; ; seq++ {
				if _, exists := allCFGs[funcName]; !exists {
					break
				}
				funcName = fmt.Sprintf("%s_%d_%d", base, n.Location.StartLine, seq)
			}
		}

		funcBuilder := NewCFGBuilder()
		funcCFG, err := funcBuilder.Build(n)
		if err == nil {
			allCFGs[funcName] = funcCFG
			// Also discover nested functions from this builder
			for nestedName, nestedCFG := range funcBuilder.functionCFGs {
				if _, exists := allCFGs[nestedName]; !exists {
					allCFGs[nestedName] = nestedCFG
				}
			}
		}
		return false // Don't descend into this function's body (Build handles it)
	})

	return allCFGs, nil
}

// buildProgram processes a program node
func (b *CFGBuilder) buildProgram(node *parser.Node) {
	// Process all statements in the program body
	for _, stmt := range node.Body {
		if b.currentBlock == nil {
			break
		}
		b.processStatement(stmt)
	}
}

// buildFunction processes a function node
func (b *CFGBuilder) buildFunction(node *parser.Node) {
	// Add function body statements to CFG
	for _, stmt := range node.Body {
		if b.currentBlock == nil {
			break
		}
		b.processStatement(stmt)
	}
}

// buildClass processes a class node
func (b *CFGBuilder) buildClass(node *parser.Node) {
	// For classes, we process method definitions
	for _, member := range node.Body {
		if member.Type == parser.NodeMethodDefinition {
			// Build separate CFG for each method
			methodName := member.Name
			if methodName == "" {
				continue
			}

			funcBuilder := NewCFGBuilder()
			methodCFG, err := funcBuilder.Build(member)
			if err == nil {
				fullName := node.Name + "." + methodName
				b.functionCFGs[fullName] = methodCFG
			}
		}
	}
}

// processStatement processes a single statement node
func (b *CFGBuilder) processStatement(node *parser.Node) {
	if node == nil || b.currentBlock == nil {
		return
	}

	switch node.Type {
	case parser.NodeIfStatement:
		b.buildIfStatement(node)
	case parser.NodeSwitchStatement:
		b.buildSwitchStatement(node)
	case parser.NodeForStatement:
		b.buildForStatement(node)
	case parser.NodeForInStatement:
		b.buildForInStatement(node)
	case parser.NodeForOfStatement:
		b.buildForOfStatement(node)
	case parser.NodeWhileStatement:
		b.buildWhileStatement(node)
	case parser.NodeDoWhileStatement:
		b.buildDoWhileStatement(node)
	case parser.NodeTryStatement:
		b.buildTryStatement(node)
	case parser.NodeReturnStatement:
		b.buildReturnStatement(node)
	case parser.NodeBreakStatement:
		b.buildBreakStatement(node)
	case parser.NodeContinueStatement:
		b.buildContinueStatement(node)
	case parser.NodeThrowStatement:
		b.buildThrowStatement(node)
	case parser.NodeBlockStatement:
		b.buildBlockStatement(node)
	case parser.NodeFunction, parser.NodeArrowFunction, parser.NodeAsyncFunction,
		parser.NodeGeneratorFunction, parser.NodeFunctionExpression:
		// Nested function - build separate CFG
		funcName := resolveFunctionName(node)

		funcBuilder := NewCFGBuilder()
		funcCFG, err := funcBuilder.Build(node)
		if err == nil {
			b.functionCFGs[funcName] = funcCFG
		}

		// Add function expression as statement in current block
		b.currentBlock.Statements = append(b.currentBlock.Statements, node)
	default:
		// Regular statement - add to current block
		b.currentBlock.Statements = append(b.currentBlock.Statements, node)
	}
}

// buildIfStatement builds CFG for if statement
func (b *CFGBuilder) buildIfStatement(node *parser.Node) {
	// Add test expression to current block
	if node.Test != nil {
		b.currentBlock.Statements = append(b.currentBlock.Statements, node.Test)
	}

	// Create blocks for then, else, and merge
	thenBlock := b.newBlock("if_then")
	var elseBlock *BasicBlock
	mergeBlock := b.newBlock("if_merge")

	// Connect current to then (true branch)
	b.cfg.ConnectBlocks(b.currentBlock, thenBlock, EdgeCondTrue)

	// Process then branch
	b.currentBlock = thenBlock
	if node.Consequent != nil {
		if node.Consequent.Type == parser.NodeBlockStatement {
			for _, stmt := range node.Consequent.Body {
				b.processStatement(stmt)
			}
		} else {
			b.processStatement(node.Consequent)
		}
	}

	// Connect then block to merge if it doesn't end with return/break/continue/throw
	if b.currentBlock != nil && !b.endsWithJump(b.currentBlock) {
		b.cfg.ConnectBlocks(b.currentBlock, mergeBlock, EdgeNormal)
	}

	// Process else branch if exists
	if node.Alternate != nil {
		elseBlock = b.newBlock("if_else")
		b.currentBlock = elseBlock

		if node.Alternate.Type == parser.NodeBlockStatement {
			for _, stmt := range node.Alternate.Body {
				b.processStatement(stmt)
			}
		} else {
			b.processStatement(node.Alternate)
		}

		// Connect else block to merge if it doesn't end with jump
		if b.currentBlock != nil && !b.endsWithJump(b.currentBlock) {
			b.cfg.ConnectBlocks(b.currentBlock, mergeBlock, EdgeNormal)
		}
	}

	// Connect test block to else or merge (false branch)
	if elseBlock != nil {
		// Find the block before thenBlock (which has the test)
		for _, block := range b.cfg.Blocks {
			for _, edge := range block.Successors {
				if edge.To == thenBlock && edge.Type == EdgeCondTrue {
					b.cfg.ConnectBlocks(block, elseBlock, EdgeCondFalse)
					break
				}
			}
		}
	} else {
		// No else - connect directly to merge
		for _, block := range b.cfg.Blocks {
			for _, edge := range block.Successors {
				if edge.To == thenBlock && edge.Type == EdgeCondTrue {
					b.cfg.ConnectBlocks(block, mergeBlock, EdgeCondFalse)
					break
				}
			}
		}
	}

	b.currentBlock = mergeBlock
}

// buildSwitchStatement builds CFG for switch statement
func (b *CFGBuilder) buildSwitchStatement(node *parser.Node) {
	// Add discriminant to current block
	if node.Test != nil {
		b.currentBlock.Statements = append(b.currentBlock.Statements, node.Test)
	}

	testBlock := b.currentBlock
	mergeBlock := b.newBlock(LabelSwitchMerge)
	var prevCaseBlock *BasicBlock
	var defaultBlock *BasicBlock

	// Process each case
	for i, caseNode := range node.Cases {
		caseBlock := b.newBlock(LabelSwitchCase + "_" + strconv.Itoa(i))

		// Connect test block to case block
		if caseNode.Type == parser.NodeDefaultClause {
			defaultBlock = caseBlock
		} else {
			b.cfg.ConnectBlocks(testBlock, caseBlock, EdgeCondTrue)
		}

		b.currentBlock = caseBlock

		// Process case body
		for _, stmt := range caseNode.Body {
			if b.currentBlock == nil {
				break
			}
			b.processStatement(stmt)
		}

		// Check if case ends with break
		hasBreak := false
		if len(caseNode.Body) > 0 {
			lastStmt := caseNode.Body[len(caseNode.Body)-1]
			if lastStmt.Type == parser.NodeBreakStatement {
				hasBreak = true
			}
		}

		if hasBreak || b.endsWithJump(b.currentBlock) {
			// Case ends with break/return - connect to merge
			if b.currentBlock != nil {
				b.cfg.ConnectBlocks(b.currentBlock, mergeBlock, EdgeBreak)
			}
		} else if i < len(node.Cases)-1 {
			// Fall-through to next case
			prevCaseBlock = b.currentBlock
		} else {
			// Last case without break
			if b.currentBlock != nil {
				b.cfg.ConnectBlocks(b.currentBlock, mergeBlock, EdgeNormal)
			}
		}

		// Connect previous case (fall-through)
		if prevCaseBlock != nil && prevCaseBlock != caseBlock {
			b.cfg.ConnectBlocks(prevCaseBlock, caseBlock, EdgeNormal)
			prevCaseBlock = nil
		}
	}

	// Connect test block to default or merge
	if defaultBlock != nil {
		b.cfg.ConnectBlocks(testBlock, defaultBlock, EdgeCondFalse)
	} else {
		b.cfg.ConnectBlocks(testBlock, mergeBlock, EdgeCondFalse)
	}

	b.currentBlock = mergeBlock
}

// buildForStatement builds CFG for for loop
func (b *CFGBuilder) buildForStatement(node *parser.Node) {
	// Add initializer to current block
	if node.Init != nil {
		b.currentBlock.Statements = append(b.currentBlock.Statements, node.Init)
	}

	// Create loop blocks
	headerBlock := b.newBlock(LabelLoopHeader)
	bodyBlock := b.newBlock(LabelLoopBody)
	exitBlock := b.newBlock(LabelLoopExit)

	// Connect current to header
	b.cfg.ConnectBlocks(b.currentBlock, headerBlock, EdgeNormal)

	// Add test to header block
	if node.Test != nil {
		headerBlock.Statements = append(headerBlock.Statements, node.Test)
	}

	// Connect header to body (true) and exit (false)
	b.cfg.ConnectBlocks(headerBlock, bodyBlock, EdgeCondTrue)
	b.cfg.ConnectBlocks(headerBlock, exitBlock, EdgeCondFalse)

	// Push loop context for break/continue
	b.loopStack = append(b.loopStack, &loopContext{
		headerBlock: headerBlock,
		exitBlock:   exitBlock,
		loopType:    "for",
	})

	// Process body
	b.currentBlock = bodyBlock
	for _, stmt := range node.Body {
		if b.currentBlock == nil {
			break
		}
		b.processStatement(stmt)
	}

	// Add update expression and connect back to header
	if b.currentBlock != nil && !b.endsWithJump(b.currentBlock) {
		if node.Update != nil {
			b.currentBlock.Statements = append(b.currentBlock.Statements, node.Update)
		}
		b.cfg.ConnectBlocks(b.currentBlock, headerBlock, EdgeLoop)
	}

	// Pop loop context
	b.loopStack = b.loopStack[:len(b.loopStack)-1]

	b.currentBlock = exitBlock
}

// buildForInStatement builds CFG for for-in loop
func (b *CFGBuilder) buildForInStatement(node *parser.Node) {
	// Add iterator setup to current block
	if node.Init != nil && node.Test != nil {
		b.currentBlock.Statements = append(b.currentBlock.Statements, node.Init, node.Test)
	}

	// Create loop blocks
	headerBlock := b.newBlock(LabelLoopHeader)
	bodyBlock := b.newBlock(LabelLoopBody)
	exitBlock := b.newBlock(LabelLoopExit)

	// Connect current to header
	b.cfg.ConnectBlocks(b.currentBlock, headerBlock, EdgeNormal)

	// Connect header to body and exit
	b.cfg.ConnectBlocks(headerBlock, bodyBlock, EdgeCondTrue)
	b.cfg.ConnectBlocks(headerBlock, exitBlock, EdgeCondFalse)

	// Push loop context
	b.loopStack = append(b.loopStack, &loopContext{
		headerBlock: headerBlock,
		exitBlock:   exitBlock,
		loopType:    "for-in",
	})

	// Process body
	b.currentBlock = bodyBlock
	for _, stmt := range node.Body {
		if b.currentBlock == nil {
			break
		}
		b.processStatement(stmt)
	}

	// Connect back to header
	if b.currentBlock != nil && !b.endsWithJump(b.currentBlock) {
		b.cfg.ConnectBlocks(b.currentBlock, headerBlock, EdgeLoop)
	}

	// Pop loop context
	b.loopStack = b.loopStack[:len(b.loopStack)-1]

	b.currentBlock = exitBlock
}

// buildForOfStatement builds CFG for for-of loop
func (b *CFGBuilder) buildForOfStatement(node *parser.Node) {
	// Same as for-in for CFG purposes
	b.buildForInStatement(node)
}

// buildWhileStatement builds CFG for while loop
func (b *CFGBuilder) buildWhileStatement(node *parser.Node) {
	// Create loop blocks
	headerBlock := b.newBlock(LabelLoopHeader)
	bodyBlock := b.newBlock(LabelLoopBody)
	exitBlock := b.newBlock(LabelLoopExit)

	// Connect current to header
	b.cfg.ConnectBlocks(b.currentBlock, headerBlock, EdgeNormal)

	// Add test to header
	if node.Test != nil {
		headerBlock.Statements = append(headerBlock.Statements, node.Test)
	}

	// Connect header to body and exit
	b.cfg.ConnectBlocks(headerBlock, bodyBlock, EdgeCondTrue)
	b.cfg.ConnectBlocks(headerBlock, exitBlock, EdgeCondFalse)

	// Push loop context
	b.loopStack = append(b.loopStack, &loopContext{
		headerBlock: headerBlock,
		exitBlock:   exitBlock,
		loopType:    "while",
	})

	// Process body
	b.currentBlock = bodyBlock
	for _, stmt := range node.Body {
		if b.currentBlock == nil {
			break
		}
		b.processStatement(stmt)
	}

	// Connect back to header
	if b.currentBlock != nil && !b.endsWithJump(b.currentBlock) {
		b.cfg.ConnectBlocks(b.currentBlock, headerBlock, EdgeLoop)
	}

	// Pop loop context
	b.loopStack = b.loopStack[:len(b.loopStack)-1]

	b.currentBlock = exitBlock
}

// buildDoWhileStatement builds CFG for do-while loop
func (b *CFGBuilder) buildDoWhileStatement(node *parser.Node) {
	// Create loop blocks
	bodyBlock := b.newBlock(LabelLoopBody)
	headerBlock := b.newBlock(LabelLoopHeader) // Test comes after body
	exitBlock := b.newBlock(LabelLoopExit)

	// Connect current to body (do-while always executes once)
	b.cfg.ConnectBlocks(b.currentBlock, bodyBlock, EdgeNormal)

	// Push loop context
	b.loopStack = append(b.loopStack, &loopContext{
		headerBlock: headerBlock,
		exitBlock:   exitBlock,
		loopType:    "do-while",
	})

	// Process body
	b.currentBlock = bodyBlock
	for _, stmt := range node.Body {
		if b.currentBlock == nil {
			break
		}
		b.processStatement(stmt)
	}

	// Connect body to header (test)
	if b.currentBlock != nil && !b.endsWithJump(b.currentBlock) {
		b.cfg.ConnectBlocks(b.currentBlock, headerBlock, EdgeNormal)
	}

	// Add test to header
	if node.Test != nil {
		headerBlock.Statements = append(headerBlock.Statements, node.Test)
	}

	// Connect header back to body (true) or to exit (false)
	b.cfg.ConnectBlocks(headerBlock, bodyBlock, EdgeCondTrue)
	b.cfg.ConnectBlocks(headerBlock, exitBlock, EdgeCondFalse)

	// Pop loop context
	b.loopStack = b.loopStack[:len(b.loopStack)-1]

	b.currentBlock = exitBlock
}

// buildTryStatement builds CFG for try-catch-finally
func (b *CFGBuilder) buildTryStatement(node *parser.Node) {
	tryBlock := b.newBlock(LabelTryBlock)
	var catchBlock *BasicBlock
	var finallyBlock *BasicBlock
	mergeBlock := b.newBlock("try_merge")

	// Connect current to try block
	b.cfg.ConnectBlocks(b.currentBlock, tryBlock, EdgeNormal)

	// Process try block
	b.currentBlock = tryBlock
	for _, stmt := range node.Body {
		if b.currentBlock == nil {
			break
		}
		b.processStatement(stmt)
	}
	tryEndBlock := b.currentBlock

	// Process catch block if exists
	if node.Handler != nil {
		catchBlock = b.newBlock(LabelCatchBlock)
		b.currentBlock = catchBlock

		// Add exception parameter to catch block
		if node.Handler.Type == parser.NodeCatchClause {
			for _, stmt := range node.Handler.Body {
				if b.currentBlock == nil {
					break
				}
				b.processStatement(stmt)
			}
		}

		// Connect try block to catch with exception edge
		b.cfg.ConnectBlocks(tryBlock, catchBlock, EdgeException)
	}

	// Process finally block if exists
	if node.Finalizer != nil {
		finallyBlock = b.newBlock(LabelFinallyBlock)

		// Add finally statements
		addJSStatements(finallyBlock, node.Finalizer.Body)

		// Connect try and catch to finally
		if tryEndBlock != nil && !b.endsWithJump(tryEndBlock) {
			b.cfg.ConnectBlocks(tryEndBlock, finallyBlock, EdgeNormal)
		}
		if catchBlock != nil && b.currentBlock != nil && !b.endsWithJump(b.currentBlock) {
			b.cfg.ConnectBlocks(b.currentBlock, finallyBlock, EdgeNormal)
		}

		// Finally connects to merge
		b.cfg.ConnectBlocks(finallyBlock, mergeBlock, EdgeNormal)
	} else {
		// No finally - connect try and catch directly to merge
		if tryEndBlock != nil && !b.endsWithJump(tryEndBlock) {
			b.cfg.ConnectBlocks(tryEndBlock, mergeBlock, EdgeNormal)
		}
		if catchBlock != nil && b.currentBlock != nil && !b.endsWithJump(b.currentBlock) {
			b.cfg.ConnectBlocks(b.currentBlock, mergeBlock, EdgeNormal)
		}
	}

	b.currentBlock = mergeBlock
}

// buildReturnStatement builds CFG for return statement
func (b *CFGBuilder) buildReturnStatement(node *parser.Node) {
	// Add return to current block
	b.currentBlock.Statements = append(b.currentBlock.Statements, node)

	// Connect to exit
	b.cfg.ConnectBlocks(b.currentBlock, b.cfg.Exit, EdgeReturn)

	// Create unreachable block for code after return
	b.currentBlock = b.newBlock(LabelUnreachableAfterReturn)
}

// buildBreakStatement builds CFG for break statement
func (b *CFGBuilder) buildBreakStatement(node *parser.Node) {
	// Add break to current block
	b.currentBlock.Statements = append(b.currentBlock.Statements, node)

	// Connect to loop exit if in a loop
	if len(b.loopStack) > 0 {
		loopCtx := b.loopStack[len(b.loopStack)-1]
		b.cfg.ConnectBlocks(b.currentBlock, loopCtx.exitBlock, EdgeBreak)
	}

	// Create unreachable block for code after break
	b.currentBlock = b.newBlock(LabelUnreachableAfterBreak)
}

// buildContinueStatement builds CFG for continue statement
func (b *CFGBuilder) buildContinueStatement(node *parser.Node) {
	// Add continue to current block
	b.currentBlock.Statements = append(b.currentBlock.Statements, node)

	// Connect to loop header if in a loop
	if len(b.loopStack) > 0 {
		loopCtx := b.loopStack[len(b.loopStack)-1]
		b.cfg.ConnectBlocks(b.currentBlock, loopCtx.headerBlock, EdgeContinue)
	}

	// Create unreachable block for code after continue
	b.currentBlock = b.newBlock(LabelUnreachableAfterContinue)
}

// buildThrowStatement builds CFG for throw statement
func (b *CFGBuilder) buildThrowStatement(node *parser.Node) {
	// Add throw to current block
	b.currentBlock.Statements = append(b.currentBlock.Statements, node)

	// Connect to nearest catch block or exit with exception edge
	if len(b.exceptionStack) > 0 {
		excCtx := b.exceptionStack[len(b.exceptionStack)-1]
		if excCtx.catchBlock != nil {
			b.cfg.ConnectBlocks(b.currentBlock, excCtx.catchBlock, EdgeException)
		} else if excCtx.finallyBlock != nil {
			b.cfg.ConnectBlocks(b.currentBlock, excCtx.finallyBlock, EdgeException)
		} else {
			b.cfg.ConnectBlocks(b.currentBlock, b.cfg.Exit, EdgeException)
		}
	} else {
		b.cfg.ConnectBlocks(b.currentBlock, b.cfg.Exit, EdgeException)
	}

	// Create unreachable block for code after throw
	b.currentBlock = b.newBlock(LabelUnreachableAfterThrow)
}

// buildBlockStatement builds CFG for block statement
func (b *CFGBuilder) buildBlockStatement(node *parser.Node) {
	// Process all statements in block
	for _, stmt := range node.Body {
		if b.currentBlock == nil {
			break
		}
		b.processStatement(stmt)
	}
}

// Helper methods

// newBlock creates a new basic block with a unique ID
func (b *CFGBuilder) newBlock(label string) *BasicBlock {
	b.blockCounter++
	blockID := label + "_" + strconv.Itoa(int(b.blockCounter))
	block := NewBasicBlock(blockID)
	b.cfg.Blocks[blockID] = block
	return block
}

// hasSuccessor checks if block has a successor
func (b *CFGBuilder) hasSuccessor(block *BasicBlock, target *BasicBlock) bool {
	for _, edge := range block.Successors {
		if edge.To == target {
			return true
		}
	}
	return false
}

// endsWithJump checks if a block ends with a jump statement (return, break, continue, throw)
func (b *CFGBuilder) endsWithJump(block *BasicBlock) bool {
	if len(block.Statements) == 0 {
		return false
	}

	lastStmt := block.Statements[len(block.Statements)-1]
	classifier := javaScriptCFGClassifier{}
	return classifier.IsReturn(lastStmt) || classifier.IsBreak(lastStmt) ||
		classifier.IsContinue(lastStmt) || classifier.IsThrow(lastStmt)
}
