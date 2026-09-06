// internal/deploy/up.go

package deploy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/joho/godotenv"
	"github.com/smegg99/s99logger"

	"github.com/smegg99/s99deploy/internal/manifest"
)

// Up deploys /opt/<name>: pull, build, restart, health check.
func (d *Deployer) Up(ctx context.Context, name string, timeout time.Duration) error {
	if err := d.requireRoot(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	root := d.Root(name)
	appDir := filepath.Join(root, "app")
	envPath := filepath.Join(root, ".env")
	if _, err := os.Stat(filepath.Join(appDir, ".git")); err != nil {
		return fmt.Errorf("no checkout at %s -- run `s99deploy install <git-url>` first", appDir)
	}
	if _, err := os.Stat(envPath); err != nil {
		return fmt.Errorf("missing %s", envPath)
	}

	account, err := d.cfg.Accounts.Lookup(name)
	if err != nil {
		return err
	}
	as := AsUser{
		Name: name, Home: account.Home, Dir: appDir,
		UID: account.UID, GID: account.GID, Groups: account.Groups,
	}

	end := d.cfg.Progress.Begin(StepPull, name)
	err = d.cfg.Runner.RunAsUser(ctx, as, "git pull --ff-only", nil)
	end(err)
	if err != nil {
		return err
	}
	d.cfg.Log.Info(s99logger.NewEvent(EventPulling, s99logger.String("app", name)))

	// Read the manifest after the pull so a deploy picks up its own changes.
	m, err := manifest.Load(filepath.Join(appDir, "deploy.json"))
	if err != nil {
		return err
	}
	if m.Name != name {
		return fmt.Errorf("manifest name %q does not match %q", m.Name, name)
	}
	// The flag wins when it was given; otherwise the manifest decides.
	if timeout <= 0 {
		timeout = time.Duration(m.Check.TimeoutSeconds) * time.Second
	}

	dotenv, err := godotenv.Read(envPath)
	if err != nil {
		return fmt.Errorf("parse %s: %w", envPath, err)
	}
	env := BuildEnv(os.Environ(), dotenv, m.Env)
	if _, err := os.Stat(d.cfg.GoBinDir); err == nil {
		PrependPath(env, d.cfg.GoBinDir)
	}
	if err := CheckRequired(env, m.RequireEnv); err != nil {
		return err
	}

	// No HOME, USER, LOGNAME or SHELL here: the runner derives them from AsUser.
	envSlice := EnvSlice(env)
	for _, command := range m.Build {
		end = d.cfg.Progress.Begin(StepBuild, command)
		err = d.cfg.Runner.RunAsUser(ctx, as, command, envSlice)
		end(err)
		if err != nil {
			return err
		}
		d.cfg.Log.Info(s99logger.NewEvent(EventBuilding, s99logger.String("command", command)))
	}

	end = d.cfg.Progress.Begin(StepRestart, name)
	err = d.cfg.Units.Restart(ctx, name)
	end(err)
	if err != nil {
		return err
	}
	d.cfg.Log.Info(s99logger.NewEvent(EventRestarting, s99logger.String("service", name)))

	// Read after the restart, not before: a manual restart resets the restart
	// accounting, so a pre-restart reading would mean nothing.
	baseline, err := d.cfg.Units.State(ctx, name)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("http://127.0.0.1:%d%s", m.Check.Port, m.Check.Path)
	end = d.cfg.Progress.Begin(StepHealth, url)
	err = d.waitHealthy(ctx, name, url, timeout, baseline)
	end(err)
	if err != nil {
		return fmt.Errorf("%w -- journalctl -u %s", err, name)
	}

	if commit, err := d.cfg.Runner.OutputAsUser(ctx, as, "git log --oneline -1", envSlice); err == nil {
		d.cfg.Log.Info(s99logger.NewEvent(EventDeployed, s99logger.String("commit", commit)))
	}
	return nil
}

// waitHealthy replaces the fixed sleep and the one-shot state reading.
func (d *Deployer) waitHealthy(ctx context.Context, name, url string, timeout time.Duration, baseline UnitState) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// The deadline is the manifest's own timeout, the proof of life is the
	// health endpoint, and a unit that failed or restarted under us aborts the
	// wait at once.
	attempt := func() error {
		state, err := d.cfg.Units.State(ctx, name)
		switch {
		case err != nil:
			return backoff.Permanent(err)
		case state.Active == "failed":
			return backoff.Permanent(fmt.Errorf("unit %s failed: result %s", name, state.Result))
		case baseline.InvocationID != "" && state.InvocationID != baseline.InvocationID:
			return backoff.Permanent(fmt.Errorf("unit %s restarted while starting", name))
		case state.Active != "active":
			return fmt.Errorf("unit %s is %s/%s", name, state.Active, state.Sub)
		}
		return d.cfg.Prober.Probe(ctx, url)
	}

	policy := backoff.NewExponentialBackOff()
	policy.InitialInterval = 250 * time.Millisecond
	policy.MaxInterval = 2 * time.Second
	policy.MaxElapsedTime = timeout
	if err := backoff.Retry(attempt, backoff.WithContext(policy, ctx)); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return fmt.Errorf("%s did not become healthy within %s: %w", name, timeout, err)
	}
	return nil
}
