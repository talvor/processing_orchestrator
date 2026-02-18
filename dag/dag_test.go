package dag

import (
	"testing"
)

func TestLoadAndValidateDAG(t *testing.T) {
	// Path to the example minimal.yaml
	examplePath := "../dag/examples/minimal.yaml"

	dag, err := LoadDAGFromYAML(examplePath)
	if err != nil {
		t.Fatalf("Failed to load DAG from YAML: %v", err)
	}

	t.Logf("Loaded DAG: %v\n", dag)

	// Validate the DAG
	err = dag.Validate()
	if err != nil {
		t.Errorf("DAG validation failed: %v", err)
	}

	t.Log("DAG loaded and validated successfully")
}

// TestValidateMissingCommandAndScript tests that a node without command or script fails validation
func TestValidateMissingCommandAndScript(t *testing.T) {
	dag := NewDAG("test")
	dag.Nodes["A"] = &Node{
		Name: "A",
		// No Command or Script
	}

	err := dag.Validate()
	if err == nil {
		t.Errorf("Expected validation to fail for node without command or script")
	}
}

// TestValidateBothCommandAndScript tests that a node with both command and script fails validation
func TestValidateBothCommandAndScript(t *testing.T) {
	dag := NewDAG("test")
	dag.Nodes["A"] = &Node{
		Name:    "A",
		Command: "echo hello",
		Script:  "echo world",
	}

	err := dag.Validate()
	if err == nil {
		t.Errorf("Expected validation to fail for node with both command and script")
	}
}

// TestValidateScriptOnly tests that a node with only script passes validation
func TestValidateScriptOnly(t *testing.T) {
	dag := NewDAG("test")
	dag.Nodes["A"] = &Node{
		Name:   "A",
		Script: "echo hello",
	}

	err := dag.Validate()
	if err != nil {
		t.Errorf("Expected validation to pass for node with script: %v", err)
	}
}

// TestValidateCommandOnly tests that a node with only command passes validation
func TestValidateCommandOnly(t *testing.T) {
	dag := NewDAG("test")
	dag.Nodes["A"] = &Node{
		Name:    "A",
		Command: "echo hello",
	}

	err := dag.Validate()
	if err != nil {
		t.Errorf("Expected validation to pass for node with command: %v", err)
	}
}

