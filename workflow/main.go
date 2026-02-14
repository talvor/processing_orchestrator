package workflow

import (
	"fmt"

	"processing_orchestrator/dag"
)

type Workflow struct {
	Config       *WorkflowConfig
	Dag          *dag.DAG
	Orchestrator *WorkflowOrchestrator
	Job          any
}

func NewWorkflow(filename string) (*Workflow, error) {
	// config, err := LoadConfig(filename)
	dag, err := dag.LoadDAGFromYAML(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	orchestrator := NewWorkflowOrchestrator(dag)

	return &Workflow{
		// Config:       config,
		Dag:          dag,
		Orchestrator: orchestrator,
		Job:          nil,
	}, nil
}

func (w *Workflow) SetJob(job any) {
	w.Job = job
}
