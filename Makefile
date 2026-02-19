# goaquatemp Makefile

# Binary names
CLI_BINARY := aquatemp
EXAMPLE_BINARY := temperature
MONITOR_BINARY := monitor

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)"

# Go commands
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOCLEAN := $(GOCMD) clean
GOMOD := $(GOCMD) mod

# Default target - print help
.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help message
	@echo "goaquatemp - Aqua Temp Heat Pump SDK and CLI"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## Build the CLI binary
	$(GOBUILD) $(LDFLAGS) -o $(CLI_BINARY) ./cmd/aquatemp

.PHONY: build-example
build-example: ## Build the example binary
	$(GOBUILD) -o $(EXAMPLE_BINARY) ./examples/temperature

.PHONY: build-monitor
build-monitor: ## Build the monitor example binary
	$(GOBUILD) -o $(MONITOR_BINARY) ./examples/monitor

.PHONY: build-all
build-all: build build-example build-monitor ## Build all binaries

.PHONY: test
test: ## Run all tests
	$(GOTEST) ./...

.PHONY: run-tests
run-tests: test ## Alias for test target

.PHONY: test-verbose
test-verbose: ## Run all tests with verbose output
	$(GOTEST) -v ./...

.PHONY: test-race
test-race: ## Run tests with race detector
	$(GOTEST) -race ./...

.PHONY: test-cover
test-cover: ## Run tests with coverage report
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: clean
clean: ## Remove build artifacts
	$(GOCLEAN)
	rm -f $(CLI_BINARY) $(EXAMPLE_BINARY) $(MONITOR_BINARY)
	rm -f coverage.out coverage.html

.PHONY: deps
deps: ## Download dependencies
	$(GOMOD) download

.PHONY: tidy
tidy: ## Tidy go.mod
	$(GOMOD) tidy

.PHONY: fmt
fmt: ## Format code
	$(GOCMD) fmt ./...

.PHONY: vet
vet: ## Run go vet
	$(GOCMD) vet ./...

.PHONY: lint
lint: fmt vet ## Run all linters

.PHONY: all
all: clean deps build test ## Clean, fetch deps, build, and test
