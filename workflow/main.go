package workflow

import "fmt"

type Workflow struct {
	Config       *WorkflowConfig
	Orchestrator *WorkflowOrchestrator
	Job          any
}

func NewWorkflow(filename string) (*Workflow, error) {
	config, err := LoadConfig(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	orchestrator := NewWorkflowOrchestrator(config)

	return &Workflow{
		Config:       config,
		Orchestrator: orchestrator,
		Job:          nil,
	}, nil
}

func (w *Workflow) SetJob(job any) {
	w.Job = job
}
