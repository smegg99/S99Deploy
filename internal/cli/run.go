// internal/cli/run.go

package cli

import (
	"github.com/spf13/cobra"

	"github.com/smegg99/s99deploy/internal/exit"
)

// newRunCmd is what the unit's ExecStart calls; it execs the app and never returns.
func newRunCmd(opts *rootOptions) *cobra.Command {
	return &cobra.Command{
		Use:     "run <root>",
		Short:   "Exec the app from its manifest (systemd ExecStart)",
		Example: "  /usr/local/bin/s99deploy run /opt/myapp",
		Args:    cobra.ExactArgs(1),
		RunE: exit.Work(func(cmd *cobra.Command, args []string) error {
			return opts.deployer.Run(cmd.Context(), args[0])
		}),
	}
}
