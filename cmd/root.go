// Package cmd contains the cobra boilerplate for build the CLI
package cmd

import (
	"context"
	"log/slog"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	var verbosity int
	cmd := &cobra.Command{
		Use:   "slap",
		Short: "Slapper slaps you service into AppCat form!",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			slog.SetLogLoggerLevel(slog.Level(verbosity))
		},
	}
	cmd.PersistentFlags().IntVarP(&verbosity, "verbosity", "v", 0, "Verbosity level for logging. Lower values produce more detailed logs. Default is 0 (info). See https://pkg.go.dev/log/slog#Level for thresholds.")
	cmd.AddCommand(newConvertCmd())
	return cmd
}

func Execute() {
	root := newRootCmd()
	// If the first non-flag arg is not a registered subcommand, default to
	// `convert` so that `slap mybundle.yaml` is equivalent to
	// `slap convert mybundle.yaml`. Any cobra subcommands added later are
	// matched here via root.Find before the fallback kicks in.
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		if cmd, _, err := root.Find(os.Args[1:]); err != nil || cmd == root {
			root.SetArgs(append([]string{"convert"}, os.Args[1:]...))
		}
	}
	if err := root.ExecuteContext(context.Background()); err != nil {
		slog.Error("encountered an error", "error", err)
		os.Exit(1)
	}
}
