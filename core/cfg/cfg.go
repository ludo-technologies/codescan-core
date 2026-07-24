package cfg

import "fmt"

// EdgeType represents the type of edge between basic blocks.
type EdgeType int

const (
	EdgeNormal    EdgeType = iota // Normal sequential flow
	EdgeCondTrue                  // Conditional true branch
	EdgeCondFalse                 // Conditional false branch
	EdgeException                 // Exception flow
	EdgeLoop                      // Loop back edge
	EdgeBreak                     // Break statement flow
	EdgeContinue                  // Continue statement flow
	EdgeReturn                    // Return statement flow
)

// String returns string representation of EdgeType.
func (e EdgeType) String() string {
	switch e {
	case EdgeNormal:
		return "normal"
	case EdgeCondTrue:
		return "true"
	case EdgeCondFalse:
		return "false"
	case EdgeException:
		return "exception"
	case EdgeLoop:
		return "loop"
	case EdgeBreak:
		return "break"
	case EdgeContinue:
		return "continue"
	case EdgeReturn:
		return "return"
	default:
		return "unknown"
	}
}

// Edge represents a directed edge between two basic blocks.
type Edge struct {
	From  *BasicBlock
	To    *BasicBlock
	Type  EdgeType
	Label string // Language-specific flow description (e.g. switch case value, yield, await).
	Data  any    // Optional language-specific edge metadata.
}

// BasicBlock represents a basic block in the control flow graph.
type BasicBlock struct {
	ID    string
	Label string

	// Statements contains the AST nodes in this block.
	// Each element is a language-specific AST node stored as any.
	// Language adapters recover the concrete type via type assertion:
	//
	//   for _, stmt := range block.Statements {
	//       node := stmt.(*parser.Node) // pyscn or jscan
	//   }
	Statements []any

	Predecessors []*Edge
	Successors   []*Edge

	IsEntry bool
	IsExit  bool
}

// NewBasicBlock creates a new basic block with the given ID.
func NewBasicBlock(id string) *BasicBlock {
	return &BasicBlock{
		ID:           id,
		Statements:   []any{},
		Predecessors: []*Edge{},
		Successors:   []*Edge{},
	}
}

// AddStatement adds a statement to this block.
func (bb *BasicBlock) AddStatement(stmt any) {
	if stmt != nil {
		bb.Statements = append(bb.Statements, stmt)
	}
}

// AddSuccessor adds an outgoing edge to another block.
func (bb *BasicBlock) AddSuccessor(to *BasicBlock, edgeType EdgeType) *Edge {
	edge := &Edge{From: bb, To: to, Type: edgeType}
	bb.Successors = append(bb.Successors, edge)
	to.Predecessors = append(to.Predecessors, edge)
	return edge
}

// RemoveSuccessor removes an edge to the specified block.
func (bb *BasicBlock) RemoveSuccessor(to *BasicBlock) {
	newSuccessors := []*Edge{}
	for _, edge := range bb.Successors {
		if edge.To != to {
			newSuccessors = append(newSuccessors, edge)
		}
	}
	bb.Successors = newSuccessors

	newPredecessors := []*Edge{}
	for _, edge := range to.Predecessors {
		if edge.From != bb {
			newPredecessors = append(newPredecessors, edge)
		}
	}
	to.Predecessors = newPredecessors
}

// IsEmpty returns true if the block has no statements.
func (bb *BasicBlock) IsEmpty() bool {
	return len(bb.Statements) == 0
}

// String returns a string representation of the basic block.
func (bb *BasicBlock) String() string {
	label := bb.Label
	if label == "" {
		label = bb.ID
	}
	if bb.IsEntry {
		return fmt.Sprintf("[ENTRY: %s]", label)
	}
	if bb.IsExit {
		return fmt.Sprintf("[EXIT: %s]", label)
	}
	return fmt.Sprintf("[%s: %d stmts]", label, len(bb.Statements))
}

