# Go configuration
CLI_BINARY_NAME=processing_pipeline
SQS_CONSUMER_BINARY_NAME=sqs_consumer
BIN_DIR=./bin

.PHONY: go-build
go-build: go-build-cli go-build-sqs-consumer
	@echo "All binaries built successfully"

.PHONY: go-build-cli
go-build-cli:
	@echo "Building the CLI application..."
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_DIR)/$(CLI_BINARY_NAME) ./cmd/cli

.PHONY: go-build-sqs-consumer
go-build-sqs-consumer:
	@echo "Building the SQS consumer..."
	@mkdir -p $(BIN_DIR)
	@go build -o $(BIN_DIR)/$(SQS_CONSUMER_BINARY_NAME) ./cmd/sqs-consumer

.PHONY: go-clean
go-clean:
	@echo "Cleaning up..."
	@rm -rf $(BIN_DIR)

.PHONY: go-test
go-test:
	@echo "Running tests..."
	@go test ./...

