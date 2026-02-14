package workflow

import (
	"fmt"

	"processing_pipeline/dag"
	"processing_pipeline/orchestrator"
)

type Workflow struct {
	Dag          *dag.DAG
	Orchestrator *orchestrator.Orchestrator
	Job          any
}

func NewWorkflow(filename string) (*Workflow, error) {
	dag, err := dag.LoadDAGFromYAML(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	orchestrator := orchestrator.NewOrchestrator(dag)

	return &Workflow{
		Dag:          dag,
		Orchestrator: orchestrator,
		Job:          nil,
	}, nil
}

func (w *Workflow) SetJob(job any) {
	w.Job = job
}
