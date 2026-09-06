// internal/cli/root.go

// Package cli is the s99deploy command tree.
package cli

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/smegg99/s99deploy"
	"github.com/smegg99/s99deploy/internal/deploy"
)

// rootOptions is every persistent flag plus the wiring the subcommands share.
type rootOptions struct {
	verbose  bool
	deployer *deploy.Deployer
	in       io.Reader
	out      io.Writer
	errOut   io.Writer
}

// NewRootCmd builds the command tree over opts.
func NewRootCmd(opts *rootOptions) *cobra.Command {
	root := &cobra.Command{
		Use:   "s99deploy",
		Short: "Deploy manifest-carrying apps to /opt under systemd",
		Long: "One deploy CLI for every same-shape app on the box. Each app repo\n" +
			"carries a deploy.json manifest; s99deploy owns install, up, run\n" +
			"and uninstall around that manifest.",
		Version: s99deploy.Version(),
		// Stray arguments on the root are a mistake, not a command.
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	root.SetVersionTemplate("s99deploy {{.Version}}\n")
	root.PersistentFlags().BoolVar(&opts.verbose, "verbose", false, "log every step, not just the ones that matter")

	root.AddCommand(
		newInstallCmd(opts),
		newUpCmd(opts),
		newRunCmd(opts),
		newUninstallCmd(opts),
	)
	return root
}
