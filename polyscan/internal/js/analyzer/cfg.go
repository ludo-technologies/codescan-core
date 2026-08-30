package analyzer

import corecfg "github.com/ludo-technologies/polyscan/core/cfg"

// CFG aliases keep analyzer's internal API stable while the implementation is
// owned by polyscan core.
type (
	EdgeType   = corecfg.EdgeType
	Edge       = corecfg.Edge
	BasicBlock = corecfg.BasicBlock
	CFG        = corecfg.CFG
	CFGVisitor = corecfg.Visitor
)

const (
	EdgeNormal    = corecfg.EdgeNormal
	EdgeCondTrue  = corecfg.EdgeCondTrue
	EdgeCondFalse = corecfg.EdgeCondFalse
	EdgeException = corecfg.EdgeException
	EdgeLoop      = corecfg.EdgeLoop
	EdgeBreak     = corecfg.EdgeBreak
	EdgeContinue  = corecfg.EdgeContinue
	EdgeReturn    = corecfg.EdgeReturn
)

var (
	NewBasicBlock = corecfg.NewBasicBlock
	NewCFG        = corecfg.NewCFG
)
