// Package dag defines the structures and methods for representing a directed acyclic graph (DAG) of steps in a workflow.
package dag

// Node represents a single step in the DAG.
type Node struct {
	Name            string       // Unique name of the step
	Description     string       // Description of this step
	Noop            bool         // Whether this is a no-op step (no command or script required)
	Command         string       // Command to run for the step
	Script          string       // Script to run for the step (multiline string)
	Args            []string     // Arguments for the command
	Depends         []string     // Steps this step depends on
	After           []string     // Steps whose entire subtree (step + all dependents) must complete before this step runs
	Preconditions   []Condition  // Preconditions for this step
	When            *Condition   // Optional condition to determine if the step should be executed
	RetryPolicy     *RetryPolicy // Optional retry policy for the step
	ContinueOnError bool         // Whether to continue execution if this step fails
	Console         *Console     // Optional console output configuration for the step
	Output          *Output      // Optional variable output configuration for the step
}

// DAG represents the directed acyclic graph structure.
type DAG struct {
	Name   string
	Env    map[string]string
	Params []string
	Nodes  map[string]*Node    // Keyed by node name for quick access
	Edges  map[string][]string // Adjacency list representation of edges
}

// NewDAG initializes an empty DAG.
func NewDAG(name string) *DAG {
	return &DAG{
		Name:  name,
		Nodes: make(map[string]*Node),
		Edges: make(map[string][]string),
	}
}

// AddNode adds a new node to the DAG.
func (d *DAG) AddNode(name string, command string, depends []string) {
	if _, exists := d.Nodes[name]; exists {
		return // Node already exists, skip adding
	}
	node := &Node{
		Name:    name,
		Command: command,
		Depends: depends,
	}
	d.Nodes[name] = node
	for _, dep := range depends {
		d.Edges[dep] = append(d.Edges[dep], name)
	}
}

// GetNodes returns all nodes in the DAG.
func (d *DAG) GetNodes() map[string]*Node {
	return d.Nodes
}

// GetEdges returns all edges in the DAG.
func (d *DAG) GetEdges() map[string][]string {
	return d.Edges
}

// descendants returns all nodes reachable from nodeName via the Edges map
// (i.e., nodeName itself plus all nodes that transitively depend on it).
func (d *DAG) descendants(nodeName string) []string {
	visited := make(map[string]bool)
	var result []string

	var dfs func(string)
	dfs = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		result = append(result, name)
		for _, child := range d.Edges[name] {
			dfs(child)
		}
	}
	dfs(nodeName)
	return result
}
