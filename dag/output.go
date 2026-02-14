package dag

import (
	"bytes"
	"fmt"
)

// OutputMetadata returns a string representation of the DAG's metadata, including its name and parameters.
func (d *DAG) OutputMetadata() string {
	var buffer bytes.Buffer

	buffer.WriteString("DAG Metadata:\n")

	buffer.WriteString(fmt.Sprintf("- Name: %s\n", d.Name))
	buffer.WriteString(fmt.Sprintf("- Params: %v\n", d.Params))

	return buffer.String()
}

// OutputGraph returns a hierarchical representation of the DAG with descriptions.
func (d *DAG) OutputGraph() string {
	var buffer bytes.Buffer
	visited := make(map[string]bool)

	buffer.WriteString("DAG Hierarchical Structure:\n")

	// Start from nodes with no dependencies
	for name, node := range d.Nodes {
		if len(node.Depends) == 0 {
			d.writeNodeHierarchy(&buffer, name, 0, visited)
		}
	}

	return buffer.String()
}

// writeNodeHierarchy is a helper function to recursively write the node and its children with descriptions.
func (d *DAG) writeNodeHierarchy(buffer *bytes.Buffer, nodeName string, level int, visited map[string]bool) {
	// Prevent cycles and re-visiting nodes
	if visited[nodeName] {
		return
	}
	visited[nodeName] = true

	// Indent based on the level in the hierarchy
	for range level {
		buffer.WriteString("  ")
	}

	// Write the current node and its description if available
	node := d.Nodes[nodeName]
	fmt.Fprintf(buffer, "- %s", nodeName)
	if node.Description != "" {
		fmt.Fprintf(buffer, " (%s)", node.Description)
	}
	buffer.WriteString("\n")

	// Recurse for each dependent node
	for _, child := range d.Edges[nodeName] {
		d.writeNodeHierarchy(buffer, child, level+1, visited)
	}
}
