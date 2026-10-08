MODULE_BINARY := bin/cocktail-bot

GO_SOURCES := $(shell find . -name '*.go' -not -path './web-app/*')

$(MODULE_BINARY): Makefile go.mod $(GO_SOURCES)
	go build -o $(MODULE_BINARY) cmd/module/main.go

CLI_BINARY := bin/cocktail-cli

$(CLI_BINARY): Makefile go.mod $(GO_SOURCES)
	go build -o $(CLI_BINARY) ./cmd/cli

cli: $(CLI_BINARY)

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

web-app-install:
	cd web-app && npm ci

web-app-build: web-app-install
	cd web-app && npm run build

web-app-dev:
	cd web-app && npm run dev

web-app-test:
	cd web-app && npm test

web-app-lint:
	cd web-app && npm run lint

WEB_APP_BINARY := web-app/cocktail-bot-app

$(WEB_APP_BINARY): cmd/web-app/main.go
	go build -o $@ ./cmd/web-app/

web-app-module: web-app-build $(WEB_APP_BINARY)
	cd web-app && tar czf module.tar.gz out cocktail-bot-app meta.json

.PHONY: cli lint test setup module web-app-install web-app-build web-app-dev web-app-test web-app-lint web-app-module
