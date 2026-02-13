package kvstore

import (
	"fmt"
	"os"
	"syscall"

	"github.com/ppborah/svacara-db/internal/btree"
	"github.com/ppborah/svacara-db/internal/storage"
)

type SyncMode int

const (
	SyncOff    SyncMode = 0
	SyncNormal SyncMode = 1
	SyncFull   SyncMode = 2
)

type KV struct {
	Path     string
	fd       int
	file     *os.File
	tree     btree.BTree
	free     btree.FreeList
	pg       *storage.PageMgr
	syncMode SyncMode
	failed   bool
}

func Open(path string, mode SyncMode) (*KV, error) {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0664)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	fd := int(file.Fd())
	stat, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("stat: %w", err)
	}

	db := &KV{
		Path:     path,
		fd:       fd,
		file:     file,
		syncMode: mode,
	}

	db.pg = storage.NewPageMgr(file)

	if stat.Size() == 0 {
		db.pg.ResetTo(1)
	} else {
		if err := db.pg.ExtendMMap(int(stat.Size())); err != nil {
			file.Close()
			return nil, err
		}
		ptr := db.pg.Read(0)
		root, flushed, headPage, headSeq, tailPage, tailSeq, ok := storage.ReadMeta(ptr)
		if !ok {
			file.Close()
			return nil, fmt.Errorf("not a valid SvacaraDB file")
		}
		db.pg.ResetTo(flushed)
		db.tree.Root = root

		db.free = btree.FreeList{
			HeadPage: headPage,
			HeadSeq:  headSeq,
			TailPage: tailPage,
			TailSeq:  tailSeq,
		}
	}

	db.free.GetPage = func(ptr uint64) btree.BNode {
		return btree.BNode(db.pg.Read(ptr))
	}
	db.free.NewPage = func(node btree.BNode) uint64 {
		return db.pg.Alloc([]byte(node))
	}
	db.free.SetPage = func(ptr uint64) btree.BNode {
		return btree.BNode(db.pg.Write(ptr))
	}

	db.tree.GetPage = func(ptr uint64) btree.BNode {
		return btree.BNode(db.pg.Read(ptr))
	}
	db.tree.NewPage = func(node btree.BNode) uint64 {
		if ptr := db.free.PopHead(); ptr != 0 {
			page := db.pg.Write(ptr)
			copy(page, node)
			return ptr
		}
		return db.pg.Alloc([]byte(node))
	}
	db.tree.DelPage = func(ptr uint64) {
		db.free.PushTail(ptr)
	}

	return db, nil
}

func (db *KV) Get(key []byte) ([]byte, bool) {
	return db.tree.Get(key)
}

func (db *KV) Set(key []byte, val []byte) error {
	meta := db.saveMeta()
	db.tree.Insert(key, val)
	return db.updateOrRevert(meta)
}

func (db *KV) Del(key []byte) (bool, error) {
	meta := db.saveMeta()
	deleted := db.tree.Delete(key)
	if !deleted {
		return false, nil
	}
	return true, db.updateOrRevert(meta)
}

func (db *KV) Seek(key []byte) *btree.BIter {
	return db.tree.SeekLE(key)
}

func (db *KV) saveMeta() []byte {
	page := make([]byte, storage.PageSize)
	storage.WriteMeta(page, db.tree.Root, db.pg.Flushed(),
		db.free.HeadPage, db.free.HeadSeq,
		db.free.TailPage, db.free.TailSeq)
	return page
}

func (db *KV) updateOrRevert(meta []byte) error {
	if db.failed {
		if _, err := db.file.WriteAt(meta, 0); err != nil {
			return fmt.Errorf("recover meta: %w", err)
		}
		if err := syscall.Fsync(db.fd); err != nil {
			return fmt.Errorf("recover fsync: %w", err)
		}
		db.failed = false
	}

	if _, err := db.pg.FlushPages(); err != nil {
		db.failed = true
		return fmt.Errorf("flush: %w", err)
	}

	if db.syncMode >= SyncNormal {
		if err := syscall.Fsync(db.fd); err != nil {
			db.failed = true
			return fmt.Errorf("fsync1: %w", err)
		}
	}
	db.pg.ClearUpdates()

	if _, err := db.file.WriteAt(meta, 0); err != nil {
		db.failed = true
		return fmt.Errorf("write meta: %w", err)
	}

	if db.syncMode == SyncFull {
		if err := syscall.Fsync(db.fd); err != nil {
			db.failed = true
			return fmt.Errorf("fsync2: %w", err)
		}
		return nil
	}

	if db.syncMode == SyncNormal {
		if err := syscall.Fsync(db.fd); err != nil {
			db.failed = true
			return fmt.Errorf("fsync: %w", err)
		}
	}

	return nil
}

func (db *KV) Close() error {
	return db.file.Close()
}

func (db *KV) TreeRoot() uint64 {
	return db.tree.Root
}

func (db *KV) PageRead(ptr uint64) btree.BNode {
	return btree.BNode(db.pg.Read(ptr))
}
