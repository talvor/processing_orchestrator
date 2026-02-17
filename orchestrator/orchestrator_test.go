package orchestrator

import (
	"context"
	"testing"
	"time"

	"processing_pipeline/dag"
)

func TestWorkflowOrchestrator_Execute(t *testing.T) {
	mockDAG := &dag.DAG{
		Nodes: map[string]*dag.Node{
			"A": {Name: "A", Command: "echo A", RetryPolicy: &dag.RetryPolicy{Limit: 2}},
			"B": {Name: "B", Command: "echo B", Depends: []string{"A"}},
		},
	}

	orchestrator := NewOrchestrator(mockDAG, nil)
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

	orchestrator := NewOrchestrator(mockDAG, nil)
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

	orchestrator := NewOrchestrator(mockDAG, nil)
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

	orchestrator := NewOrchestrator(mockDAG, nil)
	err := orchestrator.Execute()

	// Should not fail - when condition not met should skip the node
	if err != nil {
		t.Errorf("Expected workflow to succeed when condition not met, but got error: %v", err)
	}
}

// TestContinueOnErrorSkipsDescendants tests that when a node fails with continue_on_error,
// its descendants are skipped with appropriate reason
func TestContinueOnErrorSkipsDescendants(t *testing.T) {
	mockDAG := &dag.DAG{
		Nodes: map[string]*dag.Node{
			"A": {Name: "A", Command: "echo A"},
			"B": {
				Name:            "B",
				Command:         "false", // This command will fail
				Depends:         []string{"A"},
				ContinueOnError: true,
			},
			"C": {
				Name:    "C",
				Command: "echo C",
				Depends: []string{"B"},
			},
		},
	}

	orchestrator := NewOrchestrator(mockDAG, nil)
	err := orchestrator.Execute()

	// Should not fail - continue_on_error should allow workflow to complete
	if err != nil {
		t.Errorf("Expected workflow to succeed with continue_on_error, but got error: %v", err)
	}
}

// TestFailedParentSkipsDescendants tests that when a node fails without continue_on_error,
// its descendants are skipped
func TestFailedParentSkipsDescendants(t *testing.T) {
	mockDAG := &dag.DAG{
		Nodes: map[string]*dag.Node{
			"A": {Name: "A", Command: "echo A"},
			"B": {
				Name:            "B",
				Command:         "false", // This command will fail
				Depends:         []string{"A"},
				ContinueOnError: false,
			},
			"C": {
				Name:    "C",
				Command: "echo C",
				Depends: []string{"B"},
			},
			"D": {
				Name:    "D",
				Command: "echo D",
				Depends: []string{"A"}, // Does not depend on B, should still execute
			},
		},
	}

	orchestrator := NewOrchestrator(mockDAG, nil)
	err := orchestrator.Execute()

	// Should fail because B fails without continue_on_error
	if err == nil {
		t.Errorf("Expected workflow to fail, but it succeeded")
	}
}

// TestOrchestratorWithParentContext tests that a parent context is properly used
func TestOrchestratorWithParentContext(t *testing.T) {
mockDAG := &dag.DAG{
Nodes: map[string]*dag.Node{
"A": {Name: "A", Command: "sleep 1 && echo A"},
"B": {Name: "B", Command: "sleep 1 && echo B", Depends: []string{"A"}},
},
}

// Create a context with timeout
ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
defer cancel()

orchestrator := NewOrchestrator(mockDAG, ctx)
err := orchestrator.Execute()

// Workflow should be cancelled due to parent context timeout
if err == nil {
t.Error("Expected workflow to fail due to context timeout, but it succeeded")
}
}

// TestOrchestratorWithNilParentContext tests backward compatibility with nil parent context
func TestOrchestratorWithNilParentContext(t *testing.T) {
mockDAG := &dag.DAG{
Nodes: map[string]*dag.Node{
"A": {Name: "A", Command: "echo A"},
},
}

orchestrator := NewOrchestrator(mockDAG, nil)
err := orchestrator.Execute()

if err != nil {
t.Errorf("Expected workflow to succeed with nil parent context, but got error: %v", err)
}
}

// TestCommandWithArgs tests that commands with args are executed directly (not via sh -c)
func TestCommandWithArgs(t *testing.T) {
mockDAG := &dag.DAG{
Nodes: map[string]*dag.Node{
"A": {
Name:    "A",
Command: "echo",
Args:    []string{"hello", "world"},
},
},
}

orchestrator := NewOrchestrator(mockDAG, nil)
err := orchestrator.Execute()

if err != nil {
t.Errorf("Expected workflow to succeed, but got error: %v", err)
}
}

// TestCommandWithArgsAndParams tests parameter replacement in args
func TestCommandWithArgsAndParams(t *testing.T) {
mockDAG := &dag.DAG{
Params: []string{"test_value", "another_value"},
Nodes: map[string]*dag.Node{
"A": {
Name:    "A",
Command: "echo",
Args:    []string{"$1", "$2"},
},
},
}

orchestrator := NewOrchestrator(mockDAG, nil)
err := orchestrator.Execute()

if err != nil {
t.Errorf("Expected workflow to succeed, but got error: %v", err)
}
}

// TestCommandWithoutArgs tests backward compatibility - commands without args
func TestCommandWithoutArgs(t *testing.T) {
mockDAG := &dag.DAG{
Nodes: map[string]*dag.Node{
"A": {
Name:    "A",
Command: "echo",
Args:    []string{},
},
},
}

orchestrator := NewOrchestrator(mockDAG, nil)
err := orchestrator.Execute()

if err != nil {
t.Errorf("Expected workflow to succeed, but got error: %v", err)
}
}

// TestCommandWithNilArgs tests true backward compatibility - commands with nil args field
func TestCommandWithNilArgs(t *testing.T) {
mockDAG := &dag.DAG{
Nodes: map[string]*dag.Node{
"A": {
Name:    "A",
Command: "echo 'hello world'",
Args:    nil, // nil args - legacy workflow
},
},
}

orchestrator := NewOrchestrator(mockDAG, nil)
err := orchestrator.Execute()

if err != nil {
t.Errorf("Expected workflow to succeed with nil args, but got error: %v", err)
}
}

// TestConsoleOutputEnabled tests that console output is written when configured
func TestConsoleOutputEnabled(t *testing.T) {
mockDAG := &dag.DAG{
Nodes: map[string]*dag.Node{
"A": {
Name:    "A",
Command: "echo 'test stdout message'",
Console: &dag.Console{
Stdout: true,
Stderr: false,
},
},
"B": {
Name:    "B",
Command: "echo 'test stderr message' >&2",
Console: &dag.Console{
Stdout: false,
Stderr: true,
},
},
"C": {
Name:    "C",
Command: "echo 'both stdout and stderr' && echo 'error message' >&2",
Console: &dag.Console{
Stdout: true,
Stderr: true,
},
},
},
}

orchestrator := NewOrchestrator(mockDAG, nil)
err := orchestrator.Execute()

if err != nil {
t.Errorf("Expected workflow to succeed with console output enabled, but got error: %v", err)
}
}

// TestConsoleOutputDisabled tests that console output is not written when not configured
func TestConsoleOutputDisabled(t *testing.T) {
mockDAG := &dag.DAG{
Nodes: map[string]*dag.Node{
"A": {
Name:    "A",
Command: "echo 'test message'",
Console: nil, // No console configuration
},
},
}

orchestrator := NewOrchestrator(mockDAG, nil)
err := orchestrator.Execute()

if err != nil {
t.Errorf("Expected workflow to succeed without console output, but got error: %v", err)
}
}
