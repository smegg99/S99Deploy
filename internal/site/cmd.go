// internal/site/cmd.go

package site

import (
	"context"
	"io"

	"github.com/gin-gonic/gin"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/smegg99/s99logger"
	"github.com/spf13/cobra"

	"github.com/smegg99/s99deploy"
	"github.com/smegg99/s99deploy/internal/exit"
	"github.com/smegg99/s99deploy/internal/messages"
)

// siteOptions is the site's flag set, its language and the stream its log goes to.
type siteOptions struct {
	configPath string
	lang       string
	words      *i18n.Localizer
	errOut     io.Writer
}

// Main is the process entrypoint: the same three exit codes the CLI uses.
func Main(ctx context.Context, out, errOut io.Writer, args []string) int {
	lang := messages.LanguageFromEnv()
	opts := &siteOptions{lang: lang, words: messages.Localizer(lang), errOut: errOut}
	root := NewSiteRootCmd(opts)
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs(args)

	return exit.Run(ctx, root, errOut, exit.Texts{
		Name: "s99deploy-site",
		Interrupted: messages.CliCommandInterrupted(opts.words,
			messages.CliCommandInterruptedParams{Name: "s99deploy-site"}),
	})
}

// NewSiteRootCmd builds the site's command tree.
func NewSiteRootCmd(opts *siteOptions) *cobra.Command {
	root := &cobra.Command{
		Use:           "s99deploy-site",
		Short:         messages.CliCommandSiteShort(opts.words),
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
		Short: messages.CliCommandServeShort(opts.words),
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
				Service:    "s99deploy-site",
				MinLevel:   s99logger.LevelInfo,
				Language:   opts.lang,
				Translator: messages.Translator(),
			})
			log.Info(s99logger.NewEvent(messages.KeyLogsListening, s99logger.String("addr", cfg.ListenAddr)))
			err = server.Serve(cmd.Context(), cfg.ListenAddr)
			log.Info(s99logger.NewEvent(messages.KeyLogsStopping))
			return err
		}),
	}
	cmd.Flags().StringVar(&opts.configPath, "config", "config.json",
		messages.CliCommandConfigFlag(opts.words))
	return cmd
}
