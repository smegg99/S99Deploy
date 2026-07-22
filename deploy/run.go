// deploy/run.go

package deploy

import (
	"os"
	"path/filepath"
	"syscall"

	"github.com/smegg99/s99deploy/manifest"
)

// Run is the generic systemd ExecStart: load the manifest, lay its env over
// what systemd provided (EnvironmentFile included), and exec the app so
// systemd supervises the real binary, not a wrapper.
func Run(root string) error {
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
	return syscall.Exec(argv0, m.Run, EnvSlice(BuildEnv(os.Environ(), m.Env)))
}
