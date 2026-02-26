/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	_ "expvar"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	"processing_pipeline/consumer"
	"processing_pipeline/workflow"
)

func main() {
	go func() {
		http.ListenAndServe(":8080", nil)
	}()

	// Get queue URL from environment variable
	queueURL := os.Getenv("SQS_QUEUE_URL")
	if queueURL == "" {
		log.Fatal("SQS_QUEUE_URL environment variable is required")
	}

	// Get AWS region from environment variable (optional, will use default from AWS config)
	region := os.Getenv("AWS_REGION")

	fmt.Println("Starting SQS Consumer...")
	fmt.Printf("Queue URL: %s\n", queueURL)
	if region != "" {
		fmt.Printf("Region: %s\n", region)
	}

	// Load AWS configuration
	ctx := context.Background()
	var opts []func(*config.LoadOptions) error
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	awsConfig, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		log.Fatalf("Failed to load AWS configuration: %v", err)
	}

	// Create SQS client
	sqsClient := sqs.NewFromConfig(awsConfig)

	// Create consumer
	processor := workflow.NewWorkflowMessageProcessor()
	sqsConsumer := consumer.NewSQSConsumer(sqsClient, queueURL, processor)

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Create context for consumer
	consumerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start consumer in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- sqsConsumer.Start(consumerCtx)
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		fmt.Printf("\nReceived signal: %v\n", sig)
		fmt.Println("Shutting down SQS Consumer...")
		cancel()
	case err := <-errChan:
		if err != nil && err != context.Canceled {
			log.Printf("Consumer error: %v\n", err)
		}
	}

	fmt.Println("SQS Consumer stopped")
}
