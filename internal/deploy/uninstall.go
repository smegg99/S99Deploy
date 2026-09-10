// internal/deploy/uninstall.go

package deploy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/smegg99/s99logger"
)

// Uninstall disables and removes the unit.
func (d *Deployer) Uninstall(ctx context.Context, name string, purge bool) error {
	if err := d.requireRoot(); err != nil {
		return err
	}
	// filepath.Base("/") is "/", so the Base check alone lets that one through,
	// and Root("/") is OptDir itself: a --purge would take every app with it.
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, filepath.Separator) ||
		filepath.Base(name) != name || strings.HasPrefix(name, "-") {
		return fmt.Errorf("invalid service name %q", name)
	}
	unitPath := d.unitPath(name)
	if _, err := os.Stat(unitPath); err != nil {
		return fmt.Errorf("no unit at %s: is %s installed?", unitPath, name)
	}

	// The checkout, .env and account survive unless purge is set.
	root := d.Root(name)
	var account Account
	var deletable bool
	if purge {
		var err error
		if account, deletable, err = d.purgeTarget(name, root); err != nil {
			return err
		}
	}

	if err := d.cfg.Units.Disable(ctx, name); err != nil {
		return err
	}
	// A disabled unit whose file is still on disk is a worse half-state than
	// finishing, so the rest of the removal ignores a late cancel.
	ctx = context.WithoutCancel(ctx)

	if err := os.Remove(unitPath); err != nil {
		return err
	}
	if err := d.cfg.Units.Reload(ctx); err != nil {
		return err
	}
	if !purge {
		d.cfg.Log.Info(s99logger.NewEvent(EventUninstalled, s99logger.String("kept", root)))
		return nil
	}

	// Removed by the path this command validated, not by whatever passwd says.
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("remove %s: %w", root, err)
	}
	if deletable {
		if err := d.cfg.Accounts.Delete(ctx, account.Name); err != nil {
			return err
		}
	}
	d.cfg.Log.Info(s99logger.NewEvent(EventPurged,
		s99logger.String("app", name), s99logger.String("removed", root)))
	return nil
}

// purgeTarget resolves what --purge would delete, and refuses a foreign home.
func (d *Deployer) purgeTarget(name, root string) (Account, bool, error) {
	// The bool reports whether an account is still there to delete.
	account, err := d.cfg.Accounts.Lookup(name)
	switch {
	case errors.Is(err, ErrNoAccount):
		// Somebody already ran userdel: nothing to delete, tree still goes.
		if _, err := os.Lstat(root); err != nil {
			return Account{}, false, fmt.Errorf("purge %s: %s: %w", name, root, err)
		}
		return Account{}, false, nil
	case err != nil:
		return Account{}, false, fmt.Errorf("purge %s: resolve account: %w", name, err)
	}

	if home := filepath.Clean(account.Home); home != root {
		return Account{}, false, fmt.Errorf(
			"purge %s: account %s has home %s, not %s; remove it by hand or rerun without --purge",
			name, account.Name, home, root)
	}
	if _, err := os.Lstat(root); err != nil {
		return Account{}, false, fmt.Errorf("purge %s: %s: %w", name, root, err)
	}
	return account, true, nil
}
