package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// TestProcessMessageWithValidWorkflow tests processing a message with a valid workflow file
func TestProcessMessageWithValidWorkflow(t *testing.T) {
	// Create a temporary workflow file
	workflowContent := `name: test workflow
steps:
  - name: step1
    command: "echo 'test step'"
`
	tmpFile, err := os.CreateTemp("", "workflow-*.yaml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(workflowContent); err != nil {
		t.Fatalf("Failed to write workflow: %v", err)
	}
	tmpFile.Close()

	deleteCalled := false
	mockClient := &MockSQSClient{
		DeleteMessageFunc: func(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
			deleteCalled = true
			return &sqs.DeleteMessageOutput{}, nil
		},
		ChangeMessageVisibilityFunc: func(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
			return &sqs.ChangeMessageVisibilityOutput{}, nil
		},
	}

	consumer := NewSQSConsumer(mockClient, "test-queue-url")

	// Create a valid message
	msg := WorkflowMessage{
		WorkflowFile: tmpFile.Name(),
	}
	msgBody, _ := json.Marshal(msg)

	messageId := "test-message-id"
	receiptHandle := "test-receipt-handle"
	body := string(msgBody)
	sqsMessage := types.Message{
		MessageId:     &messageId,
		ReceiptHandle: &receiptHandle,
		Body:          &body,
	}

	// Process the message
	consumer.processMessage(context.Background(), sqsMessage)

	// Verify message was deleted (workflow succeeded)
	if !deleteCalled {
		t.Error("Expected message to be deleted after successful workflow execution")
	}
}

// TestProcessMessageWithInvalidWorkflow tests processing a message with an invalid workflow file
func TestProcessMessageWithInvalidWorkflow(t *testing.T) {
	deleteCalled := false
	mockClient := &MockSQSClient{
		DeleteMessageFunc: func(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
			deleteCalled = true
			return &sqs.DeleteMessageOutput{}, nil
		},
		ChangeMessageVisibilityFunc: func(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
			return &sqs.ChangeMessageVisibilityOutput{}, nil
		},
	}

	consumer := NewSQSConsumer(mockClient, "test-queue-url")

	// Create a message with non-existent workflow file
	msg := WorkflowMessage{
		WorkflowFile: "/non/existent/file.yaml",
	}
	msgBody, _ := json.Marshal(msg)

	messageId := "test-message-id"
	receiptHandle := "test-receipt-handle"
	body := string(msgBody)
	sqsMessage := types.Message{
		MessageId:     &messageId,
		ReceiptHandle: &receiptHandle,
		Body:          &body,
	}

	// Process the message
	consumer.processMessage(context.Background(), sqsMessage)

	// Verify message was NOT deleted (workflow failed, should return to queue)
	if deleteCalled {
		t.Error("Expected message to NOT be deleted after failed workflow execution")
	}
}

// TestProcessMessageWithMalformedJSON tests processing a message with malformed JSON
func TestProcessMessageWithMalformedJSON(t *testing.T) {
	deleteCalled := false
	mockClient := &MockSQSClient{
		DeleteMessageFunc: func(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
			deleteCalled = true
			return &sqs.DeleteMessageOutput{}, nil
		},
	}

	consumer := NewSQSConsumer(mockClient, "test-queue-url")

	// Create a message with malformed JSON
	messageId := "test-message-id"
	receiptHandle := "test-receipt-handle"
	body := "this is not valid json"
	sqsMessage := types.Message{
		MessageId:     &messageId,
		ReceiptHandle: &receiptHandle,
		Body:          &body,
	}

	// Process the message
	consumer.processMessage(context.Background(), sqsMessage)

	// Verify message WAS deleted (malformed messages should be removed)
	if !deleteCalled {
		t.Error("Expected malformed message to be deleted")
	}
}

