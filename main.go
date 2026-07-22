// main.go

// s99deploy: one deploy CLI for every same-shape app on the box. Each app
// declares itself in a deploy.json manifest; s99deploy owns install, deploy,
// run, and uninstall around that manifest.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/smegg99/s99logger"

	"github.com/smegg99/s99deploy/deploy"
)

const usage = `s99deploy <command>

Commands:
  install <git-url>            one-time app setup: clone, user, systemd unit
  up <name>                    deploy: pull, build, restart, health check
  run <root>                   exec the app from its manifest (systemd ExecStart)
  uninstall [--purge] <name>   remove the unit; --purge also removes /opt/<name> and the user
`

func main() {
	s99logger.SetDefault(s99logger.New(
		s99logger.NewConsoleSink(os.Stderr),
		s99logger.Options{Service: "s99deploy"},
	))

	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "install":
		err = withArg(os.Args[2:], "git-url", deploy.Install)
	case "up":
		err = withArg(os.Args[2:], "name", deploy.Up)
	case "run":
		err = withArg(os.Args[2:], "root", deploy.Run)
	case "uninstall":
		fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
		purge := fs.Bool("purge", false, "also remove /opt/<name> and the service user")
		_ = fs.Parse(os.Args[2:])
		if fs.NArg() != 1 {
			err = fmt.Errorf("usage: s99deploy uninstall [--purge] <name>")
		} else {
			err = deploy.Uninstall(fs.Arg(0), *purge)
		}
	case "help", "-h", "--help":
		fmt.Print(usage)
		return
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		s99logger.Error(s99logger.NewEvent("failed", s99logger.Err(err)))
		os.Exit(1)
	}
}

func withArg(args []string, name string, fn func(string) error) error {
	if len(args) != 1 {
		return fmt.Errorf("expected exactly one <%s> argument", name)
	}
	return fn(args[0])
}
