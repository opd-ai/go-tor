.PHONY: all build test clean install fmt vet lint coverage help

# Build variables
BINARY_NAME=tor-client
BINARY_PATH=bin/$(BINARY_NAME)
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Go variables
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOFMT=$(GOCMD) fmt

# Targets
all: clean fmt vet test build ## Clean, format, vet, test, and build

build: ## Build the binary
	@echo "Building $(BINARY_NAME) version $(VERSION)..."
	@mkdir -p bin
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_PATH) ./cmd/tor-client
	@echo "Build complete: $(BINARY_PATH)"

test: ## Run tests
	@echo "Running tests..."
	$(GOTEST) -v -race ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -v -race -coverprofile=coverage.out -covermode=atomic ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

fmt: ## Format code
	@echo "Formatting code..."
	$(GOFMT) ./...

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOCMD) vet ./...

lint: ## Run golint
	@echo "Running golint..."
	@which golint > /dev/null || (echo "Installing golint..." && go install golang.org/x/lint/golint@latest)
	@golint ./...

staticcheck: ## Run staticcheck
	@echo "Running staticcheck..."
	@which staticcheck > /dev/null || (echo "Installing staticcheck..." && go install honnef.co/go/tools/cmd/staticcheck@latest)
	@staticcheck ./...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf bin/
	rm -f coverage.out coverage.html

install: ## Install binary to $GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	$(GOCMD) install $(LDFLAGS) ./cmd/tor-client

mod-download: ## Download dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download

mod-tidy: ## Tidy dependencies
	@echo "Tidying dependencies..."
	$(GOMOD) tidy

mod-verify: ## Verify dependencies
	@echo "Verifying dependencies..."
	$(GOMOD) verify

# Cross-compilation targets
build-linux-amd64: ## Build for Linux AMD64
	@echo "Building for Linux AMD64..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/tor-client

build-linux-arm: ## Build for Linux ARM
	@echo "Building for Linux ARM..."
	@mkdir -p bin
	GOOS=linux GOARCH=arm GOARM=7 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm ./cmd/tor-client

build-linux-arm64: ## Build for Linux ARM64
	@echo "Building for Linux ARM64..."
	@mkdir -p bin
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/tor-client

build-linux-mips: ## Build for Linux MIPS
	@echo "Building for Linux MIPS..."
	@mkdir -p bin
	GOOS=linux GOARCH=mips $(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-mips ./cmd/tor-client

build-all: build-linux-amd64 build-linux-arm build-linux-arm64 build-linux-mips ## Build for all target platforms

# Docker targets
docker-build: ## Build Docker image
	@echo "Building Docker image..."
	docker build -t go-tor:$(VERSION) .

docker-run: ## Run Docker container
	@echo "Running Docker container..."
	docker run --rm -p 9050:9050 -p 9051:9051 go-tor:$(VERSION)

help: ## Show this help message
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-20s %s\n", $$1, $$2}'
