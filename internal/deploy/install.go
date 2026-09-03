// internal/deploy/install.go

package deploy

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/smegg99/s99logger"

	"github.com/smegg99/s99deploy/internal/manifest"
)

// Installed is what one install produced, for the caller to print.
type Installed struct{ Name, Root string }

// Install is the one-time app setup: clone, account, /opt layout, unit.
func (d *Deployer) Install(ctx context.Context, gitURL string) (Installed, error) {
	// Safe to rerun: every step below skips what already exists.
	if err := d.requireRoot(); err != nil {
		return Installed{}, err
	}
	if err := os.MkdirAll(d.cfg.OptDir, 0o755); err != nil {
		return Installed{}, err
	}

	// The temp clone lives under OptDir so it can be renamed into place; /tmp is often tmpfs, where a cross-device rename fails.
	tmp, err := os.MkdirTemp(d.cfg.OptDir, ".s99deploy-install-")
	if err != nil {
		return Installed{}, err
	}
	defer os.RemoveAll(tmp)

	checkout := filepath.Join(tmp, "app")
	end := d.cfg.Progress.Begin(StepClone, gitURL)
	err = d.cfg.Runner.Run(ctx, nil, "git", "clone", gitURL, checkout)
	end(err)
	if err != nil {
		return Installed{}, err
	}
	d.cfg.Log.Info(s99logger.NewEvent(EventCloning, s99logger.String("repo", gitURL)))

	m, err := manifest.Load(filepath.Join(checkout, "deploy.json"))
	if err != nil {
		return Installed{}, err
	}
	root := d.root(m.Name)
	appDir := filepath.Join(root, "app")

	account, err := d.ensureAccount(ctx, m.Name, root)
	if err != nil {
		return Installed{}, err
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Installed{}, err
	}
	if err := os.Chown(root, int(account.UID), int(account.GID)); err != nil {
		return Installed{}, err
	}
	if err := os.Chmod(root, 0o755); err != nil {
		return Installed{}, err
	}

	if _, err := os.Stat(filepath.Join(appDir, ".git")); err == nil {
		d.cfg.Log.Info(s99logger.NewEvent(EventCheckoutPresent, s99logger.String("path", appDir)))
	} else {
		if err := os.Rename(checkout, appDir); err != nil {
			return Installed{}, err
		}
		if err := d.cfg.Runner.Run(ctx, nil, "chown", "-R", m.Name+":"+m.Name, appDir); err != nil {
			return Installed{}, err
		}
	}

	if err := d.seedEnv(root, appDir, int(account.UID), int(account.GID)); err != nil {
		return Installed{}, err
	}

	unit := RenderUnit(UnitParams{Name: m.Name, Root: root, SelfPath: d.cfg.SelfPath})
	if err := os.WriteFile(d.unitPath(m.Name), []byte(unit), 0o644); err != nil {
		return Installed{}, err
	}
	if err := d.cfg.Units.Reload(ctx); err != nil {
		return Installed{}, err
	}
	if err := d.cfg.Units.Enable(ctx, m.Name); err != nil {
		return Installed{}, err
	}

	d.cfg.Log.Info(s99logger.NewEvent(EventInstalled, s99logger.String("app", m.Name)))
	return Installed{Name: m.Name, Root: root}, nil
}

// ensureAccount creates the service account when it is missing.
func (d *Deployer) ensureAccount(ctx context.Context, name, home string) (Account, error) {
	account, err := d.cfg.Accounts.Lookup(name)
	if err == nil {
		return account, nil
	}
	if !errors.Is(err, ErrNoAccount) {
		return Account{}, err
	}
	if err := d.cfg.Accounts.Create(ctx, name, home); err != nil {
		return Account{}, err
	}
	return d.cfg.Accounts.Lookup(name)
}

// seedEnv creates /opt/<name>/.env on first install.
func (d *Deployer) seedEnv(root, appDir string, uid, gid int) error {
	// From .env.example when the app ships one, because both up and the unit's EnvironmentFile need the file to exist.
	envPath := filepath.Join(root, ".env")
	if _, err := os.Stat(envPath); err == nil {
		return lockDownEnv(envPath, uid, gid)
	}

	example, err := os.ReadFile(filepath.Join(appDir, ".env.example"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.WriteFile(envPath, example, 0o600); err != nil {
		return err
	}
	d.cfg.Log.Info(s99logger.NewEvent(EventCreatedEnv, s99logger.String("path", envPath)))
	return lockDownEnv(envPath, uid, gid)
}

func lockDownEnv(envPath string, uid, gid int) error {
	if err := os.Chown(envPath, uid, gid); err != nil {
		return err
	}
	return os.Chmod(envPath, 0o600)
}
