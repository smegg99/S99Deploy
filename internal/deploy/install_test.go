// internal/deploy/install_test.go

package deploy_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/smegg99/s99deploy/internal/deploy"
	"github.com/smegg99/s99deploy/internal/deploy/deploytest"
)

const manifestJSON = `{
	"name": "myapp",
	"build": ["go build -o bin/myapp ."],
	"run": ["bin/myapp"],
	"check": { "port": 9300 }
}`

// clones makes the fake git clone leave a checkout where a real one would.
func clones(t *testing.T, runner *deploytest.Runner, extra map[string]string) {
	t.Helper()
	runner.On["git clone"] = func(args []string) error {
		checkout := args[len(args)-1]
		if err := os.MkdirAll(filepath.Join(checkout, ".git"), 0o755); err != nil {
			return err
		}
		files := map[string]string{"deploy.json": manifestJSON}
		for name, body := range extra {
			files[name] = body
		}
		for name, body := range files {
			if err := os.WriteFile(filepath.Join(checkout, name), []byte(body), 0o644); err != nil {
				return err
			}
		}
		return nil
	}
}

func TestInstallLaysOutTheApp(t *testing.T) {
	cfg, runner, accounts, units, _ := deploytest.NewConfig(t)
	clones(t, runner, map[string]string{".env.example": "TOKEN=\n"})
	// A group this process is in but is not already running as, so a chown the
	// install makes is something the test can read back. When there is none the
	// layout is still checked; only the ownership read-back is skipped.
	altGID, hasAlt := deploytest.AltGID(t)
	if hasAlt {
		accounts.GID = altGID
	}

	installed, err := deploy.New(cfg).Install(context.Background(), "https://example.com/myapp.git")
	if err != nil {
		t.Fatal(err)
	}

	if installed.Name != "myapp" {
		t.Errorf("Installed.Name = %q, want myapp", installed.Name)
	}
	root := filepath.Join(cfg.OptDir, "myapp")
	if installed.Root != root {
		t.Errorf("Installed.Root = %q, want %q", installed.Root, root)
	}
	if _, err := os.Stat(filepath.Join(root, "app", ".git")); err != nil {
		t.Errorf("the checkout was not renamed into place: %v", err)
	}

	info, err := os.Stat(filepath.Join(root, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf(".env mode = %v, want 0600", info.Mode().Perm())
	}
	body, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "TOKEN=\n" {
		t.Errorf(".env = %q, want it seeded from .env.example", body)
	}

	unit, err := os.ReadFile(filepath.Join(cfg.UnitDir, "myapp.service"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"User=myapp", "ExecStart=/usr/local/bin/depl run " + root,
		"ReadWritePaths=" + root + "/app " + root + "/home",
		"Environment=HOME=" + root + "/home", "EnvironmentFile=" + root + "/.env",
	} {
		if !strings.Contains(string(unit), want) {
			t.Errorf("unit is missing %q:\n%s", want, unit)
		}
	}

	if accounts.Created == nil || accounts.Created[0] != "myapp" {
		t.Errorf("Created = %v, want [myapp]", accounts.Created)
	}
	if units.Reloads != 1 || len(units.Enabled) != 1 || units.Enabled[0] != "myapp" {
		t.Errorf("reloads = %d, enabled = %v, want 1 and [myapp]", units.Reloads, units.Enabled)
	}
	if !runner.Ran("git", "clone") {
		t.Error("nothing cloned")
	}

	if !hasAlt {
		return
	}
	// The account owns only the checkout it builds in.
	for _, path := range []string{filepath.Join(root, "app"),
		filepath.Join(root, "app", "deploy.json")} {
		if stat := lstat(t, path); stat.Uid != accounts.UID || stat.Gid != accounts.GID {
			t.Errorf("%s is owned by %d:%d, want the account %d:%d",
				path, stat.Uid, stat.Gid, accounts.UID, accounts.GID)
		}
	}
	// root and .env stay root's, so the account cannot swap the secrets: here
	// that is the test's own ids, not the account's alternate group.
	for _, path := range []string{root, filepath.Join(root, ".env")} {
		if stat := lstat(t, path); stat.Gid == accounts.GID {
			t.Errorf("%s was handed to the account's group %d", path, accounts.GID)
		}
	}
}

// lstat returns the raw ownership of path, or fails the test.
func lstat(t *testing.T, path string) *syscall.Stat_t {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("no stat for %s", path)
	}
	return stat
}

// Install is safe to rerun: every step skips what already exists.
func TestInstallIsRerunnable(t *testing.T) {
	cfg, runner, accounts, units, _ := deploytest.NewConfig(t)
	clones(t, runner, nil)
	d := deploy.New(cfg)

	if _, err := d.Install(context.Background(), "https://example.com/myapp.git"); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Install(context.Background(), "https://example.com/myapp.git"); err != nil {
		t.Fatal(err)
	}

	if units.Reloads != 2 {
		t.Errorf("reloads = %d, want 2", units.Reloads)
	}
	entries, err := os.ReadDir(cfg.OptDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".depl-install-") {
			t.Errorf("a temp checkout survived: %s", entry.Name())
		}
	}
	// The second install finds the home the first laid out and leaves it as it
	// was, rather than failing on the directory that is already there.
	home := filepath.Join(cfg.OptDir, "myapp", "home")
	info, err := os.Lstat(home)
	if err != nil {
		t.Fatalf("the rerun lost %s: %v", home, err)
	}
	if !info.IsDir() || info.Mode().Perm() != 0o700 {
		t.Errorf("%s is %v, want a 0700 directory", home, info.Mode())
	}
	if stat := lstat(t, home); stat.Uid != accounts.UID || stat.Gid != accounts.GID {
		t.Errorf("%s is owned by %d:%d, want the account %d:%d",
			home, stat.Uid, stat.Gid, accounts.UID, accounts.GID)
	}
}

