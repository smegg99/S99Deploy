// internal/deploy/run.go

package deploy

import (
	"context"
	"os"
	"path/filepath"
	"syscall"

	"github.com/smegg99/s99deploy/internal/manifest"
)

// Run is the generic systemd ExecStart: load the manifest and exec the app.
func (d *Deployer) Run(ctx context.Context, root string) error {
	appDir := filepath.Join(root, "app")
	m, err := manifest.Load(filepath.Join(appDir, "deploy.json"))
	if err != nil {
		return err
	}
	argv0, err := resolveArgv0(appDir, m.Run[0])
	if err != nil {
		return err
	}
	if err := os.Chdir(appDir); err != nil {
		return err
	}
	// execve resets handled signal dispositions, so a SIGTERM that arrived during startup must not exec a process systemd has already given up on.
	if err := ctx.Err(); err != nil {
		return err
	}
	// The env is laid over what systemd provided, EnvironmentFile included, so systemd supervises the real binary rather than a wrapper.
	return syscall.Exec(argv0, m.Run, EnvSlice(BuildEnv(os.Environ(), m.Env)))
}
