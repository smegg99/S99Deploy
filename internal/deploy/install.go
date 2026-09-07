// internal/deploy/install.go

package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

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
	if err := ctx.Err(); err != nil {
		return Installed{}, err
	}
	if err := requireAgent(gitURL); err != nil {
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
	err = d.cfg.Runner.Run(ctx, cloneEnv(), "git", "clone", gitURL, checkout)
	end(err)
	if err != nil {
		return Installed{}, err
	}
	d.cfg.Log.Info(s99logger.NewEvent(EventCloning, s99logger.String("repo", gitURL)))

	m, err := manifest.Load(filepath.Join(checkout, "deploy.json"))
	if err != nil {
		return Installed{}, err
	}
	root := d.Root(m.Name)
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
		if err := chownTree(appDir, int(account.UID), int(account.GID)); err != nil {
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

// ensureAccount creates the service account, and refuses to adopt a foreign one.
func (d *Deployer) ensureAccount(ctx context.Context, name, home string) (Account, error) {
	// Adopting an account whose home is elsewhere is how a later --purge deletes
	// the wrong directory.
	account, err := d.cfg.Accounts.Lookup(name)
	switch {
	case errors.Is(err, ErrNoAccount):
		if err := d.cfg.Accounts.Create(ctx, name, home); err != nil {
			return Account{}, err
		}
		return d.cfg.Accounts.Lookup(name)
	case err != nil:
		return Account{}, err
	}

	if got := filepath.Clean(account.Home); got != home {
		return Account{}, fmt.Errorf(
			"account %s already exists with home %s, want %s; s99deploy will not adopt it",
			name, got, home)
	}
	return account, nil
}

// seedEnv creates /opt/<name>/.env on first install.
func (d *Deployer) seedEnv(root, appDir string, uid, gid int) error {
	// From .env.example when the app ships one, because both up and the unit's EnvironmentFile need the file to exist.
	// The service account owns root, so it can put a symlink where .env goes.
	// Every step below is taken through an os.Root, which refuses to follow one
	// out of the directory, and refuses a .env that is not a regular file.
	dir, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer dir.Close()

	envPath := filepath.Join(root, ".env")
	switch info, err := dir.Lstat(".env"); {
	case err == nil && !info.Mode().IsRegular():
		return fmt.Errorf("%s is %s, not a regular file", envPath, info.Mode().Type())
	case err == nil:
		return lockDownEnv(dir, envPath, uid, gid)
	case !os.IsNotExist(err):
		return err
	}

	example, err := os.ReadFile(filepath.Join(appDir, ".env.example"))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	file, err := dir.OpenFile(".env", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(example); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	d.cfg.Log.Info(s99logger.NewEvent(EventCreatedEnv, s99logger.String("path", envPath)))
	return lockDownEnv(dir, envPath, uid, gid)
}

func lockDownEnv(dir *os.Root, envPath string, uid, gid int) error {
	if err := dir.Lchown(".env", uid, gid); err != nil {
		return fmt.Errorf("chown %s: %w", envPath, err)
	}
	return dir.Chmod(".env", 0o600)
}

// sshURL matches the two spellings git treats as SSH.
var sshURL = regexp.MustCompile(`^(ssh://|[^/]+@[^/]+:)`)

// requireAgent fails before a clone that would hang on a prompt.
func requireAgent(gitURL string) error {
	// No unattended deploy can answer it, and sudo's env_reset drops SSH_AUTH_SOCK.
	if !sshURL.MatchString(gitURL) {
		return nil
	}
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return fmt.Errorf(
			"%s needs SSH and SSH_AUTH_SOCK is unset (sudo drops it). Rerun as:\n"+
				"  sudo --preserve-env=SSH_AUTH_SOCK s99deploy install %s", gitURL, gitURL)
	}

	info, err := os.Stat(sock)
	if err != nil {
		return fmt.Errorf("SSH_AUTH_SOCK=%s: %w", sock, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("SSH_AUTH_SOCK=%s is not a socket", sock)
	}
	return nil
}

// cloneEnv makes a missing key fail in a second instead of waiting for a prompt.
func cloneEnv() []string {
	return append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes",
	)
}
