package kvstore

type ValueType uint8

const (
	TypeInt64   ValueType = 1
	TypeBytes   ValueType = 2
	TypeBool    ValueType = 3
	TypeFloat64 ValueType = 4
)

type Value struct {
	Type ValueType
	I64  int64
	Str  []byte
	Bool bool
	F64  float64
}
