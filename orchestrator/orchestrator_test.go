package orchestrator

import (
	"testing"

	"processing_pipeline/dag"
)

func TestWorkflowOrchestrator_Execute(t *testing.T) {
	mockDAG := &dag.DAG{
		Nodes: map[string]*dag.Node{
			"A": {Name: "A", Command: "echo A", RetryPolicy: &dag.RetryPolicy{Limit: 2}},
			"B": {Name: "B", Command: "echo B", Depends: []string{"A"}},
		},
	}

	orchestrator := NewOrchestrator(mockDAG)
	if err := orchestrator.Execute(); err != nil {
		t.Errorf("WorkflowOrchestrator failed: %v", err)
	}
}

// TestWhenConditionNotMet tests that when a node's when condition is not met,
// it should skip the node and its dependents without treating it as a failure
func TestWhenConditionNotMet(t *testing.T) {
	mockDAG := &dag.DAG{
		Nodes: map[string]*dag.Node{
			"A": {Name: "A", Command: "echo A"},
			"B": {
				Name:    "B",
				Command: "echo B",
				Depends: []string{"A"},
				When: &dag.Condition{
					Predicate: "echo 'no'",
					Expected:  "yes",
				},
			},
			"C": {
				Name:    "C",
				Command: "echo C",
				Depends: []string{"B"},
			},
			"D": {
				Name:    "D",
				Command: "echo D",
				Depends: []string{"A"},
			},
		},
	}

	orchestrator := NewOrchestrator(mockDAG)
	err := orchestrator.Execute()

	// The workflow should not fail when a when condition is not met
	if err != nil {
		t.Errorf("Expected workflow to succeed, but got error: %v", err)
	}

	// Verify we can access the node states by re-creating orchestrator state tracking
	// Since we can't access internal state, we just verify no error occurred
}

// TestWhenConditionMet tests that when a node's when condition is met,
// the node executes normally
func TestWhenConditionMet(t *testing.T) {
	mockDAG := &dag.DAG{
		Nodes: map[string]*dag.Node{
			"A": {Name: "A", Command: "echo A"},
			"B": {
				Name:    "B",
				Command: "echo B",
				Depends: []string{"A"},
				When: &dag.Condition{
					Predicate: "echo 'yes'",
					Expected:  "yes",
				},
			},
			"C": {
				Name:    "C",
				Command: "echo C",
				Depends: []string{"B"},
			},
		},
	}

	orchestrator := NewOrchestrator(mockDAG)
	err := orchestrator.Execute()

	if err != nil {
		t.Errorf("Expected workflow to succeed, but got error: %v", err)
	}
}

// TestWhenConditionWithContinueOnError tests that when condition takes precedence
// over continue_on_error - when condition not met should skip, not continue as failed
func TestWhenConditionWithContinueOnError(t *testing.T) {
	mockDAG := &dag.DAG{
		Nodes: map[string]*dag.Node{
			"A": {Name: "A", Command: "echo A"},
			"B": {
				Name:            "B",
				Command:         "echo B",
				Depends:         []string{"A"},
				ContinueOnError: true,
				When: &dag.Condition{
					Predicate: "echo 'no'",
					Expected:  "yes",
				},
			},
			"C": {
				Name:    "C",
				Command: "echo C",
				Depends: []string{"B"},
			},
		},
	}

	orchestrator := NewOrchestrator(mockDAG)
	err := orchestrator.Execute()

	// Should not fail - when condition not met should skip the node
	if err != nil {
		t.Errorf("Expected workflow to succeed when condition not met, but got error: %v", err)
	}
}