func TestInstallRefusesWithoutRoot(t *testing.T) {
	cfg, _, _, _, _ := deploytest.NewConfig(t)
	cfg.Euid = 1000

	if _, err := deploy.New(cfg).Install(context.Background(), "https://example.com/x.git"); err == nil {
		t.Fatal("an unprivileged install was allowed")
	}
}

// A clone that can prompt is a clone that can hang a deploy forever.
func TestCloneRunsWithoutPrompts(t *testing.T) {
	cfg, runner, _, _, _ := deploytest.NewConfig(t)
	clones(t, runner, nil)

	if _, err := deploy.New(cfg).Install(context.Background(), "https://example.com/myapp.git"); err != nil {
		t.Fatal(err)
	}

	for _, call := range runner.Calls {
		if call.Name != "git" {
			continue
		}
		joined := strings.Join(call.Env, "\n")
		for _, want := range []string{"GIT_TERMINAL_PROMPT=0", "GIT_SSH_COMMAND=ssh -o BatchMode=yes"} {
			if !strings.Contains(joined, want) {
				t.Errorf("git clone env is missing %q", want)
			}
		}
		return
	}
	t.Fatal("git clone was never called")
}

// The mismatch is caught at adoption time, not when --purge has two homes.
func TestInstallRefusesToAdoptAForeignAccount(t *testing.T) {
	cfg, runner, accounts, _, _ := deploytest.NewConfig(t)
	clones(t, runner, nil)
	accounts.Users["myapp"] = deploy.Account{
		Name: "myapp", Home: "/home/someone-else",
		UID: uint32(os.Getuid()), GID: uint32(os.Getgid()),
	}

	_, err := deploy.New(cfg).Install(context.Background(), "https://example.com/myapp.git")
	if err == nil || !strings.Contains(err.Error(), "/home/someone-else") {
		t.Fatalf("err = %v, want it to name the foreign home", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.UnitDir, "myapp.service")); !os.IsNotExist(err) {
		t.Error("a unit was written for an account depl refused to adopt")
	}
}

