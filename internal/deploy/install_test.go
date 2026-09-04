// internal/deploy/install_test.go

package deploy_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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
		"User=myapp", "ExecStart=/usr/local/bin/s99deploy run " + root,
		"ReadWritePaths=" + root, "EnvironmentFile=" + root + "/.env",
	} {
		if !strings.Contains(string(unit), want) {
			t.Errorf("unit is missing %q:\n%s", want, unit)
		}
	}

	if accounts.Created == nil || accounts.Created[0] != "myapp" {
		t.Errorf("Created = %v, want [myapp]", accounts.Created)
	}
	if units.Reloads != 1 || len(units.Enabled) != 1 {
		t.Errorf("reloads = %d, enabled = %v, want 1 and [myapp]", units.Reloads, units.Enabled)
	}
	if !runner.Ran("git", "clone") {
		t.Error("nothing cloned")
	}
}

// Install is safe to rerun: every step skips what already exists.
func TestInstallIsRerunnable(t *testing.T) {
	cfg, runner, _, units, _ := deploytest.NewConfig(t)
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
		if strings.HasPrefix(entry.Name(), ".s99deploy-install-") {
			t.Errorf("a temp checkout survived: %s", entry.Name())
		}
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
		t.Error("a unit was written for an account s99deploy refused to adopt")
	}
}
