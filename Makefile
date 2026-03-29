# Makefile for gostac-validator

.PHONY: build test tidy check

# Default target
all: check build

# Run everything safe
check:
	go mod tidy
	go fmt ./...
	go vet ./...
	go test ./...

build:
	go build -o stac-server ./cmd/server

test:
	go test -v ./...

clean:
	rm -f stac-server