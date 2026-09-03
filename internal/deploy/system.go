// internal/deploy/system.go

package deploy

import (
	"context"
	"errors"
)

// ErrNoAccount is what Lookup returns for a name no account exists for.
var ErrNoAccount = errors.New("no such account")

// Account is one system account as passwd sees it.
type Account struct {
	Name, Home string
	UID, GID   uint32
	Groups     []uint32
}

// AsUser is the identity, home and working directory a command runs under.
type AsUser struct {
	Name, Home, Dir string
	UID, GID        uint32
	Groups          []uint32
}

// UnitState is the part of `systemctl show` this tool decides on.
type UnitState struct{ Active, Sub, Result, InvocationID string }

// Runner is every external process the flow starts; a nil env inherits.
type Runner interface {
	Run(ctx context.Context, env []string, name string, args ...string) error
	Output(ctx context.Context, env []string, name string, args ...string) (string, error)
	RunAsUser(ctx context.Context, a AsUser, command string, env []string) error
	OutputAsUser(ctx context.Context, a AsUser, command string, env []string) (string, error)
}

// Accounts is the passwd side: everything that could delete a home directory.
type Accounts interface {
	Lookup(name string) (Account, error)
	Create(ctx context.Context, name, home string) error
	Delete(ctx context.Context, name string) error
}

// Units is systemd.
type Units interface {
	Reload(ctx context.Context) error
	Enable(ctx context.Context, name string) error
	Disable(ctx context.Context, name string) error
	Restart(ctx context.Context, name string) error
	State(ctx context.Context, name string) (UnitState, error)
}

// Prober is the post-deploy health check.
type Prober interface {
	Probe(ctx context.Context, url string) error
}
