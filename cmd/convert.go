package cmd

import (
	"github.com/spf13/cobra"
)

func init() {
	RootCmd.AddCommand(convertCmd)
}

var convertCmd = &cobra.Command{
	Use:   "convert mybundle.yaml",
	Short: "Converts a bundle into a Crossplane package",
	Long:  "Converts a bundle into a Crossplane package. It uses a VSHN provided stdlib of functions and function inputs.",
	RunE:  convert,
}

func convert(cmd *cobra.Command, args []string) error {
	return nil
}
