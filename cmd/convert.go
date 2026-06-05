package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/vshn/slapper/pkg/converter"
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
	if len(args) == 0 {
		return fmt.Errorf("no file path provided")
	}

	c := converter.ServiceBundleConverter{}

	if err := c.LoadBundle(args[0]); err != nil {
		return err
	}

	return c.Convert()
}
