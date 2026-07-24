package apted

import (
	"testing"
)

func TestNewTreeNode(t *testing.T) {
	node := NewTreeNode(1, "Test")
	if node.ID != 1 {
		t.Errorf("Expected ID 1, got %d", node.ID)
	}
	if node.Label != "Test" {
		t.Errorf("Expected label 'Test', got %s", node.Label)
	}
	if len(node.Children) != 0 {
		t.Errorf("Expected empty children, got %d", len(node.Children))
	}
	if node.Parent != nil {
		t.Error("Expected nil parent")
	}
}

func TestAddChild(t *testing.T) {
	parent := NewTreeNode(0, "Parent")
	child := NewTreeNode(1, "Child")
	parent.AddChild(child)

	if len(parent.Children) != 1 {
		t.Fatalf("Expected 1 child, got %d", len(parent.Children))
	}
	if parent.Children[0] != child {
		t.Error("Child mismatch")
	}
	if child.Parent != parent {
		t.Error("Parent reference not set")
	}
}

func TestAddChild_Nil(t *testing.T) {
	parent := NewTreeNode(0, "Parent")
	parent.AddChild(nil)
	if len(parent.Children) != 0 {
		t.Errorf("Expected no children after adding nil, got %d", len(parent.Children))
	}
}

func TestIsLeaf(t *testing.T) {
	leaf := NewTreeNode(0, "Leaf")
	if !leaf.IsLeaf() {
		t.Error("Expected leaf")
	}

	parent := NewTreeNode(0, "Parent")
	parent.AddChild(NewTreeNode(1, "Child"))
	if parent.IsLeaf() {
		t.Error("Expected non-leaf")
	}
}

