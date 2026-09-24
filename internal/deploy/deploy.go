// internal/deploy/deploy.go

// Package deploy installs, deploys and removes manifest-carrying apps under systemd.
package deploy

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"

	"github.com/smegg99/s99logger"
)

// namePattern is the manifest's own name rule: the unit, the account and /opt/<name>.
var namePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// ValidateName rejects a service name the manifest would not accept, before it reaches a path.
func ValidateName(name string) error {
	if !namePattern.MatchString(name) {
		return fmt.Errorf("invalid service name %q", name)
	}
	return nil
}

// Config is where things live and who does the privileged work.
type Config struct {
	OptDir   string
	UnitDir  string
	GoBinDir string
	SelfPath string
	// SystemUnitDirs are the other places systemd finds units, checked so a new
	// app cannot shadow one of them. Empty in tests, which own their UnitDir.
	SystemUnitDirs []string
	// Euid gates the privileged flows. OwnerUID and OwnerGID are who install
	// hands the root-owned parts to: the effective ids on a box, so a
	// setuid-root binary does not hand them to the invoking user, and the
	// test's own ids where a chown to root would be refused.
	Euid     int
	OwnerUID int
	OwnerGID int
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

// Root is /opt/<name>, the one directory this tool owns per app.
func (d *Deployer) Root(name string) string { return filepath.Join(d.cfg.OptDir, name) }

// accountHome is the account's writable home inside the root-owned app root.
// The unit sets the same path through Environment=HOME.
func accountHome(root string) string { return filepath.Join(root, "home") }

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
