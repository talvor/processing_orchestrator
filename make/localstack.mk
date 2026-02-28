# Localstack configuration
LOCALSTACK_CONTAINER_NAME=processing-orchestrator-localstack
LOCALSTACK_PORT=4566
LOCALSTACK_ENDPOINT=http://localhost:$(LOCALSTACK_PORT)

SQS_QUEUE_NAME=test-workflow-queue
AWS_REGION=us-east-1

# Localstack targets
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

.PHONY: localstack-stop
localstack-stop:
	@echo "Stopping Localstack container..."
	@docker stop $(LOCALSTACK_CONTAINER_NAME) 2>/dev/null || true
	@docker rm $(LOCALSTACK_CONTAINER_NAME) 2>/dev/null || true
	@echo "Localstack stopped and removed"

.PHONY: localstack-up
localstack-up: localstack-start terraform-apply
	@echo ""
	@echo "Localstack is ready for testing!"
