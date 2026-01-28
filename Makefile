.PHONY: build install clean run test

# Binary name
BINARY := perms

# Build directory
BUILD_DIR := bin

# Go build flags
LDFLAGS := -s -w

build:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY) ./cmd/perms

install: build
	@mkdir -p ~/.claude/plugins/cache/b-open-io/claude-perms/latest/bin
	cp $(BUILD_DIR)/$(BINARY) ~/.claude/plugins/cache/b-open-io/claude-perms/latest/bin/

run: build
	./$(BUILD_DIR)/$(BINARY)

clean:
	rm -rf $(BUILD_DIR)

test:
	go test ./...

# Development: run with go run
dev:
	go run ./cmd/perms

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...
