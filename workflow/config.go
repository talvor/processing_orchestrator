package workflow

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Step represents a step in the workflow
type Step struct {
	Name    string   `yaml:"name,omitempty"`
	Command string   `yaml:"command,omitempty"`
	Inputs  []string `yaml:"inputs,omitempty"`
	Outputs []string `yaml:"outputs,omitempty"`
}

// WorkflowConfig defines the workflow configuration
type WorkflowConfig struct {
	Steps []Step `yaml:"steps"`
}

// LoadConfig loads and parses the YAML configuration file
func LoadConfig(filename string) (*WorkflowConfig, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	var config WorkflowConfig
	decoder := yaml.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("error decoding YAML: %w", err)
	}

	return &config, nil
}
