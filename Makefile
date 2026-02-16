# Makefile for building the Go CLI binary

BINARY_NAME=processing_pipeline
BIN_DIR=./bin

.PHONY: build
build:
	@echo "Building the CLI application..."
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/cli.go

.PHONY: clean
clean:
	@echo "Cleaning up..."
	@rm -rf $(BIN_DIR)

.PHONY: help
help:
	@echo "Valid targets are:"
	@echo "  build  : Build the CLI binary"
	@echo "  clean  : Remove generated files"
	@echo "  help   : Display this help message"

