// Package orchestrator implements the core logic for executing a workflow defined as a DAG (Directed Acyclic Graph). It handles node execution, dependency resolution, precondition checks, and retry policies with parallel execution support.
package orchestrator

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"processing_pipeline/dag"
)

// WhenConditionNotMetError is a special error type that indicates a when condition was not met
type WhenConditionNotMetError struct {
	Message string
}

func (e *WhenConditionNotMetError) Error() string {
	return e.Message
}

type Orchestrator struct {
	Dag         *dag.DAG
	WorkerCount int             // Maximum number of concurrent workers
	logger      ExecutionLogger // Pluggable logger
	parentCtx   context.Context // Optional parent context
}

// nodeState tracks the execution state of each node
type nodeState struct {
	completed      bool
	inProgress     bool
	skipped        bool // Node was skipped due to parent failure
	failed         bool
	failedContinue bool
	remainingDeps  int
	startTime      time.Time
	endTime        time.Time
	error          error
	mu             sync.Mutex
}

func NewOrchestrator(dag *dag.DAG, parentCtx context.Context) *Orchestrator {
	return &Orchestrator{
		Dag:         dag,
		WorkerCount: 5,                 // Default worker count
		logger:      NewStreamLogger(), // Default to stream logger
		parentCtx:   parentCtx,
	}
}

// NewOrchestratorWithWorkers creates an orchestrator with a specific worker count
func NewOrchestratorWithWorkers(dag *dag.DAG, workerCount int, parentCtx context.Context) *Orchestrator {
	return &Orchestrator{
		Dag:         dag,
		WorkerCount: workerCount,
		logger:      NewStreamLogger(),
		parentCtx:   parentCtx,
	}
}

// SetLogger configures the logger for the orchestrator
func (wo *Orchestrator) SetLogger(logger ExecutionLogger) {
	wo.logger = logger
}

// Helper to create DAG snapshot for loggers
func (wo *Orchestrator) createDAGSnapshot() *DAGSnapshot {
	snapshot := &DAGSnapshot{
		Nodes: make(map[string]*NodeSnapshot),
	}

	for name, node := range wo.Dag.Nodes {
		snapshot.Nodes[name] = &NodeSnapshot{
			Name:    node.Name,
			Depends: node.Depends,
		}
	}

	return snapshot
}

// Helper to create state snapshots for loggers
func (wo *Orchestrator) createStateSnapshot(nodeStates map[string]*nodeState) map[string]*NodeStateSnapshot {
	snapshots := make(map[string]*NodeStateSnapshot)
	for name, state := range nodeStates {
		state.mu.Lock()
		snapshots[name] = &NodeStateSnapshot{
			Completed:      state.completed,
			InProgress:     state.inProgress,
			Skipped:        state.skipped,
			Failed:         state.failed,
			FailedContinue: state.failedContinue,
			StartTime:      state.startTime,
			EndTime:        state.endTime,
			Error:          state.error,
		}
		state.mu.Unlock()
	}
	return snapshots
}

// Helper to safely call logger methods
func (wo *Orchestrator) logNodeEvent(nodeName string, event ExecutionEvent, state *nodeState, skipReason string) {
	if wo.logger == nil {
		return
	}

	data := NodeEventData{
		NodeName:   nodeName,
		Event:      event,
		StartTime:  state.startTime,
		EndTime:    state.endTime,
		Error:      state.error,
		SkipReason: skipReason,
	}
	if !state.endTime.IsZero() && !state.startTime.IsZero() {
		data.Duration = state.endTime.Sub(state.startTime)
	}

	wo.logger.OnNodeEvent(data)
}

