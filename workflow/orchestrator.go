package workflow

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"processing_orchestrator/dag"
)

type WorkflowOrchestrator struct {
	Dag *dag.DAG
}

func NewWorkflowOrchestrator(dag *dag.DAG) *WorkflowOrchestrator {
	return &WorkflowOrchestrator{Dag: dag}
}

func (wo *WorkflowOrchestrator) Execute() error {
	completed := make(map[string]bool)
	start := time.Now()

	for name := range wo.Dag.Nodes {
		if err := wo.executeNode(name, completed); err != nil {
			fmt.Printf("Error executing workflow: %v\n", err)
			return err
		}
	}

	duration := time.Since(start)
	fmt.Printf("Workflow completed in %s\n", duration)
	return nil
}

func (wo *WorkflowOrchestrator) executeNode(nodeName string, completed map[string]bool) error {
	if completed[nodeName] {
		return nil // Node already processed
	}

	node, exists := wo.Dag.Nodes[nodeName]
	if !exists {
		return fmt.Errorf("node '%s' not found in DAG", nodeName)
	}

	// Execute dependencies first
	for _, dep := range node.Depends {
		if err := wo.executeNode(dep, completed); err != nil {
			return err
		}
	}

	// Check preconditions
	for _, precondition := range node.Preconditions {
		cmd := exec.Command("sh", "-c", precondition.Condition)
		output, err := cmd.Output()
		actual := strings.TrimSpace(string(output))
		if err != nil || actual != precondition.Expected {
			return fmt.Errorf("Precondition failed for node '%s': expected '%s', got '%s'\n", node.Name, precondition.Expected, actual)
		}
	}

	// Retry policies
	retries := 0
	if node.RetryPolicy != nil {
		retries = node.RetryPolicy.Limit
	}

	for attempt := 0; attempt <= retries; attempt++ {
		stepStart := time.Now()
		fmt.Printf("Attempt %d: Executing node %s\n", attempt+1, node.Name)
		cmd := exec.Command("sh", "-c", node.Command)
		cmd.Stdout = nil
		cmd.Stderr = nil

		if err := cmd.Run(); err != nil {
			fmt.Printf("Error executing step '%s' (attempt %d): %v\n", node.Name, attempt+1, err)
			if attempt == retries {
				return err // Failed after all retry attempts
			}
			continue
		}

		// Successful execution
		stepDuration := time.Since(stepStart)
		fmt.Printf("Step '%s' completed in %s\n", node.Name, stepDuration)
		completed[nodeName] = true
		return nil
	}

	return nil
}
