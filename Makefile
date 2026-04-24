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

.PHONY: release-snapshot
release-snapshot:
	goreleaser release --snapshot --clean --skip=publish
	@echo "--- dist/ contents:"
	@ls -la dist/
	@test $$(ls dist/*.tar.gz 2>/dev/null | wc -l) -eq 4 || (echo "ERROR: expected 4 .tar.gz in dist/" && exit 1)
	@test $$(ls dist/*.zip 2>/dev/null | wc -l) -eq 2 || (echo "ERROR: expected 2 .zip in dist/" && exit 1)
	@test -f dist/checksums.txt || (echo "ERROR: dist/checksums.txt missing" && exit 1)
	@echo "Snapshot release OK: 4 tar.gz + 2 zip + checksums.txt"

.PHONY: ci-local
ci-local: lint build size-check startup-check stdout-check test
	@echo "--- ci-local: all Linux gates green"
