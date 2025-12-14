package btree

type BIter struct {
	tree *BTree
	path []BNode
	pos  []uint16
}

func (tree *BTree) SeekLE(key []byte) *BIter {
	iter := &BIter{tree: tree}
	if tree.root == 0 {
		return iter
	}
	node := tree.get(tree.root)
	for {
		idx := nodeLookupLE(node, key)
		iter.path = append(iter.path, node)
		iter.pos = append(iter.pos, idx)
		if node.btype() == BNODE_LEAF {
			break
		}
		node = tree.get(node.getPtr(idx))
	}
	return iter
}

func (iter *BIter) Valid() bool {
	if len(iter.path) == 0 {
		return false
	}
	leaf := iter.path[len(iter.path)-1]
	return iter.pos[len(iter.pos)-1] < leaf.nkeys()
}

func (iter *BIter) Key() []byte {
	leaf := iter.path[len(iter.path)-1]
	pos := iter.pos[len(iter.pos)-1]
	return leaf.getKey(pos)
}

func (iter *BIter) Val() []byte {
	leaf := iter.path[len(iter.path)-1]
	pos := iter.pos[len(iter.pos)-1]
	return leaf.getVal(pos)
}

func (iter *BIter) Next() {
	level := len(iter.path) - 1
	leaf := iter.path[level]
	pos := uint16(iter.pos[level])

	if pos+1 < leaf.nkeys() {
		iter.pos[level] = pos + 1
		return
	}

	for level > 0 {
		level--
		node := iter.path[level]
		pos = iter.pos[level]

		if pos+1 < node.nkeys() {
			iter.pos[level] = pos + 1
			child := iter.tree.get(node.getPtr(pos + 1))
			level++
			iter.path[level] = child
			iter.pos[level] = 0

			for child.btype() == BNODE_INTERNAL {
				child = iter.tree.get(child.getPtr(0))
				level++
				iter.path = append(iter.path, child)
				iter.pos = append(iter.pos, 0)
			}
			return
		}
	}

	iter.path = iter.path[:0]
	iter.pos = iter.pos[:0]
}
