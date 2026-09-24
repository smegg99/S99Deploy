// internal/cli/root.go

// Package cli is the depl command tree.
package cli

import (
	"io"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/smegg99/s99term"
	"github.com/spf13/cobra"

	"github.com/smegg99/s99deploy"
	"github.com/smegg99/s99deploy/internal/deploy"
	"github.com/smegg99/s99deploy/internal/messages"
)

// rootOptions is every persistent flag plus the wiring the subcommands share.
type rootOptions struct {
	lang     string
	color    string
	verbose  bool
	words    *i18n.Localizer
	console  *s99term.Console
	steps    *Stepper
	deployer *deploy.Deployer
	in       io.Reader
	out      io.Writer
	errOut   io.Writer
}

// NewRootCmd builds the command tree over opts.
func NewRootCmd(opts *rootOptions) *cobra.Command {
	root := &cobra.Command{
		Use:     "depl",
		Short:   messages.CliCommandRootShort(opts.words),
		Long:    messages.CliCommandRootLong(opts.words),
		Version: s99deploy.Version(),
		// Stray arguments on the root are a mistake, not a command.
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	root.SetVersionTemplate("depl {{.Version}}\n")

	// The value is already resolved; the flag exists so cobra accepts it and
	// prints it in the help this same language rendered.
	root.PersistentFlags().StringVar(&opts.lang, "lang", opts.lang, messages.CliCommandLangFlag(opts.words))
	root.PersistentFlags().StringVar(&opts.color, "color", "auto", messages.CliCommandColorFlag(opts.words))
	root.PersistentFlags().BoolVar(&opts.verbose, "verbose", false, messages.CliCommandVerboseFlag(opts.words))

	root.AddCommand(
		newInstallCmd(opts),
		newUpCmd(opts),
		newRunCmd(opts),
		newUninstallCmd(opts),
	)

	// Before localize: it adds the completion command, which captures the
	// writer it will generate into rather than looking it up when it runs.
	root.SetIn(opts.in)
	root.SetOut(opts.out)
	root.SetErr(opts.errOut)

	localize(root, opts.words)
	return root
}
