APP_NAME := digital-twin
BIN_DIR := bin

.PHONY: build test test-race lint vet run clean

build:
	mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME) ./cmd/server

test:
	go test ./...

test-race:
	go test -race ./...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

run:
	go run ./cmd/server

clean:
	go clean
	rm -rf $(BIN_DIR)
