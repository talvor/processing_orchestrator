// Package consumer provides a generic SQS consumer for polling, fetching and managing SQS messages.
// Decoding and processing of messages is delegated via the MessageProcessor interface.
package consumer

import (
	"context"
	"expvar"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

// Metrics exported via expvar for monitoring the SQS consumer.
var (
	MetricsBatchesProcessed     = expvar.NewInt("sqs_consumer_batches_processed")
	MetricsMessagesReceived     = expvar.NewInt("sqs_consumer_messages_received")
	MetricsMessagesProcessed    = expvar.NewInt("sqs_consumer_messages_processed")
	MetricsMessagesFailed       = expvar.NewInt("sqs_consumer_messages_failed")
	MetricsMessagesDecodeErrors = expvar.NewInt("sqs_consumer_messages_decode_errors")
	MetricsMessagesDeleted      = expvar.NewInt("sqs_consumer_messages_deleted")
	MetricsVisibilityExtensions = expvar.NewInt("sqs_consumer_visibility_extensions")
)

// MessageProcessor defines the interface for decoding and processing SQS messages.
type MessageProcessor[M any] interface {
	DecodeMessage(body string) (M, error)
	ProcessMessage(ctx context.Context, msg M) error
}

// SQSClientAPI defines the interface for SQS operations
type SQSClientAPI interface {
	ReceiveMessage(ctx context.Context, params *sqs.ReceiveMessageInput, optFns ...func(*sqs.Options)) (*sqs.ReceiveMessageOutput, error)
	DeleteMessage(ctx context.Context, params *sqs.DeleteMessageInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageOutput, error)
	ChangeMessageVisibility(ctx context.Context, params *sqs.ChangeMessageVisibilityInput, optFns ...func(*sqs.Options)) (*sqs.ChangeMessageVisibilityOutput, error)
}

// SQSConsumer handles polling, fetching and managing messages from an SQS queue.
// Decoding and processing of messages is delegated to the provided MessageProcessor.
type SQSConsumer[M any] struct {
	sqsClient                SQSClientAPI
	queueURL                 string
	maxMessages              int32
	waitTimeSeconds          int32
	visibilityTimeout        int32
	visibilityExtendInterval time.Duration
	processor                MessageProcessor[M]
	concurrency              int
	semaphore                chan struct{}
}

// NewSQSConsumer creates a new SQS consumer instance
func NewSQSConsumer[M any](sqsClient SQSClientAPI, queueURL string, processor MessageProcessor[M]) *SQSConsumer[M] {
	const defaultConcurrency = 10
	return &SQSConsumer[M]{
		sqsClient:                sqsClient,
		queueURL:                 queueURL,
		maxMessages:              10,               // Default batch size
		waitTimeSeconds:          20,               // Long polling
		visibilityTimeout:        30,               // Initial visibility timeout in seconds
		visibilityExtendInterval: 10 * time.Second, // Extend every 10 seconds
		processor:                processor,
		concurrency:              defaultConcurrency,
		semaphore:                make(chan struct{}, defaultConcurrency),
	}
}

// SetMaxMessages sets the maximum number of messages to receive in a batch
func (c *SQSConsumer[M]) SetMaxMessages(max int32) {
	c.maxMessages = max
}

// SetVisibilityTimeout sets the initial visibility timeout for messages
func (c *SQSConsumer[M]) SetVisibilityTimeout(timeout int32) {
	c.visibilityTimeout = timeout
}

// SetConcurrency sets the maximum number of messages to process concurrently.
// Must be called before Start.
func (c *SQSConsumer[M]) SetConcurrency(n int) {
	c.concurrency = n
	c.semaphore = make(chan struct{}, n)
}

// Start begins consuming messages from the SQS queue
func (c *SQSConsumer[M]) Start(ctx context.Context) error {
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
func (c *SQSConsumer[M]) processBatch(ctx context.Context) error {
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

	MetricsBatchesProcessed.Add(1)

	if len(result.Messages) == 0 {
		return nil
	}

	MetricsMessagesReceived.Add(int64(len(result.Messages)))
	log.Printf("Received %d messages\n", len(result.Messages))

	// Process each message concurrently, respecting the concurrency limit
	var wg sync.WaitGroup
loop:
	for _, message := range result.Messages {
		// Acquire a semaphore slot, or stop if context is cancelled
		select {
		case c.semaphore <- struct{}{}:
		case <-ctx.Done():
			break loop
		}
		wg.Add(1)
		go func(msg types.Message) {
			defer wg.Done()
			defer func() { <-c.semaphore }()
			c.processMessage(ctx, msg)
		}(message)
	}

	wg.Wait()
	return nil
}

// processMessage decodes and processes a single SQS message
func (c *SQSConsumer[M]) processMessage(ctx context.Context, message types.Message) {
	log.Printf("Processing message: %s\n", *message.MessageId)

	// Decode the message body via the processor
	msg, err := c.processor.DecodeMessage(*message.Body)
	if err != nil {
		log.Printf("Failed to decode message body: %v\n", err)
		MetricsMessagesDecodeErrors.Add(1)
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

	// Delegate processing to the processor
	err = c.processor.ProcessMessage(processCtx, msg)

	// Stop visibility timeout extension
	cancelProcess()
	<-extendDone

	// Handle the result
	if err != nil {
		log.Printf("Processing failed for message %s: %v\n", *message.MessageId, err)
		MetricsMessagesFailed.Add(1)
		// Message will automatically return to queue after visibility timeout
	} else {
		log.Printf("Processing succeeded for message %s\n", *message.MessageId)
		MetricsMessagesProcessed.Add(1)
		c.deleteMessage(ctx, message)
	}
}

// extendVisibilityTimeout periodically extends the visibility timeout of a message
func (c *SQSConsumer[M]) extendVisibilityTimeout(ctx context.Context, message types.Message, done chan struct{}) {
	defer close(done)

	ticker := time.NewTicker(c.visibilityExtendInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, err := c.sqsClient.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
				QueueUrl:          aws.String(c.queueURL),
				ReceiptHandle:     message.ReceiptHandle,
				VisibilityTimeout: c.visibilityTimeout,
			})
			if err != nil {
				log.Printf("Failed to extend visibility timeout for message %s: %v\n", *message.MessageId, err)
			} else {
				log.Printf("Extended visibility timeout for message %s\n", *message.MessageId)
				MetricsVisibilityExtensions.Add(1)
			}
		}
	}
}

// deleteMessage deletes a message from the queue after successful processing
func (c *SQSConsumer[M]) deleteMessage(ctx context.Context, message types.Message) {
	_, err := c.sqsClient.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(c.queueURL),
		ReceiptHandle: message.ReceiptHandle,
	})

	if err != nil {
		log.Printf("Failed to delete message %s: %v\n", *message.MessageId, err)
	} else {
		log.Printf("Deleted message %s\n", *message.MessageId)
		MetricsMessagesDeleted.Add(1)
	}
}
