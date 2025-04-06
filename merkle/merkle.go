package merkle

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
)

/*
A Merkle Tree is a binary tree of hashes where:

Leaf nodes contain hashes of transactions.
Non-leaf nodes contain hashes of their children.
The root hash (called MerkleRoot) acts as a fingerprint for the entire dataset.

*/

// Hashable is the behavior that transaction data must have to be used in merkle tree.
type Hashable[T any] interface {
	Hash() ([]byte, error)
	Equal(other T) bool
}

//==============================================================================
// Node

// Node represents a node, root, leaf in merkle tree. its stores a pointer to immediate
// relationships, a hash , data if its a leaf node and other metadata.
type Node[T Hashable[T]] struct {
	Tree   *Tree[T] //pointer to the tree for hashing.
	Parent *Node[T]
	Left   *Node[T]
	Right  *Node[T]
	Hash   []byte // hash of (left + right) OR raw data (transaction).
	Value  T      // raw data only for leaves.
	leaf   bool   // true if the node is a leaf.
	dup    bool   // true for duplicate leaf, to fix odd number of transaction, Merkle tree must be balanced.
}

// verify walks down the tree until hitting a leaf, calculating the hash at
// each level and returning the resulting hash of the node.
func (n *Node[T]) verify() ([]byte, error) {
	//if its a leaf then just return hash of the data
	if n.leaf {
		return n.Value.Hash()
	}

	//verify the right child recursively
	rightChildBytes, err := n.Right.verify()
	if err != nil {
		return nil, fmt.Errorf("verify right: %w", err)
	}

	//verify left child recursively
	leftChildBytes, err := n.Left.verify()
	if err != nil {
		return nil, fmt.Errorf("verify left: %w", err)
	}

	//recompute the current node's hash
	h := n.Tree.hashStrategy()
	bs := append(leftChildBytes, rightChildBytes...)
	if _, err := h.Write(bs); err != nil {
		return nil, fmt.Errorf("writing into hasher: %w", err)
	}

	return h.Sum(nil), nil
}

//==============================================================================
// Tree

// Tree represents the Merkle tree.
type Tree[T Hashable[T]] struct {
	Root         *Node[T]
	Leafs        []*Node[T]
	MerkleRoot   []byte           //Final root hash
	hashStrategy func() hash.Hash //default sha256.New
}

// NewTree constructs a new Merkle tree with values that have Hashable behavior.
func NewTree[T Hashable[T]](values []T, options ...func(tree *Tree[T])) (*Tree[T], error) {
	defaultHashStrategy := sha256.New

	t := Tree[T]{
		hashStrategy: defaultHashStrategy,
	}

	//apply the options
	for _, opt := range options {
		opt(&t)
	}

	if err := t.Generate(values); err != nil {
		return nil, fmt.Errorf("generating tree: %w", err)
	}

	return &t, nil
}

// Generate constructs the leafs and nodes of the tree from the specified
// data. If the tree has been generated previously, the tree is re-generated
// from scratch.
func (t *Tree[T]) Generate(values []T) error {
	if len(values) == 0 {
		return errors.New("cannot construct a tree with no contents")
	}

	//generate the leafs from TXs.
	var leafs []*Node[T]
	for _, val := range values {
		hash, err := val.Hash()
		if err != nil {
			return fmt.Errorf("hash: %w", err)
		}

		leafs = append(leafs, &Node[T]{
			Hash:  hash,
			Value: val,
			leaf:  true,
			Tree:  t,
		})
	}

	//handle odd number of tx
	if len(values)%2 == 1 {
		dup := &Node[T]{
			Hash:  leafs[len(leafs)-1].Hash,
			Value: leafs[len(leafs)-1].Value,
			leaf:  true,
			dup:   true,
			Tree:  t,
		}

		leafs = append(leafs, dup)
	}

	//build the middle of the tree
	root, err := buildIntermediate(leafs, t)
	if err != nil {
		return fmt.Errorf("buildIntermediate: %w", err)
	}

	t.Root = root
	t.Leafs = leafs
	t.MerkleRoot = root.Hash

	return nil
}

// Verify validates the hashes at each level of the tree and returns true
// if the resulting hash at the root of the tree matches the resulting root hash.
func (t *Tree[T]) Verify() error {
	calculatedMerkleRoot, err := t.Root.verify()
	if err != nil {
		return fmt.Errorf("verify root: %w", err)
	}

	if !bytes.Equal(calculatedMerkleRoot, t.MerkleRoot) {
		return errors.New("root hash is invalid")
	}
	return nil
}

// Proof returns the set of hashes and the order of concatenating those
// hashes for proving a transaction is in the tree.
func (t *Tree[T]) Proof(data T) ([][]byte, []int64, error) {
	//1. search through all leaf nodes.
	for _, node := range t.Leafs {
		//2. check if this is the node we want
		if !node.Value.Equal(data) {
			continue
		}

		var merkleProofs [][]byte
		var order []int64

		//3.start with the immediate parent node
		parent := node.Parent

		//4. walk up the tree
		for parent != nil {
			//5. check to see if the data is left or right child
			if bytes.Equal(parent.Left.Hash, node.Hash) {
				//node is the left child, so we need the right child hash as proof
				merkleProofs = append(merkleProofs, parent.Right.Hash)
				//right ones concat second, 1
				order = append(order, 1)
			} else {
				//node is the right child, so we need left child hash as proof
				merkleProofs = append(merkleProofs, parent.Left.Hash)
				//left ones concat first , 0
				order = append(order, 0)
			}
		}

		return merkleProofs, order, nil
	}
	return nil, nil, errors.New("unable to find data in tree")
}

//==============================================================================

// buildIntermediate is a helper function that for a given list of leaf nodes,
// constructs the intermediate and root levels of the tree. Returns the resulting
// root node of the tree.
func buildIntermediate[T Hashable[T]](nodeList []*Node[T], t *Tree[T]) (*Node[T], error) {
	//creating merkle tree from Bottom up.
	//leafs are the most bottom layer of the tree. the goal is to build the parent
	//node for every 2 leafs and so on.
	//pair two leaves at a time and hash their combined hashes to create a parent node.
	var nodes []*Node[T]
	for i := 0; i < len(nodeList); i += 2 {
		leftIdx, rightIdx := i, i+1

		//encounter for duplicated one
		if i+1 == len(nodeList) {
			rightIdx = i
		}

		h := t.hashStrategy()

		left := nodeList[leftIdx]
		right := nodeList[rightIdx]
		combinedHash := append(left.Hash, right.Hash...)

		//hash them as combined
		if _, err := h.Write(combinedHash); err != nil {
			return nil, fmt.Errorf("writing into hash: %w", err)
		}

		//create their parent
		parent := Node[T]{
			Left:  left,
			Right: right,
			Hash:  h.Sum(nil),
			Tree:  t,
		}

		//append to the list of nodes (intermediates)
		nodes = append(nodes, &parent)

		left.Parent = &parent
		right.Parent = &parent

		//first call will be with leafs, after that will be with the parents of the leafs
		//so in each level the nodeList will shrink in size
		if len(nodeList) == 2 {
			//when we down to 2 nodes inside of nodeList, their parent will be the root
			return &parent, nil
		}
	}
	// Recurse with the new parent nodes (next level up)
	return buildIntermediate(nodes, t)
}
