// internal/cli/up.go

package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/smegg99/s99deploy/internal/exit"
	"github.com/smegg99/s99deploy/internal/messages"
)

// upFlags is up's own flag set, bound to a struct rather than read back by name.
type upFlags struct{ timeout time.Duration }

// newUpCmd is the deploy: pull, build, restart, health check.
func newUpCmd(opts *rootOptions) *cobra.Command {
	flags := &upFlags{}
	cmd := &cobra.Command{
		Use:     "up <name>",
		Short:   messages.CliCommandUpShort(opts.words),
		Example: "  sudo depl up myapp",
		Args:    cobra.ExactArgs(1),
		RunE: exit.Work(func(cmd *cobra.Command, args []string) error {
			return opts.deployer.Up(cmd.Context(), args[0], flags.timeout)
		}),
	}
	cmd.Flags().DurationVar(&flags.timeout, "timeout", 0, messages.CliCommandTimeoutFlag(opts.words))
	return cmd
}
