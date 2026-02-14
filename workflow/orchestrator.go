package workflow

import (
	"fmt"
	"os/exec"
	"time"
)

type WorkflowOrchestrator struct {
	Config *WorkflowConfig
}

func NewWorkflowOrchestrator(config *WorkflowConfig) *WorkflowOrchestrator {
	return &WorkflowOrchestrator{Config: config}
}

func (wo *WorkflowOrchestrator) Execute() error {
	start := time.Now()

	for _, s := range wo.Config.Steps {
		// step, ok := s.(Step)
		// if !ok {
		// 	fmt.Println("Error: Invalid step definition, skipping.")
		// 	continue
		// }
		executeStep(s)
	}

	duration := time.Since(start)
	fmt.Printf("Workflow completed in %s\n", duration)
	return nil
}

func executeStep(step Step) {
	stepStart := time.Now()
	fmt.Printf("Executing step: %s\n", step.Name)
	cmd := exec.Command("sh", "-c", step.Command)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Run(); err != nil {
		fmt.Printf("Error executing step '%s': %v\n", step.Name, err)
	}

	stepDuration := time.Since(stepStart)
	fmt.Printf("Step '%s' completed in %s\n", step.Name, stepDuration)
}
