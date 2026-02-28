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

include make/go.mk
include make/docker.mk
include make/terraform.mk
include make/localstack.mk

# Namespace dispatcher targets
# Usage: make go <cmd>, make docker <cmd>, make terraform <cmd>, make localstack <cmd>

.PHONY: go
go:
	@subcmd="$(filter-out $@,$(MAKECMDGOALS))"; \
	[ -n "$$subcmd" ] || { echo "Usage: make go <cmd>"; echo "Available: build, build-cli, build-sqs-consumer, clean, test, start-sqs-consumer, send-test-message"; exit 1; }; \
	$(MAKE) go-$$subcmd

.PHONY: docker
docker:
	@subcmd="$(filter-out $@,$(MAKECMDGOALS))"; \
	[ -n "$$subcmd" ] || { echo "Usage: make docker <cmd>"; echo "Available: build"; exit 1; }; \
	$(MAKE) docker-$$subcmd

.PHONY: terraform
terraform:
	@subcmd="$(filter-out $@,$(MAKECMDGOALS))"; \
	[ -n "$$subcmd" ] || { echo "Usage: make terraform <cmd>"; echo "Available: init, plan, apply, destroy"; exit 1; }; \
	$(MAKE) terraform-$$subcmd

.PHONY: localstack
localstack:
	@subcmd="$(filter-out $@,$(MAKECMDGOALS))"; \
	[ -n "$$subcmd" ] || subcmd="up"; \
	$(MAKE) localstack-$$subcmd

# When a namespace is the first goal, prevent remaining goals from being
# treated as standalone targets (they are sub-commands, not make goals).
ifneq ($(filter go docker terraform localstack,$(firstword $(MAKECMDGOALS))),)
%:
	@:
endif

.PHONY: help
help:
	@echo "Valid targets are:"
	@echo "  make go build              : Build all binaries (CLI and SQS consumer)"
	@echo "  make go build-cli          : Build the CLI binary"
	@echo "  make go build-sqs-consumer : Build the SQS consumer binary"
	@echo "  make go clean              : Remove generated files"
	@echo "  make go test               : Run all tests in the project"
	@echo "  make go start-sqs-consumer : Start the SQS consumer with Localstack configuration"
	@echo "  make go send-test-message  : Send a test message to the SQS queue"
	@echo "  make localstack            : Start Localstack and apply Terraform config (combined)"
	@echo "  make localstack start      : Start Localstack container"
	@echo "  make localstack setup-queue: Create SQS queue in Localstack (legacy, use terraform apply)"
	@echo "  make localstack stop       : Stop and remove Localstack container"
	@echo "  make terraform init        : Initialise OpenTofu in the terraform/ directory"
	@echo "  make terraform plan        : Show Terraform plan for LocalStack SQS resources"
	@echo "  make terraform apply       : Apply Terraform config to create SQS queues in LocalStack"
	@echo "  make terraform destroy     : Destroy Terraform-managed SQS resources in LocalStack"
	@echo "  make docker build          : Build the Docker image for the SQS consumer"
	@echo "  help                       : Display this help message"