func (wo *Orchestrator) Execute() error {
	start := time.Now()
	dagSnapshot := wo.createDAGSnapshot()

	// Notify logger of workflow start
	if wo.logger != nil {
		wo.logger.OnWorkflowStart(WorkflowEventData{
			Event:      EventWorkflowStarted,
			StartTime:  start,
			TotalNodes: len(wo.Dag.Nodes),
		})
	}

	// Initialize node states and dependency tracking
	nodeStates := make(map[string]*nodeState)
	dependents := make(map[string][]string) // Maps node -> nodes that depend on it

	for name, node := range wo.Dag.Nodes {
		nodeStates[name] = &nodeState{
			completed:      false,
			inProgress:     false,
			skipped:        false,
			failed:         false,
			failedContinue: false,
			remainingDeps:  len(node.Depends),
		}

		// Build reverse dependency map
		for _, dep := range node.Depends {
			dependents[dep] = append(dependents[dep], name)
		}
	}

	// Find initially ready nodes (nodes with no dependencies)
	readyQueue := make(chan string, len(wo.Dag.Nodes))
	for name, state := range nodeStates {
		if state.remainingDeps == 0 {
			readyQueue <- name
		}
	}

	// Error handling and cancellation
	// Use parent context if provided, otherwise use background context
	baseCtx := context.Background()
	if wo.parentCtx != nil {
		baseCtx = wo.parentCtx
	}
	ctx, cancel := context.WithCancel(baseCtx)
	defer cancel()

	var executionError error
	var errorMu sync.Mutex

	// Worker pool
	var wg sync.WaitGroup
	workerSemaphore := make(chan struct{}, wo.WorkerCount)

	// Completion tracking
	completedCount := 0
	totalNodes := len(wo.Dag.Nodes)
	var completedMu sync.Mutex

	// Display update ticker
	displayTicker := time.NewTicker(100 * time.Millisecond)
	defer displayTicker.Stop()
	displayDone := make(chan struct{})

	// Start display updater
	go func() {
		for {
			select {
			case <-displayTicker.C:
				if wo.logger != nil {
					wo.logger.OnStatusUpdate(wo.createStateSnapshot(nodeStates), dagSnapshot)
				}
			case <-displayDone:
				return
			}
		}
	}()

	// Helper function to check if a node should be skipped
	shouldSkipNode := func(nodeName string) bool {
		node := wo.Dag.Nodes[nodeName]
		// A node should be skipped only if ALL of its dependencies are failed/skipped
		for _, dep := range node.Depends {
			depState := nodeStates[dep]
			depState.mu.Lock()
			isDepSuccessful := depState.completed && !depState.failed && !depState.failedContinue && !depState.skipped
			depState.mu.Unlock()

			// If any dependency succeeded, don't skip this node
			if isDepSuccessful {
				return false
			}
		}
		// All dependencies are failed/skipped, so skip this node
		return len(node.Depends) > 0
	}

	// Helper function to determine skip reason based on dependencies
	getSkipReason := func(nodeName string) string {
		node := wo.Dag.Nodes[nodeName]
		var failedDeps []string
		var failedContinueDeps []string
		var skippedDeps []string

		for _, dep := range node.Depends {
			depState := nodeStates[dep]
			depState.mu.Lock()
			if depState.failed {
				failedDeps = append(failedDeps, dep)
			} else if depState.failedContinue {
				failedContinueDeps = append(failedContinueDeps, dep)
			} else if depState.skipped {
				skippedDeps = append(skippedDeps, dep)
			}
			depState.mu.Unlock()
		}

		// Prioritize reason based on most severe failure
		if len(failedDeps) > 0 {
			if len(failedDeps) == 1 {
				return fmt.Sprintf("parent failed: %s", failedDeps[0])
			}
			return fmt.Sprintf("parents failed: %s", strings.Join(failedDeps, ", "))
		}
		if len(failedContinueDeps) > 0 {
			if len(failedContinueDeps) == 1 {
				return fmt.Sprintf("parent failed (continuing): %s", failedContinueDeps[0])
			}
			return fmt.Sprintf("parents failed (continuing): %s", strings.Join(failedContinueDeps, ", "))
		}
		if len(skippedDeps) > 0 {
			if len(skippedDeps) == 1 {
				return fmt.Sprintf("parent skipped: %s", skippedDeps[0])
			}
			return fmt.Sprintf("parents skipped: %s", strings.Join(skippedDeps, ", "))
		}
		return "dependencies not met"
	}

	// Helper function to mark descendants that should be skipped
	var checkAndSkipDescendants func(string)
	checkAndSkipDescendants = func(nodeName string) {
		for _, dependent := range dependents[nodeName] {
			depState := nodeStates[dependent]
			depState.mu.Lock()

			if depState.completed || depState.skipped || depState.inProgress {
				depState.mu.Unlock()
				continue
			}

			// Check if this dependent should be skipped
			depState.mu.Unlock()
			if shouldSkipNode(dependent) {
				depState.mu.Lock()
				depState.skipped = true
				depState.completed = true
				depState.mu.Unlock()

				// Log skip event with reason
				skipReason := getSkipReason(dependent)
				wo.logNodeEvent(dependent, EventNodeSkipped, depState, skipReason)

				completedMu.Lock()
				completedCount++
				if completedCount == totalNodes {
					cancel() // Signal completion
				}
				completedMu.Unlock()

				// Recursively check descendants
				checkAndSkipDescendants(dependent)
			}
		}
	}

	// Start worker manager
	go func() {
		for {
			select {
			case <-ctx.Done():
				// Stop processing when context is cancelled
				return
			case nodeName, ok := <-readyQueue:
				if !ok {
					// Channel closed, stop processing
					return
				}

				// Check if we should stop due to fatal error (non-ContinueOnError)
				errorMu.Lock()
				if executionError != nil {
					errorMu.Unlock()
					return
				}
				errorMu.Unlock()

				state := nodeStates[nodeName]
				state.mu.Lock()

				// Skip if already processed, in progress, or marked as skipped
				if state.completed || state.inProgress || state.skipped {
					state.mu.Unlock()
					continue
				}

				// Check if this node should be skipped due to failed dependencies
				state.mu.Unlock()
				if shouldSkipNode(nodeName) {
					state.mu.Lock()
					state.skipped = true
					state.completed = true
					state.mu.Unlock()

					// Log skip event with reason
					skipReason := getSkipReason(nodeName)
					wo.logNodeEvent(nodeName, EventNodeSkipped, state, skipReason)

					completedMu.Lock()
					completedCount++
					if completedCount == totalNodes {
						cancel()
					}
					completedMu.Unlock()

					// Check descendants
					checkAndSkipDescendants(nodeName)
					continue
				}
				state.mu.Lock()

				state.inProgress = true
				state.startTime = time.Now()
				state.mu.Unlock()

				// Log start event
				wo.logNodeEvent(nodeName, EventNodeStarted, state, "")

				wg.Add(1)

				// Acquire worker slot
				workerSemaphore <- struct{}{}

				go func(name string) {
					defer wg.Done()
					defer func() { <-workerSemaphore }() // Release worker slot

					// Check if context was cancelled before executing
					select {
					case <-ctx.Done():
						state := nodeStates[name]
						state.mu.Lock()
						state.inProgress = false
						state.mu.Unlock()
						return
					default:
					}

					node := wo.Dag.Nodes[name]

					// Execute the node
					if err := wo.executeNodeWithContext(ctx, name); err != nil {
						state := nodeStates[name]
						state.mu.Lock()
						state.endTime = time.Now()
						state.error = err
						state.inProgress = false

						// Check if this is a when condition not met (should be treated as skip)
						if _, isWhenConditionNotMet := err.(*WhenConditionNotMetError); isWhenConditionNotMet {
							state.skipped = true
							state.completed = true
							state.mu.Unlock()

							// Log skip event (due to when condition)
							wo.logNodeEvent(name, EventNodeSkipped, state, "when condition not met")

							completedMu.Lock()
							completedCount++
							if completedCount == totalNodes {
								cancel() // Signal completion
							}
							completedMu.Unlock()

							// Check which descendants should be skipped
							checkAndSkipDescendants(name)

							return
						}

						// Check if this node has ContinueOnError
						if node.ContinueOnError {
							state.failedContinue = true
							state.completed = true
							state.mu.Unlock()

							// Log failed-continue event
							wo.logNodeEvent(name, EventNodeFailedContinue, state, "")

							completedMu.Lock()
							completedCount++
							if completedCount == totalNodes {
								cancel() // Signal completion
							}
							completedMu.Unlock()

							// Check which descendants should be skipped
							checkAndSkipDescendants(name)

							return
						} else {
							// Fatal error - stop entire workflow
							state.failed = true
							state.completed = true
							state.mu.Unlock()

							// Log failed event
							wo.logNodeEvent(name, EventNodeFailed, state, "")

							errorMu.Lock()
							if executionError == nil {
								executionError = err
								cancel() // Cancel context to stop all workers
							}
							errorMu.Unlock()

							completedMu.Lock()
							completedCount++
							completedMu.Unlock()
							return
						}
					}

					state := nodeStates[name]
					state.mu.Lock()
					state.endTime = time.Now()
					state.completed = true
					state.inProgress = false
					state.mu.Unlock()

					// Log completion event
					wo.logNodeEvent(name, EventNodeCompleted, state, "")

					// Update dependents and add newly ready nodes to queue
					for _, dependent := range dependents[name] {
						depState := nodeStates[dependent]
						depState.mu.Lock()
						depState.remainingDeps--

						if depState.remainingDeps == 0 && !depState.inProgress && !depState.completed && !depState.skipped {
							select {
							case readyQueue <- dependent:
							case <-ctx.Done():
								depState.mu.Unlock()
								return
							}
						}
						depState.mu.Unlock()
					}

					// Check if all nodes are completed
					completedMu.Lock()
					completedCount++
					if completedCount == totalNodes {
						cancel() // Signal completion
					}
					completedMu.Unlock()
				}(nodeName)
			}
		}
	}()

	// Wait for completion (either success or failure)
	<-ctx.Done()

	// Close ready queue and wait for all workers to finish
	close(readyQueue)
	wg.Wait()

	// Stop display updater and show final status
	close(displayDone)
	if wo.logger != nil {
		wo.logger.OnStatusUpdate(wo.createStateSnapshot(nodeStates), dagSnapshot)
	}

	duration := time.Since(start)

	// Notify logger of workflow completion
	if wo.logger != nil {
		completedMu.Lock()
		finalCompletedCount := completedCount
		completedMu.Unlock()

		wo.logger.OnWorkflowComplete(WorkflowEventData{
			Event:          EventWorkflowCompleted,
			StartTime:      start,
			EndTime:        time.Now(),
			Duration:       duration,
			Error:          executionError,
			TotalNodes:     totalNodes,
			CompletedNodes: finalCompletedCount,
		})
		wo.logger.Close()
	}

	if executionError != nil {
		return executionError
	}

	return nil
}

