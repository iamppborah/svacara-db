package kvstore

import (
	"bytes"
	"encoding/binary"
	"math"
)

func EncodeKey(out []byte, prefix uint32, vals []Value) []byte {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], prefix)
	out = append(out, buf[:]...)
	return EncodeValues(out, vals)
}

func EncodeKeyPartial(out []byte, prefix uint32, vals []Value, cmp int) []byte {
	out = EncodeKey(out, prefix, vals)
	if cmp == 1 || cmp == -3 {
		out = append(out, 0xff)
	}
	return out
}

const (
	CmpLT = -1
	CmpLE = -2
	CmpEQ = 0
	CmpGE = 1
	CmpGT = 2
	CmpNE = 3
)

func EncodeValues(out []byte, vals []Value) []byte {
	for _, v := range vals {
		out = append(out, byte(v.Type))
		switch v.Type {
		case TypeInt64:
			var buf [8]byte
			u := uint64(v.I64) ^ (uint64(1) << 63)
			binary.BigEndian.PutUint64(buf[:], u)
			out = append(out, buf[:]...)
		case TypeBytes:
			out = append(out, escapeBytes(v.Str)...)
			out = append(out, 0)
		case TypeBool:
			if v.Bool {
				out = append(out, 1)
			} else {
				out = append(out, 0)
			}
		case TypeFloat64:
			var buf [8]byte
			u := math.Float64bits(v.F64)
			if v.F64 >= 0 {
				u ^= uint64(1) << 63
			} else {
				u = ^u
			}
			binary.BigEndian.PutUint64(buf[:], u)
			out = append(out, buf[:]...)
		default:
			panic("unknown type")
		}
	}
	return out
}

func escapeBytes(in []byte) []byte {
	var out []byte
	for _, b := range in {
		switch b {
		case 0:
			out = append(out, 0x01, 0x01)
		case 0x01:
			out = append(out, 0x01, 0x02)
		default:
			out = append(out, b)
		}
	}
	return out
}

func unescapeBytes(in []byte) []byte {
	var out []byte
	for i := 0; i < len(in); i++ {
		if in[i] == 0x01 && i+1 < len(in) {
			switch in[i+1] {
			case 0x01:
				out = append(out, 0)
			case 0x02:
				out = append(out, 0x01)
			default:
				out = append(out, in[i])
				continue
			}
			i++
		} else {
			out = append(out, in[i])
		}
	}
	return out
}

func DecodeValues(in []byte, out *[]Value) {
	for len(in) > 0 {
		if len(in) < 1 {
			panic("decode: unexpected eof")
		}
		t := ValueType(in[0])
		in = in[1:]
		var v Value
		v.Type = t
		switch t {
		case TypeInt64:
			u := binary.BigEndian.Uint64(in[:8])
			v.I64 = int64(u ^ (uint64(1) << 63))
			in = in[8:]
		case TypeBytes:
			idx := bytes.IndexByte(in, 0)
			if idx < 0 {
				panic("decode: unterminated string")
			}
			v.Str = unescapeBytes(in[:idx])
			in = in[idx+1:]
		case TypeBool:
			v.Bool = in[0] != 0
			in = in[1:]
		case TypeFloat64:
			u := binary.BigEndian.Uint64(in[:8])
			if u>>63 == 0 {
				u = ^u
			} else {
				u ^= uint64(1) << 63
			}
			v.F64 = math.Float64frombits(u)
			in = in[8:]
		default:
			panic("decode: unknown type")
		}
		*out = append(*out, v)
	}
}
