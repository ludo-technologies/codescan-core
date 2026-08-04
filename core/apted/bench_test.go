package apted

import (
	"fmt"
	"testing"
)

// benchLabels is a small closed label alphabet, mirroring how a real AST draws
// node labels from a fixed set of node types.
var benchLabels = []string{
	"FunctionDeclaration", "IfStatement", "BinaryExpression", "CallExpression",
	"MemberExpression", "Identifier", "Literal", "ReturnStatement",
	"BlockStatement", "VariableDeclaration",
}

// buildBenchTree builds a deterministic tree of exactly nodeCount nodes with the
// given branching factor. Labels cycle through benchLabels, offset by seed so
// two trees built with different seeds differ in labels without differing in
// shape — the case APTED spends the most work on.
func buildBenchTree(nodeCount, branching, seed int) *TreeNode {
	if nodeCount < 1 {
		nodeCount = 1
	}
	root := NewTreeNode(0, benchLabels[seed%len(benchLabels)])
	frontier := []*TreeNode{root}
	for id := 1; id < nodeCount; id++ {
		parent := frontier[0]
		if len(parent.Children) == branching {
			frontier = frontier[1:]
			parent = frontier[0]
		}
		child := NewTreeNode(id, benchLabels[(id+seed)%len(benchLabels)])
		parent.AddChild(child)
		frontier = append(frontier, child)
	}
	return root
}

// benchTreeSizes covers the exact APTED path (below largeTreeThreshold) and the
// bounded large-tree path above it.
var benchTreeSizes = []int{100, 500, 1000, 2000, 5000}

func BenchmarkAPTED_BySize(b *testing.B) {
	for _, size := range benchTreeSizes {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			analyzer := NewAPTEDAnalyzer(NewDefaultCostModel())
			tree1 := buildBenchTree(size, 3, 0)
			tree2 := buildBenchTree(size, 3, 1)
			PrepareTreeForAPTED(tree1)
			PrepareTreeForAPTED(tree2)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				analyzer.ComputeDistance(tree1, tree2)
			}
		})
	}
}

// BenchmarkAPTED_IdenticalTrees measures the best case, where every Rename
// short-circuits on equal labels.
func BenchmarkAPTED_IdenticalTrees(b *testing.B) {
	for _, size := range benchTreeSizes {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			analyzer := NewAPTEDAnalyzer(NewDefaultCostModel())
			tree1 := buildBenchTree(size, 3, 0)
			tree2 := buildBenchTree(size, 3, 0)
			PrepareTreeForAPTED(tree1)
			PrepareTreeForAPTED(tree2)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				analyzer.ComputeDistance(tree1, tree2)
			}
		})
	}
}

// BenchmarkAPTED_DeepTrees exercises the chain-shaped worst case for the
// recursive traversals, where branching is 1 and depth equals node count.
func BenchmarkAPTED_DeepTrees(b *testing.B) {
	for _, size := range []int{100, 500, 1000} {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			analyzer := NewAPTEDAnalyzer(NewDefaultCostModel())
			tree1 := buildBenchTree(size, 1, 0)
			tree2 := buildBenchTree(size, 1, 1)
			PrepareTreeForAPTED(tree1)
			PrepareTreeForAPTED(tree2)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				analyzer.ComputeDistance(tree1, tree2)
			}
		})
	}
}

func BenchmarkPrepareTreeForAPTED_BySize(b *testing.B) {
	for _, size := range benchTreeSizes {
		b.Run(fmt.Sprintf("nodes=%d", size), func(b *testing.B) {
			tree := buildBenchTree(size, 3, 0)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				PrepareTreeForAPTED(tree)
			}
		})
	}
}
