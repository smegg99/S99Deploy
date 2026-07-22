// deploy/up.go

package deploy

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/joho/godotenv"
	"github.com/smegg99/s99logger"

	"github.com/smegg99/s99deploy/manifest"
)

const goBinDir = "/usr/local/go/bin"

// Up deploys /opt/<name>: pull, build as the service user, restart, health
// check. Any failure aborts with the service journal as the next stop.
func Up(name string) error {
	if err := requireRoot(); err != nil {
		return err
	}
	root := filepath.Join("/opt", name)
	appDir := filepath.Join(root, "app")
	envPath := filepath.Join(root, ".env")
	if _, err := os.Stat(filepath.Join(appDir, ".git")); err != nil {
		return fmt.Errorf("no checkout at %s -- run `s99deploy install <git-url>` first", appDir)
	}
	if _, err := os.Stat(envPath); err != nil {
		return fmt.Errorf("missing %s", envPath)
	}

	s99logger.Info(s99logger.NewEvent("pulling", s99logger.String("app", name)))
	if err := runAsUser(name, appDir, "git pull --ff-only", nil); err != nil {
		return err
	}

	// Read the manifest after the pull so a deploy picks up its own changes.
	m, err := manifest.Load(filepath.Join(appDir, "deploy.json"))
	if err != nil {
		return err
	}
	if m.Name != name {
		return fmt.Errorf("manifest name %q does not match %q", m.Name, name)
	}

	dotenv, err := godotenv.Read(envPath)
	if err != nil {
		return fmt.Errorf("parse %s: %w", envPath, err)
	}
	env := BuildEnv(os.Environ(), dotenv, m.Env)
	if _, err := os.Stat(goBinDir); err == nil {
		PrependPath(env, goBinDir)
	}
	if err := CheckRequired(env, m.RequireEnv); err != nil {
		return err
	}

	envSlice := EnvSlice(env)
	for _, command := range m.Build {
		s99logger.Info(s99logger.NewEvent("building", s99logger.String("command", command)))
		if err := runAsUser(name, appDir, command, envSlice); err != nil {
			return err
		}
	}

	s99logger.Info(s99logger.NewEvent("restarting", s99logger.String("service", name)))
	if err := run("systemctl", "restart", name); err != nil {
		return err
	}
	time.Sleep(3 * time.Second)
	if err := run("systemctl", "is-active", "--quiet", name); err != nil {
		return fmt.Errorf("service failed to start -- journalctl -u %s", name)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d%s", m.Check.Port, m.Check.Path)
	if err := healthCheck(url, time.Duration(m.Check.TimeoutSeconds)*time.Second); err != nil {
		return fmt.Errorf("%w -- journalctl -u %s", err, name)
	}

	if commit, err := outputAsUser(name, appDir, "git log --oneline -1"); err == nil {
		s99logger.Info(s99logger.NewEvent("deployed", s99logger.String("commit", commit)))
	}
	return nil
}

// healthCheck polls url until it returns 200 or the deadline passes. At least
// one attempt is always made.
func healthCheck(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("%s returned %d", url, resp.StatusCode)
		} else {
			lastErr = err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("health check failed: %v", lastErr)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
