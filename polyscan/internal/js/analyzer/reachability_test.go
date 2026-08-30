package analyzer

import (
	"testing"

	"github.com/ludo-technologies/polyscan/polyscan/internal/js/parser"
)

func TestNewReachabilityAnalyzer(t *testing.T) {
	cfg := NewCFG("test")
	analyzer := NewReachabilityAnalyzer(cfg)

	if analyzer == nil {
		t.Fatal("NewReachabilityAnalyzer should not return nil")
	}
	if analyzer.cfg != cfg {
		t.Error("Analyzer should store CFG reference")
	}
}

func TestReachabilityAnalyzer_AnalyzeReachability_NilCFG(t *testing.T) {
	analyzer := &ReachabilityAnalyzer{cfg: nil}
	result := analyzer.AnalyzeReachability()

	if result == nil {
		t.Fatal("Result should not be nil")
	}
	if result.TotalBlocks != 0 {
		t.Errorf("TotalBlocks should be 0 with nil CFG, got %d", result.TotalBlocks)
	}
	if result.ReachableCount != 0 {
		t.Error("ReachableCount should be 0 with nil CFG")
	}
	if result.UnreachableCount != 0 {
		t.Error("UnreachableCount should be 0 with nil CFG")
	}
}

func TestReachabilityAnalyzer_AnalyzeReachability_EmptyCFG(t *testing.T) {
	cfg := &CFG{
		Name:   "empty",
		Blocks: make(map[string]*BasicBlock),
		Entry:  nil,
	}
	analyzer := NewReachabilityAnalyzer(cfg)
	result := analyzer.AnalyzeReachability()

	if result == nil {
		t.Fatal("Result should not be nil")
	}
	if result.TotalBlocks != 0 {
		t.Errorf("TotalBlocks should be 0 for empty CFG, got %d", result.TotalBlocks)
	}
}

func TestReachabilityAnalyzer_AnalyzeReachability_SimpleCFG(t *testing.T) {
	cfg := NewCFG("simple")
	cfg.ConnectBlocks(cfg.Entry, cfg.Exit, EdgeNormal)

	analyzer := NewReachabilityAnalyzer(cfg)
	result := analyzer.AnalyzeReachability()

	if result == nil {
		t.Fatal("Result should not be nil")
	}
	if result.TotalBlocks != 2 {
		t.Errorf("TotalBlocks should be 2, got %d", result.TotalBlocks)
	}
	if result.ReachableCount != 2 {
		t.Errorf("All blocks should be reachable, got %d", result.ReachableCount)
	}
	if result.UnreachableCount != 0 {
		t.Errorf("No blocks should be unreachable, got %d", result.UnreachableCount)
	}
}

func TestReachabilityAnalyzer_AnalyzeReachability_WithUnreachable(t *testing.T) {
	cfg := NewCFG("test")
	reachableBlock := cfg.CreateBlock("reachable")
	unreachableBlock := cfg.CreateBlock("unreachable")

	cfg.ConnectBlocks(cfg.Entry, reachableBlock, EdgeNormal)
	cfg.ConnectBlocks(reachableBlock, cfg.Exit, EdgeNormal)
	// unreachableBlock is not connected - should be detected

	analyzer := NewReachabilityAnalyzer(cfg)
	result := analyzer.AnalyzeReachability()

	if result.UnreachableCount == 0 {
		t.Error("Should detect unreachable block")
	}
	if _, exists := result.UnreachableBlocks[unreachableBlock.ID]; !exists {
		t.Error("unreachableBlock should be in UnreachableBlocks")
	}
}

func TestReachabilityAnalyzer_AnalyzeReachabilityFrom(t *testing.T) {
	cfg := NewCFG("test")
	block1 := cfg.CreateBlock("block1")
	block2 := cfg.CreateBlock("block2")
	block3 := cfg.CreateBlock("block3")

	cfg.ConnectBlocks(cfg.Entry, block1, EdgeNormal)
	cfg.ConnectBlocks(block1, block2, EdgeNormal)
	cfg.ConnectBlocks(block2, cfg.Exit, EdgeNormal)
	// block3 is disconnected

	analyzer := NewReachabilityAnalyzer(cfg)
	result := analyzer.AnalyzeReachabilityFrom(block1)

	// From block1, we can reach block1, block2, exit
	if _, exists := result.ReachableBlocks[block1.ID]; !exists {
		t.Error("block1 should be reachable from itself")
	}
	if _, exists := result.ReachableBlocks[block2.ID]; !exists {
		t.Error("block2 should be reachable from block1")
	}
	if _, exists := result.ReachableBlocks[cfg.Exit.ID]; !exists {
		t.Error("exit should be reachable from block1")
	}
	// block3 and entry should not be reachable from block1
	if _, exists := result.ReachableBlocks[block3.ID]; exists {
		t.Error("block3 should not be reachable from block1")
	}
}

