# Docker targets

.PHONY: docker-build
docker-build:
	@echo "Building Docker image for SQS consumer..."
	@docker build -t sqs-consumer:latest .
