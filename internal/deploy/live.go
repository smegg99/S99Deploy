// internal/deploy/live.go

package deploy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"strconv"
	"strings"
	"time"
)

// LiveConfig is the box: real /opt, real systemd, real processes.
func LiveConfig() Config {
	runner := &liveRunner{out: os.Stdout, errOut: os.Stderr}
	return Config{
		OptDir:  "/opt",
		UnitDir: "/etc/systemd/system",
		// The rest of systemd's search path, so a manifest name cannot claim a unit that lives here.
		SystemUnitDirs: []string{"/run/systemd/system", "/usr/lib/systemd/system", "/lib/systemd/system"},
		GoBinDir:       "/usr/local/go/bin",
		SelfPath:       "/usr/local/bin/s99deploy",
		Euid:           os.Geteuid(),
		OwnerUID:       os.Geteuid(),
		OwnerGID:       os.Getegid(),
		Runner:         runner,
		Accounts:       liveAccounts{runner: runner},
		Units:          liveUnits{runner: runner},
		Prober:         liveProber{client: &http.Client{Timeout: 5 * time.Second}},
	}
}

// liveAccounts is useradd, userdel and the passwd database.
type liveAccounts struct{ runner Runner }

func (a liveAccounts) Lookup(name string) (Account, error) {
	found, err := user.Lookup(name)
	var unknown user.UnknownUserError
	if errors.As(err, &unknown) {
		return Account{}, ErrNoAccount
	}
	if err != nil {
		return Account{}, err
	}

	uid, err := strconv.ParseUint(found.Uid, 10, 32)
	if err != nil {
		return Account{}, fmt.Errorf("account %s: uid %q: %w", name, found.Uid, err)
	}
	gid, err := strconv.ParseUint(found.Gid, 10, 32)
	if err != nil {
		return Account{}, fmt.Errorf("account %s: gid %q: %w", name, found.Gid, err)
	}
	ids, err := found.GroupIds()
	if err != nil {
		return Account{}, fmt.Errorf("account %s: groups: %w", name, err)
	}

	groups := make([]uint32, 0, len(ids))
	for _, raw := range ids {
		id, err := strconv.ParseUint(raw, 10, 32)
		if err != nil {
			return Account{}, fmt.Errorf("account %s: group %q: %w", name, raw, err)
		}
		groups = append(groups, uint32(id))
	}
	return Account{
		Name: found.Username, Home: found.HomeDir,
		UID: uint32(uid), GID: uint32(gid), Groups: groups,
	}, nil
}

// Create makes a system account with no password and no sudo.
func (a liveAccounts) Create(ctx context.Context, name, home string) error {
	// A web-facing process should not be able to escalate if it is compromised.
	return a.runner.Run(ctx, nil, "useradd", "--system", "--create-home",
		"--home-dir", home, "--shell", "/bin/bash", name)
}

// Delete removes the account and nothing else; the home is removed by path.
func (a liveAccounts) Delete(ctx context.Context, name string) error {
	return a.runner.Run(ctx, nil, "userdel", name)
}

// liveUnits drives systemd through systemctl.
type liveUnits struct{ runner Runner }

func (u liveUnits) Reload(ctx context.Context) error {
	return u.runner.Run(ctx, nil, "systemctl", "daemon-reload")
}

func (u liveUnits) Enable(ctx context.Context, name string) error {
	return u.runner.Run(ctx, nil, "systemctl", "enable", name)
}

func (u liveUnits) Disable(ctx context.Context, name string) error {
	return u.runner.Run(ctx, nil, "systemctl", "disable", "--now", name)
}

func (u liveUnits) Restart(ctx context.Context, name string) error {
	return u.runner.Run(ctx, nil, "systemctl", "restart", name)
}

// State reads the four properties the health wait decides on.
func (u liveUnits) State(ctx context.Context, name string) (UnitState, error) {
	out, err := u.runner.Output(ctx, nil, "systemctl", "show", name,
		"--property=ActiveState", "--property=SubState",
		"--property=Result", "--property=InvocationID")
	if err != nil {
		return UnitState{}, err
	}
	return parseUnitState(out), nil
}

// parseUnitState reads the key=value lines `systemctl show` prints.
func parseUnitState(out string) UnitState {
	var state UnitState
	for _, line := range strings.Split(out, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "ActiveState":
			state.Active = value
		case "SubState":
			state.Sub = value
		case "Result":
			state.Result = value
		case "InvocationID":
			state.InvocationID = value
		}
	}
	return state
}

// liveProber is one GET against the loopback address the manifest names.
type liveProber struct{ client *http.Client }

func (p liveProber) Probe(ctx context.Context, url string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned %d", url, resp.StatusCode)
	}
	return nil
}
