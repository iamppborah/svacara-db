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
	GetPage       func(uint64) BNode
	NewPage       func(BNode) uint64
	SetPage       func(uint64) BNode
	HeadPage      uint64
	HeadSeq       uint64
	TailPage      uint64
	TailSeq       uint64
	maxSeq        uint64
}

func (fl *FreeList) PopHead() uint64 {
	if fl.HeadPage == 0 {
		return 0
	}
	node := LNode(fl.GetPage(fl.HeadPage))
	ptr := node.getPtr(int(fl.HeadSeq))
	fl.HeadSeq++

	next := node.getNext()
	if fl.HeadSeq*8 >= FREE_LIST_CAP {
		if next != 0 {
			fl.HeadSeq = 0
		}
		newHead := node.getNext()
		if newHead != fl.HeadPage {
			fl.SetPage(fl.HeadPage)
		}
		fl.HeadPage = newHead
	}
	return ptr
}

func (fl *FreeList) PushTail(ptr uint64) {
	if fl.TailPage == 0 {
		data := make([]byte, BTREE_PAGE_SIZE)
		LNode(data).setNext(0)
		fl.TailPage = fl.NewPage(data)
		fl.HeadPage = fl.TailPage
	}
	node := LNode(fl.SetPage(fl.TailPage))
	idx := fl.TailSeq % FREE_LIST_CAP
	node.setPtr(int(idx), ptr)
	fl.TailSeq++

	if fl.TailSeq%FREE_LIST_CAP == 0 {
		data := make([]byte, BTREE_PAGE_SIZE)
		LNode(data).setNext(0)
		next := fl.NewPage(data)
		node.setNext(next)
		fl.TailPage = next
	}
}

func (fl *FreeList) SetMaxSeq() {
	fl.maxSeq = fl.TailSeq
}
