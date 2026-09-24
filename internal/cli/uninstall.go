// internal/cli/uninstall.go

package cli

import (
	"github.com/spf13/cobra"

	"github.com/smegg99/s99deploy/internal/deploy"
	"github.com/smegg99/s99deploy/internal/exit"
	"github.com/smegg99/s99deploy/internal/messages"
)

// uninstallFlags is uninstall's own flag set.
type uninstallFlags struct{ purge, yes bool }

// newUninstallCmd removes the unit, and with --purge the tree and the account.
func newUninstallCmd(opts *rootOptions) *cobra.Command {
	flags := &uninstallFlags{}
	cmd := &cobra.Command{
		Use:   "uninstall <name>",
		Short: messages.CliCommandUninstallShort(opts.words),
		Example: "  sudo s99deploy uninstall myapp\n" +
			"  sudo s99deploy uninstall --purge myapp",
		Args: cobra.ExactArgs(1),
		RunE: exit.Work(func(cmd *cobra.Command, args []string) error {
			name := args[0]
			// Validated before the prompt, so the typed-name warning never names
			// a path a bad name would have escaped to.
			if err := deploy.ValidateName(name); err != nil {
				return err
			}
			if !flags.yes {
				if err := confirmName(cmd.InOrStdin(), cmd.OutOrStdout(), opts.words,
					name, opts.deployer.Root(name), flags.purge); err != nil {
					return err
				}
			}
			return opts.deployer.Uninstall(cmd.Context(), name, flags.purge)
		}),
	}
	cmd.Flags().BoolVar(&flags.purge, "purge", false, messages.CliCommandPurgeFlag(opts.words))
	cmd.Flags().BoolVar(&flags.yes, "yes", false, messages.CliCommandYesFlag(opts.words))
	return cmd
}
