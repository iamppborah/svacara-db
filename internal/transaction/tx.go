package transaction

import (
	"fmt"
	"sync"

	"github.com/ppborah/svacara-db/internal/btree"
	"github.com/ppborah/svacara-db/internal/kvstore"
)

type IsolationLevel int

const (
	ReadCommitted   IsolationLevel = 1
	SnapshotIsolation IsolationLevel = 2
	Serializable    IsolationLevel = 3
)

type KeyRange struct {
	Start []byte
	Stop  []byte
}

type CommittedTX struct {
	Version uint64
	Writes  []KeyRange
}

type Tx struct {
	db         kvstore.Storage
	version    uint64
	isolation  IsolationLevel
	pending    map[string][]byte
	reads      []KeyRange
	snapshot   Snapshot
	committed  bool
	aborted    bool
}

type Snapshot struct {
	Root uint64
	Get  func(uint64) btree.BNode
}

type Manager struct {
	kv       kvstore.Storage
	mu       sync.Mutex
	version  uint64
	ongoing  map[uint64]bool
	history  []CommittedTX
	minVer   uint64
}

func NewManager(kv kvstore.Storage) *Manager {
	return &Manager{
		kv:      kv,
		ongoing: make(map[uint64]bool),
	}
}

func (m *Manager) Begin(isolation IsolationLevel) *Tx {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.version++
	ver := m.version
	m.ongoing[ver] = true

	tx := &Tx{
		db:        m.kv,
		version:   ver,
		isolation: isolation,
		pending:   make(map[string][]byte),
	}

	tx.snapshot.Root = m.kv.TreeRoot()
	tx.snapshot.Get = m.kv.PageRead

	return tx
}

func (m *Manager) Commit(tx *Tx) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tx.committed || tx.aborted {
		return fmt.Errorf("tx already finished")
	}

	if tx.isolation >= Serializable {
		if m.detectConflicts(tx) {
			tx.aborted = true
			delete(m.ongoing, tx.version)
			return fmt.Errorf("serialization conflict")
		}
	}

	if len(tx.pending) == 0 {
		tx.committed = true
		delete(m.ongoing, tx.version)
		m.updateMinVer()
		return nil
	}

	var writes []KeyRange
	for k := range tx.pending {
		writes = append(writes, KeyRange{Start: []byte(k), Stop: []byte(k)})
	}

	m.history = append(m.history, CommittedTX{
		Version: tx.version,
		Writes:  writes,
	})

	for k, v := range tx.pending {
		if v == nil {
			if _, err := m.kv.Del([]byte(k)); err != nil {
				return fmt.Errorf("commit del: %w", err)
			}
		} else {
			if err := m.kv.Set([]byte(k), v); err != nil {
				return fmt.Errorf("commit set: %w", err)
			}
		}
	}

	tx.committed = true
	delete(m.ongoing, tx.version)
	m.updateMinVer()
	m.trimHistory()
	return nil
}

func (m *Manager) Abort(tx *Tx) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tx.committed || tx.aborted {
		return
	}
	tx.aborted = true
	delete(m.ongoing, tx.version)
	m.updateMinVer()
}

func (tx *Tx) Get(key []byte) ([]byte, bool) {
	tx.reads = append(tx.reads, KeyRange{Start: key, Stop: key})

	if val, ok := tx.pending[string(key)]; ok {
		if val == nil {
			return nil, false
		}
		return val, true
	}
	return tx.db.Get(key)
}

func (tx *Tx) Set(key []byte, val []byte) {
	tx.pending[string(key)] = val
}

func (tx *Tx) Del(key []byte) {
	tx.pending[string(key)] = nil
}

func (tx *Tx) Seek(key []byte) *btree.BIter {
	tx.reads = append(tx.reads, KeyRange{Start: key, Stop: nil})
	return tx.db.Seek(key)
}

func (m *Manager) detectConflicts(tx *Tx) bool {
	for i := len(m.history) - 1; i >= 0; i-- {
		if !versionBefore(tx.version, m.history[i].Version) {
			break
		}
		if rangesOverlap(tx.reads, m.history[i].Writes) {
			return true
		}
	}
	return false
}

func versionBefore(a, b uint64) bool {
	return a < b
}

func rangesOverlap(reads, writes []KeyRange) bool {
	for _, r := range reads {
		for _, w := range writes {
			if keyRangeOverlap(r, w) {
				return true
			}
		}
	}
	return false
}

func keyRangeOverlap(a, b KeyRange) bool {
	if len(b.Stop) > 0 && btree.CompareKeys(a.Start, b.Stop) > 0 {
		return false
	}
	if len(a.Stop) > 0 && btree.CompareKeys(b.Start, a.Stop) > 0 {
		return false
	}
	return true
}

func (m *Manager) updateMinVer() {
	min := m.version
	for v := range m.ongoing {
		if v < min {
			min = v
		}
	}
	m.minVer = min
}

func (m *Manager) trimHistory() {
	cut := 0
	for i, h := range m.history {
		if h.Version >= m.minVer {
			cut = i
			break
		}
	}
	m.history = m.history[cut:]
}
