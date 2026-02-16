// Package workflow provides the Workflow struct which encapsulates the DAG and Orchestrator for managing and executing data processing pipelines. It allows users to load a DAG configuration from a YAML file, set a job to be processed, and orchestrate the execution of the workflow.
package workflow

import (
	"context"
	"fmt"

	"processing_pipeline/dag"
	"processing_pipeline/orchestrator"
)

type Workflow struct {
	Orchestrator *orchestrator.Orchestrator
	Job          any
}

func NewWorkflow(filename string, parentCtx context.Context) (*Workflow, error) {
	dag, err := dag.LoadDAGFromYAML(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	orchestrator := orchestrator.NewOrchestrator(dag, parentCtx)

	return &Workflow{
		Orchestrator: orchestrator,
		Job:          nil,
	}, nil
}

func (w *Workflow) SetJob(job any) {
	w.Job = job
}

func (w *Workflow) Execute() error {
	return w.Orchestrator.Execute()
}
