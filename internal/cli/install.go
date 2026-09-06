// internal/cli/install.go

package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/smegg99/s99deploy/internal/exit"
)

// newInstallCmd is the one-time setup: clone, account, /opt layout, unit.
func newInstallCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "install <git-url>",
		Short:   "One-time app setup: clone, account, systemd unit",
		Example: "  sudo --preserve-env=SSH_AUTH_SOCK s99deploy install https://example.com/myapp.git",
		Args:    cobra.ExactArgs(1),
		RunE: exit.Work(func(cmd *cobra.Command, args []string) error {
			installed, err := opts.deployer.Install(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nNext:\n  1. fill in %s/.env\n  2. sudo s99deploy up %s\n",
				installed.Root, installed.Name)
			return nil
		}),
	}
}
