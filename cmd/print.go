package cmd

import (
	"fmt"
	"os"

	"processing_pipeline/dag"

	"github.com/spf13/cobra"
)

// printCmd represents the graph command
var printCmd = &cobra.Command{
	Use:   "print",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("Usage: graph <workflow-config-file>")
			os.Exit(1)
		}

		d, err := dag.LoadDAGFromYAML(args[0])
		if err != nil {
			fmt.Printf("Failed to create dag: %v\n", err)
			os.Exit(1)
		}

		fmt.Print(d.OutputGraph())
	},
}

func init() {
	rootCmd.AddCommand(printCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// graphCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// graphCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
