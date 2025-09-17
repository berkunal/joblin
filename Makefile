# Makefile for Joblin CLI
# Go version and binary info
GO_VERSION := 1.24
BINARY_NAME := joblin
PACKAGE := github.com/berkunal/joblin

# Build info
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT := $(shell git rev-parse HEAD)
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE) -s -w"

# Directories
SRC_DIR := ./src
CMD_DIR := ./cmd
TEST_DIR := ./tests
BUILD_DIR := ./build
DIST_DIR := ./dist

# Tools
GOLANGCI_LINT_VERSION := v1.55.2

.PHONY: help
help: ## Show this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

##@ Development

.PHONY: setup
setup: ## Install development dependencies
	@echo "Installing development dependencies..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin $(GOLANGCI_LINT_VERSION); \
	fi
	@go mod download
	@go mod tidy

.PHONY: fmt
fmt: ## Format Go source code
	@echo "Formatting Go code..."
	@gofmt -s -w .
	@goimports -w .

.PHONY: lint
lint: ## Run linting
	@echo "Running golangci-lint..."
	@golangci-lint run --config .golangci.yml

.PHONY: lint-fix
lint-fix: ## Run linting with auto-fix
	@echo "Running golangci-lint with fixes..."
	@golangci-lint run --config .golangci.yml --fix

##@ Build

.PHONY: build
build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(CMD_DIR)/$(BINARY_NAME)

.PHONY: build-all
build-all: ## Build for all platforms
	@echo "Building for all platforms..."
	@mkdir -p $(BUILD_DIR)
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(CMD_DIR)/$(BINARY_NAME)
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-arm64 $(CMD_DIR)/$(BINARY_NAME)
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(CMD_DIR)/$(BINARY_NAME)
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(CMD_DIR)/$(BINARY_NAME)
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(CMD_DIR)/$(BINARY_NAME)

.PHONY: install
install: ## Install the binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	@go install $(LDFLAGS) $(CMD_DIR)/$(BINARY_NAME)

##@ Testing

.PHONY: test
test: ## Run all tests
	@echo "Running tests..."
	@go test -race -cover ./...

.PHONY: test-verbose
test-verbose: ## Run tests with verbose output
	@echo "Running tests (verbose)..."
	@go test -race -cover -v ./...


.PHONY: test-unit
test-unit: ## Run unit tests only
	@echo "Running unit tests..."
	@go test -race -cover ./$(TEST_DIR)/unit/...

.PHONY: test-performance
test-performance: ## Run performance tests
	@echo "Running performance tests..."
	@go test -race -cover ./$(TEST_DIR)/performance/...

.PHONY: coverage
coverage: ## Generate test coverage report
	@echo "Generating coverage report..."
	@go test -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

##@ Quality

.PHONY: check
check: fmt lint test ## Run all quality checks (format, lint, test)

.PHONY: pre-commit
pre-commit: fmt lint-fix test ## Run pre-commit checks

.PHONY: vet
vet: ## Run go vet
	@echo "Running go vet..."
	@go vet ./...

.PHONY: mod-tidy
mod-tidy: ## Run go mod tidy
	@echo "Tidying go.mod..."
	@go mod tidy

.PHONY: mod-verify
mod-verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	@go mod verify

##@ Documentation

.PHONY: docs
docs: ## Generate documentation
	@echo "Generating documentation..."
	@godoc -http=:6060 &
	@echo "Documentation server started at http://localhost:6060"

##@ Release

.PHONY: release
release: ## Create a release using GoReleaser
	@echo "Creating release..."
	@goreleaser release --clean

.PHONY: release-snapshot
release-snapshot: ## Create a snapshot release
	@echo "Creating snapshot release..."
	@goreleaser release --snapshot --clean

##@ Cleanup

.PHONY: clean
clean: ## Clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -rf $(DIST_DIR)
	@rm -f coverage.out coverage.html

.PHONY: clean-deps
clean-deps: ## Clean dependency cache
	@echo "Cleaning dependency cache..."
	@go clean -modcache

##@ Development Workflow

.PHONY: dev-setup
dev-setup: setup fmt lint ## Complete development setup

.PHONY: quick-test
quick-test: ## Quick test run (no race detection)
	@echo "Running quick tests..."
	@go test ./...

.PHONY: watch
watch: ## Watch for changes and run tests (requires entr)
	@echo "Watching for changes..."
	@find . -name "*.go" | entr -c make quick-test

# Default target
.DEFAULT_GOAL := help