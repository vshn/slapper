// cmd contains the cobra boilerplate for build the CLI
package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "slap",
	Short: "Slapper slaps you service into form!",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
	},
}

func Execute() {
	if err := RootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Println(err)
	}
}
