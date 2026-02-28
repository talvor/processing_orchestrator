package dag

import (
	"maps"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Condition represents a condition that must be met before a step is executed.
type Condition struct {
	Predicate string `yaml:"predicate"` // The condition to evaluate
	Expected  string `yaml:"expected"`  // The expected outcome of the condition
}

// RetryPolicy represents the retry policy for a step, defining how many times a step should be retried upon failure.“
type RetryPolicy struct {
	Limit int `yaml:"limit"` // Maximum number of retries
}

// Console represents the console output configuration for a step
type Console struct {
	Stdout bool `yaml:"stdout"` // Whether to write stdout to console
	Stderr bool `yaml:"stderr"` // Whether to write stderr to console
}

// Output represents the variable output configuration for a step
type Output struct {
	Stdout string `yaml:"stdout"` // Variable name to store stdout
	Stderr string `yaml:"stderr"` // Variable name to store stderr
}

// Step represents a single node in the DAG, corresponding to a step in the workflow. It includes the command to execute,
type Step struct {
	Name            string       `yaml:"name"`                        // Unique name of the step
	Description     string       `yaml:"description,omitempty"`       // Description of this step
	Noop            bool         `yaml:"noop,omitempty"`              // Whether this is a no-op step (no command or script required)
	Command         string       `yaml:"command,omitempty"`           // Command to run for the step
	Script          string       `yaml:"script,omitempty"`            // Script to run for the step (multiline YAML string)
	Args            []string     `yaml:"args,omitempty"`              // Arguments for the command
	Depends         []string     `yaml:"depends"`                     // Steps this step depends on
	After           []string     `yaml:"after,omitempty"`             // Steps whose entire subtree must complete before this step runs
	Preconditions   []Condition  `yaml:"preconditions,omitempty"`     // Preconditions for this step
	When            *Condition   `yaml:"when,omitempty"`              // Optional condition to determine if the step should be executed
	RetryPolicy     *RetryPolicy `yaml:"retry_policy,omitempty"`      // Optional retry policy for the step
	ContinueOnError bool         `yaml:"continue_on_error,omitempty"` // Whether to continue execution if this step fails
	Console         *Console     `yaml:"console,omitempty"`           // Optional console output configuration for the step
	Output          *Output      `yaml:"output,omitempty"`            // Optional variable output configuration for the step
}

// DAGConfig represents the structure of the YAML file
// Corresponds to the "minimal.yaml" and "complex.yaml" examples.
type DAGConfig struct {
	Name   string            `yaml:"name"`
	Env    map[string]string `yaml:"env"`
	Params string            `yaml:"params"`
	Steps  []Step            `yaml:"steps"`
}

// LoadDAGFromYAML reads a YAML file and converts it to a DAG object.
func LoadDAGFromYAML(filePath string) (*DAG, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config DAGConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	dag := NewDAG(config.Name)
	dag.Env = make(map[string]string)
	maps.Copy(dag.Env, config.Env)

	dag.Params = strings.Split(config.Params, " ")

	for _, step := range config.Steps {
		node := &Node{
			Name:            step.Name,
			Noop:            step.Noop,
			Command:         step.Command,
			Script:          step.Script,
			Args:            step.Args,
			Depends:         step.Depends,
			After:           step.After,
			Description:     step.Description,
			Preconditions:   step.Preconditions,
			When:            step.When,
			RetryPolicy:     step.RetryPolicy,
			ContinueOnError: step.ContinueOnError,
			Console:         step.Console,
			Output:          step.Output,
		}
		dag.Nodes[step.Name] = node
		for _, dep := range step.Depends {
			dag.Edges[dep] = append(dag.Edges[dep], step.Name)
		}
	}

	// Second pass: resolve `after` fields.
	// For each step with `after: [X]`, find all descendants of X (X plus all nodes
	// that transitively depend on X) and add them as explicit dependencies so that
	// the step runs only after the entire subtree rooted at X has completed.
	for _, step := range config.Steps {
		if len(step.After) == 0 {
			continue
		}

		node := dag.Nodes[step.Name]

		// Build a set of existing depends to avoid duplicates.
		existingDepends := make(map[string]bool, len(node.Depends))
		for _, dep := range node.Depends {
			existingDepends[dep] = true
		}

		for _, afterStep := range step.After {
			for _, desc := range dag.descendants(afterStep) {
				if !existingDepends[desc] && desc != step.Name {
					existingDepends[desc] = true
					node.Depends = append(node.Depends, desc)
					dag.Edges[desc] = append(dag.Edges[desc], step.Name)
				}
			}
		}
	}

	return dag, nil
}
