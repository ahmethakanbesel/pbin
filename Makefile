APP       := pbin
MODULE    := github.com/ahmethakanbesel/pbin
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BUILD_DATE := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS := -s -w \
	-X main.version=$(VERSION) \
	-X main.commit=$(COMMIT) \
	-X main.date=$(BUILD_DATE)

.PHONY: all build run test lint vet clean dev migrate-new

all: build

## build: Compile CGO-free binary
build:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o $(APP) ./cmd/pbin

## run: Build and run the server
run: build
	./$(APP)

## dev: Run with go run (no build step)
dev:
	go run ./cmd/pbin

## test: Run all tests
test:
	go test ./...

## test-race: Run tests with race detector
test-race:
	go test -race ./...

## test-cover: Run tests with coverage report
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## vet: Run go vet
vet:
	go vet ./...

## lint: Run staticcheck (install: go install honnef.co/go/tools/cmd/staticcheck@latest)
lint: vet
	staticcheck ./...

## migrate-new: Create a new goose migration (usage: make migrate-new NAME=add_something)
migrate-new:
	@test -n "$(NAME)" || (echo "Usage: make migrate-new NAME=add_something" && exit 1)
	goose -dir internal/storage/migrations create $(NAME) sql

## clean: Remove build artifacts
clean:
	rm -f $(APP) coverage.out coverage.html

## help: Show this help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'
