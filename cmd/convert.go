package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/vshn/slapper/pkg/converter"
	"github.com/vshn/slapper/pkg/converter/stdlib"
)

type convertOpts struct {
	stdlibPath     string
	noStdlib       bool
	stdlibCacheDir string
	output         string
}

func newConvertCmd() *cobra.Command {
	opts := &convertOpts{}
	cmd := &cobra.Command{
		Use:   "convert mybundle.yaml",
		Short: "Converts a bundle into a Crossplane package",
		Long:  "Converts a bundle into a Crossplane package. It uses a VSHN provided stdlib of functions and function inputs.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConvert(cmd.Context(), opts, args)
		},
	}
	flag := cmd.PersistentFlags()
	flag.StringVarP(&opts.stdlibPath, "stdlib-path", "p", "", "Path to a local stdlib")
	flag.BoolVar(&opts.noStdlib, "no-stdlib", false, "Disable the stdlib, for debugging only")
	flag.StringVarP(&opts.stdlibCacheDir, "stdlib-cache-dir", "d", stdlib.DefaultCacheDir(), "Default path for the stdlib cache")
	flag.StringVarP(&opts.output, "output", "o", "xpkg", "Path where the Crossplane package should be written")
	return cmd
}

func runConvert(ctx context.Context, opts *convertOpts, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no file path provided")
	}

	if opts.noStdlib && opts.stdlibPath != "" {
		return fmt.Errorf("--stdlib-path and --no-stdlib are mutually exclusive")
	}

	if opts.stdlibPath != "" {
		if _, err := os.Stat(opts.stdlibPath); err != nil {
			return fmt.Errorf("stdlib path: %w", err)
		}
	}

	c := converter.ServiceBundleConverter{}
	c.OutputDir = opts.output

	if err := c.LoadBundle(args[0]); err != nil {
		return err
	}

	c.StdlibSource = resolveStdlibSource(opts, c.Meta().Stdlib)

	return c.Convert(ctx)
}

// resolveStdlibSource picks the stdlib source per CLI flags + bundle meta.
// Precedence: --no-stdlib > --stdlib-path > meta.stdlib > none.
// Warns when --no-stdlib suppresses a bundle-declared stdlib so the maintainer
// notices that the emitted package omits the Configuration meta.
func resolveStdlibSource(opts *convertOpts, metaStdlib string) stdlib.Source {
	switch {
	case opts.noStdlib:
		if metaStdlib != "" {
			slog.Warn("--no-stdlib set; ignoring meta.stdlib from bundle", "stdlib", metaStdlib)
		}
		return nil
	case opts.stdlibPath != "":
		return stdlib.LocalSource(opts.stdlibPath)
	case metaStdlib != "":
		return stdlib.OCISource{Ref: metaStdlib, CacheDir: opts.stdlibCacheDir}
	default:
		return nil
	}
}
