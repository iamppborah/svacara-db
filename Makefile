.PHONY: build test bench fuzz clean all

BIN_DIR := bin
SERVER := $(BIN_DIR)/svacara-server
CLI := $(BIN_DIR)/svacara-cli

GO := go
GOFLAGS := -ldflags="-s -w"

all: build

build: $(SERVER) $(CLI)

$(BIN_DIR):
	@mkdir -p $(BIN_DIR)

$(SERVER): $(BIN_DIR) cmd/svacara-server/main.go
	$(GO) build $(GOFLAGS) -o $@ ./cmd/svacara-server/

$(CLI): $(BIN_DIR) cmd/svacara-cli/main.go
	$(GO) build $(GOFLAGS) -o $@ ./cmd/svacara-cli/

test:
	$(GO) test -race -count=1 ./...

test-v:
	$(GO) test -race -v -count=1 ./...

bench:
	$(GO) test -bench=. -benchmem -run=^$$ ./internal/...

fuzz:
	$(GO) test -fuzz='^FuzzKVEncoding$$' -fuzztime=30s ./internal/kvstore/...
	$(GO) test -fuzz='^FuzzKVEncodingBytes$$' -fuzztime=30s ./internal/kvstore/...
	$(GO) test -fuzz='^FuzzKVOperations$$' -fuzztime=30s ./internal/kvstore/...

lint:
	$(GO) vet ./...

clean:
	rm -rf $(BIN_DIR) *.db *.db.*
