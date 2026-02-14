package dag

import (
	"errors"
)

// Validate checks the DAG for correctness.
// It ensures that there are no cycles, all dependencies exist, and the graph is well-formed.
func (d *DAG) Validate() error {
	// Check that all dependencies exist
	for _, node := range d.Nodes {
		for _, dep := range node.Depends {
			if _, exists := d.Nodes[dep]; !exists {
				return errors.New("dependency \"" + dep + "\" does not exist for node \"" + node.Name + "\"")
			}
		}
	}

	// Check for cycles in the DAG
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for nodeName := range d.Nodes {
		if visited[nodeName] {
			continue
		}
		if d.detectCycle(nodeName, visited, recStack) {
			return errors.New("cycle detected in the DAG")
		}
	}

	return nil
}

// detectCycle is a helper function to check for cycles using DFS.
func (d *DAG) detectCycle(nodeName string, visited, recStack map[string]bool) bool {
	visited[nodeName] = true
	recStack[nodeName] = true

	for _, neighbor := range d.Edges[nodeName] {
		if !visited[neighbor] {
			if d.detectCycle(neighbor, visited, recStack) {
				return true
			}
		} else if recStack[neighbor] {
			return true
		}
	}

	recStack[nodeName] = false
	return false
}

