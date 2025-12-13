package btree

func leafInsert(new BNode, old BNode, idx uint16, key []byte, val []byte) {
	new.setHeader(BNODE_LEAF, old.nkeys()+1)
	nodeAppendRange(new, old, 0, 0, idx)
	nodeAppendKV(new, idx, 0, key, val)
	nodeAppendRange(new, old, idx+1, idx, old.nkeys()-idx)
}

func leafUpdate(new BNode, old BNode, idx uint16, key []byte, val []byte) {
	new.setHeader(BNODE_LEAF, old.nkeys())
	nodeAppendRange(new, old, 0, 0, idx)
	nodeAppendKV(new, idx, 0, key, val)
	nodeAppendRange(new, old, idx+1, idx+1, old.nkeys()-(idx+1))
}

func nodeReplaceKidN(tree *BTree, new BNode, old BNode, idx uint16, kids ...BNode) {
	inc := uint16(len(kids))
	new.setHeader(BNODE_INTERNAL, old.nkeys()+inc-1)
	nodeAppendRange(new, old, 0, 0, idx)
	for i, kid := range kids {
		nodeAppendKV(new, idx+uint16(i), tree.new(kid), kid.getKey(0), nil)
	}
	nodeAppendRange(new, old, idx+inc, idx+1, old.nkeys()-(idx+1))
}

func nodeSplit2(left BNode, right BNode, old BNode) {
	nkeys := old.nkeys()
	totalBytes := old.nbytes()
	splitIdx := uint16(0)
	accum := uint16(0)
	for i := uint16(0); i < nkeys; i++ {
		kvBytes := uint16(4) + uint16(len(old.getKey(i))) + uint16(len(old.getVal(i)))
		if accum+kvBytes > totalBytes/2 && i > 0 {
			break
		}
		accum += kvBytes
		splitIdx = i + 1
	}
	left.setHeader(old.btype(), splitIdx)
	nodeAppendRange(left, old, 0, 0, splitIdx)
	right.setHeader(old.btype(), nkeys-splitIdx)
	nodeAppendRange(right, old, 0, splitIdx, nkeys-splitIdx)
}

func nodeSplit3(old BNode) (uint16, [3]BNode) {
	if old.nbytes() <= BTREE_PAGE_SIZE {
		return 1, [3]BNode{old}
	}
	left := BNode(make([]byte, 2*BTREE_PAGE_SIZE))
	right := BNode(make([]byte, BTREE_PAGE_SIZE))
	nodeSplit2(left, right, old)
	if left.nbytes() <= BTREE_PAGE_SIZE {
		return 2, [3]BNode{left, right}
	}
	leftleft := BNode(make([]byte, BTREE_PAGE_SIZE))
	middle := BNode(make([]byte, BTREE_PAGE_SIZE))
	nodeSplit2(leftleft, middle, left)
	return 3, [3]BNode{leftleft, middle, right}
}

func treeInsert(tree *BTree, node BNode, key []byte, val []byte) BNode {
	new := BNode(make([]byte, 2*BTREE_PAGE_SIZE))
	idx := nodeLookupLE(node, key)
	switch node.btype() {
	case BNODE_LEAF:
		if node.nkeys() > 0 && cmp(key, node.getKey(idx)) == 0 {
			leafUpdate(new, node, idx, key, val)
		} else {
			insertIdx := idx
			if node.nkeys() > 0 && cmp(key, node.getKey(idx)) > 0 {
				insertIdx++
			}
			leafInsert(new, node, insertIdx, key, val)
		}
	case BNODE_INTERNAL:
		kptr := node.getPtr(idx)
		knode := tree.get(kptr)
		tree.del(kptr)
		knode = treeInsert(tree, knode, key, val)
		nsplit, split := nodeSplit3(knode)
		nodeReplaceKidN(tree, new, node, idx, split[:nsplit]...)
	}
	return new
}

func (tree *BTree) Insert(key []byte, val []byte) {
	if tree.root == 0 {
		root := BNode(make([]byte, BTREE_PAGE_SIZE))
		root.setHeader(BNODE_LEAF, 1)
		nodeAppendKV(root, 0, 0, key, val)
		tree.root = tree.new(root)
		return
	}
	node := tree.get(tree.root)
	tree.del(tree.root)
	node = treeInsert(tree, node, key, val)
	nsplit, split := nodeSplit3(node)
	if nsplit > 1 {
		root := BNode(make([]byte, BTREE_PAGE_SIZE))
		root.setHeader(BNODE_INTERNAL, nsplit)
		for i, knode := range split[:nsplit] {
			ptr := tree.new(knode)
			copy(root[offsetPos(root, uint16(i)+1):], make([]byte, 2))
			nodeAppendKV(root, uint16(i), ptr, knode.getKey(0), nil)
		}
		tree.root = tree.new(root)
	} else {
		tree.root = tree.new(split[0])
	}
}

func (tree *BTree) Get(key []byte) ([]byte, bool) {
	if tree.root == 0 {
		return nil, false
	}
	node := tree.get(tree.root)
	for {
		idx := nodeLookupLE(node, key)
		switch node.btype() {
		case BNODE_LEAF:
			if cmp(key, node.getKey(idx)) == 0 {
				return node.getVal(idx), true
			}
			return nil, false
		case BNODE_INTERNAL:
			node = tree.get(node.getPtr(idx))
		}
	}
}
