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
