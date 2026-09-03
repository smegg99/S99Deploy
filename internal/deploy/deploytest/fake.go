// internal/deploy/deploytest/fake.go

// Package deploytest holds the fakes the deploy flows are tested against.
package deploytest

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/smegg99/s99logger"

	"github.com/smegg99/s99deploy/internal/deploy"
)

// Call is one process the flow asked for and this package did not start.
type Call struct {
	Name    string
	Args    []string
	Env     []string
	AsUser  string
	Command string
}

// Runner records every process the flow asked for and starts none of them.
type Runner struct {
	Calls []Call
	Fail  map[string]error
	On    map[string]func(args []string) error
	Out   map[string]string
}

func (r *Runner) key(name string, args []string) string {
	if len(args) == 0 {
		return name
	}
	return name + " " + args[0]
}

func (r *Runner) Run(_ context.Context, env []string, name string, args ...string) error {
	r.Calls = append(r.Calls, Call{Name: name, Args: args, Env: env})
	key := r.key(name, args)
	if err := r.Fail[key]; err != nil {
		return err
	}
	if effect := r.On[key]; effect != nil {
		return effect(args)
	}
	return nil
}

func (r *Runner) Output(ctx context.Context, env []string, name string, args ...string) (string, error) {
	if err := r.Run(ctx, env, name, args...); err != nil {
		return "", err
	}
	return r.Out[r.key(name, args)], nil
}

func (r *Runner) RunAsUser(_ context.Context, a deploy.AsUser, command string, env []string) error {
	r.Calls = append(r.Calls, Call{AsUser: a.Name, Command: command, Env: env})
	if err := r.Fail[command]; err != nil {
		return err
	}
	if effect := r.On[command]; effect != nil {
		return effect(nil)
	}
	return nil
}

func (r *Runner) OutputAsUser(ctx context.Context, a deploy.AsUser, command string, env []string) (string, error) {
	if err := r.RunAsUser(ctx, a, command, env); err != nil {
		return "", err
	}
	return r.Out[command], nil
}

// Ran reports whether a process was started with this name and these leading args.
func (r *Runner) Ran(name string, args ...string) bool {
	for _, call := range r.Calls {
		if call.Name != name || len(call.Args) < len(args) {
			continue
		}
		match := true
		for i, want := range args {
			if call.Args[i] != want {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// Accounts is the passwd database as a map.
type Accounts struct {
	Users   map[string]deploy.Account
	Created []string
	Deleted []string
}

func NewAccounts() *Accounts { return &Accounts{Users: map[string]deploy.Account{}} }

func (a *Accounts) Lookup(name string) (deploy.Account, error) {
	account, ok := a.Users[name]
	if !ok {
		return deploy.Account{}, deploy.ErrNoAccount
	}
	return account, nil
}

func (a *Accounts) Create(_ context.Context, name, home string) error {
	a.Created = append(a.Created, name)
	// This process's own uid and gid, so the ownership calls under test are real syscalls an unprivileged test can make.
	a.Users[name] = deploy.Account{
		Name: name, Home: home,
		UID: uint32(os.Getuid()), GID: uint32(os.Getgid()),
	}
	return nil
}

func (a *Accounts) Delete(_ context.Context, name string) error {
	a.Deleted = append(a.Deleted, name)
	delete(a.Users, name)
	return nil
}

// Units is systemd as counters and slices.
type Units struct {
	Reloads   int
	Enabled   []string
	Disabled  []string
	Restarts  int
	States    map[string]deploy.UnitState
	OnRestart func(*Units)
	Fail      map[string]error
}

func NewUnits() *Units {
	return &Units{States: map[string]deploy.UnitState{}, Fail: map[string]error{}}
}

func (u *Units) Reload(context.Context) error { u.Reloads++; return u.Fail["reload"] }

func (u *Units) Enable(_ context.Context, name string) error {
	u.Enabled = append(u.Enabled, name)
	return u.Fail["enable"]
}

func (u *Units) Disable(_ context.Context, name string) error {
	u.Disabled = append(u.Disabled, name)
	return u.Fail["disable"]
}

func (u *Units) Restart(_ context.Context, name string) error {
	u.Restarts++
	// OnRestart runs after each restart, so a test can decide what State reports next.
	if u.OnRestart != nil {
		u.OnRestart(u)
	}
	return u.Fail["restart"]
}

func (u *Units) State(_ context.Context, name string) (deploy.UnitState, error) {
	return u.States[name], u.Fail["state"]
}

// Prober fails its first Fail calls, then succeeds.
type Prober struct {
	Fail  int
	Calls int
	Err   error
}

func (p *Prober) Probe(context.Context, string) error {
	p.Calls++
	if p.Calls > p.Fail {
		return nil
	}
	if p.Err != nil {
		return p.Err
	}
	return errors.New("connection refused")
}

// Sink keeps every record, so a test can assert on an event id.
type Sink struct{ Records []s99logger.Record }

func (s *Sink) Write(_ context.Context, rec s99logger.Record) error {
	s.Records = append(s.Records, rec)
	return nil
}

// NewConfig returns a Config over real temp directories with faked privilege.
func NewConfig(t *testing.T) (deploy.Config, *Runner, *Accounts, *Units, *Prober) {
	t.Helper()
	base := t.TempDir()
	opt := filepath.Join(base, "opt")
	units := filepath.Join(base, "systemd")
	for _, dir := range []string{opt, units} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	runner := &Runner{
		Fail: map[string]error{},
		On:   map[string]func([]string) error{},
		Out:  map[string]string{},
	}
	accounts, unitd, prober := NewAccounts(), NewUnits(), &Prober{}
	cfg := deploy.Config{
		OptDir:   opt,
		UnitDir:  units,
		GoBinDir: filepath.Join(base, "go", "bin"),
		SelfPath: "/usr/local/bin/s99deploy",
		Euid:     0,
		Runner:   runner,
		Accounts: accounts,
		Units:    unitd,
		Prober:   prober,
	}
	return cfg, runner, accounts, unitd, prober
}
