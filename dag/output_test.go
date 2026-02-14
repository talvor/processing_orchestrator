package dag

import (
	"testing"
)

func TestOutputGraph(t *testing.T) {
	examplePaths := []string{
		"../dag/examples/minimal.yaml",
		"../dag/examples/complex.yaml",
	}

	for _, examplePath := range examplePaths {
		dag, err := LoadDAGFromYAML(examplePath)
		if err != nil {
			t.Fatalf("Failed to load DAG: %v", err)
		}

		// Validate the DAG
		if err = dag.Validate(); err != nil {
			t.Fatalf("Validation failed: %v", err)
		}

		// Output the hierarchical graph
		output := dag.OutputGraph()
		if output == "" {
			t.Error("OutputGraph produced an empty string")
		}

		t.Log(output)

	}
}

func TestOutputMetadata(t *testing.T) {
	examplePaths := []string{
		"../dag/examples/minimal.yaml",
		"../dag/examples/complex.yaml",
	}

	for _, examplePath := range examplePaths {
		dag, err := LoadDAGFromYAML(examplePath)
		if err != nil {
			t.Fatalf("Failed to load DAG: %v", err)
		}

		// Validate the DAG
		if err = dag.Validate(); err != nil {
			t.Fatalf("Validation failed: %v", err)
		}

		// Output the metadata
		metadataOutput := dag.OutputMetadata()
		if metadataOutput == "" {
			t.Error("OutputMetadata produced an empty string")
		}
		t.Log(metadataOutput)
	}
}
