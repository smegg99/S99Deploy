// cmd/s99deploy/main.go

// s99deploy deploys manifest-carrying apps to /opt under systemd.
package main

import (
	"os"

	"github.com/smegg99/s99deploy/internal/cli"
)

func main() { os.Exit(cli.Main()) }
