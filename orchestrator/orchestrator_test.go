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
