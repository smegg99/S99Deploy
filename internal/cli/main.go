// internal/cli/main.go

package cli

import (
	"context"
	"errors"
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
	color, _ := flagFromArgs(args, "--color")
	verbose := boolFromArgs(args, "--verbose")

	// install writes this binary's own path into every unit's ExecStart, so a
	// box that cannot report it gets an error instead of an assumed path.
	cfg, selfErr := deploy.LiveConfig()
	if selfErr != nil {
		fmt.Fprintln(errOut, "depl:", selfErr)
		return exit.Error
	}

	opts, colorErr := newRootOptions(cfg, lang, color, verbose, in, out, errOut)
	if opts.console != nil {
		defer opts.console.Close()
	}
	if err := errors.Join(langErr, colorErr); err != nil {
		// The tree is built anyway, so the usage block is in the language the
		// environment asked for rather than in none.
		fmt.Fprintln(errOut, "depl:", err)
		fmt.Fprint(errOut, NewRootCmd(opts).UsageString())
		return exit.Usage
	}
	return run(ctx, opts, args)
}

// newRootOptions wires the one console, the logger that writes through it, and the live deployer.
func newRootOptions(cfg deploy.Config, lang, color string, verbose bool, in io.Reader, out, errOut io.Writer) (*rootOptions, error) {
	words := messages.Localizer(lang)
	profile, err := profileFor(color, words)
	if err != nil {
		return &rootOptions{lang: lang, words: words, in: in, out: out, errOut: errOut}, err
	}

	// The zero value of MinLevel is LevelDebug, so the quiet default has to be
	// written out or every run logs everything.
	level := s99logger.LevelInfo
	if verbose {
		level = s99logger.LevelDebug
	}

	console := newConsole(errOut, profile, words)
	log := s99logger.New(console.Sink(), s99logger.Options{
		Service:    "depl",
		MinLevel:   level,
		Language:   lang,
		Translator: messages.Translator(),
	})
	s99logger.SetDefault(log)

	steps := NewStepper(console, words)
	cfg.Log = log
	cfg.Progress = steps
	return &rootOptions{
		lang: lang, color: color, verbose: verbose, words: words,
		console: console, steps: steps, deployer: deploy.New(cfg),
		in: in, out: out, errOut: errOut,
	}, nil
}

// run executes the tree opts describes and returns the process exit code.
func run(ctx context.Context, opts *rootOptions, args []string) int {
	if lang, err := languageFrom(args); err == nil && lang != opts.lang {
		opts.lang, opts.words = lang, messages.Localizer(lang)
	} else if err != nil {
		root := NewRootCmd(opts)
		fmt.Fprintln(opts.errOut, "depl:", err)
		fmt.Fprint(opts.errOut, root.UsageString())
		return exit.Usage
	}

	root := NewRootCmd(opts)
	root.SetArgs(args)

	return exit.Run(ctx, root, opts.errOut, exit.Texts{
		Name: "depl",
		Interrupted: messages.CliCommandInterrupted(opts.words,
			messages.CliCommandInterruptedParams{Name: "depl"}),
	})
}
