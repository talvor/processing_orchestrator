// Package consumer provides SQS consumer functionality for processing workflow messages
package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"processing_pipeline/workflow"
)

// SQSClientAPI defines the interface for SQS operations
type SQSClientAPI interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	ChangeMessageVisibility(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
}

// WorkflowMessage represents the expected message structure from SQS
type WorkflowMessage struct {
	WorkflowFile string `json:"workflow_file"`
}

// SQSConsumer handles receiving messages from SQS and processing workflows
type SQSConsumer struct {
	sqsClient      SQSClientAPI
	queueURL       string
	maxMessages    int32
	waitTimeSeconds int32
	visibilityTimeout int32
	visibilityExtendInterval time.Duration
}

// NewSQSConsumer creates a new SQS consumer instance
func NewSQSConsumer(sqsClient SQSClientAPI, queueURL string) *SQSConsumer {
	return &SQSConsumer{
		sqsClient:      sqsClient,
		queueURL:       queueURL,
		maxMessages:    10, // Default batch size
		waitTimeSeconds: 20, // Long polling
		visibilityTimeout: 30, // Initial visibility timeout in seconds
		visibilityExtendInterval: 10 * time.Second, // Extend every 10 seconds
	}
}

// SetMaxMessages sets the maximum number of messages to receive in a batch
func (c *SQSConsumer) SetMaxMessages(max int32) {
	c.maxMessages = max
}

// SetVisibilityTimeout sets the initial visibility timeout for messages
func (c *SQSConsumer) SetVisibilityTimeout(timeout int32) {
	c.visibilityTimeout = timeout
}

// Start begins consuming messages from the SQS queue
func (c *SQSConsumer) Start(ctx context.Context) error {
	log.Println("Starting SQS consumer...")
	
	for {
		select {
		case <-ctx.Done():
			log.Println("Context cancelled, stopping consumer...")
			return ctx.Err()
		default:
			if err := c.processBatch(ctx); err != nil {
				log.Printf("Error processing batch: %v\n", err)
			}
		}
	}
}

// processBatch receives and processes a batch of messages
func (c *SQSConsumer) processBatch(ctx context.Context) error {
	// Receive messages from the queue
	result, err := c.sqsClient.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(c.queueURL),
		MaxNumberOfMessages: c.maxMessages,
		WaitTimeSeconds:     c.waitTimeSeconds,
		VisibilityTimeout:   c.visibilityTimeout,
	})
	
	if err != nil {
		return fmt.Errorf("failed to receive messages: %w", err)
	}
	
	if len(result.Messages) == 0 {
		return nil
	}
	
	log.Printf("Received %d messages\n", len(result.Messages))
	
	// Process each message
	var wg sync.WaitGroup
	for _, message := range result.Messages {
		wg.Add(1)
		go func(msg types.Message) {
			defer wg.Done()
			c.processMessage(ctx, msg)
		}(message)
	}
	
	wg.Wait()
	return nil
}

// processMessage processes a single SQS message
func (c *SQSConsumer) processMessage(ctx context.Context, message types.Message) {
	log.Printf("Processing message: %s\n", *message.MessageId)
	
	// Parse the message body
	var workflowMsg WorkflowMessage
	if err := json.Unmarshal([]byte(*message.Body), &workflowMsg); err != nil {
		log.Printf("Failed to parse message body: %v\n", err)
		// Delete malformed messages to prevent repeated processing
		c.deleteMessage(ctx, message)
		return
	}
	
	// Create a context for visibility timeout extension
	processCtx, cancelProcess := context.WithCancel(ctx)
	defer cancelProcess()
	
	// Start visibility timeout extension in a separate goroutine
	extendDone := make(chan struct{})
	go c.extendVisibilityTimeout(processCtx, message, extendDone)
	
	// Create and execute the workflow
	err := c.executeWorkflow(processCtx, workflowMsg.WorkflowFile)
	
	// Stop visibility timeout extension
	cancelProcess()
	<-extendDone
	
	// Handle the result
	if err != nil {
		log.Printf("Workflow execution failed for message %s: %v\n", *message.MessageId, err)
		// Message will automatically return to queue after visibility timeout
		// No action needed for failure case
	} else {
		log.Printf("Workflow execution succeeded for message %s\n", *message.MessageId)
		c.deleteMessage(ctx, message)
	}
}

// executeWorkflow creates and executes a workflow from the specified file
func (c *SQSConsumer) executeWorkflow(ctx context.Context, workflowFile string) error {
	w, err := workflow.NewWorkflow(workflowFile)
	if err != nil {
		return fmt.Errorf("failed to create workflow: %w", err)
	}
	
	if err := w.Execute(); err != nil {
		return fmt.Errorf("workflow execution failed: %w", err)
	}
	
	return nil
}

// extendVisibilityTimeout periodically extends the visibility timeout of a message
func (c *SQSConsumer) extendVisibilityTimeout(ctx context.Context, message types.Message, done chan struct{}) {
	defer close(done)
	
	ticker := time.NewTicker(c.visibilityExtendInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			newTimeout := c.visibilityTimeout
			_, err := c.sqsClient.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
				QueueUrl:          aws.String(c.queueURL),
				ReceiptHandle:     message.ReceiptHandle,
				VisibilityTimeout: newTimeout,
			})
			if err != nil {
				log.Printf("Failed to extend visibility timeout for message %s: %v\n", *message.MessageId, err)
			} else {
				log.Printf("Extended visibility timeout for message %s\n", *message.MessageId)
			}
		}
	}
}

// deleteMessage deletes a message from the queue after successful processing
func (c *SQSConsumer) deleteMessage(ctx context.Context, message types.Message) {
	_, err := c.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: message.ReceiptHandle,
	})
	
	if err != nil {
		log.Printf("Failed to delete message %s: %v\n", *message.MessageId, err)
	} else {
		log.Printf("Deleted message %s\n", *message.MessageId)
	}
}
