// internal/cli/main.go

package cli

import (
	"context"
	"fmt"
	"io"

	"github.com/smegg99/s99logger"

	"github.com/smegg99/s99deploy/internal/deploy"
	"github.com/smegg99/s99deploy/internal/exit"
	"github.com/smegg99/s99deploy/internal/messages"
)

// Main is the process entrypoint: the live box, the real streams.
func Main(ctx context.Context, in io.Reader, out, errOut io.Writer, args []string) int {
	lang, langErr := languageFrom(args)
	opts := newRootOptions(lang, in, out, errOut)
	if langErr != nil {
		// The tree is built anyway, so the usage block is in the language the
		// environment asked for rather than in none.
		fmt.Fprintln(errOut, "s99deploy:", langErr)
		fmt.Fprint(errOut, NewRootCmd(opts).UsageString())
		return exit.Usage
	}
	return run(ctx, opts, args)
}

// newRootOptions wires the live deployer and the logger the domain writes to.
func newRootOptions(lang string, in io.Reader, out, errOut io.Writer) *rootOptions {
	words := messages.Localizer(lang)
	log := s99logger.New(s99logger.NewConsoleSink(errOut), s99logger.Options{
		Service:    "s99deploy",
		MinLevel:   s99logger.LevelInfo,
		Language:   lang,
		Translator: messages.Translator(),
	})
	s99logger.SetDefault(log)

	cfg := deploy.LiveConfig()
	cfg.Log = log
	return &rootOptions{
		lang: lang, words: words, deployer: deploy.New(cfg),
		in: in, out: out, errOut: errOut,
	}
}

// run executes the tree opts describes and returns the process exit code.
func run(ctx context.Context, opts *rootOptions, args []string) int {
	if lang, err := languageFrom(args); err == nil && lang != opts.lang {
		opts.lang, opts.words = lang, messages.Localizer(lang)
	} else if err != nil {
		root := NewRootCmd(opts)
		fmt.Fprintln(opts.errOut, "s99deploy:", err)
		fmt.Fprint(opts.errOut, root.UsageString())
		return exit.Usage
	}

	root := NewRootCmd(opts)
	root.SetArgs(args)

	return exit.Run(ctx, root, opts.errOut, exit.Texts{
		Name: "s99deploy",
		Interrupted: messages.CliCommandInterrupted(opts.words,
			messages.CliCommandInterruptedParams{Name: "s99deploy"}),
	})
}
