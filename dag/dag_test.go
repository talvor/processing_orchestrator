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

// TestValidateNoCommandOrScript tests that a node without command or script is valid (inferred as noop)
func TestValidateNoCommandOrScript(t *testing.T) {
	dag := NewDAG("test")
	dag.Nodes["A"] = &Node{
		Name: "A",
		// No Command or Script — treated as a noop step
	}

	err := dag.Validate()
	if err != nil {
		t.Errorf("Expected node without command or script to pass validation as noop, got: %v", err)
	}
}

// TestValidateNoopStep tests that a node with no command or script passes validation.
func TestValidateNoopStep(t *testing.T) {
	dag := NewDAG("test")
	dag.Nodes["A"] = &Node{
		Name: "A",
	}

	err := dag.Validate()
	if err != nil {
		t.Errorf("Expected noop node to pass validation, got: %v", err)
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

// TestValidateAfterInvalidReference tests that an `after` field referencing a non-existent step fails validation
func TestValidateAfterInvalidReference(t *testing.T) {
	dag := NewDAG("test")
	dag.Nodes["A"] = &Node{
		Name:    "A",
		Command: "echo hello",
	}
	dag.Nodes["B"] = &Node{
		Name:    "B",
		Command: "echo world",
		After:   []string{"NonExistent"},
	}

	err := dag.Validate()
	if err == nil {
		t.Errorf("Expected validation to fail for node with invalid after reference")
	}
}

// TestLoadDAGWithAfter tests that the after field is loaded and resolved correctly.
// In the after.yaml example:
//   - start has dependents: process_a, side_task
//   - process_a has dependent: process_b
//   - cleanup uses `after: [start]` meaning it should depend on start + ALL of start's
//     descendants (process_a, process_b, side_task).
func TestLoadDAGWithAfter(t *testing.T) {
	dag, err := LoadDAGFromYAML("../dag/examples/after.yaml")
	if err != nil {
		t.Fatalf("Failed to load DAG from YAML: %v", err)
	}

	if err := dag.Validate(); err != nil {
		t.Fatalf("DAG validation failed: %v", err)
	}

	cleanup := dag.Nodes["cleanup"]
	if cleanup == nil {
		t.Fatalf("Expected 'cleanup' node to exist")
	}

	// cleanup.After should preserve the original after declaration
	if len(cleanup.After) != 1 || cleanup.After[0] != "start" {
		t.Errorf("Expected cleanup.After = [start], got %v", cleanup.After)
	}

	// cleanup.Depends should contain start and all its descendants
	expectedDeps := map[string]bool{
		"start":     true,
		"process_a": true,
		"process_b": true,
		"side_task": true,
	}
	if len(cleanup.Depends) != len(expectedDeps) {
		t.Errorf("Expected cleanup to have %d depends, got %d: %v", len(expectedDeps), len(cleanup.Depends), cleanup.Depends)
	}
	for _, dep := range cleanup.Depends {
		if !expectedDeps[dep] {
			t.Errorf("Unexpected dependency %q in cleanup.Depends", dep)
		}
	}
}

// TestAfterDescendants tests the descendants helper directly.
func TestAfterDescendants(t *testing.T) {
	d := NewDAG("test")
	// Build: A -> B -> C, A -> D
	d.Nodes["A"] = &Node{Name: "A", Command: "echo A"}
	d.Nodes["B"] = &Node{Name: "B", Command: "echo B", Depends: []string{"A"}}
	d.Nodes["C"] = &Node{Name: "C", Command: "echo C", Depends: []string{"B"}}
	d.Nodes["D"] = &Node{Name: "D", Command: "echo D", Depends: []string{"A"}}
	d.Edges["A"] = []string{"B", "D"}
	d.Edges["B"] = []string{"C"}

	desc := d.descendants("A")

	// Should contain A, B, C, D
	descSet := make(map[string]bool, len(desc))
	for _, v := range desc {
		descSet[v] = true
	}
	for _, expected := range []string{"A", "B", "C", "D"} {
		if !descSet[expected] {
			t.Errorf("Expected %q in descendants of A, got %v", expected, desc)
		}
	}
	if len(desc) != 4 {
		t.Errorf("Expected 4 descendants, got %d: %v", len(desc), desc)
	}
}

// TestLoadDAGWithNoop tests that a noop step is loaded and validated correctly from YAML,
// and that steps using `after` on a noop step get the correct dependencies resolved.
func TestLoadDAGWithNoop(t *testing.T) {
	dag, err := LoadDAGFromYAML("../dag/examples/noop.yaml")
	if err != nil {
		t.Fatalf("Failed to load noop DAG from YAML: %v", err)
	}

	if err := dag.Validate(); err != nil {
		t.Fatalf("Noop DAG validation failed: %v", err)
	}

	processing := dag.Nodes["processing"]
	if processing == nil {
		t.Fatalf("Expected 'processing' noop node to exist")
	}

	if processing.Command != "" || processing.Script != "" {
		t.Errorf("Expected 'processing' node to have no command or script (inferred noop)")
	}

	// load_data uses `after: [processing]`, so it should depend on processing
	// and all of processing's descendants (nodes that depend on it). Since no
	// other step depends on processing, the only dependency added is processing itself.
	loadData := dag.Nodes["load_data"]
	if loadData == nil {
		t.Fatalf("Expected 'load_data' node to exist")
	}

	expectedDeps := map[string]bool{
		"processing": true,
	}
	if len(loadData.Depends) != len(expectedDeps) {
		t.Errorf("Expected load_data to have %d depends, got %d: %v", len(expectedDeps), len(loadData.Depends), loadData.Depends)
	}
	for _, dep := range loadData.Depends {
		if !expectedDeps[dep] {
			t.Errorf("Unexpected dependency %q in load_data.Depends", dep)
		}
	}
}

