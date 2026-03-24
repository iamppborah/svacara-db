package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ppborah/svacara-db/internal/kvstore"
	"github.com/ppborah/svacara-db/internal/pool"
	"github.com/ppborah/svacara-db/internal/protocol"
	"github.com/ppborah/svacara-db/internal/relational"
	"github.com/ppborah/svacara-db/internal/shard"
	"github.com/ppborah/svacara-db/internal/sql"
)

type Config struct {
	ListenAddr string
	DBPath     string
	SyncMode   kvstore.SyncMode
	AuthToken  string
	MaxConns   int
	TLSCert    string
	TLSKey     string
}

type Server struct {
	config   Config
	store    kvstore.Storage
	db       *relational.DB
	p        *pool.Pool
	listener net.Listener
	mu       sync.Mutex
	shutdown bool
	logger   *slog.Logger
}

func NewServer(cfg Config) (*Server, error) {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	store, err := shard.Open(cfg.DBPath, cfg.SyncMode)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	rdb, err := relational.OpenDB(store)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("open relational: %w", err)
	}

	p := pool.NewPool(store, pool.Config{
		MaxConns:    cfg.MaxConns,
		MinConns:    2,
		MaxIdleTime: 10 * time.Minute,
		Mode:        pool.SessionMode,
	})

	return &Server{
		config: cfg,
		store:  store,
		db:     rdb,
		p:      p,
		logger: logger,
	}, nil
}

func (s *Server) Start() error {
	var ln net.Listener
	var err error

	if s.config.TLSCert != "" && s.config.TLSKey != "" {
		cert, err := tls.LoadX509KeyPair(s.config.TLSCert, s.config.TLSKey)
		if err != nil {
			return fmt.Errorf("tls cert: %w", err)
		}
		tlsCfg := &tls.Config{Certificates: []tls.Certificate{cert}}
		ln, err = tls.Listen("tcp", s.config.ListenAddr, tlsCfg)
		s.logger.Info("server started with TLS", "addr", s.config.ListenAddr)
	} else {
		ln, err = net.Listen("tcp", s.config.ListenAddr)
		s.logger.Info("server started", "addr", s.config.ListenAddr)
	}
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.listener = ln

	return s.serve()
}

func (s *Server) serve() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		s.logger.Info("shutting down...")
		s.Shutdown()
		cancel()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			default:
				s.logger.Error("accept", "err", err)
				continue
			}
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()
	
	if s.config.AuthToken != "" {
		buf := make([]byte, 512)
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		token := ""
		for i := 0; i < n; i++ {
			if buf[i] == '\n' || buf[i] == '\r' {
				token = string(buf[:i])
				break
			}
		}
		if token != s.config.AuthToken {
			s.sendRaw(conn, protocol.EncodeMessage(protocol.MsgError, []byte("authentication failed")))
			return
		}
		s.sendRaw(conn, protocol.EncodeMessage(protocol.MsgReady, []byte("AUTHENTICATED")))
	}

	s.logger.Info("new connection", "remote", conn.RemoteAddr())

	pc, err := s.p.Acquire()
	if err != nil {
		s.sendError(conn, fmt.Errorf("pool: %w", err))
		return
	}
	defer s.p.Release(pc)

	buf := make([]byte, 4096)
	for {
		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				s.logger.Error("read", "err", err)
			}
			return
		}

		msgType, payload, ok := protocol.DecodeMessage(buf[:n])
		if !ok {
			s.sendError(conn, fmt.Errorf("invalid message"))
			continue
		}

		s.handleMessage(conn, pc, msgType, payload)
	}
}

func (s *Server) handleMessage(conn net.Conn, pc *pool.Conn, msgType uint8, payload []byte) {
	switch msgType {
	case protocol.MsgQuery:
		s.handleQuery(conn, pc, string(payload))
	case protocol.MsgBegin:
		err := pc.Begin(0)
		if err != nil {
			s.sendError(conn, err)
			return
		}
		s.sendReady(conn)
	case protocol.MsgCommit:
		err := pc.Commit()
		if err != nil {
			s.sendError(conn, err)
			return
		}
		s.sendReady(conn)
	case protocol.MsgAbort:
		pc.Abort()
		s.sendReady(conn)
	default:
		s.sendError(conn, fmt.Errorf("unknown message type: %d", msgType))
	}
}

func (s *Server) handleQuery(conn net.Conn, pc *pool.Conn, query string) {
	exec := sql.NewExecutor(s.db)
	result, err := exec.ExecuteRaw(query)
	if err != nil {
		s.sendError(conn, err)
		return
	}

	switch v := result.(type) {
	case []relational.Record:
		cols := []string{}
		rows := []map[string]string{}
		for _, rec := range v {
			row := map[string]string{}
			for i, col := range rec.Cols {
				row[col] = valToString(rec.Vals[i])
			}
			rows = append(rows, row)
			cols = rec.Cols
		}
		resp := protocol.EncodeResult(cols, rows)
		conn.Write(resp)
	case string:
		s.sendReady(conn)
	default:
		s.sendReady(conn)
	}
}

func (s *Server) sendRaw(conn net.Conn, data []byte) {
	conn.Write(data)
}

func (s *Server) sendError(conn net.Conn, err error) {
	payload := []byte(err.Error())
	resp := protocol.EncodeMessage(protocol.MsgError, payload)
	conn.Write(resp)
}

func (s *Server) sendReady(conn net.Conn) {
	resp := protocol.EncodeMessage(protocol.MsgReady, []byte("READY"))
	conn.Write(resp)
}

func (s *Server) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shutdown {
		return
	}
	s.shutdown = true
	s.listener.Close()
	s.p.Close()
	s.store.Close()
}

func valToString(v kvstore.Value) string {
	switch v.Type {
	case kvstore.TypeInt64:
		return fmt.Sprintf("%d", v.I64)
	case kvstore.TypeBytes:
		return string(v.Str)
	case kvstore.TypeBool:
		if v.Bool {
			return "true"
		}
		return "false"
	case kvstore.TypeFloat64:
		return fmt.Sprintf("%g", v.F64)
	default:
		return "NULL"
	}
}
