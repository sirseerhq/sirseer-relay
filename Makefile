# SirSeer Relay Makefile

# Variables
BINARY_NAME=sirseer-relay
MAIN_PATH=./cmd/relay
GO=go
GOFLAGS=-v
BUILD_DIR=build
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X github.com/sirseerhq/sirseer-relay/pkg/version.Version=$(VERSION)"

# Build the binary
.PHONY: build
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(GOFLAGS) $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PATH)

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	$(GO) test $(GOFLAGS) -race -cover ./...

# Run tests with coverage report
.PHONY: test-coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test $(GOFLAGS) -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Add license headers to all Go files
.PHONY: license
license:
	@echo "Adding/updating license headers..."
	@if ! command -v addlicense >/dev/null 2>&1; then \
		echo "Installing addlicense..."; \
		go install github.com/google/addlicense@latest; \
	fi
	PATH=$$PATH:$$(go env GOPATH)/bin addlicense -f .license-header -c "SirSeer Inc." -y 2025 $$(find . -name '*.go')

# Check license headers
.PHONY: license-check
license-check:
	@echo "Checking license headers..."
	@if ! command -v addlicense >/dev/null 2>&1; then \
		echo "Installing addlicense..."; \
		go install github.com/google/addlicense@latest; \
	fi
	@if PATH=$$PATH:$$(go env GOPATH)/bin addlicense -check -f .license-header -c "SirSeer Inc." -y 2025 $$(find . -name '*.go') 2>&1 | grep -q .; then \
		echo "ERROR: Some files are missing required license headers."; \
		PATH=$$PATH:$$(go env GOPATH)/bin addlicense -check -f .license-header -c "SirSeer Inc." -y 2025 $$(find . -name '*.go'); \
		exit 1; \
	else \
		echo "All files have proper license headers."; \
	fi

# Run linter
.PHONY: lint
lint: license-check
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not installed. Running go vet instead..."; \
		$(GO) vet ./...; \
	fi

# Format code
.PHONY: fmt
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

# Clean build artifacts
.PHONY: clean
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	rm -f *.test
	rm -f *.ndjson
	rm -f *.state

# Install dependencies
.PHONY: deps
deps:
	@echo "Installing dependencies..."
	$(GO) mod download
	$(GO) mod tidy

# Run benchmarks
.PHONY: bench
bench:
	@echo "Running benchmarks..."
	$(GO) test -bench=. -benchmem ./...

# Build for multiple platforms
.PHONY: build-all
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(MAIN_PATH)
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(MAIN_PATH)
	GOOS=windows GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PATH)

# Default target
.DEFAULT_GOAL := build

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build          - Build the binary"
	@echo "  test           - Run tests"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  license        - Add/update license headers in all Go files"
	@echo "  license-check  - Check if all Go files have license headers"
	@echo "  lint           - Run linters (includes license check)"
	@echo "  fmt            - Format code"
	@echo "  clean          - Remove build artifacts"
	@echo "  deps           - Install dependencies"
	@echo "  bench          - Run benchmarks"
	@echo "  build-all      - Build for multiple platforms"
	@echo "  help           - Show this help message"