package relational

import (
	"encoding/binary"
	"fmt"

	"github.com/iamppborah/svacara-db/internal/btree"
	"github.com/iamppborah/svacara-db/internal/kvstore"
)

type DB struct {
	kv     kvstore.Storage
	tables map[string]*TableDef
}

func OpenDB(kv kvstore.Storage) (*DB, error) {
	db := &DB{
		kv:     kv,
		tables: make(map[string]*TableDef),
	}
	return db, nil
}

func (db *DB) TableDef(name string) *TableDef {
	return db.tables[name]
}

func (db *DB) CreateTable(tdef *TableDef) error {
	if _, ok := db.tables[tdef.Name]; ok {
		return fmt.Errorf("table %s already exists", tdef.Name)
	}
	if tdef.PKeys < 1 {
		return fmt.Errorf("table must have at least one primary key column")
	}
	tdef.Prefix = hashTableName(tdef.Name)
	db.tables[tdef.Name] = tdef
	return nil
}

func (db *DB) Get(table string, rec *Record) (bool, error) {
	tdef := db.tables[table]
	if tdef == nil {
		return false, fmt.Errorf("table not found: %s", table)
	}
	return db.dbGet(tdef, rec)
}

func (db *DB) dbGet(tdef *TableDef, rec *Record) (bool, error) {
	vals, err := checkRecord(tdef, *rec, tdef.PKeys)
	if err != nil {
		return false, err
	}
	key := make([]byte, 0, 256)
	key = kvstore.EncodeKey(key, tdef.Prefix, vals)

	val, ok := db.kv.Get(key)
	if !ok {
		return false, nil
	}

	rec.Vals = rec.Vals[:0]
	kvstore.DecodeValues(val, &rec.Vals)
	rec.Cols = make([]string, len(tdef.Cols))
	for i, c := range tdef.Cols {
		rec.Cols[i] = c.Name
	}
	return true, nil
}

func (db *DB) Insert(table string, rec Record) (bool, error) {
	tdef := db.tables[table]
	if tdef == nil {
		return false, fmt.Errorf("table not found: %s", table)
	}
	return db.dbUpdate(tdef, rec, 0)
}

func (db *DB) Update(table string, rec Record) (bool, error) {
	tdef := db.tables[table]
	if tdef == nil {
		return false, fmt.Errorf("table not found: %s", table)
	}
	return db.dbUpdate(tdef, rec, 1)
}

func (db *DB) Delete(table string, rec Record) (bool, error) {
	tdef := db.tables[table]
	if tdef == nil {
		return false, fmt.Errorf("table not found: %s", table)
	}
	return db.dbDelete(tdef, rec)
}

func (db *DB) dbUpdate(tdef *TableDef, rec Record, mode int) (bool, error) {
	vals, err := checkRecord(tdef, rec, len(tdef.Cols))
	if err != nil {
		return false, err
	}

	key := make([]byte, 0, 256)
	key = kvstore.EncodeKey(key, tdef.Prefix, vals[:tdef.PKeys])

	val := make([]byte, 0, 1024)
	val = kvstore.EncodeValues(val, vals[tdef.PKeys:])

	if mode == 0 {
		if _, ok := db.kv.Get(key); ok {
			return false, nil
		}
	}

	if err := db.kv.Set(key, val); err != nil {
		return false, err
	}

	for _, idx := range tdef.Indexes {
		if err := db.updateIndex(tdef, &idx, vals, mode); err != nil {
			return false, err
		}
	}

	return true, nil
}

func (db *DB) dbDelete(tdef *TableDef, rec Record) (bool, error) {
	vals, err := checkRecord(tdef, rec, tdef.PKeys)
	if err != nil {
		return false, err
	}

	key := make([]byte, 0, 256)
	key = kvstore.EncodeKey(key, tdef.Prefix, vals)

	oldVal, ok := db.kv.Get(key)
	if !ok {
		return false, nil
	}

	deleted, err := db.kv.Del(key)
	if err != nil || !deleted {
		return false, err
	}

	if len(tdef.Indexes) > 0 {
		oldVals := make([]kvstore.Value, len(tdef.Cols))
		copy(oldVals, vals)
		kvstore.DecodeValues(oldVal, &oldVals)
		for _, idx := range tdef.Indexes {
			if err := db.deleteIndex(tdef, &idx, oldVals); err != nil {
				return false, err
			}
		}
	}

	return true, nil
}

func (db *DB) updateIndex(tdef *TableDef, idx *IndexDef, vals []kvstore.Value, mode int) error {
	ikey := make([]byte, 0, 256)
	ikey = kvstore.EncodeKey(ikey, hashTableName(tdef.Name+"_"+idx.Name), indexKeyVals(tdef, idx, vals))
	ival := make([]byte, 0, 256)
	ival = kvstore.EncodeValues(ival, vals[:tdef.PKeys])
	return db.kv.Set(ikey, ival)
}

func (db *DB) deleteIndex(tdef *TableDef, idx *IndexDef, vals []kvstore.Value) error {
	ikey := make([]byte, 0, 256)
	ikey = kvstore.EncodeKey(ikey, hashTableName(tdef.Name+"_"+idx.Name), indexKeyVals(tdef, idx, vals))
	_, err := db.kv.Del(ikey)
	return err
}

func indexKeyVals(tdef *TableDef, idx *IndexDef, row []kvstore.Value) []kvstore.Value {
	vals := make([]kvstore.Value, 0, len(idx.Keys))
	for _, k := range idx.Keys {
		for j, c := range tdef.Cols {
			if c.Name == k {
				vals = append(vals, row[j])
				break
			}
		}
	}
	return vals
}

func hashTableName(name string) uint32 {
	h := uint32(5381)
	for _, c := range []byte(name) {
		h = ((h << 5) + h) + uint32(c)
	}
	return h
}

func (db *DB) KV() kvstore.Storage {
	return db.kv
}

func (db *DB) Scan(tdef *TableDef, start, stop []kvstore.Value, cmp1, cmp2 int) (*Scanner, error) {
	key1 := make([]byte, 0, 256)
	if len(start) > 0 {
		key1 = kvstore.EncodeKeyPartial(key1, tdef.Prefix, start, cmp1)
	} else {
		key1 = kvstore.EncodeKey(key1, tdef.Prefix, nil)
	}
	key2 := make([]byte, 0, 256)
	if len(stop) > 0 {
		key2 = kvstore.EncodeKeyPartial(key2, tdef.Prefix, stop, cmp2)
	}

	return &Scanner{
		db:   db,
		tdef: tdef,
		iter: db.kv.Seek(key1),
		stop: key2,
	}, nil
}

type Scanner struct {
	db   *DB
	tdef *TableDef
	iter *btree.BIter
	stop []byte
	err  error
}

func (s *Scanner) Valid() bool {
	if !s.iter.Valid() {
		return false
	}
	if len(s.stop) > 0 && btree.CompareKeys(s.iter.Key(), s.stop) > 0 {
		return false
	}
	if binary.BigEndian.Uint32(s.iter.Key()[:4]) != s.tdef.Prefix {
		return false
	}
	return true
}

func (s *Scanner) Next() {
	s.iter.Next()
}

func (s *Scanner) Record() Record {
	rec := Record{
		Cols: make([]string, len(s.tdef.Cols)),
		Vals: make([]kvstore.Value, 0, len(s.tdef.Cols)),
	}
	for i, c := range s.tdef.Cols {
		rec.Cols[i] = c.Name
	}
	kvstore.DecodeValues(s.iter.Val(), &rec.Vals)
	return rec
}

var _ = binary.BigEndian
