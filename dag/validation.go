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

	// Check that all `after` references exist
	for _, node := range d.Nodes {
		for _, afterStep := range node.After {
			if _, exists := d.Nodes[afterStep]; !exists {
				return errors.New("after reference \"" + afterStep + "\" does not exist for node \"" + node.Name + "\"")
			}
		}
	}

	// Check that each node has either Command or Script, but not both
	for _, node := range d.Nodes {
		hasCommand := node.Command != ""
		hasScript := node.Script != ""

		if node.Noop {
			if hasCommand || hasScript {
				return errors.New("node \"" + node.Name + "\" is a noop step and cannot have 'command' or 'script' specified")
			}
			continue
		}

		if !hasCommand && !hasScript {
			return errors.New("node \"" + node.Name + "\" must have either 'command' or 'script' specified")
		}

		if hasCommand && hasScript {
			return errors.New("node \"" + node.Name + "\" cannot have both 'command' and 'script' specified")
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

