package relational

import (
	"fmt"

	"github.com/ppborah/svacara-db/internal/kvstore"
)

type ColType uint8

const (
	TInt64   ColType = 1
	TBytes   ColType = 2
	TBool    ColType = 3
	TFloat64 ColType = 4
)

type ColumnDef struct {
	Name string
	Type ColType
}

type IndexDef struct {
	Name   string
	Keys   []string
	Unique bool
}

type TableDef struct {
	Name    string
	Cols    []ColumnDef
	PKeys   int
	Indexes []IndexDef
	Prefix  uint32
}

type Record struct {
	Cols []string
	Vals []kvstore.Value
}

func (r *Record) Get(col string) (kvstore.Value, int) {
	for i, c := range r.Cols {
		if c == col {
			return r.Vals[i], i
		}
	}
	return kvstore.Value{}, -1
}

func checkRecord(tdef *TableDef, rec Record, n int) ([]kvstore.Value, error) {
	vals := make([]kvstore.Value, len(tdef.Cols))
	for i := range vals {
		vals[i] = kvstore.Value{Type: kvstore.TypeByColType(uint8(tdef.Cols[i].Type))}
	}
	for i, col := range rec.Cols {
		found := false
		for j, c := range tdef.Cols {
			if c.Name == col {
				vals[j] = rec.Vals[i]
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("column not found: %s", col)
		}
	}
	if n == tdef.PKeys {
		return vals[:tdef.PKeys], nil
	}
	for i := 0; i < tdef.PKeys; i++ {
		if vals[i].Type == 0 {
			return nil, fmt.Errorf("missing primary key column: %s", tdef.Cols[i].Name)
		}
	}
	return vals, nil
}
