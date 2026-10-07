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
ifeq ($(shell uname), Darwin)
	brew tap viamrobotics/brews
	brew install nlopt-static
else ifeq ($(shell uname), Linux)
	sudo apt-get install -y --no-install-recommends libnlopt-dev
endif
	go mod tidy

module.tar.gz: meta.json $(MODULE_BINARY)
	strip $(MODULE_BINARY)
	tar czf $@ meta.json README.md $(MODULE_BINARY)

module: test module.tar.gz

.PHONY: lint test setup module
