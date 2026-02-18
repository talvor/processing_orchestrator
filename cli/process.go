package cli

import (
	"context"
	"fmt"
	"os"

	"processing_pipeline/orchestrator"
	"processing_pipeline/workflow"

	"github.com/spf13/cobra"
)

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

		if cmd.Flag("logger").Value.String() == "tree" {
			w.Orchestrator.SetLogger(orchestrator.NewTreeLogger())
		}

		if cmd.Flag("logger").Value.String() == "stream" {
			w.Orchestrator.SetLogger(orchestrator.NewStreamLogger())
		}

		err = w.Orchestrator.Execute(context.Background())
		if err != nil {
			fmt.Printf("Workflow execution failed: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(processCmd)

	// Here you define flags and configuration settings.
	processCmd.Flags().StringP("logger", "l", "noop", "Set the logger to use ('stream', 'tree' or 'noop')")
}
