// cmd/s99deploy-site/main.go

// s99deploy-site serves the s99deploy binary, its checksum and its install script.
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/smegg99/s99deploy/internal/site"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(site.Main(ctx, os.Stdout, os.Stderr, os.Args[1:]))
}
