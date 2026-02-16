# Makefile for building the Go binaries

CLI_BINARY_NAME=processing_pipeline
SQS_CONSUMER_BINARY_NAME=sqs_consumer
BIN_DIR=./bin

.PHONY: build
build: build-cli build-sqs-consumer
	@echo "All binaries built successfully"

.PHONY: build-cli
build-cli:
	@echo "Building the CLI application..."
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_DIR)/$(CLI_BINARY_NAME) ./cmd/cli

.PHONY: build-sqs-consumer
build-sqs-consumer:
	@echo "Building the SQS consumer..."
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_DIR)/$(SQS_CONSUMER_BINARY_NAME) ./cmd/sqs-consumer

.PHONY: clean
clean:
	@echo "Cleaning up..."
	@rm -rf $(BIN_DIR)

.PHONY: help
help:
	@echo "Valid targets are:"
	@echo "  build              : Build all binaries (CLI and SQS consumer)"
	@echo "  build-cli          : Build the CLI binary"
	@echo "  build-sqs-consumer : Build the SQS consumer binary"
	@echo "  clean              : Remove generated files"
	@echo "  help               : Display this help message"

