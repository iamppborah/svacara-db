# SvacaraDB

A PostgreSQL-inspired, production-grade relational database built from scratch in Go.

**Features:**

- **B+Tree** storage engine with copy-on-write semantics and overflow pages for arbitrary-sized keys/values
- **MVCC** (Multi-Version Concurrency Control) with snapshot isolation and optimistic locking
- **SQL** parser and query engine — recursive descent, supports SELECT, INSERT, UPDATE, DELETE, CREATE TABLE, CREATE INDEX
- **Aggregation functions** — COUNT, SUM, AVG, MIN, MAX
- **TCP server** with custom binary wire protocol
- **Connection pooling** — pgbouncer-style session and transaction modes
- **Auto-sharding** — hash-based and range-based data distribution
- **Crash recovery** — meta page checksums and two-phase fsync protocol
- **TLS** support for encrypted client connections
- **Token authentication** for client access

## Quick Start

### Build

```bash
make build
```

### Run the server

```bash
./bin/svacara-server --listen :8080 --data ./mydb.db --sync full
```

### Connect with the CLI

```bash
./bin/svacara-cli localhost:8080
```

### Example SQL

```sql
CREATE TABLE users (id INT64, name BYTES, age INT64, PRIMARY KEY (id));
INSERT INTO users (id, name, age) VALUES (1, 'Alice', 30);
INSERT INTO users (id, name, age) VALUES (2, 'Bob', 25);
SELECT * FROM users;
SELECT COUNT(*) FROM users;
```

## Architecture

```
Client (CLI) ──TCP──> Server ──SQL Engine──> Relational Layer ──> KV Store ──> B+Tree ──> Disk
                        │                      │                    │
                   Connection Pool          Transactions         Free List
                        │                      │
                   Wire Protocol           MVCC / Snapshots
```

## Project Structure

```
cmd/                           Entry points
  svacara-server/              TCP server binary
  svacara-cli/                 Interactive CLI client
internal/
  btree/                       B+Tree data structure, free list, overflow pages
  storage/                     Page manager, mmap, meta page, crash recovery
  kvstore/                     Persistent KV store, order-preserving encoding
  relational/                  Tables, indexes, schema, range scanning
  transaction/                 MVCC, snapshot isolation, conflict detection
  pool/                        PgBouncer-style connection pooling
  sql/                         SQL parser, AST, planner, executor
  server/                      TCP listener, auth, session management
  protocol/                    Wire protocol encode/decode
  shard/                       Hash and range-based sharding
test/
  integration/                 End-to-end pipeline tests
```

## Testing

```bash
# Unit tests with race detector
make test

# Benchmarks
make bench

# Fuzz tests
make fuzz
```

## Design Decisions

- **Copy-on-write B+Tree**: Simpler crash recovery than WAL or double-write. Trade-off: higher write amplification per page, acceptable for OLTP workloads.
- **Snapshot isolation**: Readers never block writers. Writers only block writers at commit time via optimistic concurrency.
- **Overflow pages**: No arbitrary KV size limits. Large values chain across multiple 4K pages via linked lists.
- **Custom wire protocol**: Binary, length-prefixed framing. Faster than HTTP, purpose-built for database queries.
- **Single-file database**: Like SQLite. The sharding layer manages multiple single-file instances.