// TestProcessBatchWithMultipleMessages tests batch processing with multiple messages
func TestProcessBatchWithMultipleMessages(t *testing.T) {
	// Create temporary workflow files
	workflowContent := `name: test workflow
steps:
  - name: step1
    command: "echo 'test'"
`
	tmpFile1, _ := os.CreateTemp("", "workflow1-*.yaml")
	tmpFile1.WriteString(workflowContent)
	tmpFile1.Close()
	defer os.Remove(tmpFile1.Name())

	tmpFile2, _ := os.CreateTemp("", "workflow2-*.yaml")
	tmpFile2.WriteString(workflowContent)
	tmpFile2.Close()
	defer os.Remove(tmpFile2.Name())

	deleteCount := 0
	mockClient := &MockSQSClient{
		ReceiveMessageFunc: func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
			msg1, _ := json.Marshal(WorkflowMessage{WorkflowFile: tmpFile1.Name()})
			msg2, _ := json.Marshal(WorkflowMessage{WorkflowFile: tmpFile2.Name()})

			msgId1 := "msg-1"
			msgId2 := "msg-2"
			receipt1 := "receipt-1"
			receipt2 := "receipt-2"
			body1 := string(msg1)
			body2 := string(msg2)

			return &sqs.ReceiveMessageOutput{
				Messages: []types.Message{
					{MessageId: &msgId1, ReceiptHandle: &receipt1, Body: &body1},
					{MessageId: &msgId2, ReceiptHandle: &receipt2, Body: &body2},
				},
			}, nil
		},
		DeleteMessageFunc: func(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
			deleteCount++
			return &sqs.DeleteMessageOutput{}, nil
		},
		ChangeMessageVisibilityFunc: func(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
			return &sqs.ChangeMessageVisibilityOutput{}, nil
		},
	}

	consumer := NewSQSConsumer(mockClient, "test-queue-url")
	
	err := consumer.processBatch(context.Background())
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Both messages should be deleted (both workflows succeeded)
	if deleteCount != 2 {
		t.Errorf("Expected 2 messages to be deleted, got %d", deleteCount)
	}
}

// TestReceiveMessageParameters tests that the correct parameters are passed to ReceiveMessage
func TestReceiveMessageParameters(t *testing.T) {
	var capturedParams *sqs.ReceiveMessageInput
	mockClient := &MockSQSClient{
		ReceiveMessageFunc: func(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error) {
			capturedParams = params
			return &sqs.ReceiveMessageOutput{Messages: []types.Message{}}, nil
		},
	}

	consumer := NewSQSConsumer(mockClient, "test-queue-url")
	consumer.SetMaxMessages(5)
	consumer.SetVisibilityTimeout(60)

	consumer.processBatch(context.Background())

	if capturedParams == nil {
		t.Fatal("Expected ReceiveMessage to be called")
	}

	if *capturedParams.QueueUrl != "test-queue-url" {
		t.Errorf("Expected QueueUrl to be 'test-queue-url', got %s", *capturedParams.QueueUrl)
	}

	if capturedParams.MaxNumberOfMessages != 5 {
		t.Errorf("Expected MaxNumberOfMessages to be 5, got %d", capturedParams.MaxNumberOfMessages)
	}

	if capturedParams.VisibilityTimeout != 60 {
		t.Errorf("Expected VisibilityTimeout to be 60, got %d", capturedParams.VisibilityTimeout)
	}

	if capturedParams.WaitTimeSeconds != 20 {
		t.Errorf("Expected WaitTimeSeconds to be 20, got %d", capturedParams.WaitTimeSeconds)
	}
}

// TestExecuteWorkflowError tests that executeWorkflow properly returns errors
func TestExecuteWorkflowError(t *testing.T) {
	consumer := &SQSConsumer{}
	
	// Test with non-existent file
	err := consumer.executeWorkflow(context.Background(), "/non/existent/file.yaml")
	if err == nil {
		t.Error("Expected error for non-existent workflow file")
	}
	
	// Error message should contain details
	if err != nil {
		expectedSubstring := "failed to create workflow"
		if !contains(err.Error(), expectedSubstring) {
			t.Errorf("Expected error to contain '%s', got: %s", expectedSubstring, err.Error())
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// BenchmarkProcessMessage benchmarks message processing
func BenchmarkProcessMessage(b *testing.B) {
	// Create a temporary workflow file
	workflowContent := `name: test workflow
steps:
  - name: step1
    command: "echo 'test'"
`
	tmpFile, _ := os.CreateTemp("", "workflow-*.yaml")
	tmpFile.WriteString(workflowContent)
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	mockClient := &MockSQSClient{
		DeleteMessageFunc: func(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error) {
			return &sqs.DeleteMessageOutput{}, nil
		},
		ChangeMessageVisibilityFunc: func(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error) {
			return &sqs.ChangeMessageVisibilityOutput{}, nil
		},
	}

	consumer := NewSQSConsumer(mockClient, "test-queue-url")

	msg := WorkflowMessage{WorkflowFile: tmpFile.Name()}
	msgBody, _ := json.Marshal(msg)

	messageId := fmt.Sprintf("msg-%d", 0)
	receiptHandle := fmt.Sprintf("receipt-%d", 0)
	body := string(msgBody)
	sqsMessage := types.Message{
		MessageId:     &messageId,
		ReceiptHandle: &receiptHandle,
		Body:          &body,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		consumer.processMessage(context.Background(), sqsMessage)
	}
}
