.PHONY: all build run test clean help

BINARY_NAME=polyglot
VERSION=$(shell git describe --tags --always --dirty)
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse HEAD)

LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.GitCommit=$(GIT_COMMIT)"

all: build

build:
	@echo "🔨 Building..."
	go build $(LDFLAGS) -o $(BINARY_NAME) cmd/polyglot/main.go
	@echo "✅ Build complete: ./$(BINARY_NAME)"

run:
	@echo "🚀 Running..."
	go run cmd/polyglot/main.go

test:
	@echo "🧪 Testing..."
	go test -v -cover ./...

lint:
	@echo "🔍 Linting..."
	golangci-lint run

fmt:
	@echo "✨ Formatting..."
	go fmt ./...
	goimports -w .

clean:
	@echo "🧹 Cleaning..."
	rm -f $(BINARY_NAME)
	@echo "✅ Clean complete"

deps:
	@echo "📦 Installing dependencies..."
	go mod download
	go mod tidy

help:
	@echo "Polyglot Makefile Commands:"
	@echo "  make build  - Build the binary"
	@echo "  make run    - Run the server"
	@echo "  make test   - Run tests"
	@echo "  make lint   - Run linters"
	@echo "  make fmt    - Format code"
	@echo "  make deps   - Install dependencies"
	@echo "  make clean  - Clean build artifacts"
