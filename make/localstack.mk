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

.PHONY: localstack-up
localstack-up: localstack-start terraform-apply
	@echo ""
	@echo "Localstack is ready for testing!"
