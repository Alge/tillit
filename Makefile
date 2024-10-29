# Makefile for the Go RBM SDK project

.PHONY: all build run fmt clean test vet lint mod install uninstall help

# Variables
BINARY_NAME=tillit
OUTPUT_DIR=bin
DIST_DIR=dist
GO_FILES=$(shell find . -type f -name '*.go')
INSTALL_DIR=/usr/local/bin

# Default target
all: build

# Build the binary
build: $(GO_FILES)
	@echo "Building the binary..."
	mkdir -p $(OUTPUT_DIR)
	go build -o $(OUTPUT_DIR)/$(BINARY_NAME)

# Run the application
run:
	go run main.go


# =========================
# Build All Platforms Target
# =========================

buildall: $(DIST_DIR) windows linux darwin
	@echo "All binaries built successfully in the '$(DIST_DIR)' directory."

# =========================
# Platform-Specific Build Targets
# =========================

# Target to build Windows binary
.PHONY: windows
windows:
	@echo "Building Windows binary..."
	GOOS=windows GOARCH=$(GOARCH) CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o $(DIST_DIR)/$(BINARY_NAME)-windows-$(GOARCH).exe
	@echo "Windows binary created at $(DIST_DIR)/$(BINARY_NAME)-windows-$(GOARCH).exe"

# Target to build Linux binary
.PHONY: linux
linux:
	@echo "Building Linux (Ubuntu) binary..."
	GOOS=linux GOARCH=$(GOARCH) CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o $(DIST_DIR)/$(BINARY_NAME)-linux-$(GOARCH)
	@echo "Linux binary created at $(DIST_DIR)/$(BINARY_NAME)-linux-$(GOARCH)"

# Target to build macOS binary
.PHONY: darwin
darwin:
	@echo "Building macOS binary..."
	GOOS=darwin GOARCH=$(GOARCH) CGO_ENABLED=0 go build -ldflags "-X main.version=$(VERSION)" -o $(DIST_DIR)/$(BINARY_NAME)-darwin-$(GOARCH)
	@echo "macOS binary created at $(DIST_DIR)/$(BINARY_NAME)-darwin-$(GOARCH)"

# Format the code
fmt:
	@echo "Formatting the code..."
	go fmt ./...

# Run tests
test:
	@echo "Running tests..."
	go test ./...

# Run 'go vet' to check for issues
vet:
	@echo "Running go vet..."
	go vet ./...

# Run linter (requires golangci-lint)
lint:
	@echo "Running linter..."
	golangci-lint run

# Clean up generated files
clean:
	@echo "Cleaning up..."
	rm -f $(OUTPUT_DIR)/$(BINARY_NAME)

# Tidy up go.mod and go.sum
mod:
	@echo "Tidying up modules..."
	go mod tidy

# Install the CLI binary to /usr/local/bin
install: build
	@echo "Installing the binary to $(INSTALL_DIR)..."
	install -d $(INSTALL_DIR)
	install -m 755 $(OUTPUT_DIR)/$(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installation complete. You can now run '$(BINARY_NAME)' from anywhere."

# Uninstall the CLI binary from /usr/local/bin
uninstall:
	@echo "Uninstalling the binary from $(INSTALL_DIR)..."
	@if [ -f "$(INSTALL_DIR)/$(BINARY_NAME)" ]; then \
		rm -f "$(INSTALL_DIR)/$(BINARY_NAME)"; \
		echo "Uninstallation complete."; \
	else \
		echo "No installed binary found at $(INSTALL_DIR)/$(BINARY_NAME)."; \
	fi

# Display help
help:
	@echo "Makefile commands:"
	@echo "  make build       - Build the CLI binary"
	@echo "  make run         - Run the CLI application"
	@echo "  make fmt         - Format the code"
	@echo "  make vet         - Run 'go vet' to examine code"
	@echo "  make lint        - Run linter (requires golangci-lint)"
	@echo "  make test        - Run tests"
	@echo "  make clean       - Remove built binary"
	@echo "  make mod         - Tidy up go.mod"
	@echo "  make install     - Install the CLI binary to $(INSTALL_DIR)"
	@echo "  make uninstall   - Uninstall the CLI binary from $(INSTALL_DIR)"
	@echo "  make help        - Show this help message"

