// internal/deploy/uninstall.go

package deploy

import (
	"context"
	"fmt"
	"os"

	"github.com/smegg99/s99logger"
)

// Uninstall disables and removes the unit.
func (d *Deployer) Uninstall(ctx context.Context, name string, purge bool) error {
	if err := d.requireRoot(); err != nil {
		return err
	}
	unitPath := d.unitPath(name)
	if _, err := os.Stat(unitPath); err != nil {
		return fmt.Errorf("no unit at %s: is %s installed?", unitPath, name)
	}

	if err := d.cfg.Units.Disable(ctx, name); err != nil {
		return err
	}
	if err := os.Remove(unitPath); err != nil {
		return err
	}
	if err := d.cfg.Units.Reload(ctx); err != nil {
		return err
	}

	// The checkout, .env and account survive unless purge is set.
	root := d.root(name)
	if !purge {
		d.cfg.Log.Info(s99logger.NewEvent(EventUninstalled, s99logger.String("kept", root)))
		return nil
	}
	if err := os.RemoveAll(root); err != nil {
		return fmt.Errorf("remove %s: %w", root, err)
	}
	if err := d.cfg.Accounts.Delete(ctx, name); err != nil {
		return err
	}
	d.cfg.Log.Info(s99logger.NewEvent(EventPurged,
		s99logger.String("app", name), s99logger.String("removed", root)))
	return nil
}
