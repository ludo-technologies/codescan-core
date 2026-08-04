package apted

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

// TreeNode represents a node in the ordered tree for APTED algorithm.
// This is a language-agnostic representation; language-specific parsers
// should convert their AST nodes into TreeNode via a TreeConverter.
type TreeNode struct {
	ID    int
	Label string

	Children []*TreeNode
	Parent   *TreeNode

	// APTED-specific fields for optimization
	PostOrderID  int
	LeftMostLeaf int
	KeyRoot      bool

	preparationMu  sync.Mutex
	cachedKeyRoots atomic.Pointer[[]int]

	// OriginalNode holds the original language-specific AST node.
	// This field is opaque to polyscan core; language adapters store their
	// own node type here and recover it via type assertion:
	//
	//   // pyscn
	//   pyNode := treeNode.OriginalNode.(*parser.Node)
	//
	// Populating it is a deliberate choice, not a default. A parser node that
	// carries a parent pointer pins its whole file AST for as long as the
	// converted tree lives, which is the entire run for clone detection —
	// jscan's converter therefore leaves this empty, so do not assume an
	// adapter has filled it in.
	OriginalNode any
}

// NewTreeNode creates a new tree node with the given ID and label.
func NewTreeNode(id int, label string) *TreeNode {
	return &TreeNode{
		ID:       id,
		Label:    label,
		Children: []*TreeNode{},
	}
}

// AddChild adds a child node to this node.
func (t *TreeNode) AddChild(child *TreeNode) {
	if child != nil {
		child.Parent = t
		t.Children = append(t.Children, child)

		child.cachedKeyRoots.Store(nil)
		for node := t; node != nil; node = node.Parent {
			node.cachedKeyRoots.Store(nil)
		}
	}
}

// IsLeaf returns true if this node has no children.
func (t *TreeNode) IsLeaf() bool {
	return len(t.Children) == 0
}

const defaultMaxDepth = 1000

// Size returns the size of the subtree rooted at this node.
func (t *TreeNode) Size() int {
	return t.SizeWithDepthLimit(defaultMaxDepth)
}

// SizeWithDepthLimit returns the size with maximum recursion depth limit.
func (t *TreeNode) SizeWithDepthLimit(maxDepth int) int {
	if maxDepth <= 0 {
		return 1
	}
	size := 1
	for _, child := range t.Children {
		size += child.SizeWithDepthLimit(maxDepth - 1)
	}
	return size
}

// Height returns the height of the subtree rooted at this node.
func (t *TreeNode) Height() int {
	return t.HeightWithDepthLimit(defaultMaxDepth)
}

// HeightWithDepthLimit returns the height with maximum recursion depth limit.
func (t *TreeNode) HeightWithDepthLimit(maxDepth int) int {
	if maxDepth <= 0 {
		return 0
	}
	if t.IsLeaf() {
		return 0
	}
	maxHeight := 0
	for _, child := range t.Children {
		if h := child.HeightWithDepthLimit(maxDepth - 1); h > maxHeight {
			maxHeight = h
		}
	}
	return maxHeight + 1
}

// String returns a string representation of the node.
func (t *TreeNode) String() string {
	return fmt.Sprintf("Node{ID: %d, Label: %s, Children: %d}", t.ID, t.Label, len(t.Children))
}

// PostOrderTraversal performs post-order traversal and assigns post-order IDs.
func PostOrderTraversal(root *TreeNode) {
	if root == nil {
		return
	}
	postOrderID := 0
	postOrderTraversalRecursive(root, &postOrderID)
}

func postOrderTraversalRecursive(node *TreeNode, postOrderID *int) {
	if node == nil {
		return
	}
	for _, child := range node.Children {
		postOrderTraversalRecursive(child, postOrderID)
	}
	node.PostOrderID = *postOrderID
	*postOrderID++
}

