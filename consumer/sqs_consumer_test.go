package consumer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// MockSQSClient is a mock implementation of SQS client for testing
type MockSQSClient struct {
	ReceiveMessageFunc           func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessageFunc            func(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	ChangeMessageVisibilityFunc  func(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
}

func (m *MockSQSClient) ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
	if m.ReceiveMessageFunc != nil {
		return m.ReceiveMessageFunc(ctx, params, optFns...)
	}
	return &sqs.ReceiveMessageOutput{Messages: []types.Message{}}, nil
}

func (m *MockSQSClient) DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
	if m.DeleteMessageFunc != nil {
		return m.DeleteMessageFunc(ctx, params, optFns...)
	}
	return &sqs.DeleteMessageOutput{}, nil
}

func (m *MockSQSClient) ChangeMessageVisibility(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
	if m.ChangeMessageVisibilityFunc != nil {
		return m.ChangeMessageVisibilityFunc(ctx, params, optFns...)
	}
	return &sqs.ChangeMessageVisibilityOutput{}, nil
}

func TestNewSQSConsumer(t *testing.T) {
	mockClient := &MockSQSClient{}
	queueURL := "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue"

	consumer := NewSQSConsumer(mockClient, queueURL)

	if consumer == nil {
		t.Fatal("Expected consumer to be created, got nil")
	}

	if consumer.queueURL != queueURL {
		t.Errorf("Expected queueURL to be %s, got %s", queueURL, consumer.queueURL)
	}

	if consumer.maxMessages != 10 {
		t.Errorf("Expected default maxMessages to be 10, got %d", consumer.maxMessages)
	}

	if consumer.waitTimeSeconds != 20 {
		t.Errorf("Expected default waitTimeSeconds to be 20, got %d", consumer.waitTimeSeconds)
	}
}

func TestSetMaxMessages(t *testing.T) {
	mockClient := &MockSQSClient{}
	consumer := NewSQSConsumer(mockClient, "test-queue-url")

	consumer.SetMaxMessages(5)

	if consumer.maxMessages != 5 {
		t.Errorf("Expected maxMessages to be 5, got %d", consumer.maxMessages)
	}
}

func TestSetVisibilityTimeout(t *testing.T) {
	mockClient := &MockSQSClient{}
	consumer := NewSQSConsumer(mockClient, "test-queue-url")

	consumer.SetVisibilityTimeout(60)

	if consumer.visibilityTimeout != 60 {
		t.Errorf("Expected visibilityTimeout to be 60, got %d", consumer.visibilityTimeout)
	}
}

func TestProcessBatchNoMessages(t *testing.T) {
	mockClient := &MockSQSClient{
		ReceiveMessageFunc: func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
			return &sqs.ReceiveMessageOutput{
				Messages: []types.Message{},
			}, nil
		},
	}

	consumer := NewSQSConsumer(mockClient, "test-queue-url")
	ctx := context.Background()

	err := consumer.processBatch(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestWorkflowMessageParsing(t *testing.T) {
	msg := WorkflowMessage{
		WorkflowFile: "/path/to/workflow.yaml",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	var parsed WorkflowMessage
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if parsed.WorkflowFile != msg.WorkflowFile {
		t.Errorf("Expected WorkflowFile to be %s, got %s", msg.WorkflowFile, parsed.WorkflowFile)
	}
}

func TestDeleteMessage(t *testing.T) {
	deleteCalled := false
	mockClient := &MockSQSClient{
		DeleteMessageFunc: func(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
			deleteCalled = true
			return &sqs.DeleteMessageOutput{}, nil
		},
	}

	consumer := NewSQSConsumer(mockClient, "test-queue-url")
	ctx := context.Background()

	messageId := "test-message-id"
	receiptHandle := "test-receipt-handle"
	message := types.Message{
		MessageId:     &messageId,
		ReceiptHandle: &receiptHandle,
	}

	consumer.deleteMessage(ctx, message)

	if !deleteCalled {
		t.Error("Expected DeleteMessage to be called")
	}
}

func TestExtendVisibilityTimeout(t *testing.T) {
	extendCalled := false
	mockClient := &MockSQSClient{
		ChangeMessageVisibilityFunc: func(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
			extendCalled = true
			return &sqs.ChangeMessageVisibilityOutput{}, nil
		},
	}

	consumer := NewSQSConsumer(mockClient, "test-queue-url")
	consumer.visibilityExtendInterval = 50 * time.Millisecond // Short interval for testing

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	messageId := "test-message-id"
	receiptHandle := "test-receipt-handle"
	message := types.Message{
		MessageId:     &messageId,
		ReceiptHandle: &receiptHandle,
	}

	done := make(chan struct{})
	go consumer.extendVisibilityTimeout(ctx, message, done)

	<-done

	if !extendCalled {
		t.Error("Expected ChangeMessageVisibility to be called")
	}
}

func TestStartWithContextCancellation(t *testing.T) {
	receiveCalled := false
	mockClient := &MockSQSClient{
		ReceiveMessageFunc: func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
			receiveCalled = true
			// Simulate a delay
			time.Sleep(10 * time.Millisecond)
			return &sqs.ReceiveMessageOutput{Messages: []types.Message{}}, nil
		},
	}

	consumer := NewSQSConsumer(mockClient, "test-queue-url")

	ctx, cancel := context.WithCancel(context.Background())

	// Start consumer in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- consumer.Start(ctx)
	}()

	// Give it time to receive at least one message
	time.Sleep(50 * time.Millisecond)

	// Cancel the context
	cancel()

	// Wait for consumer to stop
	err := <-errChan
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}

	if !receiveCalled {
		t.Error("Expected ReceiveMessage to be called")
	}
}
