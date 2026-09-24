// internal/deploy/install.go

package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/joho/godotenv"
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
	tmp, err := os.MkdirTemp(d.cfg.OptDir, ".depl-install-")
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
	m, err := manifest.Load(filepath.Join(checkout, "deploy.json"))
	if err != nil {
		return Installed{}, err
	}
	root := d.Root(m.Name)
	appDir := filepath.Join(root, "app")

	// The unit name is decided before anything is created, so a name that
	// collides with a foreign or a system unit leaves no account or tree behind.
	unitExists, err := d.ownedUnit(m.Name)
	if err != nil {
		return Installed{}, err
	}
	if !unitExists {
		if err := d.refuseShadow(m.Name); err != nil {
			return Installed{}, err
		}
	}

	account, err := d.ensureAccount(ctx, m.Name, root)
	if err != nil {
		return Installed{}, err
	}
	// root and .env are root's, so the account cannot swap the secrets that
	// systemd reads back into its environment; the account owns app/ and the
	// home/ below. useradd --create-home makes root the account's, so this
	// reclaims it.
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Installed{}, err
	}
	if err := os.Chown(root, d.cfg.OwnerUID, d.cfg.OwnerGID); err != nil {
		return Installed{}, err
	}
	if err := os.Chmod(root, 0o755); err != nil {
		return Installed{}, err
	}
	if err := ensureHome(root, account); err != nil {
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

	if err := d.seedEnv(root); err != nil {
		return Installed{}, err
	}

	if err := d.writeUnit(m.Name, RenderUnit(UnitParams{Name: m.Name, Root: root, SelfPath: d.cfg.SelfPath})); err != nil {
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

// ownedUnit reports whether name's unit exists and depl wrote it, and refuses a foreign one.
func (d *Deployer) ownedUnit(name string) (bool, error) {
	path := d.unitPath(name)
	info, err := os.Lstat(path)
	switch {
	case os.IsNotExist(err):
		return false, nil
	case err != nil:
		return false, err
	}
	if !info.Mode().IsRegular() {
		return false, fmt.Errorf("%s is not a regular file; depl will not touch it", path)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	if !strings.Contains(string(body), unitMarker) {
		return false, fmt.Errorf("%s was not written by depl; remove it by hand", path)
	}
	return true, nil
}

// refuseShadow rejects a name that already resolves to a unit elsewhere on the search path.
func (d *Deployer) refuseShadow(name string) error {
	for _, dir := range d.cfg.SystemUnitDirs {
		if _, err := os.Lstat(filepath.Join(dir, name+".service")); err == nil {
			return fmt.Errorf("%s already has a unit in %s; choose another app name", name, dir)
		}
	}
	return nil
}

// writeUnit writes the rendered unit, refusing to follow a symlink in its place.
func (d *Deployer) writeUnit(name, unit string) error {
	file, err := os.OpenFile(d.unitPath(name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC|syscall.O_NOFOLLOW, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(unit); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

// ensureHome lays out the account's writable home inside the root-owned tree.
func ensureHome(root string, account Account) error {
	// Every build tool writes under HOME: `go build` its build cache, pnpm and
	// npm their ~/.cache and ~/.local. The app root is root's, and passwd keeps
	// naming it as the account's home because that is the directory --purge
	// checks before it deletes, so the writable home is this one inside it. It
	// is laid out through an os.Root and refused when it is not a directory, so
	// an older install that left the root to the account cannot have left a
	// link here.
	dir, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer dir.Close()

	home := accountHome(root)
	switch info, err := dir.Lstat("home"); {
	case err == nil && !info.IsDir():
		return fmt.Errorf("%s is %s, not a directory", home, info.Mode().Type())
	case os.IsNotExist(err):
		if err := dir.Mkdir("home", 0o700); err != nil {
			return err
		}
	case err != nil:
		return err
	}
	if err := dir.Lchown("home", int(account.UID), int(account.GID)); err != nil {
		return fmt.Errorf("chown %s: %w", home, err)
	}
	return dir.Chmod("home", 0o700)
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
			"account %s already exists with home %s, want %s; depl will not adopt it",
			name, got, home)
	}
	return account, nil
}

// maxEnvExample caps the .env.example read, so a fifo or a device cannot hang install or exhaust it.
const maxEnvExample = 1 << 20

// seedEnv creates root-owned /opt/<name>/.env on first install; the account never opens it.
func (d *Deployer) seedEnv(root string) error {
	// The account owns the checkout, so both the example it is seeded from and
	// .env itself are opened through an os.Root that cannot be followed out of
	// the app root, and each is required to be a regular file with one link.
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
		return lockDownEnv(dir, envPath, d.cfg.OwnerUID, d.cfg.OwnerGID)
	case !os.IsNotExist(err):
		return err
	}

	example, err := readEnvExample(dir, root)
	if err != nil {
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
	return lockDownEnv(dir, envPath, d.cfg.OwnerUID, d.cfg.OwnerGID)
}

// readEnvExample reads app/.env.example through the app root, only when it is a regular file with one link.
func readEnvExample(dir *os.Root, root string) ([]byte, error) {
	// O_NONBLOCK so a fifo returns a descriptor instead of blocking; the fstat then rejects it.
	file, err := dir.OpenFile("app/.env.example", os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open %s/app/.env.example: %w", root, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s/app/.env.example is %s, not a regular file", root, info.Mode().Type())
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Nlink != 1 {
		return nil, fmt.Errorf("%s/app/.env.example has %d hard links, want 1", root, st.Nlink)
	}
	return io.ReadAll(io.LimitReader(file, maxEnvExample))
}

// lockDownEnv keeps .env root's at 0600, refusing a link or a multiply-linked file.
func lockDownEnv(dir *os.Root, envPath string, uid, gid int) error {
	file, err := dir.OpenFile(".env", os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", envPath, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%s is %s, not a regular file", envPath, info.Mode().Type())
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Nlink != 1 {
		return fmt.Errorf("%s has %d hard links, want 1", envPath, st.Nlink)
	}
	// Reclaimed to root, so an old install's account-owned .env is taken back.
	if err := file.Chown(uid, gid); err != nil {
		return fmt.Errorf("chown %s: %w", envPath, err)
	}
	return file.Chmod(0o600)
}

// readEnvFile reads /opt/<name>/.env through the app root, refusing a link or a multiply-linked file.
func readEnvFile(root string) (map[string]string, error) {
	dir, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	file, err := dir.OpenFile(".env", os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s/.env: %w", root, err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%s/.env is %s, not a regular file", root, info.Mode().Type())
	}
	if st, ok := info.Sys().(*syscall.Stat_t); ok && st.Nlink != 1 {
		return nil, fmt.Errorf("%s/.env has %d hard links, want 1", root, st.Nlink)
	}
	return godotenv.Parse(file)
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
			"%s needs SSH but no agent is reachable (SSH_AUTH_SOCK is unset). Start one\n"+
				"and load a key with `ssh-add`, then keep it across sudo:\n"+
				"  sudo --preserve-env=SSH_AUTH_SOCK depl install %s", gitURL, gitURL)
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
