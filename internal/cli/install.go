// internal/cli/install.go

package cli

import (
	"errors"
	"fmt"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/spf13/cobra"

	"github.com/smegg99/s99deploy/internal/deploy"
	"github.com/smegg99/s99deploy/internal/exit"
	"github.com/smegg99/s99deploy/internal/messages"
)

// newInstallCmd is the one-time setup: clone, account, /opt layout, unit.
func newInstallCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "install <git-url>",
		Short:   messages.CliCommandInstallShort(opts.words),
		Example: "  sudo --preserve-env=SSH_AUTH_SOCK depl install git@github.com:smegg99/MyApp.git",
		Args:    cobra.ExactArgs(1),
		RunE: exit.Work(func(cmd *cobra.Command, args []string) error {
			installed, err := opts.deployer.Install(cmd.Context(), args[0])
			if err != nil {
				return localizeSelfPath(err, opts.words)
			}
			fmt.Fprint(cmd.OutOrStdout(), messages.CliCommandNextSteps(opts.words,
				messages.CliCommandNextStepsParams{Root: installed.Root, Name: installed.Name}))
			return nil
		}),
	}
}

// localizeSelfPath replaces the deploy layer's own words for a binary the unit
// could not exec with the catalog sentence, which names the remedy too.
func localizeSelfPath(err error, words *i18n.Localizer) error {
	var bad *deploy.SelfPathError
	if !errors.As(err, &bad) {
		return err
	}
	if bad.Blocked == "" {
		return errors.New(messages.CliCommandSelfPathRelative(words,
			messages.CliCommandSelfPathRelativeParams{Path: bad.Path}))
	}
	return errors.New(messages.CliCommandSelfPathProtected(words,
		messages.CliCommandSelfPathProtectedParams{Path: bad.Path, Blocked: bad.Blocked}))
}