func replaceParams(input string, params map[string]string) string {
	// Check for references like $1, $2, etc.
	result := input
	for param, value := range params {
		result = strings.ReplaceAll(result, "$"+param, value)
	}
	return result
}

func (wo *Orchestrator) executeNodeWithContext(ctx context.Context, nodeName string) error {
	node, exists := wo.Dag.Nodes[nodeName]
	if !exists {
		return fmt.Errorf("node '%s' not found in DAG", nodeName)
	}

	params := map[string]string{}
	for i, v := range wo.Dag.Params {
		params[fmt.Sprintf("%d", i+1)] = v
	}

	if node.When != nil {
		predicate := replaceParams(node.When.Predicate, params)
		expected := replaceParams(node.When.Expected, params)
		cmd := exec.CommandContext(ctx, "sh", "-c", predicate)
		output, err := cmd.Output()

		// Check if context was cancelled
		if ctx.Err() != nil {
			return fmt.Errorf("node '%s' cancelled", node.Name)
		}

		actual := strings.TrimSpace(string(output))
		if err != nil || actual != expected {
			return &WhenConditionNotMetError{
				Message: fmt.Sprintf("when condition not met: expected '%s', got '%s'", expected, actual),
			}
		}
	}

	// Check preconditions
	for _, precondition := range node.Preconditions {
		predicate := replaceParams(precondition.Predicate, params)
		expected := replaceParams(precondition.Expected, params)
		cmd := exec.CommandContext(ctx, "sh", "-c", predicate)
		output, err := cmd.Output()

		// Check if context was cancelled
		if ctx.Err() != nil {
			return fmt.Errorf("node '%s' cancelled", node.Name)
		}

		actual := strings.TrimSpace(string(output))
		if err != nil || actual != expected {
			return fmt.Errorf("precondition failed: expected '%s', got '%s'", expected, actual)
		}
	}

	replacedCommand := replaceParams(node.Command, params)

	// Replace parameters in args if args are provided
	var replacedArgs []string
	if len(node.Args) > 0 {
		replacedArgs = make([]string, len(node.Args))
		for i, arg := range node.Args {
			replacedArgs[i] = replaceParams(arg, params)
		}
	}

	// Retry policies
	retries := 0
	if node.RetryPolicy != nil {
		retries = node.RetryPolicy.Limit
	}

	for attempt := 0; attempt <= retries; attempt++ {
		// Check if context was cancelled before attempting
		if ctx.Err() != nil {
			return fmt.Errorf("cancelled")
		}

		// Execute command directly with args if args are provided, otherwise use sh -c for backward compatibility
		var cmd *exec.Cmd
		if len(node.Args) > 0 {
			// Execute command directly with args (not via sh -c)
			cmd = exec.CommandContext(ctx, replacedCommand, replacedArgs...)
		} else {
			// Use sh -c for backward compatibility when no args are provided
			cmd = exec.CommandContext(ctx, "sh", "-c", replacedCommand)
		}
		cmd.Stdout = nil
		cmd.Stderr = nil

		if err := cmd.Run(); err != nil {
			// Check if error was due to context cancellation
			if ctx.Err() != nil {
				return fmt.Errorf("cancelled")
			}

			if attempt == retries {
				return fmt.Errorf("failed after %d attempts: %w", retries+1, err)
			}
			continue
		}

		// Successful execution
		return nil
	}

	return nil
}

func (wo *Orchestrator) executeNode(nodeName string) error {
	return wo.executeNodeWithContext(context.Background(), nodeName)
}

