package orchestrator

import (
	"sync"
	"time"
)

// ExecutionEvent represents different stages of node execution
type ExecutionEvent string

const (
	EventWorkflowStarted    ExecutionEvent = "workflow_started"
	EventWorkflowCompleted  ExecutionEvent = "workflow_completed"
	EventWorkflowFailed     ExecutionEvent = "workflow_failed"
	EventNodeStarted        ExecutionEvent = "node_started"
	EventNodeCompleted      ExecutionEvent = "node_completed"
	EventNodeFailed         ExecutionEvent = "node_failed"
	EventNodeFailedContinue ExecutionEvent = "node_failed_continue"
	EventNodeSkipped        ExecutionEvent = "node_skipped"
)

// NodeEventData contains information about a node execution event
type NodeEventData struct {
	NodeName  string
	Event     ExecutionEvent
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Error     error
}

// WorkflowEventData contains information about workflow-level events
type WorkflowEventData struct {
	Event          ExecutionEvent
	StartTime      time.Time
	EndTime        time.Time
	Duration       time.Duration
	Error          error
	TotalNodes     int
	CompletedNodes int
}

// ExecutionLogger defines the interface for observing orchestrator execution
type ExecutionLogger interface {
	// OnWorkflowStart is called when the workflow begins execution
	OnWorkflowStart(data WorkflowEventData)

	// OnWorkflowComplete is called when the workflow completes (success or failure)
	OnWorkflowComplete(data WorkflowEventData)

	// OnNodeEvent is called for each node state change
	OnNodeEvent(data NodeEventData)

	// OnStatusUpdate is called periodically with a snapshot of all node states
	// This allows loggers like TreeLogger to update their display
	OnStatusUpdate(states map[string]*NodeStateSnapshot, dag *DAGSnapshot)

	// Close is called to clean up logger resources
	Close()
}

// NodeStateSnapshot provides a read-only view of node state for loggers
type NodeStateSnapshot struct {
	Completed      bool
	InProgress     bool
	Skipped        bool
	Failed         bool
	FailedContinue bool
	StartTime      time.Time
	EndTime        time.Time
	Error          error
}

// DAGSnapshot provides read-only DAG information for loggers
type DAGSnapshot struct {
	Nodes      map[string]*NodeSnapshot
	NodesMutex sync.RWMutex
}

type NodeSnapshot struct {
	Name    string
	Depends []string
}

// NoOpLogger is a logger that does nothing
type NoOpLogger struct{}

func (n *NoOpLogger) OnWorkflowStart(data WorkflowEventData)                                     {}
func (n *NoOpLogger) OnWorkflowComplete(data WorkflowEventData)                                  {}
func (n *NoOpLogger) OnNodeEvent(data NodeEventData)                                             {}
func (n *NoOpLogger) OnStatusUpdate(states map[string]*NodeStateSnapshot, dag *DAGSnapshot)      {}
func (n *NoOpLogger) Close()                                                                      {}

func NewNoOpLogger() *NoOpLogger {
	return &NoOpLogger{}
}