// ComputeLeftMostLeaves computes left-most leaf descendants for all nodes.
func ComputeLeftMostLeaves(root *TreeNode) {
	if root == nil {
		return
	}
	computeLeftMostLeavesRecursive(root)
}

func computeLeftMostLeavesRecursive(node *TreeNode) int {
	if node.IsLeaf() || len(node.Children) == 0 {
		node.LeftMostLeaf = node.PostOrderID
		return node.LeftMostLeaf
	}
	leftMostLeaf := computeLeftMostLeavesRecursive(node.Children[0])
	node.LeftMostLeaf = leftMostLeaf
	for i := 1; i < len(node.Children); i++ {
		computeLeftMostLeavesRecursive(node.Children[i])
	}
	return leftMostLeaf
}

// ComputeKeyRoots identifies key roots for path decomposition.
func ComputeKeyRoots(root *TreeNode) []int {
	if root == nil {
		return []int{}
	}
	keyRoots := []int{}
	visited := make(map[int]bool)
	computeKeyRootsRecursive(root, &keyRoots, visited)
	return keyRoots
}

func computeKeyRootsRecursive(node *TreeNode, keyRoots *[]int, visited map[int]bool) {
	if node == nil {
		return
	}
	if !visited[node.LeftMostLeaf] {
		node.KeyRoot = true
		*keyRoots = append(*keyRoots, node.PostOrderID)
		visited[node.LeftMostLeaf] = true
	}
	for _, child := range node.Children {
		computeKeyRootsRecursive(child, keyRoots, visited)
	}
}

// PrepareTreeForAPTED prepares a tree for APTED algorithm by computing all necessary indices.
func PrepareTreeForAPTED(root *TreeNode) []int {
	if root == nil {
		return []int{}
	}
	root.preparationMu.Lock()
	defer root.preparationMu.Unlock()
	return prepareTreeForAPTED(root)
}

func prepareTreeForAPTED(root *TreeNode) []int {
	PostOrderTraversal(root)
	ComputeLeftMostLeaves(root)
	keyRoots := ComputeKeyRoots(root)
	sort.Ints(keyRoots)
	root.cachedKeyRoots.Store(&keyRoots)
	return keyRoots
}

// ensurePreparedForAPTED prepares a tree at most once between structural
// mutations. Once prepared, comparisons only read the tree metadata and can
// safely run concurrently with separate analyzers.
func ensurePreparedForAPTED(root *TreeNode) []int {
	if keyRoots := root.cachedKeyRoots.Load(); keyRoots != nil {
		return *keyRoots
	}
	root.preparationMu.Lock()
	defer root.preparationMu.Unlock()
	if keyRoots := root.cachedKeyRoots.Load(); keyRoots != nil {
		return *keyRoots
	}
	return prepareTreeForAPTED(root)
}

// GetNodeByPostOrderID finds a node by its post-order ID.
func GetNodeByPostOrderID(root *TreeNode, postOrderID int) *TreeNode {
	if root == nil {
		return nil
	}
	if root.PostOrderID == postOrderID {
		return root
	}
	for _, child := range root.Children {
		if node := GetNodeByPostOrderID(child, postOrderID); node != nil {
			return node
		}
	}
	return nil
}

// GetSubtreeNodes returns all nodes in the subtree rooted at the given node.
func GetSubtreeNodes(root *TreeNode) []*TreeNode {
	return GetSubtreeNodesWithDepthLimit(root, defaultMaxDepth)
}

// GetSubtreeNodesWithDepthLimit returns all nodes with maximum recursion depth limit.
func GetSubtreeNodesWithDepthLimit(root *TreeNode, maxDepth int) []*TreeNode {
	if root == nil || maxDepth <= 0 {
		return []*TreeNode{}
	}
	nodes := []*TreeNode{root}
	for _, child := range root.Children {
		nodes = append(nodes, GetSubtreeNodesWithDepthLimit(child, maxDepth-1)...)
	}
	return nodes
}
