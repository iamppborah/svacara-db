package sql

const (
	QLUnknown  = 0
	QLIdent    = 1
	QLI64      = 2
	QLStr      = 3
	QLAdd      = 4
	QLSub      = 5
	QLMul      = 6
	QLDiv      = 7
	QLMod      = 8
	QLAnd      = 9
	QLOr       = 10
	QLNot      = 11
	QLLT       = 12
	QLLE       = 13
	QLGT       = 14
	QLGE       = 15
	QLEQ       = 16
	QLNE       = 17
	QLLike     = 18
	QLFunc     = 19
	QLIndex    = 20
	QLNeg      = 21
)

type QLNode struct {
	Type uint32
	I64  int64
	Str  []byte
	Cmp  int
	Kids []QLNode
}

type QLScan struct {
	Table  string
	Key1   QLNode
	Key2   QLNode
	Filter QLNode
	Offset int64
	Limit  int64
}

type QLSelect struct {
	QLScan
	Names  []string
	Output []QLNode
}

type QLInsert struct {
	Table  string
	Names  []string
	Values []QLNode
}

type QLUpdate struct {
	QLScan
	Names  []string
	Values []QLNode
}

type QLDelete struct {
	QLScan
}

type QLCreateTable struct {
	Table  string
	Cols   []ColSpec
	PKeys  []string
}

type ColSpec struct {
	Name string
	Type uint32
}

type QLCreateIndex struct {
	Table  string
	Name   string
	Keys   []string
	Unique bool
}
