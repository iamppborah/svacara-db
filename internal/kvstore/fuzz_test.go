package kvstore

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"

	"github.com/iamppborah/svacara-db/internal/btree"
)

func FuzzKVEncoding(f *testing.F) {
	f.Add(int64(0))
	f.Add(int64(-1))
	f.Add(int64(42))
	f.Add(int64(1 << 62))
	f.Add(int64(-(1 << 62)))

	f.Fuzz(func(t *testing.T, i int64) {
		v := Value{Type: TypeInt64, I64: i}
		out := EncodeValues(nil, []Value{v})
		var decoded []Value
		DecodeValues(out, &decoded)
		if len(decoded) != 1 {
			t.Fatalf("expected 1 value, got %d", len(decoded))
		}
		if decoded[0].Type != TypeInt64 {
			t.Fatalf("expected int64, got %d", decoded[0].Type)
		}
		if decoded[0].I64 != i {
			t.Fatalf("round-trip failed: %d -> %d", i, decoded[0].I64)
		}
	})
}

func FuzzKVEncodingBytes(f *testing.F) {
	f.Add([]byte("hello"))
	f.Add([]byte(""))
	f.Add([]byte{0, 0, 0})
	f.Add([]byte{0x01, 0x02})

	f.Fuzz(func(t *testing.T, data []byte) {
		v := Value{Type: TypeBytes, Str: data}
		out := EncodeValues(nil, []Value{v})
		var decoded []Value
		DecodeValues(out, &decoded)
		if len(decoded) != 1 {
			t.Fatalf("expected 1 value, got %d", len(decoded))
		}
		if !bytes.Equal(decoded[0].Str, data) {
			t.Fatalf("round-trip failed: %q -> %q", data, decoded[0].Str)
		}
	})
}

func FuzzKVOperations(f *testing.F) {
	rng := rand.New(rand.NewSource(42))
	keys := []string{}
	for i := 0; i < 50; i++ {
		keys = append(keys, fmt.Sprintf("key-%d", i))
	}

	f.Add([]byte("test"))

	f.Fuzz(func(t *testing.T, seed []byte) {
		pages := map[uint64]btree.BNode{}
		tree := btree.BTree{
			GetPage: func(ptr uint64) btree.BNode { return pages[ptr] },
			NewPage: func(node btree.BNode) uint64 {
				ptr := uint64(len(pages) + 1)
				pages[ptr] = node
				return ptr
			},
			DelPage: func(ptr uint64) { delete(pages, ptr) },
		}
		ref := map[string]string{}

		ops := len(seed) % 100 + 1
		for i := 0; i < ops; i++ {
			key := keys[rng.Intn(len(keys))]
			val := fmt.Sprintf("val-%d-%d", i, rng.Intn(1000))

			switch rng.Intn(3) {
			case 0:
				tree.Insert([]byte(key), []byte(val))
				ref[key] = val
			case 1:
				if tree.Delete([]byte(key)) {
					delete(ref, key)
				}
			case 2:
				v, ok := tree.Get([]byte(key))
				expected, exists := ref[key]
				if ok != exists || (ok && string(v) != expected) {
					t.Fatalf("key %q: got (%q,%v) want (%q,%v)", key, v, ok, expected, exists)
				}
			}
		}
	})
}
