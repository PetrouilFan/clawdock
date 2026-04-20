.PHONY: build run test clean install-dev

BINARY_NAME=openclaw-manager
INSTALL_DIR=/opt/openclaw-manager
CONFIG_DIR=/etc/openclaw-manager

# Build the binary
build:
	go build -o $(BINARY_NAME) ./cmd/server

# Build for Linux AMD64
build-linux:
	GOOS=linux GOARCH=amd64 go build -o $(BINARY_NAME)-linux-amd64 ./cmd/server

# Build for Linux ARM64
build-arm64:
	GOOS=linux GOARCH=arm64 go build -o $(BINARY_NAME)-linux-arm64 ./cmd/server

# Run locally (uses default config path)
run:
	go run ./cmd/server

# Run with custom config
run-dev:
	go run ./cmd/server --config ./config.dev.yaml

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f $(BINARY_NAME)*
	rm -f *.tar.gz

# Lint
lint:
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...

# Tidy dependencies
tidy:
	go mod tidy

# Install to system (requires root)
install: build
	mkdir -p $(INSTALL_DIR)
	mkdir -p $(CONFIG_DIR)
	cp $(BINARY_NAME) $(INSTALL_DIR)/
	cp -r scripts/* $(INSTALL_DIR)/

# Create release tarballs
release: build-linux build-arm64 clean
	mkdir -p release
	mkdir -p $(BINARY_NAME)-linux-amd64/web/static
	mkdir -p $(BINARY_NAME)-linux-arm64/web/static
	cp $(BINARY_NAME)-linux-amd64 $(BINARY_NAME)-linux-amd64/openclaw-manager
	cp $(BINARY_NAME)-linux-arm64 $(BINARY_NAME)-linux-arm64/openclaw-manager
	cp -r web/static/* $(BINARY_NAME)-linux-amd64/web/static/ 2>/dev/null || true
	cp -r web/static/* $(BINARY_NAME)-linux-arm64/web/static/ 2>/dev/null || true
	tar -czf release/$(BINARY_NAME)-linux-amd64.tar.gz $(BINARY_NAME)-linux-amd64
	tar -czf release/$(BINARY_NAME)-linux-arm64.tar.gz $(BINARY_NAME)-linux-arm64
	rm -rf $(BINARY_NAME)-linux-*
	cd release && sha256sum * > checksums.txt
	@echo "Release files created in release/"