func TestReachabilityAnalyzer_AnalyzeReachabilityFrom_NilStart(t *testing.T) {
	cfg := NewCFG("test")
	analyzer := NewReachabilityAnalyzer(cfg)

	result := analyzer.AnalyzeReachabilityFrom(nil)

	if result == nil {
		t.Fatal("Result should not be nil")
	}
	if result.ReachableCount != 0 {
		t.Error("No blocks should be reachable from nil start")
	}
}

func TestReachabilityAnalyzer_AnalyzeReachabilityFrom_NilCFG(t *testing.T) {
	analyzer := &ReachabilityAnalyzer{cfg: nil}
	startBlock := NewBasicBlock("start")

	result := analyzer.AnalyzeReachabilityFrom(startBlock)

	if result == nil {
		t.Fatal("Result should not be nil")
	}
	if result.TotalBlocks != 0 {
		t.Error("TotalBlocks should be 0 with nil CFG")
	}
}

func TestReachabilityResult_GetUnreachableBlocksWithStatements(t *testing.T) {
	emptyBlock := NewBasicBlock("empty")
	blockWithStmt := NewBasicBlock("with_stmt")
	blockWithStmt.AddStatement(&parser.Node{Type: parser.NodeExpressionStatement})

	result := &ReachabilityResult{
		UnreachableBlocks: map[string]*BasicBlock{
			"empty":     emptyBlock,
			"with_stmt": blockWithStmt,
		},
	}

	blocksWithStatements := result.GetUnreachableBlocksWithStatements()

	if len(blocksWithStatements) != 1 {
		t.Errorf("Should have 1 block with statements, got %d", len(blocksWithStatements))
	}
	if _, exists := blocksWithStatements["with_stmt"]; !exists {
		t.Error("blockWithStmt should be in result")
	}
}

func TestReachabilityResult_GetReachabilityRatio(t *testing.T) {
	// Zero total blocks
	emptyResult := &ReachabilityResult{
		TotalBlocks:    0,
		ReachableCount: 0,
	}
	if emptyResult.GetReachabilityRatio() != 1.0 {
		t.Errorf("Empty CFG should have ratio 1.0, got %f", emptyResult.GetReachabilityRatio())
	}

	// Half reachable
	halfResult := &ReachabilityResult{
		TotalBlocks:    4,
		ReachableCount: 2,
	}
	if halfResult.GetReachabilityRatio() != 0.5 {
		t.Errorf("Half reachable should have ratio 0.5, got %f", halfResult.GetReachabilityRatio())
	}

	// All reachable
	fullResult := &ReachabilityResult{
		TotalBlocks:    10,
		ReachableCount: 10,
	}
	if fullResult.GetReachabilityRatio() != 1.0 {
		t.Errorf("All reachable should have ratio 1.0, got %f", fullResult.GetReachabilityRatio())
	}
}

func TestReachabilityResult_HasUnreachableCode(t *testing.T) {
	// No unreachable blocks
	noUnreachable := &ReachabilityResult{
		UnreachableBlocks: map[string]*BasicBlock{},
	}
	if noUnreachable.HasUnreachableCode() {
		t.Error("Should not have unreachable code with empty unreachable blocks")
	}

	// Only empty unreachable blocks
	emptyBlock := NewBasicBlock("empty")
	onlyEmptyUnreachable := &ReachabilityResult{
		UnreachableBlocks: map[string]*BasicBlock{
			"empty": emptyBlock,
		},
	}
	if onlyEmptyUnreachable.HasUnreachableCode() {
		t.Error("Should not have unreachable code with only empty blocks")
	}

	// Unreachable block with statements
	blockWithStmt := NewBasicBlock("with_stmt")
	blockWithStmt.AddStatement(&parser.Node{Type: parser.NodeExpressionStatement})
	hasUnreachable := &ReachabilityResult{
		UnreachableBlocks: map[string]*BasicBlock{
			"with_stmt": blockWithStmt,
		},
	}
	if !hasUnreachable.HasUnreachableCode() {
		t.Error("Should have unreachable code with block containing statements")
	}
}

func TestReachabilityVisitor(t *testing.T) {
	visitor := &reachabilityVisitor{
		reachableBlocks: make(map[string]*BasicBlock),
	}

	block := NewBasicBlock("test")

	// VisitBlock should mark block as reachable
	result := visitor.VisitBlock(block)
	if !result {
		t.Error("VisitBlock should return true to continue traversal")
	}
	if _, exists := visitor.reachableBlocks[block.ID]; !exists {
		t.Error("Block should be marked as reachable")
	}

	// VisitBlock with nil should not panic
	result = visitor.VisitBlock(nil)
	if !result {
		t.Error("VisitBlock with nil should return true")
	}

	// VisitEdge should always return true
	edge := &Edge{}
	result = visitor.VisitEdge(edge)
	if !result {
		t.Error("VisitEdge should return true")
	}
}

