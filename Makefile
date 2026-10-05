MODULE_BINARY := bin/cocktail-bot

GO_SOURCES := $(shell find . -name '*.go')

$(MODULE_BINARY): Makefile go.mod $(GO_SOURCES)
	go build -o $(MODULE_BINARY) cmd/module/main.go

lint:
	gofmt -s -w .
	golangci-lint run

test:
	go test ./...

setup:
	go mod tidy

.PHONY: lint test setup
