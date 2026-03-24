package pool

import (
	"fmt"
	"sync"
	"time"

	"github.com/ppborah/svacara-db/internal/kvstore"
	"github.com/ppborah/svacara-db/internal/transaction"
)

type PoolMode int

const (
	SessionMode     PoolMode = 0
	TransactionMode PoolMode = 1
)

type Conn struct {
	id        uint64
	db        kvstore.Storage
	tx        *transaction.Tx
	mgr       *transaction.Manager
	createdAt time.Time
	lastUsed  time.Time
	inUse     bool
}

type Config struct {
	MaxConns     int
	MinConns     int
	MaxIdleTime  time.Duration
	MaxLifetime  time.Duration
	Mode         PoolMode
}

type Pool struct {
	mu       sync.Mutex
	config   Config
	db       kvstore.Storage
	mgr      *transaction.Manager
	conns    []*Conn
	free     []*Conn
	nextID   uint64
	closed   bool
}

func NewPool(db kvstore.Storage, config Config) *Pool {
	if config.MaxConns == 0 {
		config.MaxConns = 50
	}
	if config.MinConns == 0 {
		config.MinConns = 2
	}
	if config.MaxIdleTime == 0 {
		config.MaxIdleTime = 10 * time.Minute
	}
	if config.MaxLifetime == 0 {
		config.MaxLifetime = 1 * time.Hour
	}

	p := &Pool{
		config: config,
		db:     db,
		mgr:    transaction.NewManager(db),
	}

	for i := 0; i < config.MinConns; i++ {
		p.createConn()
	}

	return p
}

func (p *Pool) createConn() *Conn {
	p.nextID++
	c := &Conn{
		id:        p.nextID,
		db:        p.db,
		mgr:       p.mgr,
		createdAt: time.Now(),
		lastUsed:  time.Now(),
	}
	p.conns = append(p.conns, c)
	p.free = append(p.free, c)
	return c
}

func (p *Pool) Acquire() (*Conn, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.closed {
		return nil, fmt.Errorf("pool is closed")
	}

	now := time.Now()

	for i := 0; i < len(p.free); i++ {
		c := p.free[i]
		if now.Sub(c.lastUsed) > p.config.MaxIdleTime && len(p.conns) > p.config.MinConns {
			p.removeConn(i)
			i--
			continue
		}
		if now.Sub(c.createdAt) > p.config.MaxLifetime {
			p.removeConn(i)
			i--
			continue
		}
		if !c.inUse {
			c.inUse = true
			c.lastUsed = now
			p.free = append(p.free[:i], p.free[i+1:]...)
			return c, nil
		}
	}

	if len(p.conns) < p.config.MaxConns {
		c := p.createConn()
		c.inUse = true
		p.free = p.free[:len(p.free)-1]
		return c, nil
	}

	return nil, fmt.Errorf("no available connections")
}

func (p *Pool) Release(c *Conn) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !c.inUse {
		return
	}

	if c.tx != nil && p.config.Mode == TransactionMode {
		p.mgr.Abort(c.tx)
		c.tx = nil
	}

	c.inUse = false
	c.lastUsed = time.Now()
	p.free = append(p.free, c)
}

func (p *Pool) removeConn(idx int) {
	p.conns = append(p.conns[:idx], p.conns[idx+1:]...)
}

func (p *Pool) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	p.conns = nil
	p.free = nil
}

func (c *Conn) Begin(isolation transaction.IsolationLevel) error {
	if c.tx != nil {
		return fmt.Errorf("transaction already in progress")
	}
	c.tx = c.mgr.Begin(isolation)
	return nil
}

func (c *Conn) Commit() error {
	if c.tx == nil {
		return fmt.Errorf("no transaction in progress")
	}
	return c.mgr.Commit(c.tx)
}

func (c *Conn) Abort() {
	if c.tx != nil {
		c.mgr.Abort(c.tx)
		c.tx = nil
	}
}

func (c *Conn) Get(key []byte) ([]byte, bool) {
	if c.tx != nil {
		return c.tx.Get(key)
	}
	return c.db.Get(key)
}

func (c *Conn) Set(key, val []byte) error {
	if c.tx != nil {
		c.tx.Set(key, val)
		return nil
	}
	return c.db.Set(key, val)
}
