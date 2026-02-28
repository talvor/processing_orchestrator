# Docker targets

.PHONY: docker-build
docker-build:
	@echo "Building Docker image for SQS consumer..."
	@docker build -t local/sqs-consumer:latest .

.PHONY: docker-start
docker-start:
	@echo "Start SQS consumer..."
	@docker compose up -d

.PHONY: docker-stop
docker-stop:
	@echo "Stopping SQS consumer..."
	@docker compose down
