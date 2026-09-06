// internal/cli/main.go

package cli

import (
	"context"
	"io"

	"github.com/smegg99/s99logger"

	"github.com/smegg99/s99deploy/internal/deploy"
	"github.com/smegg99/s99deploy/internal/exit"
)

// Main is the process entrypoint: the live box, the real streams.
func Main(ctx context.Context, in io.Reader, out, errOut io.Writer, args []string) int {
	return run(ctx, newRootOptions(in, out, errOut), args)
}

// newRootOptions wires the live deployer and the logger the domain writes to.
func newRootOptions(in io.Reader, out, errOut io.Writer) *rootOptions {
	log := s99logger.New(s99logger.NewConsoleSink(errOut), s99logger.Options{
		Service: "s99deploy", MinLevel: s99logger.LevelInfo,
	})
	s99logger.SetDefault(log)

	cfg := deploy.LiveConfig()
	cfg.Log = log
	return &rootOptions{deployer: deploy.New(cfg), in: in, out: out, errOut: errOut}
}

// run executes the tree opts describes and returns the process exit code.
func run(ctx context.Context, opts *rootOptions, args []string) int {
	root := NewRootCmd(opts)
	root.SetIn(opts.in)
	root.SetOut(opts.out)
	root.SetErr(opts.errOut)
	root.SetArgs(args)

	return exit.Run(ctx, root, opts.errOut, exit.Texts{
		Name:        "s99deploy",
		Interrupted: "s99deploy: interrupted",
	})
}
