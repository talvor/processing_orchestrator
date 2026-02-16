# Makefile for building the Go binaries

CLI_BINARY_NAME=processing_pipeline
SQS_CONSUMER_BINARY_NAME=sqs_consumer
BIN_DIR=./bin

# Localstack configuration
LOCALSTACK_CONTAINER_NAME=processing-orchestrator-localstack
LOCALSTACK_PORT=4566
SQS_QUEUE_NAME=test-workflow-queue
AWS_REGION=us-east-1
LOCALSTACK_ENDPOINT=http://localhost:$(LOCALSTACK_PORT)

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

.PHONY: localstack-start
localstack-start:
	@echo "Starting Localstack container..."
	@docker run -d \
		--name $(LOCALSTACK_CONTAINER_NAME) \
		-p $(LOCALSTACK_PORT):4566 \
		-e SERVICES=sqs \
		-e DEBUG=1 \
		localstack/localstack:latest
	@echo "Waiting for Localstack to be ready..."
	@sleep 5
	@echo "Localstack started on port $(LOCALSTACK_PORT)"

.PHONY: localstack-setup-queue
localstack-setup-queue:
	@echo "Creating SQS queue: $(SQS_QUEUE_NAME)"
	@AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
		aws --endpoint-url=$(LOCALSTACK_ENDPOINT) \
		--region $(AWS_REGION) \
		sqs create-queue \
		--queue-name $(SQS_QUEUE_NAME) \
		--no-cli-pager
	@echo "Getting queue URL..."
	@AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
		aws --endpoint-url=$(LOCALSTACK_ENDPOINT) \
		--region $(AWS_REGION) \
		sqs get-queue-url \
		--queue-name $(SQS_QUEUE_NAME) \
		--no-cli-pager
	@echo ""
	@echo "SQS Queue setup complete!"
	@echo "Queue Name: $(SQS_QUEUE_NAME)"
	@echo "Queue URL: $(LOCALSTACK_ENDPOINT)/000000000000/$(SQS_QUEUE_NAME)"
	@echo ""
	@echo "To use with sqs-consumer, run:"
	@echo "  export SQS_QUEUE_URL=$(LOCALSTACK_ENDPOINT)/000000000000/$(SQS_QUEUE_NAME)"
	@echo "  export AWS_REGION=$(AWS_REGION)"
	@echo "  export AWS_ACCESS_KEY_ID=test"
	@echo "  export AWS_SECRET_ACCESS_KEY=test"
	@echo "  export AWS_ENDPOINT_URL=$(LOCALSTACK_ENDPOINT)"
	@echo "  ./bin/sqs_consumer"

.PHONY: localstack-stop
localstack-stop:
	@echo "Stopping Localstack container..."
	@docker stop $(LOCALSTACK_CONTAINER_NAME) 2>/dev/null || true
	@docker rm $(LOCALSTACK_CONTAINER_NAME) 2>/dev/null || true
	@echo "Localstack stopped and removed"

.PHONY: localstack
localstack: localstack-start localstack-setup-queue
	@echo ""
	@echo "Localstack is ready for testing!"

.PHONY: help
help:
	@echo "Valid targets are:"
	@echo "  build              : Build all binaries (CLI and SQS consumer)"
	@echo "  build-cli          : Build the CLI binary"
	@echo "  build-sqs-consumer : Build the SQS consumer binary"
	@echo "  clean              : Remove generated files"
	@echo "  localstack         : Start Localstack and setup SQS queue (combined)"
	@echo "  localstack-start   : Start Localstack container"
	@echo "  localstack-setup-queue : Create SQS queue in Localstack"
	@echo "  localstack-stop    : Stop and remove Localstack container"
	@echo "  help               : Display this help message"