// CFG represents a control flow graph.
type CFG struct {
	Entry  *BasicBlock
	Exit   *BasicBlock
	Blocks map[string]*BasicBlock
	Name   string

	// FunctionNode is the original AST node for the function.
	// This field is opaque to polyscan core; language adapters store their
	// own function node type here and recover it via type assertion:
	//
	//   fnNode := cfg.FunctionNode.(*parser.Node)
	FunctionNode any

	nextBlockID int
}

// NewCFG creates a new control flow graph.
func NewCFG(name string) *CFG {
	cfg := &CFG{
		Name:        name,
		Blocks:      make(map[string]*BasicBlock),
		nextBlockID: 0,
	}
	cfg.Entry = cfg.CreateBlock("entry")
	cfg.Entry.IsEntry = true
	cfg.Entry.Label = "ENTRY"

	cfg.Exit = cfg.CreateBlock("exit")
	cfg.Exit.IsExit = true
	cfg.Exit.Label = "EXIT"

	return cfg
}

// CreateBlock creates a new basic block and adds it to the graph.
func (c *CFG) CreateBlock(label string) *BasicBlock {
	id := fmt.Sprintf("bb%d", c.nextBlockID)
	c.nextBlockID++
	block := NewBasicBlock(id)
	if label != "" {
		block.Label = label
	}
	c.Blocks[id] = block
	return block
}

// AddBlock adds an existing block to the graph.
func (c *CFG) AddBlock(block *BasicBlock) {
	if block != nil {
		c.Blocks[block.ID] = block
	}
}

// RemoveBlock removes a block from the graph.
func (c *CFG) RemoveBlock(block *BasicBlock) {
	if block == nil || block.IsEntry || block.IsExit {
		return
	}
	for _, pred := range block.Predecessors {
		pred.From.RemoveSuccessor(block)
	}
	for _, succ := range block.Successors {
		block.RemoveSuccessor(succ.To)
	}
	delete(c.Blocks, block.ID)
}

// ConnectBlocks creates an edge between two blocks.
func (c *CFG) ConnectBlocks(from, to *BasicBlock, edgeType EdgeType) *Edge {
	if from == nil || to == nil {
		return nil
	}
	return from.AddSuccessor(to, edgeType)
}

// GetBlock retrieves a block by its ID.
func (c *CFG) GetBlock(id string) *BasicBlock {
	return c.Blocks[id]
}

// Size returns the number of blocks in the graph.
func (c *CFG) Size() int {
	return len(c.Blocks)
}

// Visitor defines the interface for visiting CFG nodes.
type Visitor interface {
	VisitBlock(block *BasicBlock) bool
	VisitEdge(edge *Edge) bool
}

// Walk performs a depth-first traversal of the CFG.
func (c *CFG) Walk(visitor Visitor) {
	if c.Entry == nil {
		return
	}
	visited := make(map[string]bool)
	c.walkBlock(c.Entry, visitor, visited)
}

func (c *CFG) walkBlock(block *BasicBlock, visitor Visitor, visited map[string]bool) {
	if block == nil || visited[block.ID] {
		return
	}
	visited[block.ID] = true
	if !visitor.VisitBlock(block) {
		return
	}
	for _, edge := range block.Successors {
		if !visitor.VisitEdge(edge) {
			return
		}
		c.walkBlock(edge.To, visitor, visited)
	}
}

// BreadthFirstWalk performs a breadth-first traversal of the CFG.
func (c *CFG) BreadthFirstWalk(visitor Visitor) {
	if c.Entry == nil {
		return
	}
	visited := make(map[string]bool)
	queue := []*BasicBlock{c.Entry}

	for len(queue) > 0 {
		block := queue[0]
		queue = queue[1:]
		if visited[block.ID] {
			continue
		}
		visited[block.ID] = true
		if !visitor.VisitBlock(block) {
			return
		}
		for _, edge := range block.Successors {
			if !visitor.VisitEdge(edge) {
				return
			}
			if !visited[edge.To.ID] {
				queue = append(queue, edge.To)
			}
		}
	}
}

// String returns a string representation of the CFG.
func (c *CFG) String() string {
	return fmt.Sprintf("CFG(%s): %d blocks", c.Name, c.Size())
}
