# CommitForge Makefile
# Common developer tasks: make build | run | test | lint | fmt | clean | install

BINARY_NAME := commitforge
MAIN_PKG    := .

# Version from git tags (falls back to "dev" when building without tags).
VERSION  := $(shell git describe --tags --always --dirty --match 'v*' 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)

GO ?= go

.PHONY: all build run test lint fmt vet clean install

## all: lint, test, then build (default target)
all: lint test build

## build: compile the commitforge binary into the repo root
build:
	$(GO) build -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) $(MAIN_PKG)

## run: build and launch the TUI panel
run: build
	./$(BINARY_NAME)

## test: run all tests with coverage report
test:
	$(GO) test ./... -cover

## lint: run golangci-lint (install: https://golangci-lint.run/usage/install/)
lint:
	golangci-lint run

## fmt: format sources with gofmt and goimports
fmt:
	gofmt -s -w $(shell $(GO) list -f '{{.Dir}}' ./... 2>/dev/null)
	go run golang.org/x/tools/cmd/goimports@latest -local commitforge -w .

## vet: run go vet
vet:
	$(GO) vet ./...

## clean: remove build artifacts
clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME).exe coverage coverage.html
	$(GO) clean -testcache

## install: install the binary into $(go env GOPATH)/bin
install:
	$(GO) install -ldflags "$(LDFLAGS)" $(MAIN_PKG)
