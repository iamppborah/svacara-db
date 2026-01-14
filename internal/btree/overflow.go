package btree

import "encoding/binary"

const OVFL_MAGIC = 0xBEEF

func (tree *BTree) allocOverflow(data []byte) uint64 {
	node := BNode(make([]byte, BTREE_PAGE_SIZE))
	node.setHeader(BNODE_LEAF, 1)
	remaining := data
	firstPtr := uint64(0)
	var prevPtr uint64

	for len(remaining) > 0 {
		chunk := remaining
		maxChunk := BTREE_PAGE_SIZE - 12
		if len(chunk) > maxChunk {
			chunk = chunk[:maxChunk]
		}

		page := BNode(make([]byte, BTREE_PAGE_SIZE))
		binary.LittleEndian.PutUint16(page[0:2], OVFL_MAGIC)
		binary.LittleEndian.PutUint16(page[2:4], uint16(len(chunk)))
		binary.LittleEndian.PutUint64(page[4:12], 0)
		copy(page[12:], chunk)

		ptr := tree.alloc(page)
		if firstPtr == 0 {
			firstPtr = ptr
		}
		if prevPtr != 0 {
			prevPage := BNode(tree.getPtr(prevPtr))
			binary.LittleEndian.PutUint64(prevPage[4:12], ptr)
		}
		prevPtr = ptr
		remaining = remaining[len(chunk):]
	}

	return firstPtr
}

func (tree *BTree) readOverflow(ptr uint64) []byte {
	var out []byte
	for ptr != 0 {
		page := BNode(tree.getPtr(ptr))
		if binary.LittleEndian.Uint16(page[0:2]) != OVFL_MAGIC {
			break
		}
		chunkLen := binary.LittleEndian.Uint16(page[2:4])
		out = append(out, page[12:12+chunkLen]...)
		ptr = binary.LittleEndian.Uint64(page[4:12])
	}
	return out
}

func (tree *BTree) freeOverflow(ptr uint64) {
	for ptr != 0 {
		page := BNode(tree.getPtr(ptr))
		next := binary.LittleEndian.Uint64(page[4:12])
		tree.free(ptr)
		ptr = next
	}
}

func overflowSize(data []byte) int {
	n := len(data)
	pages := 0
	for n > 0 {
		pages++
		maxChunk := BTREE_PAGE_SIZE - 12
		if n > maxChunk {
			n -= maxChunk
		} else {
			n = 0
		}
	}
	return pages
}
