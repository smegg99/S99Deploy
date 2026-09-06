// internal/site/cmd.go

package site

import (
	"context"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/smegg99/s99logger"
	"github.com/spf13/cobra"

	"github.com/smegg99/s99deploy"
	"github.com/smegg99/s99deploy/internal/exit"
)

// siteOptions is the site's flag set and its streams.
type siteOptions struct {
	configPath string
	out        io.Writer
	errOut     io.Writer
}

// Main is the process entrypoint: the same three exit codes the CLI uses.
func Main(ctx context.Context, out, errOut io.Writer, args []string) int {
	opts := &siteOptions{out: out, errOut: errOut}
	root := NewSiteRootCmd(opts)
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs(args)

	return exit.Run(ctx, root, errOut, exit.Texts{
		Name:        "s99deploy-site",
		Interrupted: "s99deploy-site: interrupted",
	})
}

// NewSiteRootCmd builds the site's command tree.
func NewSiteRootCmd(opts *siteOptions) *cobra.Command {
	root := &cobra.Command{
		Use:           "s99deploy-site",
		Short:         "Serve the s99deploy binary, its checksum and its install script",
		Version:       s99deploy.Version(),
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE:          func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	root.SetVersionTemplate("s99deploy-site {{.Version}}\n")
	root.AddCommand(newServeCmd(opts))
	return root
}

// newServeCmd listens until the context is cancelled.
func newServeCmd(opts *siteOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Listen on the configured address until interrupted",
		Args:  cobra.NoArgs,
		RunE: exit.Work(func(cmd *cobra.Command, _ []string) error {
			cfg, err := LoadConfig(opts.configPath)
			if err != nil {
				return err
			}
			gin.SetMode(cfg.GinMode)

			server, err := New(cfg, s99deploy.Version())
			if err != nil {
				return err
			}
			log := s99logger.New(s99logger.NewConsoleSink(opts.errOut), s99logger.Options{
				Service: "s99deploy-site", MinLevel: s99logger.LevelInfo,
			})
			log.Info(s99logger.NewEvent(EventListening, s99logger.String("addr", cfg.ListenAddr)))
			err = server.Serve(cmd.Context(), cfg.ListenAddr)
			log.Info(s99logger.NewEvent(EventStopping))
			return err
		}),
	}
	cmd.Flags().StringVar(&opts.configPath, "config", "config.json", "path to the site config")
	return cmd
}

// The site's log event ids, which are catalog keys like the deploy flow's.
const (
	EventListening = "logs.listening"
	EventStopping  = "logs.stopping"
)
