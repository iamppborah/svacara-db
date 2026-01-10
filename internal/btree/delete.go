package btree

func shouldMerge(tree *BTree, node BNode, idx uint16, updated BNode) (int, BNode) {
	if updated.nbytes() > BTREE_PAGE_SIZE/4 {
		return 0, BNode{}
	}
	if idx > 0 {
		sibling := BNode(tree.getPtr(node.getPtr(idx - 1)))
		merged := sibling.nbytes() + updated.nbytes() - HEADER
		if merged <= BTREE_PAGE_SIZE {
			return -1, sibling
		}
	}
	if idx+1 < node.nkeys() {
		sibling := BNode(tree.getPtr(node.getPtr(idx + 1)))
		merged := sibling.nbytes() + updated.nbytes() - HEADER
		if merged <= BTREE_PAGE_SIZE {
			return 1, sibling
		}
	}
	return 0, BNode{}
}

func nodeMerge(new BNode, left BNode, right BNode) {
	new.setHeader(left.btype(), left.nkeys()+right.nkeys())
	nodeAppendRange(new, left, 0, 0, left.nkeys())
	nodeAppendRange(new, right, left.nkeys(), 0, right.nkeys())
}

func nodeReplace2Kid(new BNode, node BNode, idx uint16, ptr uint64, key []byte) {
	new.setHeader(BNODE_INTERNAL, node.nkeys()-1)
	nodeAppendRange(new, node, 0, 0, idx)
	nodeAppendKV(new, idx, ptr, key, nil)
	nodeAppendRange(new, node, idx+1, idx+2, node.nkeys()-(idx+2))
}

func treeDelete(tree *BTree, node BNode, key []byte) BNode {
	idx := nodeLookupLE(node, key)
	switch node.btype() {
	case BNODE_LEAF:
		if cmp(key, node.getKey(idx)) != 0 {
			return BNode{}
		}
		new := BNode(make([]byte, BTREE_PAGE_SIZE))
		new.setHeader(BNODE_LEAF, node.nkeys()-1)
		nodeAppendRange(new, node, 0, 0, idx)
		nodeAppendRange(new, node, idx, idx+1, node.nkeys()-(idx+1))
		return new

	case BNODE_INTERNAL:
		kptr := node.getPtr(idx)
		updated := treeDelete(tree, tree.getPtr(kptr), key)
		if len(updated) == 0 {
			return BNode{}
		}
		tree.free(kptr)

		new := BNode(make([]byte, BTREE_PAGE_SIZE))
		mergeDir, sibling := shouldMerge(tree, node, idx, updated)

		switch {
		case mergeDir < 0:
			merged := BNode(make([]byte, BTREE_PAGE_SIZE))
			nodeMerge(merged, sibling, updated)
			tree.free(node.getPtr(idx - 1))
			nodeReplace2Kid(new, node, idx-1, tree.alloc(merged), merged.getKey(0))
		case mergeDir > 0:
			merged := BNode(make([]byte, BTREE_PAGE_SIZE))
			nodeMerge(merged, updated, sibling)
			tree.free(node.getPtr(idx + 1))
			nodeReplace2Kid(new, node, idx, tree.alloc(merged), merged.getKey(0))
		case mergeDir == 0 && updated.nkeys() == 0:
			new.setHeader(BNODE_INTERNAL, 0)
		case mergeDir == 0 && updated.nkeys() > 0:
			nodeReplaceKidN(tree, new, node, idx, updated)
		}
		return new
	}
	return BNode{}
}

func (tree *BTree) Delete(key []byte) bool {
	if tree.Root == 0 {
		return false
	}
	node := tree.getPtr(tree.Root)
	updated := treeDelete(tree, node, key)
	if len(updated) == 0 {
		return false
	}
	tree.free(tree.Root)
	if updated.nkeys() == 0 {
		tree.Root = 0
		return true
	}
	tree.Root = tree.alloc(updated)
	return true
}
