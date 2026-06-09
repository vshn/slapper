package cmd

import (
	"fmt"

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
			return runConvert(opts, args)
		},
	}
	flag := cmd.PersistentFlags()
	flag.StringVarP(&opts.stdlibPath, "stdlib-path", "p", "", "Path to a local stdlib")
	flag.BoolVar(&opts.noStdlib, "no-stdlib", false, "Disable the stdlib, for debugging only")
	flag.StringVarP(&opts.stdlibCacheDir, "stdlib-cache-dir", "d", stdlib.DefaultCacheDir(), "Default path for the stdlib cache")
	flag.StringVarP(&opts.output, "output", "o", "xpkg", "Path where the Crossplane package should be written")
	return cmd
}

func runConvert(opts *convertOpts, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no file path provided")
	}

	if opts.noStdlib && opts.stdlibPath != "" {
		return fmt.Errorf("--stdlib-path and --no-stdlib are mutually exclusive")
	}

	c := converter.ServiceBundleConverter{}
	c.OutputDir = opts.output

	if err := c.LoadBundle(args[0]); err != nil {
		return err
	}

	return c.Convert()
}