// A repo whose manifest names an existing unit must not make root overwrite it.
func TestInstallRefusesToOverwriteAForeignUnit(t *testing.T) {
	cfg, runner, _, _, _ := deploytest.NewConfig(t)
	clones(t, runner, nil)
	foreign := filepath.Join(cfg.UnitDir, "myapp.service")
	if err := os.WriteFile(foreign, []byte("[Service]\nExecStart=/usr/bin/sddm\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := deploy.New(cfg).Install(context.Background(), "https://example.com/myapp.git")
	if err == nil || !strings.Contains(err.Error(), "not written by depl") {
		t.Fatalf("err = %v, want the foreign unit refused", err)
	}
	if body, _ := os.ReadFile(foreign); !strings.Contains(string(body), "sddm") {
		t.Errorf("the foreign unit was overwritten:\n%s", body)
	}
}

// A manifest name that already resolves to a system unit is refused before any write.
func TestInstallRefusesToShadowASystemUnit(t *testing.T) {
	cfg, runner, accounts, _, _ := deploytest.NewConfig(t)
	clones(t, runner, nil)
	vendor := t.TempDir()
	if err := os.WriteFile(filepath.Join(vendor, "myapp.service"), []byte("[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg.SystemUnitDirs = []string{vendor}

	_, err := deploy.New(cfg).Install(context.Background(), "https://example.com/myapp.git")
	if err == nil || !strings.Contains(err.Error(), "already has a unit") {
		t.Fatalf("err = %v, want the shadow refused", err)
	}
	if _, err := os.Stat(filepath.Join(cfg.UnitDir, "myapp.service")); !os.IsNotExist(err) {
		t.Error("a shadowing unit was written")
	}
	// Refused before anything was created, so there is no orphan to clean up.
	if len(accounts.Created) != 0 {
		t.Errorf("Created = %v, want nothing", accounts.Created)
	}
	if _, err := os.Stat(filepath.Join(cfg.OptDir, "myapp")); !os.IsNotExist(err) {
		t.Error("a tree was laid out for a refused name")
	}
}

func TestInstallDoesNotShellOutForOwnership(t *testing.T) {
	cfg, runner, _, _, _ := deploytest.NewConfig(t)
	clones(t, runner, nil)

	if _, err := deploy.New(cfg).Install(context.Background(), "https://example.com/myapp.git"); err != nil {
		t.Fatal(err)
	}
	if runner.Ran("chown") {
		t.Error("install still runs chown -R")
	}
}

// A .env that is a link is refused, so root neither reads nor writes through it.
func TestInstallRefusesAnEnvThatIsNotARegularFile(t *testing.T) {
	cfg, runner, _, _, _ := deploytest.NewConfig(t)
	clones(t, runner, nil)
	d := deploy.New(cfg)
	if _, err := d.Install(context.Background(), "https://example.com/myapp.git"); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(cfg.OptDir, "myapp")
	victim := filepath.Join(t.TempDir(), "victim")
	if err := os.WriteFile(victim, []byte("SECRET=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".env")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, filepath.Join(root, ".env")); err != nil {
		t.Fatal(err)
	}

	_, err := d.Install(context.Background(), "https://example.com/myapp.git")
	if err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("err = %v, want the linked .env refused", err)
	}
	info, err := os.Lstat(victim)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("the file the link pointed at is now %v, want 644", perm)
	}
}

// A .env.example that is a fifo or a link out of the checkout must not be read.
func TestInstallRefusesAHostileEnvExample(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(secret, []byte("STOLEN=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(checkout string) error{
		"fifo": func(checkout string) error {
			return syscall.Mkfifo(filepath.Join(checkout, ".env.example"), 0o644)
		},
		"link out of the checkout": func(checkout string) error {
			return os.Symlink(secret, filepath.Join(checkout, ".env.example"))
		},
		"hard link in the checkout": func(checkout string) error {
			target := filepath.Join(checkout, "other")
			if err := os.WriteFile(target, []byte("STOLEN=1\n"), 0o644); err != nil {
				return err
			}
			return os.Link(target, filepath.Join(checkout, ".env.example"))
		},
	}
	for name, plant := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, runner, _, _, _ := deploytest.NewConfig(t)
			runner.On["git clone"] = func(args []string) error {
				checkout := args[len(args)-1]
				if err := os.MkdirAll(filepath.Join(checkout, ".git"), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(filepath.Join(checkout, "deploy.json"), []byte(manifestJSON), 0o644); err != nil {
					return err
				}
				return plant(checkout)
			}

			if _, err := deploy.New(cfg).Install(context.Background(), "https://example.com/myapp.git"); err == nil {
				t.Fatal("a hostile .env.example was accepted")
			}
			env, err := os.ReadFile(filepath.Join(cfg.OptDir, "myapp", ".env"))
			if err == nil && strings.Contains(string(env), "STOLEN") {
				t.Errorf(".env carries the file the link pointed at: %q", env)
			}
		})
	}
}

// Under a setuid-root binary the real uid is the invoking user's, so reading it
// would hand /opt/<name> and its secrets to whoever ran the command.
func TestInstallHandsTheRootOwnedPartsToTheConfiguredIds(t *testing.T) {
	cfg, runner, accounts, _, _ := deploytest.NewConfig(t)
	clones(t, runner, map[string]string{".env.example": "TOKEN=\n"})
	altGID, hasAlt := deploytest.AltGID(t)
	if !hasAlt {
		t.Skip("no second group this process may chown to")
	}
	// Root's side is the alternate group here, the account's is this process's
	// own, so an install that read the process's ids gets it backwards.
	cfg.OwnerGID = int(altGID)
	accounts.GID = uint32(os.Getgid())

	if _, err := deploy.New(cfg).Install(context.Background(), "https://example.com/myapp.git"); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(cfg.OptDir, "myapp")
	for _, path := range []string{root, filepath.Join(root, ".env")} {
		if stat := lstat(t, path); stat.Gid != altGID {
			t.Errorf("%s has gid %d, want the configured %d", path, stat.Gid, altGID)
		}
	}
}

// The unit sets ProtectHome=true, so a binary in a home it hides can never be
// exec'd, and systemd reports that as a bare 203/EXEC at the next restart.
func TestInstallRefusesASelfPathTheUnitCannotReach(t *testing.T) {
	cfg, runner, accounts, units, _ := deploytest.NewConfig(t)
	clones(t, runner, map[string]string{".env.example": "TOKEN=\n"})
	// The normal development case: depl run straight out of a checkout.
	cfg.SelfPath = "/home/dev/S99Deploy/bin/depl"

	_, err := deploy.New(cfg).Install(context.Background(), "https://example.com/myapp.git")

	if err == nil {
		t.Fatal("install wrote a unit whose ExecStart the unit's own ProtectHome=true hides")
	}
	if !strings.Contains(err.Error(), cfg.SelfPath) {
		t.Errorf("the error does not name the path: %v", err)
	}
	// Checked before the first side effect, so the refusal leaves no clone, no
	// account and no unit behind.
	if runner.Ran("git", "clone") {
		t.Error("the clone ran before the check")
	}
	if len(accounts.Created) != 0 {
		t.Errorf("accounts created: %v", accounts.Created)
	}
	if len(units.Enabled) != 0 {
		t.Errorf("units enabled: %v", units.Enabled)
	}
	if _, err := os.Stat(filepath.Join(cfg.UnitDir, "myapp.service")); !os.IsNotExist(err) {
		t.Errorf("a unit was written: %v", err)
	}
}
