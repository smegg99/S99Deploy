// internal/cli/install.go

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/smegg99/s99deploy/internal/exit"
	"github.com/smegg99/s99deploy/internal/messages"
)

// newInstallCmd is the one-time setup: clone, account, /opt layout, unit.
func newInstallCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "install <git-url>",
		Short:   messages.CliCommandInstallShort(opts.words),
		Example: "  sudo --preserve-env=SSH_AUTH_SOCK s99deploy install git@github.com:smegg99/MyApp.git",
		Args:    cobra.ExactArgs(1),
		RunE: exit.Work(func(cmd *cobra.Command, args []string) error {
			installed, err := opts.deployer.Install(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), messages.CliCommandNextSteps(opts.words,
				messages.CliCommandNextStepsParams{Root: installed.Root, Name: installed.Name}))
			return nil
		}),
	}
}
