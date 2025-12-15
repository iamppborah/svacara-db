package btree

import "encoding/binary"

const FREE_LIST_HEADER = 8
const FREE_LIST_CAP = (BTREE_PAGE_SIZE - FREE_LIST_HEADER) / 8

type LNode []byte

func (node LNode) getNext() uint64 {
	return binary.LittleEndian.Uint64(node[0:8])
}

func (node LNode) setNext(next uint64) {
	binary.LittleEndian.PutUint64(node[0:8], next)
}

func (node LNode) getPtr(idx int) uint64 {
	pos := FREE_LIST_HEADER + idx*8
	return binary.LittleEndian.Uint64(node[pos:])
}

func (node LNode) setPtr(idx int, ptr uint64) {
	pos := FREE_LIST_HEADER + idx*8
	binary.LittleEndian.PutUint64(node[pos:], ptr)
}

type FreeList struct {
	get     func(uint64) BNode
	new     func(BNode) uint64
	set     func(uint64) BNode
	headPage uint64
	headSeq  uint64
	tailPage uint64
	tailSeq  uint64
	maxSeq   uint64
}

func (fl *FreeList) PopHead() uint64 {
	if fl.headPage == 0 {
		return 0
	}
	node := LNode(fl.get(fl.headPage))
	ptr := node.getPtr(int(fl.headSeq))
	fl.headSeq++

	next := node.getNext()
	if fl.headSeq*8 >= FREE_LIST_CAP {
		if next != 0 {
			fl.headSeq = 0
		}
		newHead := node.getNext()
		if newHead != fl.headPage {
			fl.set(fl.headPage)
		}
		fl.headPage = newHead
	}
	return ptr
}

func (fl *FreeList) PushTail(ptr uint64) {
	if fl.tailPage == 0 {
		data := make([]byte, BTREE_PAGE_SIZE)
		LNode(data).setNext(0)
		fl.tailPage = fl.new(data)
		fl.headPage = fl.tailPage
	}
	node := LNode(fl.set(fl.tailPage))
	idx := fl.tailSeq % FREE_LIST_CAP
	node.setPtr(int(idx), ptr)
	fl.tailSeq++

	if fl.tailSeq%FREE_LIST_CAP == 0 {
		data := make([]byte, BTREE_PAGE_SIZE)
		LNode(data).setNext(0)
		next := fl.new(data)
		node.setNext(next)
		fl.tailPage = next
	}
}

func (fl *FreeList) SetMaxSeq() {
	fl.maxSeq = fl.tailSeq
}
