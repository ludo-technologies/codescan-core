# Makefile for the jscan CLI. Its Go code lives in the polyscan module
# (polyscan/cmd/jscan and polyscan/internal/js); only the npm packaging and
# documentation remain in this directory.

BINARY_NAME := jscan
MODULE_DIR := ../polyscan
GO_MODULE := github.com/ludo-technologies/polyscan/polyscan
VERSION := $(shell git describe --tags --match 'jscan/v*' --always --dirty 2>/dev/null | sed 's|^jscan/v||' || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE := $(shell date +%Y-%m-%d)
LDFLAGS := -ldflags "-s -w \
                     -X '$(GO_MODULE)/internal/js/version.Version=$(VERSION)' \
                     -X '$(GO_MODULE)/internal/js/version.Commit=$(COMMIT)' \
                     -X '$(GO_MODULE)/internal/js/version.Date=$(DATE)' \
                     -X '$(GO_MODULE)/internal/js/version.BuiltBy=make'"

.PHONY: help all build test lint fmt clean install run version

## help: Show this help message
help:
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## all: Build and test
all: test build

## build: Build the binary
build:
	go build -C $(MODULE_DIR) $(LDFLAGS) -o $(CURDIR)/$(BINARY_NAME) ./cmd/jscan

## test: Run the polyscan module's tests
test:
	go test -C $(MODULE_DIR) ./...

## lint: Run go vet and check formatting
lint:
	go vet -C $(MODULE_DIR) ./...
	test -z "$$(gofmt -l $(MODULE_DIR))"

## fmt: Format code
fmt:
	cd $(MODULE_DIR) && go fmt ./...

## clean: Remove build artifacts
clean:
	rm -f $(BINARY_NAME)

## install: Install the binary
install:
	go install -C $(MODULE_DIR) $(LDFLAGS) ./cmd/jscan

## run: Run the binary with sample files
run: build
	./$(BINARY_NAME) analyze $(MODULE_DIR)/testdata/javascript/simple/

## version: Show version information
version:
	@echo "$(BINARY_NAME) version $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Date: $(DATE)"
