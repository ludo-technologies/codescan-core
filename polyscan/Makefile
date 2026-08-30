# Makefile for polyscan

BINARY_NAME := polyscan
GO_MODULE := github.com/ludo-technologies/polyscan/polyscan
VERSION := $(shell git describe --tags --match 'polyscan/v*' --always --dirty 2>/dev/null | sed 's|^polyscan/v||' || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date +%Y-%m-%d)
# internal/js/version is stamped too: the JavaScript section of the report
# carries jscan's version fields until the report is unified.
LDFLAGS := -ldflags "-s -w \
                     -X '$(GO_MODULE)/internal/version.Version=$(VERSION)' \
                     -X '$(GO_MODULE)/internal/version.Commit=$(COMMIT)' \
                     -X '$(GO_MODULE)/internal/version.Date=$(DATE)' \
                     -X '$(GO_MODULE)/internal/version.BuiltBy=make' \
                     -X '$(GO_MODULE)/internal/js/version.Version=$(VERSION)' \
                     -X '$(GO_MODULE)/internal/js/version.Commit=$(COMMIT)' \
                     -X '$(GO_MODULE)/internal/js/version.Date=$(DATE)' \
                     -X '$(GO_MODULE)/internal/js/version.BuiltBy=make'"

.PHONY: help all build test lint fmt clean install run

## help: Show this help message
help:
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## all: Build and test
all: test build

## build: Build the binary
build:
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/polyscan

## test: Run tests
test:
	go test -race ./...

## lint: Run go vet and check formatting
lint:
	go vet ./...
	test -z "$$(gofmt -l .)"

## fmt: Format code
fmt:
	go fmt ./...

## clean: Remove build artifacts
clean:
	rm -f $(BINARY_NAME) coverage.out coverage.html
	rm -rf dist/

## install: Install the binary
install:
	go install $(LDFLAGS) ./cmd/polyscan

## run: Analyze the Go fixtures
run: build
	./$(BINARY_NAME) analyze testdata/go
