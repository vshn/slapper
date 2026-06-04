// Package cmd contains the cobra boilerplate for build the CLI
package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "slap",
	Short: "Slapper slaps you service into AppCat form!",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
	},
}

func Execute() {
	// If the first non-flag arg is not a registered subcommand, default to
	// `convert` so that `slap mybundle.yaml` is equivalent to
	// `slap convert mybundle.yaml`. Any cobra subcommands added later are
	// matched here via RootCmd.Find before the fallback kicks in.
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		if cmd, _, err := RootCmd.Find(os.Args[1:]); err != nil || cmd == RootCmd {
			RootCmd.SetArgs(append([]string{"convert"}, os.Args[1:]...))
		}
	}
	if err := RootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
