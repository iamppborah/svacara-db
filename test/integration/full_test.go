package integration

import (
	"fmt"
	"os"
	"testing"

	"github.com/ppborah/svacara-db/internal/kvstore"
	"github.com/ppborah/svacara-db/internal/relational"
	"github.com/ppborah/svacara-db/internal/sql"
)

func TestCreateInsertSelect(t *testing.T) {
	path := fmt.Sprintf("/tmp/svacara-pipe-%d.db", os.Getpid())
	defer os.Remove(path)

	kv, err := kvstore.Open(path, kvstore.SyncFull)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer kv.Close()

	db, err := relational.OpenDB(kv)
	if err != nil {
		t.Fatalf("relational: %v", err)
	}

	exec := sql.NewExecutor(db)

	_, err = exec.ExecuteRaw("CREATE TABLE users (id INT64, name BYTES, age INT64, PRIMARY KEY (id))")
	if err != nil {
		t.Fatalf("create table: %v", err)
	}

	_, err = exec.ExecuteRaw("INSERT INTO users (id, name, age) VALUES (1, 'Alice', 30)")
	if err != nil {
		t.Fatalf("insert 1: %v", err)
	}
	_, err = exec.ExecuteRaw("INSERT INTO users (id, name, age) VALUES (2, 'Bob', 25)")
	if err != nil {
		t.Fatalf("insert 2: %v", err)
	}

	result, err := exec.ExecuteRaw("SELECT * FROM users")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	rows, ok := result.([]relational.Record)
	if !ok {
		t.Fatalf("expected []relational.Record, got %T", result)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	tdef := db.TableDef("users")
	if tdef == nil {
		t.Fatal("table not found")
	}

	key := make([]byte, 0, 256)
	pkVals := []kvstore.Value{{Type: kvstore.TypeInt64, I64: 1}}
	key = kvstore.EncodeKey(key, tdef.Prefix, pkVals)
	val, ok := kv.Get(key)
	if !ok {
		t.Fatal("row not found in KV")
	}
	var decoded []kvstore.Value
	kvstore.DecodeValues(val, &decoded)
	t.Logf("row 1: name=%q age=%d", decoded[0].Str, decoded[1].I64)
}

func TestPersistence(t *testing.T) {
	path := fmt.Sprintf("/tmp/svacara-persist-%d.db", os.Getpid())
	defer os.Remove(path)

	kv1, err := kvstore.Open(path, kvstore.SyncFull)
	if err != nil {
		t.Fatalf("open1: %v", err)
	}

	if err := kv1.Set([]byte("hello"), []byte("world")); err != nil {
		t.Fatalf("set: %v", err)
	}
	kv1.Close()

	kv2, err := kvstore.Open(path, kvstore.SyncFull)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer kv2.Close()

	val, ok := kv2.Get([]byte("hello"))
	if !ok || string(val) != "world" {
		t.Fatalf("data lost after close/reopen")
	}
}
