package consumer

import (
	"context"
	"encoding/json"
	"fmt"

	"processing_pipeline/workflow"
)

// WorkflowMessage represents the expected message structure for workflow processing
type WorkflowMessage struct {
	WorkflowFile string `json:"workflow_file"`
}

// WorkflowMessageProcessor implements MessageProcessor for WorkflowMessage,
// decoding JSON messages and executing the specified workflow.
type WorkflowMessageProcessor struct{}

// NewWorkflowMessageProcessor creates a new WorkflowMessageProcessor
func NewWorkflowMessageProcessor() *WorkflowMessageProcessor {
	return &WorkflowMessageProcessor{}
}

// DecodeMessage decodes a JSON message body into a WorkflowMessage
func (p *WorkflowMessageProcessor) DecodeMessage(body string) (WorkflowMessage, error) {
	var msg WorkflowMessage
	if err := json.Unmarshal([]byte(body), &msg); err != nil {
		return WorkflowMessage{}, fmt.Errorf("failed to parse message body: %w", err)
	}
	return msg, nil
}

// ProcessMessage executes the workflow specified in the message
func (p *WorkflowMessageProcessor) ProcessMessage(ctx context.Context, msg WorkflowMessage) error {
	w, err := workflow.NewWorkflow(msg.WorkflowFile)
	if err != nil {
		return fmt.Errorf("failed to create workflow: %w", err)
	}

	w.SetJob(msg)

	if err := w.Execute(ctx); err != nil {
		return fmt.Errorf("workflow execution failed: %w", err)
	}

	return nil
}
