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

type Orchestrator struct {
	Dag         *dag.DAG
	WorkerCount int // Maximum number of concurrent workers
}

// nodeState tracks the execution state of each node
type nodeState struct {
	completed     bool
	inProgress    bool
	skipped       bool // Node was skipped due to parent failure
	remainingDeps int
	mu            sync.Mutex
}

// ANSI color codes
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorGray   = "\033[90m"
)

// Status symbols
const (
	symbolSuccess        = "✓" // Green checkmark
	symbolFailed         = "✗" // Red X
	symbolFailedContinue = "⚠" // Yellow warning
	symbolSkipped        = "○" // Gray circle
)

func NewOrchestrator(dag *dag.DAG) *Orchestrator {
	return &Orchestrator{
		Dag:         dag,
		WorkerCount: 5, // Default worker count
	}
}

// NewOrchestratorWithWorkers creates an orchestrator with a specific worker count
func NewOrchestratorWithWorkers(dag *dag.DAG, workerCount int) *Orchestrator {
	return &Orchestrator{
		Dag:         dag,
		WorkerCount: workerCount,
	}
}

func (wo *Orchestrator) Execute() error {
	start := time.Now()

	// Initialize node states and dependency tracking
	nodeStates := make(map[string]*nodeState)
	dependents := make(map[string][]string) // Maps node -> nodes that depend on it

	for name, node := range wo.Dag.Nodes {
		nodeStates[name] = &nodeState{
			completed:     false,
			inProgress:    false,
			skipped:       false,
			remainingDeps: len(node.Depends),
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
	ctx, cancel := context.WithCancel(context.Background())
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

	// Helper function to skip a node and all its descendants
	var skipDescendants func(string)
	skipDescendants = func(nodeName string) {
		for _, dependent := range dependents[nodeName] {
			depState := nodeStates[dependent]
			depState.mu.Lock()

			if depState.completed || depState.skipped {
				depState.mu.Unlock()
				continue
			}

			depState.skipped = true
			depState.completed = true
			depState.mu.Unlock()

			fmt.Printf("%s%s%s %s (0s)\n", colorGray, symbolSkipped, colorReset, dependent)

			completedMu.Lock()
			completedCount++
			if completedCount == totalNodes {
				cancel() // Signal completion
			}
			completedMu.Unlock()

			// Recursively skip descendants
			skipDescendants(dependent)
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

				state.inProgress = true
				state.mu.Unlock()

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
					nodeStart := time.Now()

					// Execute the node
					if err := wo.executeNodeWithContext(ctx, name); err != nil {
						nodeDuration := time.Since(nodeStart)

						// Check if this node has ContinueOnError
						if node.ContinueOnError {
							fmt.Printf("%s%s%s %s (%s) - %v\n", colorYellow, symbolFailedContinue, colorReset, name, nodeDuration, err)

							// Mark this node as completed (with error)
							state := nodeStates[name]
							state.mu.Lock()
							state.completed = true
							state.inProgress = false
							state.mu.Unlock()

							completedMu.Lock()
							completedCount++
							if completedCount == totalNodes {
								cancel() // Signal completion
							}
							completedMu.Unlock()

							// Skip all descendants
							skipDescendants(name)

							return
						} else {
							// Fatal error - stop entire workflow
							fmt.Printf("%s%s%s %s (%s) - %v\n", colorRed, symbolFailed, colorReset, name, nodeDuration, err)

							errorMu.Lock()
							if executionError == nil {
								executionError = err
								cancel() // Cancel context to stop all workers
							}
							errorMu.Unlock()

							// Mark as completed even on error
							state := nodeStates[name]
							state.mu.Lock()
							state.completed = true
							state.inProgress = false
							state.mu.Unlock()

							completedMu.Lock()
							completedCount++
							completedMu.Unlock()
							return
						}
					}

					nodeDuration := time.Since(nodeStart)
					fmt.Printf("%s%s%s %s (%s)\n", colorGreen, symbolSuccess, colorReset, name, nodeDuration)

					// Mark as completed successfully
					state := nodeStates[name]
					state.mu.Lock()
					state.completed = true
					state.inProgress = false
					state.mu.Unlock()

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

	duration := time.Since(start)

	if executionError != nil {
		fmt.Printf("\nWorkflow failed after %s\n", duration)
		return executionError
	}

	fmt.Printf("\nWorkflow completed successfully in %s\n", duration)
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
			return fmt.Errorf("'when' condition failed for node '%s': expected '%s', got '%s'", node.Name, expected, actual)
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
			return fmt.Errorf("'pre' condition failed for node '%s': expected '%s', got '%s'", node.Name, expected, actual)
		}
	}

	replacedCommand := replaceParams(node.Command, params)

	// Retry policies
	retries := 0
	if node.RetryPolicy != nil {
		retries = node.RetryPolicy.Limit
	}

	for attempt := 0; attempt <= retries; attempt++ {
		// Check if context was cancelled before attempting
		if ctx.Err() != nil {
			return fmt.Errorf("node '%s' cancelled", node.Name)
		}

		cmd := exec.CommandContext(ctx, "sh", "-c", replacedCommand)
		cmd.Stdout = nil
		cmd.Stderr = nil

		if err := cmd.Run(); err != nil {
			// Check if error was due to context cancellation
			if ctx.Err() != nil {
				return fmt.Errorf("node '%s' cancelled", node.Name)
			}

			if attempt == retries {
				return fmt.Errorf("node '%s' failed after %d attempts: %w", node.Name, retries+1, err)
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
