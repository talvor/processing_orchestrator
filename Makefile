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
	@echo "  docker-build       : Build the Docker image for the SQS consumer"
	@echo "  help               : Display this help message"

