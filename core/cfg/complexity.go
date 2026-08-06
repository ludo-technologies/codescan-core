package cfg

// ComplexityContribution represents a single language-specific complexity contribution.
type ComplexityContribution struct {
	Count       int
	Description string // e.g. "logical_and", "ternary", "null_coalescing"
}

// ComplexityContributor provides language-specific additional complexity counts.
// For example, jscan counts logical operators (&&, ||, ??) and ternary expressions.
type ComplexityContributor interface {
	ContributeComplexity(block *BasicBlock) ([]ComplexityContribution, error)
}

// ComplexityConfig configures complexity computation.
type ComplexityConfig struct {
	// Contributor provides language-specific extra complexity contributions.
	// If nil, no extra contributions are added.
	Contributor ComplexityContributor
}

// ComplexityResult holds McCabe cyclomatic complexity analysis results.
type ComplexityResult struct {
	McCabe             int
	DecisionPoints     int
	ExtraContributions int
	Contributions      []ComplexityContribution
	EdgeBreakdown      map[EdgeType]int
}

// ComputeComplexity computes McCabe cyclomatic complexity for a CFG.
// A block whose outgoing EdgeCondTrue/EdgeCondFalse edges lead to k distinct
// branch targets is a k-way branch and contributes k-1 decision points: an
// if-else (true + false) or a loop header (body + exit) contributes one, while
// a switch/match that emits one edge per case plus a no-match edge contributes
// one per case. A block with any EdgeException successor contributes one more,
// for the implicit raise-or-not branch. EdgeLoop is a back-edge and does not
// count as a decision point; loop headers should use EdgeCondTrue/EdgeCondFalse
// for the loop-body vs exit branch.
// McCabe = DecisionPoints + ExtraContributions + 1.
func ComputeComplexity(c *CFG, config ComplexityConfig) (*ComplexityResult, error) {
	result := &ComplexityResult{
		EdgeBreakdown: make(map[EdgeType]int),
	}

	if c == nil {
		result.McCabe = 1
		return result, nil
	}

	for _, block := range c.Blocks {
		branchTargets := 0
		hasException := false
		for i, edge := range block.Successors {
			result.EdgeBreakdown[edge.Type]++
			switch edge.Type {
			case EdgeCondTrue, EdgeCondFalse:
				if !isBranchTarget(block.Successors[:i], edge.To) {
					branchTargets++
				}
			case EdgeException:
				hasException = true
			}
		}
		if branchTargets > 1 {
			result.DecisionPoints += branchTargets - 1
		}
		if hasException {
			result.DecisionPoints++
		}

		if config.Contributor != nil {
			contributions, err := config.Contributor.ContributeComplexity(block)
			if err != nil {
				return nil, err
			}
			for _, contrib := range contributions {
				result.ExtraContributions += contrib.Count
				result.Contributions = append(result.Contributions, contrib)
			}
		}
	}

	result.McCabe = result.DecisionPoints + result.ExtraContributions + 1
	return result, nil
}

// isBranchTarget reports whether target is already reached by a conditional
// edge among the successors listed, so that repeated edges to the same block
// count as a single branch target.
func isBranchTarget(successors []*Edge, target *BasicBlock) bool {
	for _, edge := range successors {
		if edge.To != target {
			continue
		}
		if edge.Type == EdgeCondTrue || edge.Type == EdgeCondFalse {
			return true
		}
	}
	return false
}
