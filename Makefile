# Makefile — mcp-chain Phase 1 targets.
# Targets usable locally and from CI. CI workflow calls `make build` + gate scripts directly.

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GO_LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: all
all: lint build size-check startup-check stdout-check

.PHONY: build
build:
	go build -trimpath -ldflags="$(GO_LDFLAGS)" -o mcp-chain ./cmd/mcp-chain

.PHONY: lint
lint:
	golangci-lint run

.PHONY: test
test:
	go test -race -count=1 ./...

.PHONY: size-check
size-check: build
	./scripts/check-size.sh ./mcp-chain

.PHONY: startup-check
startup-check: build
	./scripts/check-startup.sh ./mcp-chain

.PHONY: stdout-check
stdout-check: build
	./scripts/check-stdout-silence.sh ./mcp-chain

.PHONY: clean
clean:
	rm -f mcp-chain
	rm -rf dist/

.PHONY: tidy
tidy:
	go mod tidy
