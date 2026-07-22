// main.go

// s99deploy: one deploy CLI for every same-shape app on the box. Each app
// declares itself in a deploy.json manifest; s99deploy owns install, deploy,
// run, and uninstall around that manifest.
package main

import (
	"os"

	"github.com/smegg99/s99logger"
	"github.com/spf13/cobra"

	"github.com/smegg99/s99deploy/deploy"
)

func main() {
	s99logger.SetDefault(s99logger.New(
		s99logger.NewConsoleSink(os.Stderr),
		s99logger.Options{Service: "s99deploy"},
	))

	if err := rootCmd().Execute(); err != nil {
		s99logger.Error(s99logger.NewEvent("failed", s99logger.Err(err)))
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "s99deploy",
		Short: "Deploy manifest-carrying apps to /opt under systemd",
		Long: "One deploy CLI for every same-shape app on the box. Each app repo\n" +
			"carries a deploy.json manifest; s99deploy owns install, deploy, run,\n" +
			"and uninstall around that manifest.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(&cobra.Command{
		Use:     "install <git-url>",
		Short:   "One-time app setup: clone, user, systemd unit",
		Example: "  sudo s99deploy install git@github.com:smegg99/MyApp.git",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return deploy.Install(args[0])
		},
	})

	root.AddCommand(&cobra.Command{
		Use:     "up <name>",
		Short:   "Deploy: pull, build, restart, health check",
		Example: "  sudo s99deploy up myapp",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return deploy.Up(args[0])
		},
	})

	root.AddCommand(&cobra.Command{
		Use:     "run <root>",
		Short:   "Exec the app from its manifest (systemd ExecStart)",
		Example: "  /usr/local/bin/s99deploy run /opt/myapp  (what the unit's ExecStart calls)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return deploy.Run(args[0])
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
			return deploy.Uninstall(args[0], purge)
		},
	}
	uninstall.Flags().Bool("purge", false, "also remove /opt/<name> and the service user")
	root.AddCommand(uninstall)

	return root
}
