# SQS Consumer

The SQS Consumer is a service that reads workflow execution requests from an AWS SQS queue and processes them using the Processing Orchestrator.

## Features

- **Batch Processing**: Receives and processes up to 10 messages at a time (configurable)
- **Parallel Execution**: Processes multiple workflows concurrently
- **Visibility Timeout Extension**: Automatically extends message visibility timeout while workflows are processing
- **Automatic Retry**: Failed workflows remain in the queue for reprocessing
- **Graceful Shutdown**: Handles SIGINT and SIGTERM signals for clean shutdown

## Configuration

The SQS Consumer is configured using environment variables:

- `SQS_QUEUE_URL` (required): The URL of the SQS queue to consume messages from
- `AWS_REGION` (optional): The AWS region. If not set, uses the default from AWS configuration

AWS credentials are loaded using the standard AWS SDK chain:
1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM role for EC2 instances or ECS tasks

## Message Format

Messages in the SQS queue should be JSON formatted with the following structure:

```json
{
  "workflow_file": "/path/to/workflow.yaml"
}
```

The `workflow_file` should be the absolute path to a valid workflow YAML configuration file.

## Usage

### Local Testing with Localstack

For local testing and development, you can use Localstack to simulate AWS SQS without needing an actual AWS account.

#### Quick Start

1. Start Localstack and create the test queue:

```bash
make localstack
```

This will:
- Start a Localstack container on port 4566
- Create a test SQS queue named `test-workflow-queue`
- Display the connection information

2. Build the SQS consumer:

```bash
make build-sqs-consumer
```

3. Run the SQS consumer with Localstack:

```bash
export SQS_QUEUE_URL=http://localhost:4566/000000000000/test-workflow-queue
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_ENDPOINT_URL=http://localhost:4566
./bin/sqs_consumer
```

4. Send test messages to the queue (in another terminal):

```bash
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
aws --endpoint-url=http://localhost:4566 --region us-east-1 sqs send-message \
  --queue-url http://localhost:4566/000000000000/test-workflow-queue \
  --message-body '{"workflow_file":"'$(pwd)'/examples/single-workflow.yaml"}'
```

5. When done testing, stop Localstack:

```bash
make localstack-stop
```

#### Available Make Targets

- `make localstack` - Start Localstack and setup SQS queue (combined)
- `make localstack-start` - Start Localstack container only
- `make localstack-setup-queue` - Create SQS queue in running Localstack
- `make localstack-stop` - Stop and remove Localstack container

### Building

```bash
make build-sqs-consumer
```

This creates the binary at `./bin/sqs_consumer`.

### Running

```bash
export SQS_QUEUE_URL="https://sqs.us-east-1.amazonaws.com/123456789012/my-workflow-queue"
export AWS_REGION="us-east-1"
./bin/sqs_consumer
```

### Running with Docker (example)

```dockerfile
FROM golang:1.25.4-alpine AS builder
WORKDIR /app
COPY . .
RUN make build-sqs-consumer

FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/bin/sqs_consumer .
CMD ["./sqs_consumer"]
```

## Behavior

### Message Processing Flow

1. Consumer receives a batch of messages from the SQS queue (long polling with 20s wait)
2. For each message:
   - Parse the JSON to extract the workflow file path
   - Create a workflow from the specified YAML file
   - Start extending the message visibility timeout in the background (every 10 seconds)
   - Execute the workflow
   - If successful: Delete the message from the queue
   - If failed: Stop extending visibility timeout and let the message return to the queue

### Visibility Timeout

- Initial visibility timeout: 30 seconds (default)
- Extension interval: 10 seconds
- The visibility timeout is extended every 10 seconds while a workflow is processing
- This prevents other consumers from receiving the same message while it's being processed
- When a workflow completes (success or failure), the extension stops

### Error Handling

- **Malformed messages**: Deleted from the queue to prevent infinite reprocessing
- **Workflow execution failures**: Message returns to the queue after visibility timeout expires
- **Missing workflow files**: Treated as workflow execution failure
- **AWS API errors**: Logged, processing continues with other messages

## IAM Permissions

The SQS Consumer requires the following IAM permissions:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "sqs:ReceiveMessage",
        "sqs:DeleteMessage",
        "sqs:ChangeMessageVisibility"
      ],
      "Resource": "arn:aws:sqs:*:*:your-queue-name"
    }
  ]
}
```

## Example Workflow

1. Create a workflow YAML file at `/workflows/example.yaml`:

```yaml
name: example workflow
steps:
  - name: step1
    command: "echo 'Processing data...'"
  - name: step2
    command: "echo 'Data processed successfully'"
    depends:
      - step1
```

2. Send a message to the SQS queue:

```bash
aws sqs send-message \
  --queue-url https://sqs.us-east-1.amazonaws.com/123456789012/my-workflow-queue \
  --message-body '{"workflow_file":"/workflows/example.yaml"}'
```

3. The SQS Consumer will receive the message, execute the workflow, and delete the message upon success.

## Monitoring

The consumer logs important events:

- Starting and stopping
- Messages received
- Workflow execution start/completion/failure
- Visibility timeout extensions
- Message deletion
- Errors

Example log output:
```
Starting SQS Consumer...
Queue URL: https://sqs.us-east-1.amazonaws.com/123456789012/my-workflow-queue
Region: us-east-1
Received 3 messages
Processing message: abc123...
Extended visibility timeout for message abc123
Workflow execution succeeded for message abc123
Deleted message abc123
```

## Graceful Shutdown

The consumer handles SIGINT (Ctrl+C) and SIGTERM signals gracefully:

1. Stops receiving new messages
2. Waits for currently processing workflows to complete (with a timeout)
3. Exits cleanly

This ensures that in-flight workflows are not interrupted during deployment or scaling operations.
