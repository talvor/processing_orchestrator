package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// MockMessageProcessor is a mock implementation of MessageProcessor for testing
type MockMessageProcessor[M any] struct {
	DecodeMessageFunc  func(body string) (M, error)
	ProcessMessageFunc func(ctx context.Context, msg M) error
}

func (m *MockMessageProcessor[M]) DecodeMessage(body string) (M, error) {
	if m.DecodeMessageFunc != nil {
		return m.DecodeMessageFunc(body)
	}
	var zero M
	return zero, nil
}

func (m *MockMessageProcessor[M]) ProcessMessage(ctx context.Context, msg M) error {
	if m.ProcessMessageFunc != nil {
		return m.ProcessMessageFunc(ctx, msg)
	}
	return nil
}

// MockSQSClient is a mock implementation of SQS client for testing
type MockSQSClient struct {
	ReceiveMessageFunc          func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessageFunc           func(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	ChangeMessageVisibilityFunc func(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
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
	mockProcessor := &MockMessageProcessor[string]{}
	queueURL := "https://sqs.us-east-1.amazonaws.com/123456789012/test-queue"

	consumer := NewSQSConsumer(mockClient, queueURL, mockProcessor)

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
	mockProcessor := &MockMessageProcessor[string]{}
	consumer := NewSQSConsumer(mockClient, "test-queue-url", mockProcessor)

	consumer.SetMaxMessages(5)

	if consumer.maxMessages != 5 {
		t.Errorf("Expected maxMessages to be 5, got %d", consumer.maxMessages)
	}
}

func TestSetVisibilityTimeout(t *testing.T) {
	mockClient := &MockSQSClient{}
	mockProcessor := &MockMessageProcessor[string]{}
	consumer := NewSQSConsumer(mockClient, "test-queue-url", mockProcessor)

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
	mockProcessor := &MockMessageProcessor[string]{}

	consumer := NewSQSConsumer(mockClient, "test-queue-url", mockProcessor)
	ctx := context.Background()

	err := consumer.processBatch(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

type MockMessage struct {
	Greeting string `json:"greeting"`
}

func TestWorkflowMessageParsing(t *testing.T) {
	msg := MockMessage{
		Greeting: "Hello, World!",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal message: %v", err)
	}

	var parsed MockMessage
	err = json.Unmarshal(data, &parsed)
	if err != nil {
		t.Fatalf("Failed to unmarshal message: %v", err)
	}

	if parsed.Greeting != msg.Greeting {
		t.Errorf("Expected Greeting to be %s, got %s", msg.Greeting, parsed.Greeting)
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
	mockProcessor := &MockMessageProcessor[string]{}

	consumer := NewSQSConsumer(mockClient, "test-queue-url", mockProcessor)
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
	mockProcessor := &MockMessageProcessor[string]{}

	consumer := NewSQSConsumer(mockClient, "test-queue-url", mockProcessor)
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
	mockProcessor := &MockMessageProcessor[string]{}

	consumer := NewSQSConsumer(mockClient, "test-queue-url", mockProcessor)

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

func TestMetricsBatchesProcessed(t *testing.T) {
	before := MetricsBatchesProcessed.Value()

	mockClient := &MockSQSClient{
		ReceiveMessageFunc: func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
			return &sqs.ReceiveMessageOutput{Messages: []types.Message{}}, nil
		},
	}
	consumer := NewSQSConsumer(mockClient, "test-queue-url", &MockMessageProcessor[string]{})

	_ = consumer.processBatch(context.Background())

	if got := MetricsBatchesProcessed.Value() - before; got != 1 {
		t.Errorf("Expected MetricsBatchesProcessed to increment by 1, got %d", got)
	}
}

func TestMetricsMessagesReceived(t *testing.T) {
	before := MetricsMessagesReceived.Value()

	messageId := "msg-recv-1"
	receiptHandle := "rh-recv-1"
	body := `"hello"`
	mockClient := &MockSQSClient{
		ReceiveMessageFunc: func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
			return &sqs.ReceiveMessageOutput{
				Messages: []types.Message{
					{MessageId: &messageId, ReceiptHandle: &receiptHandle, Body: &body},
				},
			}, nil
		},
	}
	consumer := NewSQSConsumer(mockClient, "test-queue-url", &MockMessageProcessor[string]{})

	_ = consumer.processBatch(context.Background())

	// Wait for the processing goroutine to finish to avoid leaking into subsequent tests
	deadline := time.Now().Add(2 * time.Second)
	for consumer.MessagesInflight() != 0 {
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if got := MetricsMessagesReceived.Value() - before; got != 1 {
		t.Errorf("Expected MetricsMessagesReceived to increment by 1, got %d", got)
	}
}

func TestMetricsMessagesProcessed(t *testing.T) {
	before := MetricsMessagesProcessed.Value()

	messageId := "msg-ok-1"
	receiptHandle := "rh-ok-1"
	body := `"hello"`
	mockClient := &MockSQSClient{}
	mockProcessor := &MockMessageProcessor[string]{
		DecodeMessageFunc: func(b string) (string, error) { return b, nil },
		ProcessMessageFunc: func(ctx context.Context, msg string) error { return nil },
	}
	consumer := NewSQSConsumer(mockClient, "test-queue-url", mockProcessor)
	consumer.processMessage(context.Background(), types.Message{
		MessageId: &messageId, ReceiptHandle: &receiptHandle, Body: &body,
	})

	if got := MetricsMessagesProcessed.Value() - before; got != 1 {
		t.Errorf("Expected MetricsMessagesProcessed to increment by 1, got %d", got)
	}
}

func TestMetricsMessagesFailed(t *testing.T) {
	before := MetricsMessagesFailed.Value()

	messageId := "msg-fail-1"
	receiptHandle := "rh-fail-1"
	body := `"hello"`
	mockClient := &MockSQSClient{}
	mockProcessor := &MockMessageProcessor[string]{
		DecodeMessageFunc:  func(b string) (string, error) { return b, nil },
		ProcessMessageFunc: func(ctx context.Context, msg string) error { return errors.New("processing error") },
	}
	consumer := NewSQSConsumer(mockClient, "test-queue-url", mockProcessor)
	consumer.processMessage(context.Background(), types.Message{
		MessageId: &messageId, ReceiptHandle: &receiptHandle, Body: &body,
	})

	if got := MetricsMessagesFailed.Value() - before; got != 1 {
		t.Errorf("Expected MetricsMessagesFailed to increment by 1, got %d", got)
	}
}

func TestMetricsMessagesDecodeErrors(t *testing.T) {
	before := MetricsMessagesDecodeErrors.Value()

	messageId := "msg-decode-err-1"
	receiptHandle := "rh-decode-err-1"
	body := `bad-json`
	mockClient := &MockSQSClient{}
	mockProcessor := &MockMessageProcessor[string]{
		DecodeMessageFunc: func(b string) (string, error) { return "", errors.New("decode error") },
	}
	consumer := NewSQSConsumer(mockClient, "test-queue-url", mockProcessor)
	consumer.processMessage(context.Background(), types.Message{
		MessageId: &messageId, ReceiptHandle: &receiptHandle, Body: &body,
	})

	if got := MetricsMessagesDecodeErrors.Value() - before; got != 1 {
		t.Errorf("Expected MetricsMessagesDecodeErrors to increment by 1, got %d", got)
	}
}

func TestMetricsMessagesDeleted(t *testing.T) {
	before := MetricsMessagesDeleted.Value()

	messageId := "msg-del-1"
	receiptHandle := "rh-del-1"
	mockClient := &MockSQSClient{}
	consumer := NewSQSConsumer(mockClient, "test-queue-url", &MockMessageProcessor[string]{})
	consumer.deleteMessage(context.Background(), types.Message{
		MessageId: &messageId, ReceiptHandle: &receiptHandle,
	})

	if got := MetricsMessagesDeleted.Value() - before; got != 1 {
		t.Errorf("Expected MetricsMessagesDeleted to increment by 1, got %d", got)
	}
}

func TestMetricsVisibilityExtensions(t *testing.T) {
	before := MetricsVisibilityExtensions.Value()

	extendCalled := make(chan struct{}, 1)
	mockClient := &MockSQSClient{
		ChangeMessageVisibilityFunc: func(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
			select {
			case extendCalled <- struct{}{}:
			default:
			}
			return &sqs.ChangeMessageVisibilityOutput{}, nil
		},
	}
	consumer := NewSQSConsumer(mockClient, "test-queue-url", &MockMessageProcessor[string]{})
	consumer.visibilityExtendInterval = 20 * time.Millisecond

	messageId := "msg-vis-1"
	receiptHandle := "rh-vis-1"
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go consumer.extendVisibilityTimeout(ctx, types.Message{
		MessageId: &messageId, ReceiptHandle: &receiptHandle,
	}, done)

	<-extendCalled
	cancel()
	<-done

	if got := MetricsVisibilityExtensions.Value() - before; got < 1 {
		t.Errorf("Expected MetricsVisibilityExtensions to increment by at least 1, got %d", got)
	}
}

func TestSetConcurrency(t *testing.T) {
	mockClient := &MockSQSClient{}
	mockProcessor := &MockMessageProcessor[string]{}
	consumer := NewSQSConsumer(mockClient, "test-queue-url", mockProcessor)

	consumer.SetConcurrency(5)

	if consumer.concurrency != 5 {
		t.Errorf("Expected concurrency to be 5, got %d", consumer.concurrency)
	}
	if cap(consumer.sem) != 5 {
		t.Errorf("Expected semaphore capacity to be 5, got %d", cap(consumer.sem))
	}
}

func TestDefaultConcurrency(t *testing.T) {
	mockClient := &MockSQSClient{}
	mockProcessor := &MockMessageProcessor[string]{}
	consumer := NewSQSConsumer(mockClient, "test-queue-url", mockProcessor)

	if consumer.concurrency != 10 {
		t.Errorf("Expected default concurrency to be 10, got %d", consumer.concurrency)
	}
	if cap(consumer.sem) != 10 {
		t.Errorf("Expected default semaphore capacity to be 10, got %d", cap(consumer.sem))
	}
}

func TestMessagesInflight(t *testing.T) {
	processingStarted := make(chan struct{})
	processingBlock := make(chan struct{})

	messageId := "msg-inflight-1"
	receiptHandle := "rh-inflight-1"
	body := `"hello"`

	mockClient := &MockSQSClient{
		ReceiveMessageFunc: func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
			return &sqs.ReceiveMessageOutput{
				Messages: []types.Message{
					{MessageId: &messageId, ReceiptHandle: &receiptHandle, Body: &body},
				},
			}, nil
		},
	}
	mockProcessor := &MockMessageProcessor[string]{
		DecodeMessageFunc: func(b string) (string, error) { return b, nil },
		ProcessMessageFunc: func(ctx context.Context, msg string) error {
			close(processingStarted)
			<-processingBlock
			return nil
		},
	}

	consumer := NewSQSConsumer(mockClient, "test-queue-url", mockProcessor)
	consumer.visibilityExtendInterval = time.Hour // prevent visibility extensions during test

	_ = consumer.processBatch(context.Background())

	// Wait for processing to start
	<-processingStarted

	if got := consumer.MessagesInflight(); got != 1 {
		t.Errorf("Expected MessagesInflight to be 1 while processing, got %d", got)
	}

	// Unblock processing
	close(processingBlock)

	// Poll until the goroutine releases its slot
	deadline := time.Now().Add(2 * time.Second)
	for consumer.MessagesInflight() != 0 {
		if time.Now().After(deadline) {
			t.Error("Timeout waiting for MessagesInflight to reach 0")
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if got := consumer.MessagesInflight(); got != 0 {
		t.Errorf("Expected MessagesInflight to be 0 after processing, got %d", got)
	}
}

func TestConcurrencyLimit(t *testing.T) {
	const concurrencyLimit = 3
	const totalMessages = 5

	processingStarted := make(chan struct{}, totalMessages)
	processingBlock := make(chan struct{})

	messages := make([]types.Message, totalMessages)
	for i := 0; i < totalMessages; i++ {
		id := fmt.Sprintf("msg-%d", i)
		rh := fmt.Sprintf("rh-%d", i)
		body := `"hello"`
		messages[i] = types.Message{MessageId: &id, ReceiptHandle: &rh, Body: &body}
	}

	callCount := 0
	mockClient := &MockSQSClient{
		ReceiveMessageFunc: func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
			max := int(params.MaxNumberOfMessages)
			start := callCount
			end := start + max
			if end > len(messages) {
				end = len(messages)
			}
			callCount = end
			batch := messages[start:end]
			return &sqs.ReceiveMessageOutput{Messages: batch}, nil
		},
	}
	mockProcessor := &MockMessageProcessor[string]{
		DecodeMessageFunc: func(b string) (string, error) { return b, nil },
		ProcessMessageFunc: func(ctx context.Context, msg string) error {
			processingStarted <- struct{}{}
			<-processingBlock
			return nil
		},
	}

	consumer := NewSQSConsumer(mockClient, "test-queue-url", mockProcessor)
	consumer.SetConcurrency(concurrencyLimit)
	consumer.visibilityExtendInterval = time.Hour

	// First batch: fills concurrencyLimit slots
	_ = consumer.processBatch(context.Background())

	// Wait for all goroutines in the first batch to start
	for i := 0; i < concurrencyLimit; i++ {
		<-processingStarted
	}

	// At concurrency limit: next processBatch should block; run in goroutine
	batchDone := make(chan error, 1)
	go func() {
		batchDone <- consumer.processBatch(context.Background())
	}()

	// Poll until the semaphore is full, confirming the second batch is blocked
	deadline := time.Now().Add(2 * time.Second)
	for len(consumer.sem) < concurrencyLimit {
		if time.Now().After(deadline) {
			t.Error("Timeout waiting for semaphore to fill")
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if got := consumer.MessagesInflight(); got != concurrencyLimit {
		t.Errorf("Expected MessagesInflight to be %d at limit, got %d", concurrencyLimit, got)
	}

	// Unblock all processing goroutines
	close(processingBlock)

	// The blocked processBatch should now complete
	select {
	case err := <-batchDone:
		if err != nil {
			t.Errorf("Expected nil error from processBatch, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("processBatch did not unblock after slots were freed")
	}
}
