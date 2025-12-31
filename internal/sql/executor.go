package sql

import (
	"fmt"

	"github.com/ppborah/svacara-db/internal/kvstore"
	"github.com/ppborah/svacara-db/internal/relational"
)

type Executor struct {
	db *relational.DB
}

func NewExecutor(db *relational.DB) *Executor {
	return &Executor{db: db}
}

func (e *Executor) ExecuteRaw(input string) (interface{}, error) {
	ast, err := Parse(input)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return e.Execute(ast)
}

func (e *Executor) Execute(ast interface{}) (interface{}, error) {
	switch stmt := ast.(type) {
	case *QLCreateTable:
		return nil, e.execCreateTable(stmt)
	case *QLCreateIndex:
		return nil, e.execCreateIndex(stmt)
	case *QLInsert:
		return nil, e.execInsert(stmt)
	case *QLSelect:
		return e.execSelect(stmt)
	case *QLUpdate:
		return nil, e.execUpdate(stmt)
	case *QLDelete:
		return nil, e.execDelete(stmt)
	case string:
		return stmt, nil
	case error:
		return nil, stmt
	default:
		return nil, fmt.Errorf("unknown statement type: %T", ast)
	}
}

func (e *Executor) execCreateTable(stmt *QLCreateTable) error {
	tdef := &relational.TableDef{
		Name:  stmt.Table,
		Cols:  make([]relational.ColumnDef, len(stmt.Cols)),
		PKeys: len(stmt.PKeys),
	}
	for i, col := range stmt.Cols {
		tdef.Cols[i] = relational.ColumnDef{
			Name: col.Name,
			Type: sqlTypeToColType(col.Type),
		}
	}
	return e.db.CreateTable(tdef)
}

func (e *Executor) execCreateIndex(stmt *QLCreateIndex) error {
	tdef := e.db.TableDef(stmt.Table)
	if tdef == nil {
		return fmt.Errorf("table not found: %s", stmt.Table)
	}
	tdef.Indexes = append(tdef.Indexes, relational.IndexDef{
		Name:   stmt.Name,
		Keys:   stmt.Keys,
		Unique: stmt.Unique,
	})
	return nil
}

func (e *Executor) execInsert(stmt *QLInsert) error {
	tdef := e.db.TableDef(stmt.Table)
	if tdef == nil {
		return fmt.Errorf("table not found: %s", stmt.Table)
	}

	rec := relational.Record{
		Cols: stmt.Names,
		Vals: evalNodes(stmt.Values),
	}

	_, err := e.db.Insert(stmt.Table, rec)
	return err
}

func (e *Executor) execSelect(stmt *QLSelect) ([]relational.Record, error) {
	tdef := e.db.TableDef(stmt.Table)
	if tdef == nil {
		return nil, fmt.Errorf("table not found: %s", stmt.Table)
	}

	var start, stop []kvstore.Value
	cmp1, cmp2 := 0, 0

	if stmt.Key1.Type != 0 {
		start = evalNodes(stmt.Key1.Kids)
	}

	if stmt.Key2.Type != 0 {
		stop = evalNodes(stmt.Key2.Kids)
	}

	scanner, err := e.db.Scan(tdef, start, stop, cmp1, cmp2)
	if err != nil {
		return nil, err
	}

	var results []relational.Record
	var count int64
	for scanner.Valid() {
		if count < stmt.Offset {
			count++
			scanner.Next()
			continue
		}
		if count >= stmt.Offset+stmt.Limit {
			break
		}
		results = append(results, scanner.Record())
		count++
		scanner.Next()
	}
	return results, nil
}

func (e *Executor) execUpdate(stmt *QLUpdate) error {
	tdef := e.db.TableDef(stmt.Table)
	if tdef == nil {
		return fmt.Errorf("table not found: %s", stmt.Table)
	}

	records, err := e.execSelect(&QLSelect{QLScan: stmt.QLScan, Output: []QLNode{{Type: QLIdent, Str: []byte("*")}}, Names: []string{"*"}})
	if err != nil {
		return err
	}

	for _, rec := range records {
		updated := rec
		for i, name := range stmt.Names {
			for j, col := range rec.Cols {
				if col == name {
					updated.Vals[j] = evalNode(stmt.Values[i])
					break
				}
			}
		}
		if _, err := e.db.Update(stmt.Table, updated); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) execDelete(stmt *QLDelete) error {
	tdef := e.db.TableDef(stmt.Table)
	if tdef == nil {
		return fmt.Errorf("table not found: %s", stmt.Table)
	}

	records, err := e.execSelect(&QLSelect{QLScan: stmt.QLScan, Output: []QLNode{{Type: QLIdent, Str: []byte("*")}}, Names: []string{"*"}})
	if err != nil {
		return err
	}

	for _, rec := range records {
		if _, err := e.db.Delete(stmt.Table, rec); err != nil {
			return err
		}
	}
	return nil
}

func evalNodes(nodes []QLNode) []kvstore.Value {
	vals := make([]kvstore.Value, len(nodes))
	for i, n := range nodes {
		vals[i] = evalNode(n)
	}
	return vals
}

func evalNode(n QLNode) kvstore.Value {
	switch n.Type {
	case QLI64:
		return kvstore.Value{Type: kvstore.TypeInt64, I64: n.I64}
	case QLStr:
		return kvstore.Value{Type: kvstore.TypeBytes, Str: n.Str}
	case QLIdent:
		return kvstore.Value{Type: kvstore.TypeBytes, Str: n.Str}
	default:
		return kvstore.Value{}
	}
}

func sqlTypeToColType(t uint32) relational.ColType {
	switch t {
	case QLI64:
		return relational.TInt64
	case QLStr:
		return relational.TBytes
	case QLAnd:
		return relational.TBool
	case 999:
		return relational.TFloat64
	default:
		return relational.TBytes
	}
}
