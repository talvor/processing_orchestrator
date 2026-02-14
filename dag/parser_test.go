package dag

import (
	"testing"
)

func TestLoadDAGWithEnv(t *testing.T) {
	filePath := "../dag/examples/complex.yaml"
	dag, err := LoadDAGFromYAML(filePath)
	if err != nil {
		t.Fatalf("Failed to load DAG: %v", err)
	}

	if len(dag.Env) == 0 {
		t.Errorf("DAG environment variables are not loaded")
	}

	for key, value := range dag.Env {
		t.Logf("Env: %s=%s", key, value)
	}
}

func TestLoadDAGWithParam(t *testing.T) {
	filePath := "../dag/examples/complex.yaml"
	dag, err := LoadDAGFromYAML(filePath)
	if err != nil {
		t.Fatalf("Failed to load DAG: %v", err)
	}

	if len(dag.Params) == 0 {
		t.Errorf("DAG parameters are not loaded")
	}

	t.Logf("Params: %v", dag.Params)
}
