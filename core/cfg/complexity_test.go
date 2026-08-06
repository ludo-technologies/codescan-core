package cfg

import (
	"errors"
	"testing"
)

// testContributor adds extra complexity for testing.
type testContributor struct {
	extras map[string]int // blockID -> extra complexity
}

func (tc *testContributor) ContributeComplexity(block *BasicBlock) ([]ComplexityContribution, error) {
	n := tc.extras[block.ID]
	if n == 0 {
		return nil, nil
	}
	return []ComplexityContribution{{Count: n, Description: "test"}}, nil
}

func TestComplexityLinear(t *testing.T) {
	// entry -> b1 -> exit (no decisions)
	c := NewCFG("linear")
	b1 := c.CreateBlock("b1")
	c.ConnectBlocks(c.Entry, b1, EdgeNormal)
	c.ConnectBlocks(b1, c.Exit, EdgeNormal)

	result, err := ComputeComplexity(c, ComplexityConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.McCabe != 1 {
		t.Fatalf("expected McCabe=1, got %d", result.McCabe)
	}
	if result.DecisionPoints != 0 {
		t.Fatalf("expected 0 decision points, got %d", result.DecisionPoints)
	}
}

func TestComplexityIfElse(t *testing.T) {
	// entry -> [cond] -true-> b_true -> join -> exit
	//                  -false-> b_false -> join
	c := NewCFG("if_else")
	cond := c.CreateBlock("cond")
	bTrue := c.CreateBlock("true")
	bFalse := c.CreateBlock("false")
	join := c.CreateBlock("join")

	c.ConnectBlocks(c.Entry, cond, EdgeNormal)
	c.ConnectBlocks(cond, bTrue, EdgeCondTrue)
	c.ConnectBlocks(cond, bFalse, EdgeCondFalse)
	c.ConnectBlocks(bTrue, join, EdgeNormal)
	c.ConnectBlocks(bFalse, join, EdgeNormal)
	c.ConnectBlocks(join, c.Exit, EdgeNormal)

	result, err := ComputeComplexity(c, ComplexityConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DecisionPoints != 1 {
		t.Fatalf("expected 1 decision point, got %d", result.DecisionPoints)
	}
	if result.McCabe != 2 {
		t.Fatalf("expected McCabe=2, got %d", result.McCabe)
	}
	if result.EdgeBreakdown[EdgeCondTrue] != 1 {
		t.Fatalf("expected 1 true edge, got %d", result.EdgeBreakdown[EdgeCondTrue])
	}
	if result.EdgeBreakdown[EdgeCondFalse] != 1 {
		t.Fatalf("expected 1 false edge, got %d", result.EdgeBreakdown[EdgeCondFalse])
	}
}

func TestComplexityNestedIf(t *testing.T) {
	// Two decision blocks: 2 decision points -> McCabe = 3
	c := NewCFG("nested_if")
	cond1 := c.CreateBlock("cond1")
	cond2 := c.CreateBlock("cond2")
	b1 := c.CreateBlock("b1")
	b2 := c.CreateBlock("b2")
	b3 := c.CreateBlock("b3")
	join := c.CreateBlock("join")

	c.ConnectBlocks(c.Entry, cond1, EdgeNormal)
	c.ConnectBlocks(cond1, cond2, EdgeCondTrue)
	c.ConnectBlocks(cond1, b1, EdgeCondFalse)
	c.ConnectBlocks(cond2, b2, EdgeCondTrue)
	c.ConnectBlocks(cond2, b3, EdgeCondFalse)
	c.ConnectBlocks(b1, join, EdgeNormal)
	c.ConnectBlocks(b2, join, EdgeNormal)
	c.ConnectBlocks(b3, join, EdgeNormal)
	c.ConnectBlocks(join, c.Exit, EdgeNormal)

	result, err := ComputeComplexity(c, ComplexityConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DecisionPoints != 2 {
		t.Fatalf("expected 2 decision points, got %d", result.DecisionPoints)
	}
	if result.McCabe != 3 {
		t.Fatalf("expected McCabe=3, got %d", result.McCabe)
	}
}

func TestComplexityLoop(t *testing.T) {
	// Loop header uses EdgeCondTrue/EdgeCondFalse for the branch decision.
	// EdgeLoop is only used for the back-edge from body to header.
	c := NewCFG("loop")
	header := c.CreateBlock("header")
	body := c.CreateBlock("body")

	c.ConnectBlocks(c.Entry, header, EdgeNormal)
	c.ConnectBlocks(header, body, EdgeCondTrue)
	c.ConnectBlocks(header, c.Exit, EdgeCondFalse)
	c.ConnectBlocks(body, header, EdgeLoop)

	result, err := ComputeComplexity(c, ComplexityConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DecisionPoints != 1 {
		t.Fatalf("expected 1 decision point (loop header), got %d", result.DecisionPoints)
	}
	if result.McCabe != 2 {
		t.Fatalf("expected McCabe=2, got %d", result.McCabe)
	}
	if result.EdgeBreakdown[EdgeLoop] != 1 {
		t.Fatalf("expected 1 loop edge, got %d", result.EdgeBreakdown[EdgeLoop])
	}
}

func TestComplexityMultiWayBranch(t *testing.T) {
	// A switch/match dispatch: one conditional edge per case plus a no-match
	// edge. Four cases branch four ways beyond the fall-out path, so the
	// dispatch is worth four decision points, like a chain of four ifs.
	c := NewCFG("multi_way")
	dispatch := c.CreateBlock("dispatch")
	join := c.CreateBlock("join")

	c.ConnectBlocks(c.Entry, dispatch, EdgeNormal)
	for i := 0; i < 4; i++ {
		caseBlock := c.CreateBlock("case")
		c.ConnectBlocks(dispatch, caseBlock, EdgeCondTrue)
		c.ConnectBlocks(caseBlock, join, EdgeNormal)
	}
	c.ConnectBlocks(dispatch, join, EdgeCondFalse)
	c.ConnectBlocks(join, c.Exit, EdgeNormal)

	result, err := ComputeComplexity(c, ComplexityConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DecisionPoints != 4 {
		t.Fatalf("expected 4 decision points, got %d", result.DecisionPoints)
	}
	if result.McCabe != 5 {
		t.Fatalf("expected McCabe=5, got %d", result.McCabe)
	}
}

func TestComplexitySingleBranchTarget(t *testing.T) {
	// A dispatch with no cases branches only one way, so it decides nothing.
	c := NewCFG("empty_dispatch")
	dispatch := c.CreateBlock("dispatch")
	join := c.CreateBlock("join")

	c.ConnectBlocks(c.Entry, dispatch, EdgeNormal)
	c.ConnectBlocks(dispatch, join, EdgeCondFalse)
	c.ConnectBlocks(join, c.Exit, EdgeNormal)

	result, err := ComputeComplexity(c, ComplexityConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DecisionPoints != 0 {
		t.Fatalf("expected 0 decision points, got %d", result.DecisionPoints)
	}
	if result.McCabe != 1 {
		t.Fatalf("expected McCabe=1, got %d", result.McCabe)
	}
}

func TestComplexityRepeatedBranchTarget(t *testing.T) {
	// Several conditional edges to the same block are one branch target.
	c := NewCFG("repeated_target")
	cond := c.CreateBlock("cond")
	bTrue := c.CreateBlock("true")
	join := c.CreateBlock("join")

	c.ConnectBlocks(c.Entry, cond, EdgeNormal)
	c.ConnectBlocks(cond, bTrue, EdgeCondTrue)
	c.ConnectBlocks(cond, bTrue, EdgeCondTrue)
	c.ConnectBlocks(cond, join, EdgeCondFalse)
	c.ConnectBlocks(bTrue, join, EdgeNormal)
	c.ConnectBlocks(join, c.Exit, EdgeNormal)

	result, err := ComputeComplexity(c, ComplexityConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DecisionPoints != 1 {
		t.Fatalf("expected 1 decision point, got %d", result.DecisionPoints)
	}
	if result.McCabe != 2 {
		t.Fatalf("expected McCabe=2, got %d", result.McCabe)
	}
}

func TestComplexityException(t *testing.T) {
	c := NewCFG("exception")
	tryBlock := c.CreateBlock("try")
	handler := c.CreateBlock("handler")
	after := c.CreateBlock("after")

	c.ConnectBlocks(c.Entry, tryBlock, EdgeNormal)
	c.ConnectBlocks(tryBlock, handler, EdgeException)
	c.ConnectBlocks(tryBlock, after, EdgeNormal)
	c.ConnectBlocks(handler, after, EdgeNormal)
	c.ConnectBlocks(after, c.Exit, EdgeNormal)

	result, err := ComputeComplexity(c, ComplexityConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DecisionPoints != 1 {
		t.Fatalf("expected 1 decision point (exception), got %d", result.DecisionPoints)
	}
	if result.EdgeBreakdown[EdgeException] != 1 {
		t.Fatalf("expected 1 exception edge, got %d", result.EdgeBreakdown[EdgeException])
	}
}

func TestComplexityWithContributor(t *testing.T) {
	c := NewCFG("contributor")
	b1 := c.CreateBlock("b1")
	c.ConnectBlocks(c.Entry, b1, EdgeNormal)
	c.ConnectBlocks(b1, c.Exit, EdgeNormal)

	contrib := &testContributor{
		extras: map[string]int{b1.ID: 3},
	}

	result, err := ComputeComplexity(c, ComplexityConfig{Contributor: contrib})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ExtraContributions != 3 {
		t.Fatalf("expected 3 extra contributions, got %d", result.ExtraContributions)
	}
	// McCabe = 0 (decision) + 3 (extra) + 1 = 4
	if result.McCabe != 4 {
		t.Fatalf("expected McCabe=4, got %d", result.McCabe)
	}
	if len(result.Contributions) != 1 {
		t.Fatalf("expected 1 contribution entry, got %d", len(result.Contributions))
	}
	if result.Contributions[0].Count != 3 {
		t.Fatalf("expected contribution count=3, got %d", result.Contributions[0].Count)
	}
}

func TestComplexityNilCFG(t *testing.T) {
	result, err := ComputeComplexity(nil, ComplexityConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.McCabe != 1 {
		t.Fatalf("expected McCabe=1 for nil CFG, got %d", result.McCabe)
	}
}

// errorContributor returns an error for testing.
type errorContributor struct{}

func (ec *errorContributor) ContributeComplexity(block *BasicBlock) ([]ComplexityContribution, error) {
	return nil, errors.New("contributor failed")
}

func TestComplexityContributorError(t *testing.T) {
	c := NewCFG("error")
	b1 := c.CreateBlock("b1")
	c.ConnectBlocks(c.Entry, b1, EdgeNormal)
	c.ConnectBlocks(b1, c.Exit, EdgeNormal)

	_, err := ComputeComplexity(c, ComplexityConfig{Contributor: &errorContributor{}})
	if err == nil {
		t.Fatal("expected error from contributor")
	}
}
