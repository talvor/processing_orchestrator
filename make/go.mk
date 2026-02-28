# Go targets

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

.PHONY: go-start-sqs-consumer
go-start-sqs-consumer:
	@echo "Starting SQS consumer..."
	@export SQS_QUEUE_URL=$(LOCALSTACK_ENDPOINT)/000000000000/$(SQS_QUEUE_NAME) && \
	export AWS_REGION=$(AWS_REGION) && \
	export AWS_ACCESS_KEY_ID=test && \
	export AWS_SECRET_ACCESS_KEY=test && \
	export AWS_ENDPOINT_URL=$(LOCALSTACK_ENDPOINT) && \
	./bin/sqs_consumer

.PHONY: go-send-test-message
go-send-test-message:
	@echo "Current directory: $(CURRENT_DIR)"
	@echo "Sending test message to SQS queue..."
	@export AWS_ACCESS_KEY_ID=test && \
	export AWS_SECRET_ACCESS_KEY=test && \
	aws --endpoint-url=http://localhost:4566 --region us-east-1 sqs send-message \
	  --queue-url http://localhost:4566/000000000000/test-workflow-queue \
		--message-body '{"workflow_file":"/app/examples/video-workflow.yaml"}'
