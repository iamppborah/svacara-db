package btree

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"unsafe"
)

type memTest struct {
	tree  BTree
	ref   map[string]string
	pages map[uint64]BNode
}

func newMemTest() *memTest {
	pages := map[uint64]BNode{}
	return &memTest{
		tree: BTree{
			GetPage: func(ptr uint64) BNode {
				node, ok := pages[ptr]
				if !ok {
					panic(fmt.Sprintf("bad ptr: %d", ptr))
				}
				return node
			},
			NewPage: func(node BNode) uint64 {
				if node.nbytes() > BTREE_PAGE_SIZE {
					panic("oversized node")
				}
				ptr := uint64(uintptr(unsafe.Pointer(&node[0])))
				pages[ptr] = node
				return ptr
			},
			DelPage: func(ptr uint64) {
				if _, ok := pages[ptr]; !ok {
					panic("double free")
				}
				delete(pages, ptr)
			},
		},
		ref:   map[string]string{},
		pages: pages,
	}
}

func (c *memTest) add(key string, val string) {
	c.tree.Insert([]byte(key), []byte(val))
	c.ref[key] = val
}

func (c *memTest) del(key string) bool {
	deleted := c.tree.Delete([]byte(key))
	if deleted {
		delete(c.ref, key)
	}
	return deleted
}

func (c *memTest) get(key string) (string, bool) {
	val, ok := c.tree.Get([]byte(key))
	return string(val), ok
}

func (c *memTest) verify(t *testing.T) {
	for key, expectedVal := range c.ref {
		val, ok := c.tree.Get([]byte(key))
		if !ok {
			t.Fatalf("key %q not found", key)
		}
		if string(val) != expectedVal {
			t.Fatalf("key %q: got %q, want %q", key, val, expectedVal)
		}
	}
	for ptr := range c.pages {
		node := c.pages[ptr]
		nkeys := node.nkeys()
		if nkeys == 0 {
			continue
		}
		for i := uint16(1); i < nkeys; i++ {
			if cmp(node.getKey(i-1), node.getKey(i)) >= 0 {
				t.Fatalf("keys not sorted in node %d", ptr)
			}
		}
	}
}

func TestBTreeBasic(t *testing.T) {
	c := newMemTest()

	c.add("k1", "v1")
	c.add("k2", "v2")
	c.add("k3", "v3")
	c.verify(t)

	val, ok := c.get("k2")
	if !ok || val != "v2" {
		t.Fatalf("get k2: %v, %v", val, ok)
	}

	_, ok = c.get("nonexistent")
	if ok {
		t.Fatal("found nonexistent key")
	}

	c.add("k2", "v2-updated")
	c.verify(t)

	if !c.del("k1") {
		t.Fatal("delete k1 failed")
	}
	c.verify(t)

	if c.del("k1") {
		t.Fatal("double delete k1 should return false")
	}
}

func TestBTreeRandom(t *testing.T) {
	c := newMemTest()
	rng := rand.New(rand.NewSource(42))
	keys := make(map[string]bool)

	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key-%08d", rng.Intn(200))
		val := fmt.Sprintf("val-%08d", rng.Intn(100000))

		if rng.Float32() < 0.3 && len(keys) > 0 {
			var delKey string
			for k := range keys {
				delKey = k
				break
			}
			c.del(delKey)
			delete(keys, delKey)
		} else {
			c.add(key, val)
			keys[key] = true
		}
	}
	c.verify(t)
}

func TestBTreeInsertMany(t *testing.T) {
	c := newMemTest()
	for i := 1; i <= 500; i++ {
		key := fmt.Sprintf("key-%04d", i)
		val := fmt.Sprintf("value-%04d", i)
		c.add(key, val)
	}
	c.verify(t)

	keys := make([]string, 0, len(c.ref))
	for k := range c.ref {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for i, key := range keys {
		val, ok := c.get(key)
		expectedVal := fmt.Sprintf("value-%04d", i+1)
		if !ok || val != expectedVal {
			t.Fatalf("after sort, key %q: got %q, want %q", key, val, expectedVal)
		}
	}
}

func TestBTreeDeleteAll(t *testing.T) {
	c := newMemTest()
	for i := 1; i <= 100; i++ {
		key := fmt.Sprintf("k-%d", i)
		c.add(key, fmt.Sprintf("v-%d", i))
	}
	for i := 1; i <= 100; i++ {
		key := fmt.Sprintf("k-%d", i)
		if !c.del(key) {
			t.Fatalf("failed to delete %s", key)
		}
	}
	if c.tree.Root != 0 {
		t.Fatal("root should be 0 after deleting all keys")
	}
}

func TestBTreeIterator(t *testing.T) {
	c := newMemTest()
	for i := 1; i <= 50; i++ {
		key := fmt.Sprintf("k-%03d", i)
		c.add(key, fmt.Sprintf("v-%d", i))
	}

	iter := c.tree.SeekLE([]byte("k-000"))
	if !iter.Valid() {
		t.Fatal("iterator should be valid")
	}

	count := 0
	for ; iter.Valid(); iter.Next() {
		count++
		if count > 55 {
			t.Fatal("iterator running away")
		}
	}
	if count != 50 {
		t.Fatalf("expected 50 keys, got %d", count)
	}

	iter = c.tree.SeekLE([]byte("k-025"))
	if !iter.Valid() {
		t.Fatal("iterator should find k-025")
	}

	count = 0
	for ; iter.Valid(); iter.Next() {
		count++
	}
	if count != 26 {
		t.Fatalf("expected 26 keys from k-025 onward, got %d", count)
	}
}
