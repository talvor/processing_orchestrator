# Makefile for building the Go binaries

CLI_BINARY_NAME=processing_pipeline
SQS_CONSUMER_BINARY_NAME=sqs_consumer
BIN_DIR=./bin
CURRENT_DIR=$(shell pwd)

# Localstack configuration
LOCALSTACK_CONTAINER_NAME=processing-orchestrator-localstack
LOCALSTACK_PORT=4566
SQS_QUEUE_NAME=test-workflow-queue
AWS_REGION=us-east-1
LOCALSTACK_ENDPOINT=http://localhost:$(LOCALSTACK_PORT)

# Terraform configuration
TERRAFORM_DIR=./terraform
MAX_RECEIVE_COUNT=5

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
	@if [ "$$(docker ps -q -f name=$(LOCALSTACK_CONTAINER_NAME))" ]; then \
		echo "Localstack is already running"; \
	else \
		docker run -d \
			--name $(LOCALSTACK_CONTAINER_NAME) \
			-p $(LOCALSTACK_PORT):4566 \
			-e SERVICES=sqs \
			-e DEBUG=1 \
			localstack/localstack:latest; \
	fi
	@echo "Waiting for Localstack to be ready..."
	@sleep 5
	@echo "Localstack started on port $(LOCALSTACK_PORT)"

.PHONY: localstack-setup-queue
localstack-setup-queue:
	@echo "Creating SQS dead letter queue: $(SQS_QUEUE_NAME)_dlq"
	@AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
		aws --endpoint-url=$(LOCALSTACK_ENDPOINT) \
		--region $(AWS_REGION) \
		sqs create-queue \
		--queue-name "$(SQS_QUEUE_NAME)_dlq"
	@echo "Creating SQS queue: $(SQS_QUEUE_NAME)"
	@AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test \
		aws --endpoint-url=$(LOCALSTACK_ENDPOINT) \
		--region $(AWS_REGION) \
		sqs create-queue \
		--queue-name $(SQS_QUEUE_NAME) \
		--attributes '{"RedrivePolicy": "{\"deadLetterTargetArn\":\"arn:aws:sqs:us-east-1:000000000000:$(SQS_QUEUE_NAME)_dlq\",\"maxReceiveCount\":\"5\"}"}'
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
localstack: localstack-start terraform-apply
	@echo ""
	@echo "Localstack is ready for testing!"

.PHONY: terraform-init
terraform-init:
	@echo "Initialising Terraform..."
	@tofu -chdir=$(TERRAFORM_DIR) init

.PHONY: terraform-plan
terraform-plan:
	@echo "Planning Terraform changes..."
	@tofu -chdir=$(TERRAFORM_DIR) plan \
		-var="aws_region=$(AWS_REGION)" \
		-var="localstack_endpoint=$(LOCALSTACK_ENDPOINT)" \
		-var="sqs_queue_name=$(SQS_QUEUE_NAME)" \
		-var="max_receive_count=$(MAX_RECEIVE_COUNT)"

.PHONY: terraform-apply
terraform-apply:
	@echo "Applying Terraform changes..."
	@tofu -chdir=$(TERRAFORM_DIR) apply -auto-approve \
		-var="aws_region=$(AWS_REGION)" \
		-var="localstack_endpoint=$(LOCALSTACK_ENDPOINT)" \
		-var="sqs_queue_name=$(SQS_QUEUE_NAME)" \
		-var="max_receive_count=$(MAX_RECEIVE_COUNT)"

.PHONY: terraform-destroy
terraform-destroy:
	@echo "Destroying Terraform-managed infrastructure..."
	@tofu -chdir=$(TERRAFORM_DIR) destroy -auto-approve \
		-var="aws_region=$(AWS_REGION)" \
		-var="localstack_endpoint=$(LOCALSTACK_ENDPOINT)" \
		-var="sqs_queue_name=$(SQS_QUEUE_NAME)" \
		-var="max_receive_count=$(MAX_RECEIVE_COUNT)"

.PHONY: test
test:
	@echo "Running tests..."
	@go test ./...

.PHONY: start-sqs-consumer
start-sqs-consumer:
	@echo "Starting SQS consumer..."
	@export SQS_QUEUE_URL=$(LOCALSTACK_ENDPOINT)/000000000000/$(SQS_QUEUE_NAME) && \
	export AWS_REGION=$(AWS_REGION) && \
	export AWS_ACCESS_KEY_ID=test && \
	export AWS_SECRET_ACCESS_KEY=test && \
	export AWS_ENDPOINT_URL=$(LOCALSTACK_ENDPOINT) && \
	./bin/sqs_consumer

.PHONY: send-test-message
send-test-message:
	@echo "Current directory: $(CURRENT_DIR)"
	@echo "Sending test message to SQS queue..."
	@export AWS_ACCESS_KEY_ID=test && \
	export AWS_SECRET_ACCESS_KEY=test && \
	aws --endpoint-url=http://localhost:4566 --region us-east-1 sqs send-message \
	  --queue-url http://localhost:4566/000000000000/test-workflow-queue \
		--message-body '{"workflow_file":"/app/examples/video-workflow.yaml"}'

.PHONY: docker-build
docker-build:
	@echo "Building Docker image for SQS consumer..."
	@docker build -t sqs-consumer:latest .

.PHONY: help
help:
	@echo "Valid targets are:"
	@echo "  build              : Build all binaries (CLI and SQS consumer)"
	@echo "  build-cli          : Build the CLI binary"
	@echo "  build-sqs-consumer : Build the SQS consumer binary"
	@echo "  clean              : Remove generated files"
	@echo "  test               : Run all tests in the project"
	@echo "  localstack         : Start Localstack and apply Terraform config (combined)"
	@echo "  localstack-start   : Start Localstack container"
	@echo "  localstack-setup-queue : Create SQS queue in Localstack (legacy, use terraform-apply)"
	@echo "  localstack-stop    : Stop and remove Localstack container"
	@echo "  terraform-init     : Initialise OpenTofu in the terraform/ directory"
	@echo "  terraform-plan     : Show Terraform plan for LocalStack SQS resources"
	@echo "  terraform-apply    : Apply Terraform config to create SQS queues in LocalStack"
	@echo "  terraform-destroy  : Destroy Terraform-managed SQS resources in LocalStack"
	@echo "  start-sqs-consumer : Start the SQS consumer with Localstack configuration"
	@echo "  help               : Display this help message"

