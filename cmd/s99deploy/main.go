// cmd/s99deploy/main.go

// s99deploy deploys manifest-carrying apps to /opt under systemd.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/smegg99/s99deploy/internal/cli"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(cli.Main(ctx, os.Stdin, os.Stdout, os.Stderr, os.Args[1:]))
}
