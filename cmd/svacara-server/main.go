package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/iamppborah/svacara-db/internal/kvstore"
	"github.com/iamppborah/svacara-db/internal/server"
)

func main() {
	listen := flag.String("listen", ":8080", "Listen address")
	data := flag.String("data", "./data.db", "Database file path")
	sync := flag.String("sync", "full", "Sync mode: full, normal, off")
	authToken := flag.String("auth-token", "", "Authentication token")
	maxConns := flag.Int("max-conns", 50, "Maximum connections")
	flag.Parse()

	var syncMode kvstore.SyncMode
	switch *sync {
	case "full":
		syncMode = kvstore.SyncFull
	case "normal":
		syncMode = kvstore.SyncNormal
	case "off":
		syncMode = kvstore.SyncOff
	default:
		fmt.Fprintf(os.Stderr, "invalid sync mode: %s\n", *sync)
		os.Exit(1)
	}

	cfg := server.Config{
		ListenAddr: *listen,
		DBPath:     *data,
		SyncMode:   syncMode,
		AuthToken:  *authToken,
		MaxConns:   *maxConns,
	}

	srv, err := server.NewServer(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create server: %v\n", err)
		os.Exit(1)
	}

	if err := srv.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "server error: %v\n", err)
		os.Exit(1)
	}
}