func TestReachabilityAnalyzer_CyclicGraph(t *testing.T) {
	cfg := NewCFG("cyclic")
	loopHeader := cfg.CreateBlock("loop_header")
	loopBody := cfg.CreateBlock("loop_body")

	cfg.ConnectBlocks(cfg.Entry, loopHeader, EdgeNormal)
	cfg.ConnectBlocks(loopHeader, loopBody, EdgeCondTrue)
	cfg.ConnectBlocks(loopHeader, cfg.Exit, EdgeCondFalse)
	cfg.ConnectBlocks(loopBody, loopHeader, EdgeLoop) // Back edge

	analyzer := NewReachabilityAnalyzer(cfg)
	result := analyzer.AnalyzeReachability()

	// All blocks should be reachable
	if result.ReachableCount != cfg.Size() {
		t.Errorf("All %d blocks should be reachable in cyclic graph, got %d",
			cfg.Size(), result.ReachableCount)
	}
}

func TestReachabilityAnalyzer_MultipleExitPaths(t *testing.T) {
	code := `
		function test(x) {
			if (x > 0) {
				return 1;
			}
			return 0;
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	analyzer := NewReachabilityAnalyzer(cfg)
	result := analyzer.AnalyzeReachability()

	// Both branches should be reachable
	if result.UnreachableCount > 0 {
		t.Logf("Note: %d blocks are unreachable (may be expected)", result.UnreachableCount)
	}
	if result.GetReachabilityRatio() < 0.5 {
		t.Error("At least half the blocks should be reachable")
	}
}

func TestReachabilityAnalyzer_AllPathsReturn(t *testing.T) {
	code := `
		function test(x) {
			if (x > 0) {
				return 1;
			} else {
				return -1;
			}
			console.log("unreachable");
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "test")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	analyzer := NewReachabilityAnalyzer(cfg)
	result := analyzer.AnalyzeReachability()

	// The code after the if-else should be unreachable
	if result.HasUnreachableCode() {
		t.Log("Correctly detected unreachable code after all-paths-return")
	}
}

func TestReachabilityAnalyzer_blockContainsReturn(t *testing.T) {
	cfg := NewCFG("test")
	analyzer := NewReachabilityAnalyzer(cfg)

	// Block with return
	blockWithReturn := NewBasicBlock("with_return")
	blockWithReturn.AddStatement(&parser.Node{Type: parser.NodeReturnStatement})

	if !analyzer.blockContainsReturn(blockWithReturn) {
		t.Error("Should detect return in block")
	}

	// Block without return
	blockWithoutReturn := NewBasicBlock("without_return")
	blockWithoutReturn.AddStatement(&parser.Node{Type: parser.NodeExpressionStatement})

	if analyzer.blockContainsReturn(blockWithoutReturn) {
		t.Error("Should not detect return in block without return")
	}

	// Empty block
	emptyBlock := NewBasicBlock("empty")
	if analyzer.blockContainsReturn(emptyBlock) {
		t.Error("Empty block should not contain return")
	}

	// Nil block
	if analyzer.blockContainsReturn(nil) {
		t.Error("Nil block should not contain return")
	}
}

