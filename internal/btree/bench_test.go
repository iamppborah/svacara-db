package btree

import (
	"fmt"
	"math/rand"
	"testing"
)

func BenchmarkBTreeInsert(b *testing.B) {
	c := newMemTest()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%08d", i)
		val := fmt.Sprintf("val-%08d", i)
		c.add(key, val)
	}
}

func BenchmarkBTreeGet(b *testing.B) {
	c := newMemTest()
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key-%08d", i)
		val := fmt.Sprintf("val-%08d", i)
		c.add(key, val)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%08d", i%10000)
		c.tree.Get([]byte(key))
	}
}

func BenchmarkBTreeDelete(b *testing.B) {
	c := newMemTest()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%08d", i)
		val := fmt.Sprintf("val-%08d", i)
		c.add(key, val)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%08d", i)
		c.del(key)
	}
}

func BenchmarkBTreeSeek(b *testing.B) {
	c := newMemTest()
	rng := rand.New(rand.NewSource(42))
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key-%08d", rng.Intn(100000))
		c.add(key, fmt.Sprintf("val-%d", i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		iter := c.tree.SeekLE([]byte(fmt.Sprintf("key-%08d", rng.Intn(100000))))
		for j := 0; j < 5 && iter.Valid(); j++ {
			iter.Next()
		}
	}
}
