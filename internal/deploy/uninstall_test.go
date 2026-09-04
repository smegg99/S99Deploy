// internal/deploy/uninstall_test.go

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

// installed lays out one app the way a finished install leaves it.
func installed(t *testing.T, cfg deploy.Config, accounts *deploytest.Accounts, name, home string) string {
	t.Helper()
	root := filepath.Join(cfg.OptDir, name)
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.UnitDir, name+".service"), []byte("[Unit]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	accounts.Users[name] = deploy.Account{
		Name: name, Home: home,
		UID: uint32(os.Getuid()), GID: uint32(os.Getgid()),
	}
	return root
}

func TestPurgeRemovesTheTreeAndTheAccount(t *testing.T) {
	cfg, _, accounts, units, _ := deploytest.NewConfig(t)
	root := installed(t, cfg, accounts, "myapp", filepath.Join(cfg.OptDir, "myapp"))

	if err := deploy.New(cfg).Uninstall(context.Background(), "myapp", true); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Errorf("%s survived a purge: %v", root, err)
	}
	if _, err := os.Stat(filepath.Join(cfg.UnitDir, "myapp.service")); !os.IsNotExist(err) {
		t.Error("the unit file survived")
	}
	if len(accounts.Deleted) != 1 || accounts.Deleted[0] != "myapp" {
		t.Errorf("Deleted = %v, want [myapp]", accounts.Deleted)
	}
	if len(units.Disabled) != 1 || units.Reloads != 1 {
		t.Errorf("disabled = %v, reloads = %d, want one of each", units.Disabled, units.Reloads)
	}
}

// A home somewhere else refuses before the command has changed anything at all.
func TestPurgeRefusesAMismatchedHomeAndChangesNothing(t *testing.T) {
	cfg, _, accounts, units, _ := deploytest.NewConfig(t)
	root := installed(t, cfg, accounts, "myapp", "/home/someone-else")

	err := deploy.New(cfg).Uninstall(context.Background(), "myapp", true)
	if err == nil {
		t.Fatal("a mismatched home was purged")
	}
	for _, want := range []string{"/home/someone-else", root, "without --purge"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not name %q: %v", want, err)
		}
	}

	if _, err := os.Stat(filepath.Join(cfg.UnitDir, "myapp.service")); err != nil {
		t.Error("the unit was removed despite the refusal")
	}
	if _, err := os.Stat(root); err != nil {
		t.Error("the tree was removed despite the refusal")
	}
	if len(accounts.Deleted) != 0 || len(units.Disabled) != 0 {
		t.Errorf("Deleted = %v, disabled = %v, want neither", accounts.Deleted, units.Disabled)
	}
}

// Somebody already ran userdel.
func TestPurgeFinishesWhenTheAccountIsAlreadyGone(t *testing.T) {
	// There is nothing to delete and the tree still has to go, so the command
	// finishes instead of aborting on "resolve account".
	cfg, _, accounts, _, _ := deploytest.NewConfig(t)
	root := installed(t, cfg, accounts, "myapp", filepath.Join(cfg.OptDir, "myapp"))
	delete(accounts.Users, "myapp")

	if err := deploy.New(cfg).Uninstall(context.Background(), "myapp", true); err != nil {
		t.Fatalf("purge of an accountless app failed: %v", err)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Errorf("%s survived: %v", root, err)
	}
	if len(accounts.Deleted) != 0 {
		t.Errorf("Deleted = %v, want nothing deleted", accounts.Deleted)
	}
}

func TestUninstallWithoutPurgeKeepsTheTree(t *testing.T) {
	cfg, _, accounts, _, _ := deploytest.NewConfig(t)
	root := installed(t, cfg, accounts, "myapp", filepath.Join(cfg.OptDir, "myapp"))

	if err := deploy.New(cfg).Uninstall(context.Background(), "myapp", false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "app")); err != nil {
		t.Errorf("the checkout was removed without --purge: %v", err)
	}
	if len(accounts.Deleted) != 0 {
		t.Errorf("Deleted = %v, want nothing", accounts.Deleted)
	}
}

func TestUninstallRefusesWithoutAUnit(t *testing.T) {
	cfg, _, _, _, _ := deploytest.NewConfig(t)

	err := deploy.New(cfg).Uninstall(context.Background(), "ghost", false)
	if err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("err = %v, want it to name the app", err)
	}
}

func TestUninstallRefusesANameOutsideOpt(t *testing.T) {
	for _, purge := range []bool{false, true} {
		cfg, _, accounts, units, _ := deploytest.NewConfig(t)
		root := installed(t, cfg, accounts, "../victim", filepath.Join(cfg.OptDir, "..", "victim"))
		err := deploy.New(cfg).Uninstall(context.Background(), "../victim", purge)
		if err == nil {
			t.Fatal("uninstall accepted a name outside OptDir")
		}
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("outside tree changed: %v", err)
		}
		if len(units.Disabled) != 0 || len(accounts.Deleted) != 0 {
			t.Fatal("refusal changed the host")
		}
	}
}
