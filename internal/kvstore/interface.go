package kvstore

import "github.com/iamppborah/svacara-db/internal/btree"

type Storage interface {
	Get(key []byte) ([]byte, bool)
	Set(key []byte, val []byte) error
	Del(key []byte) (bool, error)
	Seek(key []byte) *btree.BIter
	TreeRoot() uint64
	PageRead(ptr uint64) btree.BNode
	Close() error
}

var _ Storage = (*KV)(nil)
