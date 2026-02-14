/*
Package cmd provides the command-line functionality for the processing orchestrator.
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	"processing_orchestrator/workflow"

	"github.com/spf13/cobra"
)

// Step represents a step in the workflow
type Step struct {
	Name    string   `yaml:"name"`
	Command string   `yaml:"command"`
	Inputs  []string `yaml:"inputs"`
	Outputs []string `yaml:"outputs"`
}

// WorkflowConfig defines the workflow configuration
type WorkflowConfig struct {
	Steps []Step `yaml:"steps"`
}

// processCmd represents the process command
var processCmd = &cobra.Command{
	Use:   "process",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("Usage: process <workflow-config-file>")
			os.Exit(1)
		}

		w, err := workflow.NewWorkflow(args[0])
		if err != nil {
			fmt.Printf("Failed to create workflow: %v\n", err)
			os.Exit(1)
		}

		err = w.Orchestrator.Execute()
		if err != nil {
			fmt.Printf("Workflow execution failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(processCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// processCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// processCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
