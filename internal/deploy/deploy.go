// internal/deploy/deploy.go

// Package deploy installs, deploys and removes manifest-carrying apps under systemd.
package deploy

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/smegg99/s99logger"
)

// Config is where things live and who does the privileged work.
type Config struct {
	OptDir   string
	UnitDir  string
	GoBinDir string
	SelfPath string
	Euid     int
	Runner   Runner
	Accounts Accounts
	Units    Units
	Prober   Prober
	Progress Progress
	Log      *s99logger.Logger
}

// Deployer runs the flows against one Config.
type Deployer struct{ cfg Config }

// New returns a Deployer over cfg, filling in the two optional fields.
func New(cfg Config) *Deployer {
	if cfg.Progress == nil {
		cfg.Progress = noProgress{}
	}
	if cfg.Log == nil {
		cfg.Log = s99logger.New(discardSink{}, s99logger.Options{})
	}
	return &Deployer{cfg: cfg}
}

// root is /opt/<name>, the one directory this tool owns per app.
func (d *Deployer) root(name string) string { return filepath.Join(d.cfg.OptDir, name) }

// unitPath is where the generated unit for name lives.
func (d *Deployer) unitPath(name string) string {
	return filepath.Join(d.cfg.UnitDir, name+".service")
}

// requireRoot refuses the privileged flows before they touch anything.
func (d *Deployer) requireRoot() error {
	if d.cfg.Euid != 0 {
		return fmt.Errorf("must run as root: sudo s99deploy ...")
	}
	return nil
}

type discardSink struct{}

func (discardSink) Write(context.Context, s99logger.Record) error { return nil }
