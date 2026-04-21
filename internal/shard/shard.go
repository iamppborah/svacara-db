package shard

import (
	"fmt"
	"hash/crc32"
	"os"

	"github.com/iamppborah/svacara-db/internal/btree"
	"github.com/iamppborah/svacara-db/internal/kvstore"
)

type ShardStrategy int

const (
	HashShard  ShardStrategy = 1
	RangeShard ShardStrategy = 2
)

type ShardConfig struct {
	Strategy ShardStrategy
	Shards   int
	KeyCol   string
	Ranges   []RangeBound
}

type RangeBound struct {
	Start string
	End   string
	Shard int
}

type DB struct {
	shards  []*kvstore.KV
	config  ShardConfig
	single  bool
	singleKV *kvstore.KV
}

func Open(path string, mode kvstore.SyncMode) (*DB, error) {
	info, err := os.Stat(path)
	if err == nil && info.IsDir() {
		return OpenSharded(path, ShardConfig{Strategy: HashShard, Shards: 4})
	}
	kv, err := kvstore.Open(path, mode)
	if err != nil {
		return nil, err
	}
	return &DB{single: true, singleKV: kv}, nil
}

func OpenSharded(dir string, cfg ShardConfig) (*DB, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read shard dir: %w", err)
	}

	var paths []string
	for _, e := range entries {
		if !e.IsDir() {
			paths = append(paths, fmt.Sprintf("%s/%s", dir, e.Name()))
		}
	}
	if len(paths) == 0 {
		paths = []string{fmt.Sprintf("%s/shard-0.db", dir)}
	}

	db := &DB{
		config: cfg,
		shards: make([]*kvstore.KV, len(paths)),
	}

	cfg.Shards = len(paths)
	for i, p := range paths {
		kv, err := kvstore.Open(p, kvstore.SyncFull)
		if err != nil {
			for j := 0; j < i; j++ {
				db.shards[j].Close()
			}
			return nil, fmt.Errorf("open shard %d: %w", i, err)
		}
		db.shards[i] = kv
	}
	return db, nil
}

func (db *DB) shardForKey(key []byte) int {
	if db.single {
		return 0
	}
	switch db.config.Strategy {
	case HashShard:
		h := crc32.ChecksumIEEE(key)
		return int(h) % len(db.shards)
	case RangeShard:
		for _, r := range db.config.Ranges {
			if string(key) >= r.Start && string(key) <= r.End {
				return r.Shard
			}
		}
		return len(db.shards) - 1
	default:
		return 0
	}
}

func (db *DB) Get(key []byte) ([]byte, bool) {
	if db.single {
		return db.singleKV.Get(key)
	}
	return db.shards[db.shardForKey(key)].Get(key)
}

func (db *DB) Set(key, val []byte) error {
	if db.single {
		return db.singleKV.Set(key, val)
	}
	return db.shards[db.shardForKey(key)].Set(key, val)
}

func (db *DB) Del(key []byte) (bool, error) {
	if db.single {
		return db.singleKV.Del(key)
	}
	return db.shards[db.shardForKey(key)].Del(key)
}

func (db *DB) Seek(key []byte) *btree.BIter {
	if db.single {
		return db.singleKV.Seek(key)
	}
	return db.shards[db.shardForKey(key)].Seek(key)
}

func (db *DB) TreeRoot() uint64 {
	if db.single {
		return db.singleKV.TreeRoot()
	}
	return 0
}

func (db *DB) PageRead(ptr uint64) btree.BNode {
	if db.single {
		return db.singleKV.PageRead(ptr)
	}
	panic("PageRead not supported in sharded mode")
}

func (db *DB) Close() error {
	if db.single {
		return db.singleKV.Close()
	}
	for _, s := range db.shards {
		if err := s.Close(); err != nil {
			return err
		}
	}
	return nil
}

var _ kvstore.Storage = (*DB)(nil)
