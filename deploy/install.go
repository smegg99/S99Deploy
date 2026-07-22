// deploy/install.go

package deploy

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/smegg99/s99logger"

	"github.com/smegg99/s99deploy/manifest"
)

// Install performs one-time app setup: clone, service user, /opt layout,
// systemd unit. Safe to rerun; every step skips what already exists.
func Install(gitURL string) error {
	if err := requireRoot(); err != nil {
		return err
	}
	if err := os.MkdirAll("/opt", 0o755); err != nil {
		return err
	}
	// The temp clone lives under /opt so it can be renamed into place; /tmp is
	// often tmpfs, where a cross-device rename fails.
	tmp, err := os.MkdirTemp("/opt", ".s99deploy-install-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	checkout := filepath.Join(tmp, "app")
	s99logger.Info(s99logger.NewEvent("cloning", s99logger.String("repo", gitURL)))
	if err := run("git", "clone", gitURL, checkout); err != nil {
		return err
	}
	m, err := manifest.Load(filepath.Join(checkout, "deploy.json"))
	if err != nil {
		return err
	}
	root := filepath.Join("/opt", m.Name)
	appDir := filepath.Join(root, "app")

	if err := ensureUser(m.Name, root); err != nil {
		return err
	}
	uid, gid, err := lookupIDs(m.Name)
	if err != nil {
		return err
	}
	if err := os.Chown(root, uid, gid); err != nil {
		return err
	}
	if err := os.Chmod(root, 0o755); err != nil {
		return err
	}

	if _, err := os.Stat(filepath.Join(appDir, ".git")); err == nil {
		s99logger.Info(s99logger.NewEvent("checkout_present", s99logger.String("path", appDir)))
	} else {
		if err := os.Rename(checkout, appDir); err != nil {
			return err
		}
		if err := run("chown", "-R", m.Name+":"+m.Name, appDir); err != nil {
			return err
		}
	}

	if err := seedEnv(root, appDir, uid, gid); err != nil {
		return err
	}

	unitPath := filepath.Join("/etc/systemd/system", m.Name+".service")
	if err := os.WriteFile(unitPath, []byte(RenderUnit(m.Name, root)), 0o644); err != nil {
		return err
	}
	if err := run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if err := run("systemctl", "enable", m.Name); err != nil {
		return err
	}

	s99logger.Info(s99logger.NewEvent("installed", s99logger.String("app", m.Name)))
	fmt.Printf("\nNext:\n  1. fill in %s/.env\n  2. sudo s99deploy up %s\n", root, m.Name)
	return nil
}

// ensureUser creates the service user with no sudo and no password: a
// web-facing process should not be able to escalate if compromised.
func ensureUser(name, home string) error {
	if _, err := user.Lookup(name); err == nil {
		return nil
	}
	return run("useradd", "--system", "--create-home", "--home-dir", home, "--shell", "/bin/bash", name)
}

func lookupIDs(name string) (int, int, error) {
	u, err := user.Lookup(name)
	if err != nil {
		return 0, 0, err
	}
	uid, err := strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return 0, 0, err
	}
	return uid, gid, nil
}

// seedEnv creates /opt/<name>/.env on first install -- from .env.example when
// the app has one, empty otherwise, since both `up` and the unit's
// EnvironmentFile require the file to exist. Locked down either way.
func seedEnv(root, appDir string, uid, gid int) error {
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
	s99logger.Info(s99logger.NewEvent("created_env", s99logger.String("path", envPath)))
	return lockDownEnv(envPath, uid, gid)
}

func lockDownEnv(envPath string, uid, gid int) error {
	if err := os.Chown(envPath, uid, gid); err != nil {
		return err
	}
	return os.Chmod(envPath, 0o600)
}
