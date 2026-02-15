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

// Step represents a single node in the DAG, corresponding to a step in the workflow. It includes the command to execute,
type Step struct {
	Name            string       `yaml:"name"`                        // Unique name of the step
	Description     string       `yaml:"description,omitempty"`       // Description of this step
	Command         string       `yaml:"command"`                     // Command to run for the step
	Args            []string     `yaml:"args,omitempty"`              // Arguments for the command
	Depends         []string     `yaml:"depends"`                     // Steps this step depends on
	Preconditions   []Condition  `yaml:"preconditions,omitempty"`     // Preconditions for this step
	When            *Condition   `yaml:"when,omitempty"`              // Optional condition to determine if the step should be executed
	RetryPolicy     *RetryPolicy `yaml:"retry_policy,omitempty"`      // Optional retry policy for the step
	ContinueOnError bool         `yaml:"continue_on_error,omitempty"` // Whether to continue execution if this step fails
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
			Command:         step.Command,
			Args:            step.Args,
			Depends:         step.Depends,
			Description:     step.Description,
			Preconditions:   step.Preconditions,
			When:            step.When,
			RetryPolicy:     step.RetryPolicy,
			ContinueOnError: step.ContinueOnError,
		}
		dag.Nodes[step.Name] = node
		for _, dep := range step.Depends {
			dag.Edges[dep] = append(dag.Edges[dep], step.Name)
		}
	}

	return dag, nil
}
