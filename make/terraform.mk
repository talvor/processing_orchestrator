# Terraform configuration
TERRAFORM_DIR=./terraform
MAX_RECEIVE_COUNT=5

# Terraform targets
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
