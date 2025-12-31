package shard

import (
	"fmt"
	"hash/crc32"

	"github.com/ppborah/svacara-db/internal/kvstore"
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
	shards []*kvstore.KV
	config ShardConfig
}

func Open(paths []string, cfg ShardConfig) (*DB, error) {
	if len(paths) == 0 {
		return nil, fmt.Errorf("at least one shard required")
	}
	if cfg.Shards == 0 {
		cfg.Shards = len(paths)
	}

	db := &DB{
		config: cfg,
		shards: make([]*kvstore.KV, cfg.Shards),
	}

	for i, p := range paths[:cfg.Shards] {
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
	return db.shards[db.shardForKey(key)].Get(key)
}

func (db *DB) Set(key, val []byte) error {
	return db.shards[db.shardForKey(key)].Set(key, val)
}

func (db *DB) Del(key []byte) (bool, error) {
	return db.shards[db.shardForKey(key)].Del(key)
}

func (db *DB) Close() error {
	for _, s := range db.shards {
		if err := s.Close(); err != nil {
			return err
		}
	}
	return nil
}