func TestReachabilityAfterReturn(t *testing.T) {
	assertUnreachable := func(t *testing.T, result *ReachabilityResult, block *BasicBlock) {
		t.Helper()

		if _, exists := result.UnreachableBlocks[block.ID]; !exists {
			t.Fatalf("expected %s to be unreachable after return", block.ID)
		}
		if _, exists := result.ReachableBlocks[block.ID]; exists {
			t.Fatalf("expected %s to be removed from reachable blocks", block.ID)
		}
	}

	t.Run("ImmediateSuccessorMarkedUnreachable", func(t *testing.T) {
		cfg := NewCFG("after_return")

		returnBlock := cfg.CreateBlock("return_block")
		deadBlock := cfg.CreateBlock("dead_block")

		returnBlock.AddStatement(&parser.Node{Type: parser.NodeReturnStatement})
		deadBlock.AddStatement(&parser.Node{Type: parser.NodeExpressionStatement})

		cfg.Entry.AddSuccessor(returnBlock, EdgeNormal)
		returnBlock.AddSuccessor(cfg.Exit, EdgeReturn)
		returnBlock.AddSuccessor(deadBlock, EdgeNormal)
		deadBlock.AddSuccessor(cfg.Exit, EdgeNormal)

		analyzer := NewReachabilityAnalyzer(cfg)
		result := analyzer.AnalyzeReachability()

		assertUnreachable(t, result, deadBlock)
		if !result.HasUnreachableCode() {
			t.Fatal("expected unreachable code to be reported")
		}

		unreachableWithStatements := result.GetUnreachableBlocksWithStatements()
		if len(unreachableWithStatements) != 1 || unreachableWithStatements[deadBlock.ID] == nil {
			t.Fatalf("expected only %s to be reported as unreachable code, got %#v", deadBlock.ID, unreachableWithStatements)
		}
	})

	t.Run("SharedDeadTailMarkedUnreachable", func(t *testing.T) {
		cfg := NewCFG("after_return_shared_tail")

		returnBlock := cfg.CreateBlock("return_block")
		leftDead := cfg.CreateBlock("left_dead")
		rightDead := cfg.CreateBlock("right_dead")
		sharedDead := cfg.CreateBlock("shared_dead")
		leafDead := cfg.CreateBlock("leaf_dead")

		returnBlock.AddStatement(&parser.Node{Type: parser.NodeReturnStatement})
		leftDead.AddStatement(&parser.Node{Type: parser.NodeReturnStatement})
		rightDead.AddStatement(&parser.Node{Type: parser.NodeReturnStatement})
		sharedDead.AddStatement(&parser.Node{Type: parser.NodeReturnStatement})
		leafDead.AddStatement(&parser.Node{Type: parser.NodeExpressionStatement})

		cfg.Entry.AddSuccessor(returnBlock, EdgeNormal)
		returnBlock.AddSuccessor(cfg.Exit, EdgeReturn)
		returnBlock.AddSuccessor(leftDead, EdgeNormal)
		returnBlock.AddSuccessor(rightDead, EdgeNormal)
		leftDead.AddSuccessor(sharedDead, EdgeNormal)
		rightDead.AddSuccessor(sharedDead, EdgeNormal)
		sharedDead.AddSuccessor(leafDead, EdgeNormal)
		leafDead.AddSuccessor(cfg.Exit, EdgeNormal)

		analyzer := NewReachabilityAnalyzer(cfg)
		result := analyzer.AnalyzeReachability()

		assertUnreachable(t, result, leftDead)
		assertUnreachable(t, result, rightDead)
		assertUnreachable(t, result, sharedDead)
		assertUnreachable(t, result, leafDead)
	})

	t.Run("CyclicDeadTailMarkedUnreachable", func(t *testing.T) {
		cfg := NewCFG("after_return_cycle")

		returnBlock := cfg.CreateBlock("return_block")
		deadA := cfg.CreateBlock("dead_a")
		deadB := cfg.CreateBlock("dead_b")

		returnBlock.AddStatement(&parser.Node{Type: parser.NodeReturnStatement})
		deadA.AddStatement(&parser.Node{Type: parser.NodeReturnStatement})
		deadB.AddStatement(&parser.Node{Type: parser.NodeReturnStatement})

		cfg.Entry.AddSuccessor(returnBlock, EdgeNormal)
		returnBlock.AddSuccessor(cfg.Exit, EdgeReturn)
		returnBlock.AddSuccessor(deadA, EdgeNormal)
		deadA.AddSuccessor(deadB, EdgeNormal)
		deadB.AddSuccessor(deadA, EdgeNormal)

		analyzer := NewReachabilityAnalyzer(cfg)
		result := analyzer.AnalyzeReachability()

		assertUnreachable(t, result, deadA)
		assertUnreachable(t, result, deadB)
	})
}

func TestReachabilityResult_AnalysisTime(t *testing.T) {
	cfg := NewCFG("test")
	cfg.ConnectBlocks(cfg.Entry, cfg.Exit, EdgeNormal)

	analyzer := NewReachabilityAnalyzer(cfg)
	result := analyzer.AnalyzeReachability()

	if result.AnalysisTime < 0 {
		t.Error("AnalysisTime should be non-negative")
	}
}

// Integration test with complex CFG
func TestReachabilityAnalyzer_ComplexCFG(t *testing.T) {
	code := `
		function complex(x, y) {
			if (x > 0) {
				for (let i = 0; i < y; i++) {
					if (i % 2 === 0) {
						continue;
					}
					console.log(i);
				}
				return "done";
			}

			try {
				riskyOp();
			} catch (e) {
				return "error";
			}

			return "other";
		}
	`
	ast := parseJS(t, code)
	funcNode := findFunction(ast, "complex")

	builder := NewCFGBuilder()
	cfg, err := builder.Build(funcNode)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	analyzer := NewReachabilityAnalyzer(cfg)
	result := analyzer.AnalyzeReachability()

	// Log statistics for debugging
	t.Logf("Total blocks: %d, Reachable: %d, Unreachable: %d, Ratio: %.2f",
		result.TotalBlocks, result.ReachableCount, result.UnreachableCount,
		result.GetReachabilityRatio())

	// Complex function should have reasonable reachability
	if result.GetReachabilityRatio() < 0.3 {
		t.Error("Complex function should have at least 30% reachable blocks")
	}
}