func TestSize(t *testing.T) {
	tests := []struct {
		name     string
		buildFn  func() *TreeNode
		expected int
	}{
		{"single node", func() *TreeNode { return NewTreeNode(0, "A") }, 1},
		{"parent with one child", func() *TreeNode {
			p := NewTreeNode(0, "A")
			p.AddChild(NewTreeNode(1, "B"))
			return p
		}, 2},
		{"three-level tree", func() *TreeNode {
			root := NewTreeNode(0, "A")
			b := NewTreeNode(1, "B")
			b.AddChild(NewTreeNode(2, "C"))
			b.AddChild(NewTreeNode(3, "D"))
			root.AddChild(b)
			root.AddChild(NewTreeNode(4, "E"))
			return root
		}, 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := tt.buildFn()
			if got := node.Size(); got != tt.expected {
				t.Errorf("Size() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestSizeWithDepthLimit(t *testing.T) {
	// Build a chain: A -> B -> C -> D
	d := NewTreeNode(3, "D")
	c := NewTreeNode(2, "C")
	c.AddChild(d)
	b := NewTreeNode(1, "B")
	b.AddChild(c)
	a := NewTreeNode(0, "A")
	a.AddChild(b)

	tests := []struct {
		limit    int
		expected int
	}{
		{0, 1}, // depth 0: treat as leaf
		{1, 2}, // A + B (B treated as leaf)
		{2, 3}, // A + B + C (C treated as leaf)
		{3, 4}, // A + B + C + D
		{100, 4},
	}

	for _, tt := range tests {
		if got := a.SizeWithDepthLimit(tt.limit); got != tt.expected {
			t.Errorf("SizeWithDepthLimit(%d) = %d, want %d", tt.limit, got, tt.expected)
		}
	}
}

func TestHeight(t *testing.T) {
	tests := []struct {
		name     string
		buildFn  func() *TreeNode
		expected int
	}{
		{"leaf", func() *TreeNode { return NewTreeNode(0, "A") }, 0},
		{"one level", func() *TreeNode {
			p := NewTreeNode(0, "A")
			p.AddChild(NewTreeNode(1, "B"))
			return p
		}, 1},
		{"two levels", func() *TreeNode {
			root := NewTreeNode(0, "A")
			b := NewTreeNode(1, "B")
			b.AddChild(NewTreeNode(2, "C"))
			root.AddChild(b)
			return root
		}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := tt.buildFn()
			if got := node.Height(); got != tt.expected {
				t.Errorf("Height() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestHeightWithDepthLimit(t *testing.T) {
	// chain: A -> B -> C
	c := NewTreeNode(2, "C")
	b := NewTreeNode(1, "B")
	b.AddChild(c)
	a := NewTreeNode(0, "A")
	a.AddChild(b)

	if got := a.HeightWithDepthLimit(0); got != 0 {
		t.Errorf("HeightWithDepthLimit(0) = %d, want 0", got)
	}
	if got := a.HeightWithDepthLimit(1); got != 1 {
		t.Errorf("HeightWithDepthLimit(1) = %d, want 1", got)
	}
	if got := a.HeightWithDepthLimit(100); got != 2 {
		t.Errorf("HeightWithDepthLimit(100) = %d, want 2", got)
	}
}

func TestString(t *testing.T) {
	node := NewTreeNode(42, "FunctionDef")
	s := node.String()
	if s != "Node{ID: 42, Label: FunctionDef, Children: 0}" {
		t.Errorf("Unexpected string: %s", s)
	}
}

func TestPostOrderTraversal(t *testing.T) {
	//     A
	//    / \
	//   B   C
	//  / \
	// D   E
	root := NewTreeNode(0, "A")
	b := NewTreeNode(1, "B")
	c := NewTreeNode(2, "C")
	d := NewTreeNode(3, "D")
	e := NewTreeNode(4, "E")
	b.AddChild(d)
	b.AddChild(e)
	root.AddChild(b)
	root.AddChild(c)

	PostOrderTraversal(root)

	// Post-order: D(0), E(1), B(2), C(3), A(4)
	expected := map[string]int{"D": 0, "E": 1, "B": 2, "C": 3, "A": 4}
	nodes := []*TreeNode{root, b, c, d, e}
	for _, n := range nodes {
		if exp, ok := expected[n.Label]; ok {
			if n.PostOrderID != exp {
				t.Errorf("Node %s: PostOrderID = %d, want %d", n.Label, n.PostOrderID, exp)
			}
		}
	}
}

func TestPostOrderTraversal_Nil(t *testing.T) {
	PostOrderTraversal(nil) // should not panic
}

func TestComputeLeftMostLeaves(t *testing.T) {
	//     A
	//    / \
	//   B   C
	//  / \
	// D   E
	root := NewTreeNode(0, "A")
	b := NewTreeNode(1, "B")
	c := NewTreeNode(2, "C")
	d := NewTreeNode(3, "D")
	e := NewTreeNode(4, "E")
	b.AddChild(d)
	b.AddChild(e)
	root.AddChild(b)
	root.AddChild(c)

	PostOrderTraversal(root)
	ComputeLeftMostLeaves(root)

	// D is leftmost leaf for D, B, and A
	if d.LeftMostLeaf != d.PostOrderID {
		t.Errorf("D leftmost leaf = %d, want %d", d.LeftMostLeaf, d.PostOrderID)
	}
	if b.LeftMostLeaf != d.PostOrderID {
		t.Errorf("B leftmost leaf = %d, want %d", b.LeftMostLeaf, d.PostOrderID)
	}
	if root.LeftMostLeaf != d.PostOrderID {
		t.Errorf("A leftmost leaf = %d, want %d", root.LeftMostLeaf, d.PostOrderID)
	}
	// C is its own leftmost leaf
	if c.LeftMostLeaf != c.PostOrderID {
		t.Errorf("C leftmost leaf = %d, want %d", c.LeftMostLeaf, c.PostOrderID)
	}
}

func TestComputeLeftMostLeaves_Nil(t *testing.T) {
	ComputeLeftMostLeaves(nil) // should not panic
}

func TestComputeKeyRoots(t *testing.T) {
	root := NewTreeNode(0, "A")
	b := NewTreeNode(1, "B")
	c := NewTreeNode(2, "C")
	b.AddChild(NewTreeNode(3, "D"))
	root.AddChild(b)
	root.AddChild(c)

	PostOrderTraversal(root)
	ComputeLeftMostLeaves(root)
	keyRoots := ComputeKeyRoots(root)

	if len(keyRoots) == 0 {
		t.Fatal("Expected at least one key root")
	}
	// Root should always be a key root
	rootFound := false
	for _, kr := range keyRoots {
		if kr == root.PostOrderID {
			rootFound = true
		}
	}
	if !rootFound {
		t.Error("Root should be a key root")
	}
}

func TestComputeKeyRoots_Nil(t *testing.T) {
	kr := ComputeKeyRoots(nil)
	if len(kr) != 0 {
		t.Errorf("Expected empty key roots for nil, got %d", len(kr))
	}
}

func TestPrepareTreeForAPTED(t *testing.T) {
	root := NewTreeNode(0, "A")
	root.AddChild(NewTreeNode(1, "B"))
	root.AddChild(NewTreeNode(2, "C"))

	keyRoots := PrepareTreeForAPTED(root)
	if len(keyRoots) == 0 {
		t.Fatal("Expected key roots")
	}

	// Verify post-order IDs were assigned
	if root.PostOrderID <= 0 {
		t.Error("Root should have positive post-order ID")
	}
}

func TestAddChildInvalidatesPreparedTree(t *testing.T) {
	root := NewTreeNode(0, "A")
	root.AddChild(NewTreeNode(1, "B"))
	PrepareTreeForAPTED(root)

	root.AddChild(NewTreeNode(2, "C"))
	keyRoots := ensurePreparedForAPTED(root)

	if root.PostOrderID != 2 {
		t.Fatalf("root post-order ID = %d, want 2 after mutation", root.PostOrderID)
	}
	if len(keyRoots) == 0 {
		t.Fatal("expected recomputed key roots")
	}
}

func TestPrepareTreeForAPTED_Nil(t *testing.T) {
	kr := PrepareTreeForAPTED(nil)
	if len(kr) != 0 {
		t.Errorf("Expected empty key roots for nil, got %d", len(kr))
	}
}

func TestGetNodeByPostOrderID(t *testing.T) {
	root := NewTreeNode(0, "A")
	b := NewTreeNode(1, "B")
	c := NewTreeNode(2, "C")
	root.AddChild(b)
	root.AddChild(c)
	PostOrderTraversal(root)

	// Find each node by its post-order ID
	for _, node := range []*TreeNode{root, b, c} {
		found := GetNodeByPostOrderID(root, node.PostOrderID)
		if found != node {
			t.Errorf("GetNodeByPostOrderID(%d) returned wrong node", node.PostOrderID)
		}
	}

	// Non-existent ID
	if found := GetNodeByPostOrderID(root, 999); found != nil {
		t.Error("Expected nil for non-existent post-order ID")
	}

	// Nil root
	if found := GetNodeByPostOrderID(nil, 0); found != nil {
		t.Error("Expected nil for nil root")
	}
}

func TestGetSubtreeNodes(t *testing.T) {
	root := NewTreeNode(0, "A")
	b := NewTreeNode(1, "B")
	c := NewTreeNode(2, "C")
	root.AddChild(b)
	root.AddChild(c)

	nodes := GetSubtreeNodes(root)
	if len(nodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(nodes))
	}

	// Nil
	nodes = GetSubtreeNodes(nil)
	if len(nodes) != 0 {
		t.Errorf("Expected 0 nodes for nil, got %d", len(nodes))
	}
}

func TestGetSubtreeNodesWithDepthLimit(t *testing.T) {
	// A -> B -> C
	c := NewTreeNode(2, "C")
	b := NewTreeNode(1, "B")
	b.AddChild(c)
	a := NewTreeNode(0, "A")
	a.AddChild(b)

	if got := len(GetSubtreeNodesWithDepthLimit(a, 0)); got != 0 {
		t.Errorf("Depth 0: expected 0, got %d", got)
	}
	if got := len(GetSubtreeNodesWithDepthLimit(a, 1)); got != 1 {
		t.Errorf("Depth 1: expected 1, got %d", got)
	}
	if got := len(GetSubtreeNodesWithDepthLimit(a, 2)); got != 2 {
		t.Errorf("Depth 2: expected 2, got %d", got)
	}
	if got := len(GetSubtreeNodesWithDepthLimit(a, 100)); got != 3 {
		t.Errorf("Depth 100: expected 3, got %d", got)
	}
}

func TestOriginalNode_Any(t *testing.T) {
	node := NewTreeNode(0, "Test")
	// OriginalNode is `any` — verify it works with arbitrary types
	node.OriginalNode = "a string"
	if node.OriginalNode != "a string" {
		t.Error("Expected string stored as OriginalNode")
	}

	type customAST struct{ Name string }
	node.OriginalNode = &customAST{Name: "foo"}
	if ast, ok := node.OriginalNode.(*customAST); !ok || ast.Name != "foo" {
		t.Error("Expected customAST stored as OriginalNode")
	}
}
