// internal/cli/root.go

// Package cli is the s99deploy command tree.
package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/smegg99/s99logger"
	"github.com/spf13/cobra"

	"github.com/smegg99/s99deploy"
	"github.com/smegg99/s99deploy/internal/deploy"
)

// Main runs the command tree and returns the process exit code.
func Main(ctx context.Context) int {
	// The domain logs through Config.Log, which New defaults to a discard sink, so the CLI has to hand it the real one or every event disappears.
	log := s99logger.New(s99logger.NewConsoleSink(os.Stderr), s99logger.Options{Service: "s99deploy"})
	s99logger.SetDefault(log)
	cfg := deploy.LiveConfig()
	cfg.Log = log

	if err := NewRootCmd(deploy.New(cfg)).ExecuteContext(ctx); err != nil {
		log.Error(s99logger.NewEvent("failed", s99logger.Err(err)))
		return 1
	}
	return 0
}

// NewRootCmd builds the command tree.
func NewRootCmd(d *deploy.Deployer) *cobra.Command {
	root := &cobra.Command{
		Use:     "s99deploy",
		Version: s99deploy.Version(),
		Short:   "Deploy manifest-carrying apps to /opt under systemd",
		Long: "One deploy CLI for every same-shape app on the box. Each app repo\n" +
			"carries a deploy.json manifest; s99deploy owns install, deploy, run,\n" +
			"and uninstall around that manifest.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.SetVersionTemplate("s99deploy {{.Version}}\n")

	root.AddCommand(&cobra.Command{
		Use:     "install <git-url>",
		Short:   "One-time app setup: clone, user, systemd unit",
		Example: "  sudo s99deploy install git@github.com:smegg99/MyApp.git",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			installed, err := d.Install(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\nNext:\n  1. fill in %s/.env\n  2. sudo s99deploy up %s\n", installed.Root, installed.Name)
			return nil
		},
	})

	root.AddCommand(&cobra.Command{
		Use:     "up <name>",
		Short:   "Deploy: pull, build, restart, health check",
		Example: "  sudo s99deploy up myapp",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return d.Up(cmd.Context(), args[0], 0)
		},
	})

	root.AddCommand(&cobra.Command{
		Use:     "run <root>",
		Short:   "Exec the app from its manifest (systemd ExecStart)",
		Example: "  /usr/local/bin/s99deploy run /opt/myapp  (what the unit's ExecStart calls)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return d.Run(cmd.Context(), args[0])
		},
	})

	uninstall := &cobra.Command{
		Use:   "uninstall <name>",
		Short: "Remove the unit; --purge also removes /opt/<name> and the user",
		Example: "  sudo s99deploy uninstall myapp\n" +
			"  sudo s99deploy uninstall --purge myapp",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			purge, err := cmd.Flags().GetBool("purge")
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "Type %s to confirm: ", args[0])
			answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
			if err != nil {
				return err
			}
			if strings.TrimSpace(answer) != args[0] {
				return fmt.Errorf("confirmation did not match")
			}
			return d.Uninstall(cmd.Context(), args[0], purge)
		},
	}
	uninstall.Flags().Bool("purge", false, "also remove /opt/<name> and the service user")
	root.AddCommand(uninstall)

	return root
}
